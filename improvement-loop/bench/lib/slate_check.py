#!/usr/bin/env python3
"""The SLATE-level half of Loop 2: does the composed set obey §7.0, and is it
actually on disk the way it says it is?

    slate_check.py <vertical> [--clones DIR]

`admission_gate.py --slot` decides ONE candidate. This decides the SET, which is
the part the admission sign-off used to do by eye before it was retired (2026-07-29, Loop 2 is
autonomous). Everything here is a file on disk or a number from an index; prose
moves none of it.

Checks, all mechanical:

  1. EXACTLY 4 repos in repos.txt (manifesto §7.0 - "never write 1-2 big").
  2. Composition is `1 framework + 1 big + 2 medium` or `2 big + 2 medium`,
     never both a framework slot and a second big. Slots come from slate.json
     (written by the loop); sizes are measured from each index.
  3. No small repo in any slot (ruling 2026-07-20 removed the small slot).
  4. Every repo pinned in PINNED_COMMITS.json with a url + sha.
  5. BOTH arms cloned and sitting at the pinned sha - the fairness precondition.
  6. Every repo has a built index.

Exit 0 = the slate stands. Exit 1 = it does not, and the failures say why.
"""

import argparse
import json
import os
import subprocess
import sys

SIZE_MEDIUM_FLOOR = 1_000
SIZE_BIG_FLOOR = 4_000
VALID = ({"framework": 1, "big": 1, "medium": 2}, {"big": 2, "medium": 2})


def size_of(clone):
    db = os.path.join(clone, ".sense", "index.db")
    if not os.path.exists(db):
        return None, None
    import sqlite3
    con = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
    n = con.execute(
        "SELECT count(*) FROM sense_files WHERE path NOT LIKE '%vendor/%' "
        "AND path NOT LIKE '%node_modules/%' AND path NOT LIKE '%test%' "
        "AND path NOT LIKE '%spec%'").fetchone()[0]
    con.close()
    size = "big" if n >= SIZE_BIG_FLOOR else \
           "medium" if n >= SIZE_MEDIUM_FLOOR else "small"
    return n, size


def head(path):
    p = subprocess.run(["git", "-C", path, "rev-parse", "HEAD"],
                       capture_output=True, text=True)
    return p.stdout.strip() if p.returncode == 0 else None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("vertical")
    ap.add_argument("--clones", default=os.environ.get(
        "SENSE_CLONES", os.path.expanduser(
            "~/Developer/luuuc/oss/sense-benchmark/sense")))
    args = ap.parse_args()

    root = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                        "..", "..", "verticals", args.vertical)
    root = os.path.normpath(root)
    baseline_root = os.path.join(os.path.dirname(args.clones), "baseline")

    repos = [l.strip() for l in open(os.path.join(root, "repos.txt"))
             if l.strip() and not l.startswith("#")]
    pins = json.load(open(os.path.join(root, "PINNED_COMMITS.json")))
    slate_path = os.path.join(root, "slate.json")
    slots = json.load(open(slate_path)) if os.path.exists(slate_path) else {}

    fails, rows = [], []
    if len(repos) != 4:
        fails.append(f"repos.txt holds {len(repos)} repos, §7.0 says exactly 4")

    counts = {}
    for r in repos:
        clone = os.path.join(args.clones, r)
        n, size = size_of(clone)
        slot = (slots.get(r) or {}).get("slot")
        pin = pins.get(r) or {}
        sha = pin.get("sha")
        sense_head, base_head = head(clone), head(os.path.join(baseline_root, r))
        rows.append((r, slot, n, size, sha, sense_head, base_head))
        counts[slot] = counts.get(slot, 0) + 1

        if n is None:
            fails.append(f"{r}: no index built")
        elif size == "small":
            fails.append(f"{r}: {n} prod files = small; the small slot was removed "
                         "(ruling 2026-07-20)")
        if not slot:
            fails.append(f"{r}: no slot claimed (add it to slate.json)")
        if not (pin.get("url") and sha):
            fails.append(f"{r}: not pinned (url + sha) in PINNED_COMMITS.json")
        for arm, got in (("sense", sense_head), ("baseline", base_head)):
            if got is None:
                fails.append(f"{r}: {arm} arm not cloned")
            elif sha and got != sha:
                fails.append(f"{r}: {arm} arm at {got[:12]}, pinned {sha[:12]}")

    if counts and not any(counts == v for v in VALID):
        fails.append(f"composition {counts} is neither "
                     "'1 framework + 1 big + 2 medium' nor '2 big + 2 medium'")

    print(f"### Slate check - {args.vertical}")
    for r, slot, n, size, sha, sh, bh in rows:
        print(f"- `{r}` slot={slot or '-'} size={size or '-'} ({n} prod files) "
              f"pin={sha[:12] if sha else '-'} "
              f"arms={'ok' if sh and sh == bh == sha else 'MISMATCH'}")
    print(f"- composition: {counts}")
    if fails:
        print(f"- **SLATE FAILS ({len(fails)})**")
        for f in fails:
            print(f"    - {f}")
        return 1
    print("- **SLATE STANDS** - §7.0 composition, pins, both arms, indexes all check")
    return 0


if __name__ == "__main__":
    sys.exit(main())
