#!/usr/bin/env python3
"""Axis panel: score every banked cell on ALL brainstorm axes, from disk.

Observational instrument for assembly-cost-vs-visibility questions: runs
efficiency.py per cell and flattens its output into one row per cell so
every bench run feeds the multi-axis dataset (reach, cost, time, turnarounds,
navigation profile, reliability spread, grounding). The headline verdict stays
with the judging contract; panel rows are hypothesis fuel, never win claims.

Usage: VERTICAL=<key> panel.py [--json OUT.jsonl] [--md OUT.md]
Scans the PAID roots, verticals/<key>/results/<model>/<scenario-version>/, whose
`baseline/<repo>` and `sense/<repo>` hold the scored runs - the arms sit under the
scenario version, never directly under the model. Writes back into that same tree:
verticals/<key>/results/panel/. A result file lives under its vertical, never in a
shared root.
"""
import argparse
import json
import os
import re
import subprocess
import sys

LIB = os.path.dirname(os.path.abspath(__file__))
# verticals/ is a SIBLING of bench/, at the improvement-loop root - not under bench/.
# This read `bench/verticals` and died with FileNotFoundError on every report phase.
IL_ROOT = os.path.normpath(os.path.join(LIB, "..", ".."))
VERTICALS = os.path.join(IL_ROOT, "verticals")
import arms
import banked
# No default vertical: a panel is one vertical's dataset and is written into that
# vertical's results tree, so there is nowhere to put it without a key.
VERTICAL = os.environ.get("VERTICAL", "")
if not VERTICAL:
    raise SystemExit("panel: set VERTICAL=<key>")
VDIR = os.path.join(VERTICALS, VERTICAL)
MODEL = os.environ.get("BENCH_MODEL") or arms.headline(VERTICAL)
# The results tree names model dirs SANITIZED (bench-paths.sh: / and : -> _), so a raw
# arms.txt id matches nothing: `glm-5.2:cloud` is `glm-5.2_cloud` on disk. Comparing
# the two unsanitized reports zero cells and looks like "nothing banked yet".
MODEL_DIR = MODEL.replace("/", "_").replace(":", "_")

ROW = re.compile(r"^\| (?P<axis>[^|]+?) \| (?P<base>[^|]+?) \| (?P<sense>[^|]+?) \| (?P<delta>[^|]+?) \|$")
REACH = re.compile(r"cited_recall (?P<base>[\d.]+)→(?P<sense>[\d.]+) \((?P<mult>[\d.]+)×\)")
RELI = re.compile(r"baseline \[(?P<base>[^\]]+)\] → sense \[(?P<sense>[^\]]+)\]")


def cells():
    """(repo, paid root) per banked cell on the headline arm.

    banked.paid_roots owns the walk: it is the one place that knows the arms sit
    under <model>/<scenario-version>, and that `validation/`, `minibench/` and the
    doubled legacy dirs are not banked cells.
    """
    for root in banked.paid_roots(VDIR):
        if os.path.basename(os.path.dirname(root)) != MODEL_DIR:
            continue
        if not os.path.isdir(os.path.join(root, "baseline")):
            continue
        for repo in sorted(os.listdir(os.path.join(root, "baseline"))):
            if os.path.isdir(os.path.join(root, "baseline", repo)):
                yield repo, root


def spread(text):
    vals = [int(x) for x in text.replace(" ", "").split(",") if x]
    return {"runs": vals, "spread": max(vals) - min(vals) if vals else None}


def parse(repo, root, out):
    # scenario_version: a repo can be banked at more than one root, so the row is
    # only identifiable with the question it was benched on.
    row = {"vertical": VERTICAL, "repo": repo, "model": MODEL,
           "scenario_version": os.path.basename(root)}
    for line in out.splitlines():
        m = ROW.match(line.strip())
        if m:
            axis = m.group("axis").strip()
            key = {
                "billed tokens": "tokens",
                "session time (s, median)": "wall_s",
                "cost ($)": "cost_usd",
                "turnarounds (tool calls)": "tool_calls",
                "navigation (grep / mcp / read)": "nav",
                "grounding (anti-fab)": "grounding",
            }.get(axis)
            if key:
                row[key] = {"baseline": m.group("base").strip(),
                            "sense": m.group("sense").strip(),
                            "delta": m.group("delta").strip()}
        m = REACH.search(line)
        if m:
            row["cited_recall"] = {"baseline": float(m.group("base")),
                                   "sense": float(m.group("sense")),
                                   "mult": float(m.group("mult"))}
        if "reliability" in line:
            m = RELI.search(line)
            if m:
                row["reliability"] = {"baseline": spread(m.group("base")),
                                      "sense": spread(m.group("sense"))}
    return row


def main():
    ap = argparse.ArgumentParser()
    panel_dir = os.path.join(VDIR, "results", "panel")
    ap.add_argument("--json", default=os.path.join(panel_dir, "panel.jsonl"))
    ap.add_argument("--md", default=os.path.join(panel_dir, "panel.md"))
    args = ap.parse_args()
    os.makedirs(os.path.dirname(args.json), exist_ok=True)
    os.makedirs(os.path.dirname(args.md), exist_ok=True)

    rows = []
    for repo, root in cells():
        proc = subprocess.run([sys.executable, os.path.join(LIB, "efficiency.py"), repo, root],
                              capture_output=True, text=True)
        if proc.returncode != 0:
            print(f"skip {VERTICAL}/{repo}: {proc.stderr.strip().splitlines()[-1] if proc.stderr else 'no output'}",
                  file=sys.stderr)
            continue
        rows.append(parse(repo, root, proc.stdout))

    with open(args.json, "w") as f:
        for r in rows:
            f.write(json.dumps(r) + "\n")

    lines = ["# Axis panel: all banked cells, from disk (observational; headline stays with the judge)",
             "",
             "| cell | recall b→s | cost Δ | time Δ | calls Δ | nav b→s | reli spread b→s | ground b→s |",
             "|---|---|---|---|---|---|---|---|"]
    for r in rows:
        cr = r.get("cited_recall", {})
        reli = r.get("reliability", {})
        lines.append("| {v}/{r} | {cb}→{cs} | {cost} | {wall} | {calls} | {navb}→{navs} | {rb}→{rs} | {gb}→{gs} |".format(
            v=r["vertical"], r=r["repo"],
            cb=cr.get("baseline", "?"), cs=cr.get("sense", "?"),
            cost=r.get("cost_usd", {}).get("delta", "?"),
            wall=r.get("wall_s", {}).get("delta", "?"),
            calls=r.get("tool_calls", {}).get("delta", "?"),
            navb=r.get("nav", {}).get("baseline", "?"), navs=r.get("nav", {}).get("sense", "?"),
            rb=reli.get("baseline", {}).get("spread", "?"), rs=reli.get("sense", {}).get("spread", "?"),
            gb=r.get("grounding", {}).get("baseline", "?"), gs=r.get("grounding", {}).get("sense", "?")))
    with open(args.md, "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"{len(rows)} cells → {args.json}\n{args.md}")


if __name__ == "__main__":
    main()
