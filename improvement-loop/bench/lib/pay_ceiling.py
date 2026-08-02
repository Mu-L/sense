#!/usr/bin/env python3
"""pay_ceiling.py - can this cell clear the win floor AT ALL? Arithmetic, before money.

`pergroup.py` declares WIN when ANY gold group's delta reaches the floor. Recall is
capped at 1.00, so for a group whose baseline already sits at B the largest delta that
group can ever produce is `1.00 - B`. If no group's ceiling reaches the floor, the paid
pair cannot return WIN however well the sense arm does - the cell is dead before it runs.

This is the arithmetic killer, run against the unscored validation pair, where it costs
nothing. It is deliberately mechanical: the judgment phase that reads this run was
answering "does it discriminate?" (which it did, +0.31) and had no reason to ask "can
it clear +0.50?" (which it cannot, ceiling +0.375). A floor is not a judgment call, so
it does not belong to an agent.

  RESULTS_DIR=<validation root> python3 pay_ceiling.py <repo> [floor]

Exit 0 = some group can still reach the floor. Exit 1 = no group can; do not pay.
Mirrors pergroup's own math (summed cited / summed total per group) so the two cannot
disagree about what the baseline scored.
"""
import glob
import json
import os
import sys

DEFAULT_FLOOR = 0.50


def _runs(root, arm, repo):
    base = os.path.join(root, arm, repo)
    paths = sorted(glob.glob(os.path.join(base, "run-*", "scored.json")))
    if not paths and os.path.exists(os.path.join(base, "scored.json")):
        paths = [os.path.join(base, "scored.json")]
    return paths


def collect(root, arm, repo):
    """group -> (cited, total) summed across surviving runs, mirroring pergroup."""
    by_group = {}
    for path in _runs(root, arm, repo):
        with open(path) as fh:
            data = json.load(fh)
        if data.get("failed"):
            continue
        for name, g in (data.get("gold_recall", {}).get("groups", {}) or {}).items():
            cited, total = by_group.get(name, (0, 0))
            by_group[name] = (cited + g.get("cited", 0), total + g.get("total", 0))
    return by_group


def ceilings(root, repo, floor=DEFAULT_FLOOR):
    """Return (rows, best_ceiling, reachable). rows are per-group tuples."""
    base = collect(root, "baseline", repo)
    sense = collect(root, "sense", repo)
    rows, best = [], None
    for name in sorted(set(base) | set(sense)):
        bc, bt = base.get(name, (0, 0))
        sc, st = sense.get(name, (0, 0))
        if not bt:
            continue
        bmean = bc / bt
        smean = sc / st if st else 0.0
        ceiling = 1.0 - bmean
        rows.append((name, bmean, smean, smean - bmean, ceiling))
        best = ceiling if best is None else max(best, ceiling)
    if best is None:
        return rows, None, False
    return rows, best, best >= floor


def main(argv):
    if not argv:
        print("usage: pay_ceiling.py <repo> [floor]  (RESULTS_DIR pins the run root)")
        return 64
    repo = argv[0]
    floor = float(argv[1]) if len(argv) > 1 else DEFAULT_FLOOR
    root = os.environ.get("RESULTS_DIR")
    if not root:
        print("pay_ceiling: RESULTS_DIR must point at the run root", file=sys.stderr)
        return 64

    rows, best, reachable = ceilings(root, repo, floor)
    if best is None:
        print("pay_ceiling: no scored baseline runs for %s under %s" % (repo, root),
              file=sys.stderr)
        return 64

    print("### %s - can any group still reach +%.0f%%?\n" % (repo, floor * 100))
    print("%-16s %8s %8s %8s %9s" % ("group", "baseline", "sense", "delta", "ceiling"))
    for name, bmean, smean, delta, ceiling in rows:
        mark = "" if ceiling >= floor else "   <- capped below the floor"
        print("%-16s %8.3f %8.3f %+8.3f %9.3f%s" % (name, bmean, smean, delta, ceiling, mark))

    if reachable:
        print("\nPAY_CEILING: OK - best reachable delta is %+.3f, floor is +%.2f" % (best, floor))
        return 0
    print("\nPAY_CEILING: DEAD - the best any group can reach is %+.3f, floor is +%.2f."
          % (best, floor))
    print("The baseline already holds too much of the gold for a WIN to exist here.")
    print("Re-gold from what the baseline MISSED, or re-shape; do not buy this pair.")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
