#!/usr/bin/env python3
"""Rank a clone's candidate ANCHORS for the admission gate.

    anchor_rank.py <clone_dir> [--top 12] [--min-deps 12] [--kinds class,interface,trait]

Loop 2 gates CONTRACTS, not repos ("verdicts are per-contract" - the try-harder
law), so something has to decide which contracts to gate. Until 2026-07-29 that
was an agent writing SQL by hand each session, which meant a re-run of Loop 2
could not reproduce its own slate. This is that step, fixed in one place.

Ranking = distinct non-test, non-vendor files with an edge into the symbol.

EXCLUDED BY NAME: the hub-explosion set (manifesto §6 - "avoid X → User →
everything") and framework base classes, which blast to thousands and curate to
nothing.

ORDER MATTERS, and it is not the ranking. The php-laravel sweep measured 52
anchors: every Eloquent-model anchor died on K7 (one namespace-prefix grep
covers the dependents) and every survivor was an interface, a contract or a
trait, where dependents satisfy by `implements`/`use` inside the class body and
no import prefix enumerates them. So contracts are emitted BEFORE classes at
equal rank - the gate still decides, this only decides what it sees first.

THE RANKING DOES NOT FIND WINNERS, AND THAT IS MEASURED, NOT ASSUMED. Backtested
2026-07-30 against the four banked go wins, ranking pebble's `Batch` - the
STRONGEST win in the corpus, +1.000 off a 0.00 baseline twice - by every
structural signal available:

    signal                          where `Batch` lands
    direct inbound composes         50 of 901
    + holders of interfaces it      110 of 1211
      satisfies (1-hop SQL proxy)
    the real retention ring         56 of 86   (ring 14; `DiskWriteCategory`
      (one blast per candidate)                 tops it at 74 and has never
                                                featured in a bench)

Three signals, three failures, same repo. The other three wins (dolt `DoltDB`,
nomad `Server`, consul `Server`) rank 2-4 by inbound composes, so a ranker tuned
on them alone looks excellent and drops the biggest win on the floor.

WHY: the winning anchor followed from the ENGINEERING TASK, not from structural
prominence. pebble's cell asks what holds a `Batch` when you pool and reuse it -
a rework a maintainer would credibly do. No amount of graph shape predicts which
object a plausible refactor touches. So this file ranks for COVERAGE (what the
gate should see, contracts first) and never claims to rank for value; picking the
anchor is a judgment step owned by whoever writes the scenario.

Do not "improve" the ranking without backtesting it here first. Metric proposals
went 0 for 5 in one day (coverage bar, symbol volume, direct holders, interface
proxy, ring size) while every check that RAN something was right.
"""

import argparse
import json
import os
import sqlite3
import sys

HUB_EXPLOSION = {"User", "Controller", "Model", "Request", "Response", "Exception",
                 "Str", "Arr", "Helper", "Config", "Collection", "Command",
                 "ServiceProvider", "Middleware", "Kernel", "Application"}

SQL = """
SELECT s.name, s.kind, f.path, COUNT(DISTINCT ef.path) AS dep_files
FROM sense_edges e
JOIN sense_symbols s   ON s.id  = e.target_id
JOIN sense_files f     ON f.id  = s.file_id
JOIN sense_symbols src ON src.id = e.source_id
JOIN sense_files ef    ON ef.id = src.file_id
WHERE s.kind IN ({kinds})
  AND f.path  NOT LIKE '%vendor/%'  AND f.path  NOT LIKE '%node_modules/%'
  AND f.path  NOT LIKE '%test%'     AND f.path  NOT LIKE '%spec%'
  AND ef.path NOT LIKE '%vendor/%'  AND ef.path NOT LIKE '%node_modules/%'
  AND ef.path NOT LIKE '%test%'     AND ef.path NOT LIKE '%spec%'
GROUP BY s.id
HAVING dep_files >= ?
ORDER BY dep_files DESC
LIMIT 400
"""


def rank(clone, kinds, min_deps, top):
    db = os.path.join(clone, ".sense", "index.db")
    if not os.path.exists(db):
        return []
    con = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
    q = SQL.format(kinds=",".join("?" * len(kinds)))
    rows = con.execute(q, (*kinds, min_deps)).fetchall()
    con.close()
    out = []
    for name, kind, path, deps in rows:
        if name in HUB_EXPLOSION:
            continue
        out.append({"symbol": name, "kind": kind, "file": path, "dep_files": deps,
                    "contract": kind in ("interface", "trait")})
    # contracts first at equal rank - the shape that survived the php-laravel sweep
    out.sort(key=lambda r: (not r["contract"], -r["dep_files"]))
    return out[:top]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("clone")
    ap.add_argument("--top", type=int, default=12)
    ap.add_argument("--min-deps", type=int, default=12)
    ap.add_argument("--kinds", default="class,interface,trait")
    ap.add_argument("--tsv", action="store_true", help="symbol<TAB>file, for shell loops")
    args = ap.parse_args()

    rows = rank(args.clone, args.kinds.split(","), args.min_deps, args.top)
    if args.tsv:
        for r in rows:
            print(f"{r['symbol']}\t{r['file']}")
    else:
        print(json.dumps(rows, indent=1))
    return 0


if __name__ == "__main__":
    sys.exit(main())
