#!/usr/bin/env python3
"""Per-gold-item credit table, both arms, across runs.

    python3 credit_table.py <repo> [--json] [--fingerprint]

This is the one mechanical input Loop 3's struggle read stands on
(`docs/loops/03-repo-diagnosis.md`): for every gold item, did each arm CITE it,
in how many of its runs. `pergroup.py` answers the same question one level up,
per group, which is the right shape for a verdict and the wrong shape for
"which rows did the baseline miss, and what did it do instead".

Three columns matter and they mean different things:

  both        found by both arms. A DILUTER: it cannot discriminate, whatever
              else is true of it. Several diluters in one file is a gold bug.
  sense-only  the live discriminator - the reach that is already working.
  neither     nobody got it. Scenario material if the ask pointed at it, a gold
              bug if it did not.

`--fingerprint` prints a stable hash of the sense-only + neither sets. That is
the movement detector the six-cycle swap rule needs: a re-authored scenario that
moves no row has not moved the cell, however different its prose. It deliberately
ignores the `both` set, because gold churning in and out of the diluter bucket is
not progress.

Reads `<RESULTS_DIR>/{baseline,sense}/<repo>/run-*/scored.json`, the same layout
and the same RESULTS_DIR contract as pergroup.py. Validation runs live one level
deeper (`.../validation/`) and are invisible here by construction.
"""
import glob
import hashlib
import json
import os
import sys

ARMS = ("baseline", "sense")


def resolve_root(repo):
    """RESULTS_DIR wins; otherwise find the single bench root holding this repo."""
    if os.environ.get("RESULTS_DIR"):
        return os.environ["RESULTS_DIR"]
    default = os.path.normpath(os.path.join(os.path.dirname(__file__), "..", "results"))
    cands = []
    if os.path.isdir(os.path.join(default, "baseline", repo)):
        cands.append(default)
    verticals = os.path.join(os.path.dirname(default), "verticals")
    for cand in sorted(glob.glob(os.path.join(verticals, "*", "results", "*"))):
        if os.path.isdir(os.path.join(cand, "baseline", repo)):
            cands.append(cand)
    if len(cands) > 1:
        rel = "\n  ".join(os.path.relpath(c) for c in cands)
        sys.exit(f"{repo} is in several bench roots - set RESULTS_DIR to one of:\n  {rel}")
    return cands[0] if cands else default


def load_runs(root, arm, repo):
    """Every run's gold_recall details for one arm. Bare scored.json is the
    single-run legacy layout; run-*/ is current."""
    cell = os.path.join(root, arm, repo)
    paths = sorted(glob.glob(os.path.join(cell, "run-*", "scored.json")))
    if not paths and os.path.exists(os.path.join(cell, "scored.json")):
        paths = [os.path.join(cell, "scored.json")]
    out = []
    for p in paths:
        try:
            with open(p, encoding="utf-8") as fh:
                details = (json.load(fh).get("gold_recall") or {}).get("details")
        except (OSError, ValueError):
            continue
        if details:
            out.append(details)
    return out


def build(root, repo):
    """{gold_id: {group, baseline: (cited, runs), sense: (cited, runs)}}."""
    table, runs = {}, {}
    for arm in ARMS:
        arm_runs = load_runs(root, arm, repo)
        runs[arm] = len(arm_runs)
        for details in arm_runs:
            for d in details:
                row = table.setdefault(d["id"], {"group": d.get("group", "?"),
                                                 "baseline": 0, "sense": 0})
                row[arm] += 1 if d.get("cited") else 0
    return table, runs


def classify(row, runs):
    """An arm 'has' a row when it cited it in at least one run. A row cited in
    some runs and not others is still a hit here - the struggle read wants the
    reachability question; pergroup.py owns the averaging."""
    b, s = row["baseline"] > 0, row["sense"] > 0
    if b and s:
        return "both"
    if s:
        return "sense-only"
    if b:
        return "baseline-only"
    return "neither"


def fingerprint(table, runs):
    """Stable hash of the rows that can still move the number."""
    live = sorted(gid for gid, row in table.items()
                  if classify(row, runs) in ("sense-only", "neither"))
    return hashlib.sha256("\n".join(live).encode()).hexdigest()[:12]


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    if not args:
        sys.exit("usage: credit_table.py <repo> [--json] [--fingerprint]")
    repo = args[0]
    root = resolve_root(repo)
    table, runs = build(root, repo)

    if not table:
        if "--fingerprint" in sys.argv:
            print("no-runs")
            return 0
        print(f"credit_table: no scored runs for {repo} under {root}")
        return 1

    buckets = {}
    for gid, row in table.items():
        buckets.setdefault(classify(row, runs), []).append(gid)

    if "--fingerprint" in sys.argv:
        print(fingerprint(table, runs))
        return 0

    if "--json" in sys.argv:
        print(json.dumps({"repo": repo, "runs": runs,
                          "fingerprint": fingerprint(table, runs),
                          "buckets": {k: sorted(v) for k, v in buckets.items()},
                          "rows": table}, indent=1, sort_keys=True))
        return 0

    print(f"### Credit table - {repo}  "
          f"(baseline x{runs['baseline']}, sense x{runs['sense']})")
    print(f"{'gold id':38} {'group':14} base sense  verdict")
    for gid in sorted(table):
        row = table[gid]
        print(f"{gid[:38]:38} {row['group'][:14]:14} "
              f"{row['baseline']:>4} {row['sense']:>5}  {classify(row, runs)}")
    print()
    for name in ("sense-only", "neither", "baseline-only", "both"):
        ids = sorted(buckets.get(name, []))
        print(f"{name:14} {len(ids):>3}  {', '.join(ids) or '-'}")
    print(f"\nfingerprint {fingerprint(table, runs)}  "
          f"(sense-only + neither; unchanged across cycles = no movement)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
