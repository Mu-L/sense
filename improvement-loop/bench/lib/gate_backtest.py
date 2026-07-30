#!/usr/bin/env python3
"""gate_backtest.py -- run the admission gate against the BANKED WINS and fail if it kills them.

    python3 bench/lib/gate_backtest.py [--json OUT]

Exit 0 = the gate admits every known win. Exit 1 = it rejects at least one, and therefore
may not gate anything.

## WHY THIS EXISTS

The admission gate spent four verticals (python, php, go, php again) rejecting and admitting
repos, and never produced a win. Nobody checked it against the wins we already had. On
2026-07-30 someone finally ran it against the four banked go cells and it returned
**4 of 4 REJECT** - a filter that kills 100% of known positives.

Three of those it even labelled "win signature" in bar 3, correctly identifying the shape,
then rejected them on a later bar. dolt it killed outright as BALLAST-ONLY because the token
`DoltDB` covers 92.5% of dependents - true, and irrelevant: dolt's baseline still scored
0.25/0.33 because the question was not "where is the token" but "what retains this object".

The cost of not running this was a day of improving a gate that had never worked, while the
frozen evidence needed to falsify it sat on disk the whole time.

## THE LAW THIS ENFORCES

A bar may gate only while this backtest is green. `admission_gate.py` reads the attestation
this script writes and downgrades its verdict to ADVISORY when it is missing or failing, so
the rule cannot be forgotten - a falsified gate stops gating by construction rather than by
anyone remembering.

The corpus is deliberately the WINS, not a mix. This is a false-negative test, the same
standard bar 2's 0.50 bound was calibrated against ("> 0.50 kills 10 cells and 0 wins").
A gate that rejects a banked win is wrong no matter how good its reasoning sounds.
"""
import argparse
import json
import os
import subprocess
import sys

LIB = os.path.dirname(os.path.abspath(__file__))
CLONES = os.environ.get(
    "SENSE_CLONES", os.path.expanduser("~/Developer/luuuc/oss/sense-benchmark/sense"))

# Banked wins: repo, anchor, anchor file, slot, and the MEASURED headline delta.
# Deltas are read off the frozen scored runs, not remembered.
BANKED_WINS = [
    {"repo": "pebble", "symbol": "Batch", "file": "batch.go",
     "slot": "medium", "delta": 1.000, "baseline": "0.00, 0.00"},
    {"repo": "dolt", "symbol": "DoltDB",
     "file": "go/libraries/doltcore/doltdb/doltdb.go",
     "slot": "big", "delta": 0.708, "baseline": "0.25, 0.33"},
    {"repo": "nomad", "symbol": "Server", "file": "nomad/server.go",
     "slot": "big", "delta": 0.567, "baseline": "0.53, 0.33"},
    {"repo": "consul", "symbol": "Server", "file": "agent/consul/server.go",
     "slot": "big", "delta": 0.538, "baseline": "0.38, 0.54"},
]

ATTESTATION = os.path.join(LIB, "..", "gate-backtest.json")


def run_one(win):
    clone = os.path.join(CLONES, win["repo"])
    if not os.path.isdir(clone):
        return {"skipped": f"clone missing: {clone}"}
    out = subprocess.run(
        [sys.executable, os.path.join(LIB, "admission_gate.py"), clone, win["symbol"],
         "--file", win["file"], "--slot", win["slot"]],
        capture_output=True, text=True)
    verdict = "UNKNOWN"
    for line in out.stdout.splitlines():
        if "ADMISSION:" in line:
            verdict = line.split("ADMISSION:")[1].strip().strip("*").strip()
            break
    return {"verdict": verdict, "stdout_tail": out.stdout[-400:] if verdict == "UNKNOWN" else ""}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--json", dest="json_out", default=ATTESTATION)
    args = ap.parse_args()

    rows, killed, skipped = [], [], []
    for win in BANKED_WINS:
        res = run_one(win)
        if "skipped" in res:
            skipped.append((win, res["skipped"]))
            continue
        v = res["verdict"]
        # PENDING is not a rejection: it means a bar has not run, which is a
        # different statement from "this cell cannot win".
        ok = v.startswith("ADMIT") or v.startswith("PENDING")
        rows.append({"repo": win["repo"], "symbol": win["symbol"],
                     "delta": win["delta"], "verdict": v, "ok": ok})
        if not ok:
            killed.append(win["repo"])

    print("## gate backtest - does the gate admit the wins we already banked?")
    print()
    print(f"   {'repo':10} {'anchor':10} {'measured delta':>14}   verdict")
    for r in rows:
        mark = "" if r["ok"] else "   <- KILLS A BANKED WIN"
        print(f"   {r['repo']:10} {r['symbol']:10} {r['delta']:>+14.3f}   {r['verdict']}{mark}")
    for win, why in skipped:
        print(f"   {win['repo']:10} {win['symbol']:10} {'-':>14}   SKIPPED ({why})")
    print()

    passed = not killed and rows
    attest = {"passed": passed, "killed": killed, "rows": rows,
              "skipped": [w["repo"] for w, _ in skipped]}
    with open(args.json_out, "w") as fh:
        json.dump(attest, fh, indent=1, sort_keys=True)
        fh.write("\n")

    if not rows:
        print("   NO CORPUS: no banked-win clone was found, so the gate is UNVERIFIED.")
        print("   An unverified gate is treated exactly like a failing one.")
        return 1
    if killed:
        print(f"   FAIL - the gate rejects {len(killed)} of {len(rows)} banked wins: "
              f"{', '.join(killed)}.")
        print("   A filter that kills known positives may not gate anything. Its bars stay")
        print("   ADVISORY until this is green: fix the bar, or stop calling it a gate.")
        return 1
    print(f"   PASS - all {len(rows)} banked wins survive the gate.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
