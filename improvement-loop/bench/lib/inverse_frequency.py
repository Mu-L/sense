#!/usr/bin/env python3
"""inverse_frequency.py - rank gold rows by INVERSE CITATION FREQUENCY.

The rarely-cited rows sort to the top; those are the ones worth building gold from.

    inverse_frequency.py <repo> <results-root> [<results-root> ...] [--min-runs 2]

WHY THIS EXISTS. Each authoring cycle reads ONE credit table and rewrites the question
against it, so difficulty is never accumulated: a row that eleven runs have never cited
looks exactly like a row that one run happened to miss. This counts citations per row
across every run on disk and ranks by the rate, so the next scenario can be built from
the bottom of the list instead of from the last failure.

WHY COUNTING AND NOT JUDGMENT. Reading the repo and deciding what "should" be hard runs
the same search the agents ran, with the same tools, and finds the same things - the
result would be gold selected for what one more grep can reach. Citation RATE needs no
ground truth: it is the arms' own behaviour, aggregated. What it cannot see is a
dependency no agent has ever named; that residue stays invisible here and is not claimed.

READING THE OUTPUT.
  * `0/N` on BOTH arms - hard or unreachable, and those are different things. Check the
    row against the blast payload: present but uncited is a hard row worth building on;
    absent from the payload is gold the tool cannot serve and should be retired.
  * `0/N` baseline, `N/N` sense - the discriminator you already have. More of these is
    the goal.
  * `N/N` on both - free. It costs nothing to answer and dilutes the group.
"""
import argparse
import collections
import glob
import json
import os
import sys


def runs_for(repo, roots):
    """Every scored run for this repo under the given roots, as (arm, path)."""
    out = []
    for root in roots:
        for meta in glob.glob(os.path.join(root, "**", "run_meta.json"), recursive=True):
            d = os.path.dirname(meta)
            if os.path.basename(os.path.dirname(os.path.dirname(d))) != repo and repo not in d.split(os.sep):
                continue
            scored = os.path.join(d, "scored.json")
            if not os.path.isfile(scored):
                continue
            try:
                arm = json.load(open(meta)).get("tool") or "?"
            except (OSError, ValueError):
                continue
            out.append((arm, scored))
    return out


def tally(runs):
    """{row_id: {"group":g, arm: [cited, total]}} across every run."""
    rows = {}
    for arm, scored in runs:
        try:
            gr = json.load(open(scored))["gold_recall"]
        except (OSError, ValueError, KeyError):
            continue
        for item in gr.get("details", []):
            rec = rows.setdefault(item["id"], {"group": item.get("group", "")})
            cited, total = rec.get(arm, [0, 0])
            rec[arm] = [cited + (1 if item.get("cited") else 0), total + 1]
    return rows


def rate(pair):
    cited, total = pair
    return cited / total if total else None


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("repo")
    ap.add_argument("roots", nargs="+")
    ap.add_argument("--min-runs", type=int, default=1,
                    help="skip rows seen in fewer runs than this (default 1)")
    args = ap.parse_args()

    runs = runs_for(args.repo, args.roots)
    if not runs:
        print(f"no scored runs for {args.repo} under {', '.join(args.roots)}")
        return 1
    by_arm = collections.Counter(a for a, _ in runs)
    print(f"# inverse-frequency ranking (rarest cited first) - {args.repo}  ({len(runs)} runs: "
          + ", ".join(f"{a} x{n}" for a, n in sorted(by_arm.items())) + ")\n")

    rows = tally(runs)
    arms = sorted(by_arm)
    ranked = []
    for rid, rec in rows.items():
        seen = sum(rec.get(a, [0, 0])[1] for a in arms)
        if seen < args.min_runs:
            continue
        cited = sum(rec.get(a, [0, 0])[0] for a in arms)
        ranked.append((cited / seen if seen else 1.0, rid, rec, cited, seen))
    ranked.sort(key=lambda r: (r[0], r[1]))

    head = f"{'row':24s} {'group':11s} {'overall':>9s}  " + "  ".join(f"{a:>12s}" for a in arms)
    print(head)
    print("-" * len(head))
    for overall, rid, rec, cited, seen in ranked:
        cells = []
        for a in arms:
            c, t = rec.get(a, [0, 0])
            cells.append(f"{(str(c) + '/' + str(t)):>12s}" if t else f"{'-':>12s}")
        print(f"{rid:24s} {rec['group']:11s} {cited:>4}/{seen:<4} {' '.join(cells)}")

    dead = [r for r in ranked if r[3] == 0]
    free = [r for r in ranked if r[0] == 1.0]
    print(f"\n  never cited by any arm : {len(dead)}"
          + (f"  ({', '.join(r[1] for r in dead)})" if dead else ""))
    print(f"  cited by every run     : {len(free)}  <- free rows, they dilute the group")
    print("\n  A never-cited row is HARD or UNREACHABLE. Check it against the blast payload"
          "\n  before building on it: absent from the payload means the tool cannot serve it.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
