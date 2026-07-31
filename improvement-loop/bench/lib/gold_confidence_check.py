#!/usr/bin/env python3
"""gold_confidence_check.py -- does this gold survive the min_confidence an agent ACTUALLY uses?

    python3 gold_confidence_check.py <scenario.yaml> <symbol> [--file SUB] [--repo DIR]
                                     [--bin PATH] [--group NAME]

Exit 0 = every blast-reachable gold row survives BOTH thresholds (the gold is honest).
Exit 1 = at least one gold row is reachable at 0.3 and GONE at 0.7 -- a manufactured win.

## WHY

`sense blast` documents `--min-confidence` default 0.7, and agents in the wild pass the
documented default. Gold curated against a 0.3 sweep can therefore contain rows the benched
agent CANNOT retrieve at the default it actually uses. Those rows do not measure Sense's
reach; they measure the curator's threshold. A cell can then post a delta that no agent could
reproduce - which is the most expensive error in the program, because a fake win propagates
into the confirmation matrix, the harvest and a published article before anyone re-reads it.

This is the hand step the Loop 3 split priced and scripted first (01-repo-authoring.md).

## WHAT IT DOES NOT DO

It does not judge whether a row BELONGS in the gold: blast-radius gold != edit-impact gold,
and that is the hand-audit's job (the gitea case: 32 of 42 rows were
answering a different question, and no threshold check would have caught it). This script
answers one narrow question: of the rows blast can reach, do any of them vanish at 0.7?

Rows unreachable at BOTH thresholds are reported, never failed: gold legitimately sourced
from graph, search or a hand read is not blast-reachable and is not a defect.
"""
import json
import os
import subprocess
import sys

import yaml

from sense_build import default_bin

LOW, HIGH = "0.3", "0.7"   # the curation sweep, and the documented default agents pass


def blast_files(bin_path, symbol, min_confidence, file_sub=None, repo_dir=None):
    """Every file path `sense blast` returns at this threshold, as a set."""
    cmd = [bin_path, "blast", symbol, "--json", "--min-confidence", min_confidence]
    if file_sub:
        cmd += ["--file", file_sub]
    out = subprocess.run(cmd, capture_output=True, text=True, cwd=repo_dir or None)
    if out.returncode != 0:
        raise SystemExit(f"gold_confidence_check: blast failed at {min_confidence}: "
                         f"{(out.stderr or out.stdout).strip()[:400]}")
    return _paths(json.loads(out.stdout))


def _paths(node, found=None):
    """Collect file paths from any `file`/`ref` field, at any depth of the blast JSON.

    Walks rather than reading fixed keys: direct_callers carries `file`, indirect_callers
    carries only `ref`, and the composition/includes/subclass lists carry their own shapes.
    A hard-coded key list would silently under-count exactly the rows this check is about.
    """
    found = set() if found is None else found
    if isinstance(node, dict):
        for key, val in node.items():
            if key in ("file", "ref") and isinstance(val, str):
                found.add(val.split(":")[0])
            else:
                _paths(val, found)
    elif isinstance(node, list):
        for item in node:
            _paths(item, found)
    return found


def gold_rows(scenario_path, group=None):
    with open(scenario_path) as fh:
        gold = (yaml.safe_load(fh) or {}).get("gold") or []
    if not gold:
        raise SystemExit(f"gold_confidence_check: no gold in {scenario_path}")
    return [r for r in gold if not group or r.get("group") == group]


def row_reached(row, paths):
    """True if any of the row's match patterns is a suffix of a returned path."""
    for pat in row.get("match") or []:
        pat = str(pat).strip()
        if any(p == pat or p.endswith("/" + pat) for p in paths):
            return True
    return False


def classify(rows, low_paths, high_paths):
    """Split the gold into: survives both, 0.3-only (the defect), reachable by neither."""
    both, low_only, neither = [], [], []
    for row in rows:
        at_low, at_high = row_reached(row, low_paths), row_reached(row, high_paths)
        if at_high:
            both.append(row)
        elif at_low:
            low_only.append(row)
        else:
            neither.append(row)
    return both, low_only, neither


def report(rows, both, low_only, neither, symbol):
    print(f"## gold confidence check - {symbol}: min_confidence {LOW} vs {HIGH}")
    print(f"   gold rows: {len(rows)}   survive {HIGH}: {len(both)}   "
          f"{LOW}-only: {len(low_only)}   blast-unreachable: {len(neither)}")
    print()
    if neither:
        print(f"   not blast-reachable at either threshold ({len(neither)}) - reported, NOT")
        print("   failed: graph/search/hand-sourced gold is legitimately not in a blast set.")
        for row in neither:
            print(f"     - {row.get('id', '?'):<28} {', '.join(map(str, row.get('match') or []))}")
        print()
    if not low_only:
        print(f"   PASS - every blast-reachable row survives {HIGH}, the default agents pass.")
        return 0
    print(f"   FAIL - {len(low_only)} row(s) exist only at {LOW}. The benched agent cannot")
    print(f"   retrieve them at the documented default, so the delta they earn is not")
    print("   reproducible. Re-target the gold, or make the ask name the threshold.")
    for row in low_only:
        print(f"     - {row.get('id', '?'):<28} {', '.join(map(str, row.get('match') or []))}")
    return 1


def check(scenario, symbol, bin_path=None, file_sub=None, repo_dir=None, group=None):
    bin_path = os.path.abspath(bin_path or default_bin())
    rows = gold_rows(scenario, group)
    low = blast_files(bin_path, symbol, LOW, file_sub, repo_dir)
    high = blast_files(bin_path, symbol, HIGH, file_sub, repo_dir)
    both, low_only, neither = classify(rows, low, high)
    return report(rows, both, low_only, neither, symbol)


def parse_args(argv):
    if len(argv) < 3:
        raise SystemExit("usage: gold_confidence_check.py <scenario.yaml> <symbol> "
                         "[--file SUB] [--repo DIR] [--bin PATH] [--group NAME]")
    opts = {"scenario": argv[1], "symbol": argv[2]}
    keys = {"--file": "file_sub", "--repo": "repo_dir", "--bin": "bin_path",
            "--group": "group"}
    rest = argv[3:]
    while rest:
        flag = rest.pop(0)
        if flag not in keys or not rest:
            raise SystemExit(f"gold_confidence_check: bad flag {flag}")
        opts[keys[flag]] = rest.pop(0)
    return opts


def main(argv):
    return check(**parse_args(argv))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
