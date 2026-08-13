#!/usr/bin/env python3
"""gold_confidence_check.py -- is this gold in the output the benched agent is SHOWN?

    python3 gold_confidence_check.py <scenario.yaml> <symbol> [--file SUB] [--repo DIR]
                                     [--bin PATH] [--group NAME]

Exit 0 = every gold row in the checked group appears in the shown output.
Exit 1 = at least one row does not -- a manufactured win.

## WHY

Gold the sense arm cannot retrieve earns a delta no agent can reproduce, and a fake win
propagates into the confirmation matrix, the harvest and a published article before anyone
re-reads it. So the one question is: does the arm SEE this row?

That question has exactly one honest instrument, the MCP server, because that is the only
surface the sense arm ever touches (`bench-sense-local.sh` wires the clone's `.mcp.json`).
This script used to answer it by running the `sense` CLI at `--min-confidence` 0.3 vs 0.7,
on the premise that agents pass the CLI's documented 0.7 default. That premise was false for
the benched arm: MCP defaults to 0.3 (`internal/profile/profile.go`), and the tool description
tells the agent not to raise it. A gate that measures the CLI can only fail true gold.

There is no threshold comparison here any more, because no arm runs the raised threshold.
Measured on the pinned rails clone, lowering the threshold does not add rows to the shown
output at all - both sides resolve the same 80 slots, and 0.3 simply admits more competitors
(276 vs 144) so the cap evicts different ones. The shown, budgeted output IS the measurement.

## WHAT IT DOES NOT DO

It does not judge whether a row BELONGS in the gold: blast-radius gold != edit-impact gold,
and that is the hand-audit's job (the gitea case: 32 of 42 rows were answering a different
question, and no reachability check would have caught it).

It has no opinion on gold sourced from graph, search or a hand read. Scope it with `--group`
to the blast-sourced group and run it once per such group. Handed the whole gold it would
fail every non-blast row, which would narrow the bench to one tool.
"""
import json
import os
import sys

import yaml

from mcp_probe import probe
from sense_build import default_bin


def shown_paths(bin_path, symbol, file_sub=None, repo_dir=None):
    """Every file path `sense_blast` SHOWS the agent over MCP, at the arm's defaults.

    No `min_confidence` is passed, because the arm does not pass one: the MCP default is
    what the benched agent gets, and the budget/cap truncation is part of what it gets.
    """
    args = {"symbol": symbol}
    if file_sub:
        args["file"] = file_sub
    repo = repo_dir or os.getcwd()
    if not os.path.isdir(os.path.join(repo, ".sense")):
        raise SystemExit(f"gold_confidence_check: no .sense index under {repo}")
    results, proc = probe(repo, [{"name": "sense_blast", "arguments": args}], bin_path)
    if not results:
        raise SystemExit("gold_confidence_check: sense_blast returned nothing over MCP: "
                         f"{(proc.stderr or proc.stdout).strip()[:400]}")
    return _paths(json.loads(results[0][1]))


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


def classify(rows, paths):
    """Split the gold into: shown to the agent, and missing from what it is shown."""
    shown, missing = [], []
    for row in rows:
        (shown if row_reached(row, paths) else missing).append(row)
    return shown, missing


def report(rows, shown, missing, symbol, group):
    scope = group or "ALL groups"
    print(f"## gold shown check - {symbol} over MCP, arm defaults (group: {scope})")
    print(f"   gold rows: {len(rows)}   shown: {len(shown)}   missing: {len(missing)}")
    print()
    if not missing:
        print("   PASS - every row appears in the output the benched agent is shown.")
        return 0
    print(f"   FAIL - {len(missing)} row(s) are not in what the agent is shown, so the")
    print("   delta they earn is not reproducible. Re-target the gold, or re-scope the")
    print("   ask so the call that reaches them is the one the scenario rests on.")
    for row in missing:
        print(f"     - {row.get('id', '?'):<28} {', '.join(map(str, row.get('match') or []))}")
    return 1


def check(scenario, symbol, bin_path=None, file_sub=None, repo_dir=None, group=None):
    bin_path = os.path.abspath(bin_path or default_bin())
    rows = gold_rows(scenario, group)
    shown, missing = classify(rows, shown_paths(bin_path, symbol, file_sub, repo_dir))
    return report(rows, shown, missing, symbol, group)


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
