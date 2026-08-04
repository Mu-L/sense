#!/usr/bin/env python3
"""omission_lens.py - the blended cell score, beside today's headline and the plain
complete/incomplete statement.

    omission_lens.py <repo> <results-root>...

THE SCORE. Per run:

    blended = 0.4 * completion + 0.6 * omission

`omission` is the gold rate the scorer already writes (`gold_recall.cited_recall`): a gold
list of N rows prices each row at 1/N, so finding 14 of 16 is 0.875. `completion` is the
scorer's existing `completeness` field, which measures how much of what each STEP of the
question asked for actually arrived. Both terms are already computed on every run; nothing
new is measured here, they were simply never read together.

WHY THIS IS WORTH HAVING. The omission term alone is today's headline to the digit, so it
moves nothing by itself. The gain is entirely the completion term, which has been written to
every scored.json since June and never looked at: on the mastodon opus-5 cell it separates
the arms +0.289 where the gold rate manages +0.087. The blend is a small, honest improvement
in signal, and it costs no runs.

THE WEIGHTS ARE A CHOICE, NOT A DERIVATION. 0.4/0.6 was picked because omissions should
weigh more than step coverage, not because any measurement produced those numbers. They are
therefore hardcoded and NOT exposed as a flag: a metric whose parameters can be tuned after
the result is known is a dial, not a measurement. If they ever change it is a recorded
ruling, not a command-line argument.

WHAT ELSE IS PRINTED, AND WHY. Two things stand beside the blend, both parameter-free:

  - the per-group gold rate, which is what the loop's verdict has always keyed on
    (pergroup.py). The blend is a cell-level number and does not replace it.
  - complete/incomplete per run, the ask taken literally. Not a score - a plain sentence
    ("sense produced a complete audit 4 times in 5, the baseline never") that a rate cannot
    say. It is reported, never used as a verdict; at the run counts on disk it is far too
    coarse to decide anything.

WHAT IT DOES NOT DO. It writes nothing and changes no artifact. Making the blend the
headline is a STOPPER change and belongs to a human holding this diff.
"""
import argparse
import glob
import json
import os
import sys

COMPLETION_WEIGHT = 0.4
OMISSION_WEIGHT = 0.6


def load_runs(root, arm, repo):
    """Every scored run for one arm of one cell, failures dropped.

    A failed run (empty final answer, truncated stream, provider cap) is not a real 0.0 -
    blending it as one manufactures a loss. Same rule pergroup.py applies.
    """
    base = os.path.join(root, arm, repo)
    paths = sorted(glob.glob(os.path.join(base, "run-*", "scored.json")))
    if not paths and os.path.exists(os.path.join(base, "scored.json")):
        paths = [os.path.join(base, "scored.json")]
    out = []
    for path in paths:
        with open(path) as fh:
            scored = json.load(fh)
        if not scored.get("failed"):
            out.append(scored)
    return out


def omission(scored):
    """The gold rate the scorer already writes: each of N rows priced at 1/N."""
    return scored.get("gold_recall", {}).get("cited_recall", 0.0)


def completion(scored):
    """The scorer's existing per-step field - how much of what was asked for arrived."""
    return scored.get("completeness", 0.0)


def blended(scored):
    return COMPLETION_WEIGHT * completion(scored) + OMISSION_WEIGHT * omission(scored)


def group_rates(scored):
    """{group: (cited, total)} for one run. A group with no rows is not evidence, so it is
    dropped rather than scored as a free 1.0."""
    groups = scored.get("gold_recall", {}).get("groups", {}) or {}
    return {
        name: (g.get("cited", 0), g.get("total", 0))
        for name, g in groups.items()
        if g.get("total", 0) > 0
    }


def is_complete(scored):
    """Every row of every scored group cited. The ask taken literally."""
    rates = group_rates(scored)
    return bool(rates) and all(c >= t for c, t in rates.values())


def mean(values):
    return sum(values) / len(values) if values else 0.0


def arm_scores(runs):
    return {
        "n": len(runs),
        "completion": mean([completion(r) for r in runs]),
        "omission": mean([omission(r) for r in runs]),
        "blended": mean([blended(r) for r in runs]),
        "complete": [1 if is_complete(r) else 0 for r in runs],
    }


def group_table(runs):
    """{group: [(cited, total), ...]} across an arm's runs, in run order."""
    table = {}
    for scored in runs:
        for name, pair in group_rates(scored).items():
            table.setdefault(name, []).append(pair)
    return table


def group_rows(base_runs, sense_runs):
    """Per-group gold rate for both arms - the surface the loop's verdict keys on."""
    base, sense = group_table(base_runs), group_table(sense_runs)
    rows = []
    for name in sorted(set(base) | set(sense)):
        b, s = base.get(name, []), sense.get(name, [])
        if not b or not s:
            continue
        bm, sm = mean([c / t for c, t in b]), mean([c / t for c, t in s])
        rows.append({
            "group": name,
            "total": b[0][1],
            "base": bm,
            "sense": sm,
            "delta": sm - bm,
            "dead": all(c == 0 for c, _ in b + s),
        })
    return rows


def print_cell(root, repo, base_runs, sense_runs):
    print(f"\n### {repo} - {os.path.relpath(root)}")
    b, s = arm_scores(base_runs), arm_scores(sense_runs)
    print(f"  n = {b['n']} baseline, {s['n']} sense")
    print(f"{'':14} {'baseline':>9} {'sense':>9} {'delta':>9}")
    for label, key in (("completion", "completion"), ("omission", "omission"),
                       ("BLENDED", "blended")):
        print(f"{label:14} {b[key]:9.3f} {s[key]:9.3f} {s[key] - b[key]:+9.3f}")
    print(f"  complete audits: baseline {sum(b['complete'])} of {b['n']}, "
          f"sense {sum(s['complete'])} of {s['n']}  {b['complete']}{s['complete']}")

    rows = group_rows(base_runs, sense_runs)
    if rows:
        print(f"\n  {'group':12} {'rows':>4} {'baseline':>9} {'sense':>9} {'delta':>9}")
        for row in rows:
            print(f"  {row['group']:12} {row['total']:>4} {row['base']:9.3f} "
                  f"{row['sense']:9.3f} {row['delta']:+9.3f}")
        dead = [r["group"] for r in rows if r["dead"]]
        if dead:
            print(f"  never cited by either arm in any run: {', '.join(dead)} "
                  f"(discriminates nothing; check the gold rows)")


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("repo")
    ap.add_argument("roots", nargs="+", help="dirs holding baseline/ and sense/")
    args = ap.parse_args()

    print(f"# omission lens - {args.repo} (reporting only, nothing written)")
    print(f"# blended = {COMPLETION_WEIGHT} * completion + {OMISSION_WEIGHT} * omission")
    seen = 0
    for root in args.roots:
        base_runs = load_runs(root, "baseline", args.repo)
        sense_runs = load_runs(root, "sense", args.repo)
        if not base_runs or not sense_runs:
            print(f"\n### {args.repo} - {os.path.relpath(root)}: no paired scored runs "
                  f"(baseline={len(base_runs)} sense={len(sense_runs)}) - skipped")
            continue
        print_cell(root, args.repo, base_runs, sense_runs)
        seen += 1
    if not seen:
        print("\nno cell had both arms scored - nothing to compare")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
