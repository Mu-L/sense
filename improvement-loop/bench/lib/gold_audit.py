#!/usr/bin/env python3
"""gold_audit.py -- has every gold row been hand-audited, against THIS gold?

    python3 gold_audit.py stamp  <scenario.yaml>   # write/refresh the audit sheet
    python3 gold_audit.py verify <scenario.yaml>   # exit 1 unless it is complete + current

Exit 0 = every gold row carries a verdict, and the sheet was written against the gold that
is on disk right now.
Exit 1 = no sheet, a stale sheet, or a row still marked TODO.

## WHY

The per-dependency hand audit is the load-bearing check in Loop 1 - a script tally alone has
passed wrong gold before (the gold.py basename false-credit re-scored 106 of 384 runs). It is
also the only check with nobody downstream to catch it, and the easiest to skip: it produces
no green output, so a session under pressure does the runnable thing instead. On 2026-08-01 a
session hand-ran Loop 1, never audited the draft it found, and shipped a whole instrument fix
while `rails.yaml` carried a gold comment citing x3 transcripts that do not exist.

`stamp` writes one line per gold row with `verdict: TODO`. The auditor reads each credit and
replaces TODO with what they actually found. `verify` fails while any TODO remains, so the
sheet cannot be produced by the same keystroke that satisfies it.

## WHY THE HASH

The sheet records a sha256 of the GOLD BLOCK only, not the whole file. Re-target a row and the
audit for it is void, which is exactly right; edit a prose comment elsewhere in the scenario
and the audit still stands. (`scenario_version` is the whole-file hash and is deliberately NOT
reused here: it drifts on a comment edit and would void an audit that is still true.)

The sheet is a sidecar, `<repo>.gold-audit.json`, so stamping never touches the scenario file
and never drifts `scenario_version`.
"""
import hashlib
import json
import os
import sys

import yaml

TODO = "TODO"


def sheet_path(scenario_path):
    return scenario_path[:-5] + ".gold-audit.json" if scenario_path.endswith(".yaml") \
        else scenario_path + ".gold-audit.json"


def gold_of(scenario_path):
    with open(scenario_path) as fh:
        gold = (yaml.safe_load(fh) or {}).get("gold") or []
    if not gold:
        raise SystemExit(f"gold_audit: no gold in {scenario_path}")
    return gold


def gold_digest(gold):
    """sha256 over the gold rows only, key-order-independent."""
    payload = json.dumps(gold, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode()).hexdigest()


def row_ids(gold):
    return [str(r.get("id", f"#{i}")) for i, r in enumerate(gold)]


def stamp(scenario_path):
    gold = gold_of(scenario_path)
    path = sheet_path(scenario_path)
    previous = {}
    if os.path.exists(path):
        with open(path) as fh:
            previous = (json.load(fh) or {}).get("rows") or {}
    rows = {rid: previous.get(rid, TODO) for rid in row_ids(gold)}
    sheet = {"gold_sha256": gold_digest(gold), "rows": rows}
    with open(path, "w") as fh:
        json.dump(sheet, fh, indent=2, sort_keys=True)
        fh.write("\n")
    todo = [r for r, v in rows.items() if v == TODO]
    print(f"## gold audit sheet - {os.path.basename(path)}")
    print(f"   {len(rows)} row(s), {len(todo)} awaiting a verdict")
    print("   Read each credit, then replace its TODO with what you found. Carried over"
          if previous else "   Read each credit, then replace its TODO with what you found.")
    for rid in todo:
        print(f"     - {rid}")
    return 0


def verify(scenario_path):
    gold = gold_of(scenario_path)
    path = sheet_path(scenario_path)
    print(f"## gold audit - {os.path.basename(scenario_path)}")
    if not os.path.exists(path):
        print("   FAIL - no audit sheet. The per-dependency hand audit is the load-bearing")
        print(f"   check in Loop 1 and it has not been done. Start it:")
        print(f"     python3 bench/lib/gold_audit.py stamp {scenario_path}")
        return 1
    with open(path) as fh:
        sheet = json.load(fh) or {}
    if sheet.get("gold_sha256") != gold_digest(gold):
        print("   FAIL - the sheet was written against different gold. Re-stamp and re-read")
        print("   the rows that changed; an audit of gold that no longer exists is not one.")
        return 1
    rows = sheet.get("rows") or {}
    missing = [rid for rid in row_ids(gold) if rid not in rows]
    todo = sorted(r for r, v in rows.items() if v == TODO)
    if missing or todo:
        print(f"   FAIL - {len(todo)} row(s) still TODO, {len(missing)} unlisted.")
        for rid in todo + missing:
            print(f"     - {rid}")
        return 1
    print(f"   PASS - all {len(rows)} row(s) hand-audited against the current gold.")
    return 0


def main(argv):
    if len(argv) != 3 or argv[1] not in ("stamp", "verify"):
        raise SystemExit("usage: gold_audit.py stamp|verify <scenario.yaml>")
    return (stamp if argv[1] == "stamp" else verify)(argv[2])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
