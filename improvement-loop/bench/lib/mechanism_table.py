#!/usr/bin/env python3
"""The WHY behind a cell's number: did Sense supply the row, and was it used.

Cycle 2 (the cross-model board) reports, per model, not just a delta but the
mechanism behind it. That mechanism is two booleans per gold row:

    returned   some Sense call in this run returned the row's file WITH a line
    cited      the scorer credited the arm for the row (scored.json)

Crossed, they are the whole vocabulary:

                    | cited          | not cited
    ----------------+----------------+---------------------------------
    returned        | reach          | ignored      (ours: payload/format)
    not returned    | found-anyway   | missed       (ours: coverage gap)

Three of those four are findings about Sense, which is what keeps the board an
audit of the product rather than a leaderboard of models.

WHY THIS READS sense-io.jsonl AND NOT THE TRANSCRIPT. transcript_miss.py mines
the same idea from transcript.json, but that file has a different shape per
harness (claude / codex / opencode), and cycle 2 must read four vendors the same
way. mcp_tee.py writes sense-io.jsonl identically for all of them, so the MCP
log is the only uniform surface. transcript_miss.py is a cycle 1 script and is
not touched: a cycle reads artifacts off disk, it never edits another's code.

WHY path+line AND NOT path ALONE. gold.py credits a row as cited only when the
answer names the path WITH a line (`path:line`, a nearby `"lines": [N]` field,
or an unambiguous basename+line). Matching `returned` on the path alone would
put "Sense returned the right FILE but not the right symbol" into the `ignored`
cell, reporting a ranking gap as an adoption failure - backwards, and the cell
we would most want to act on. Sense responses carry an explicit `"ref"` field,
so the two sides can be held at the same granularity by construction.

ROUTING IS A STATE ABOVE THE TABLE. A Sense arm that never called Sense is a
baseline with extra config, and its delta says nothing about the product. The
MCP log separates the cases the score cannot:

    harness-failure  no log, no frames, or a run -> not a measurement, re-run
                     run_validity calls invalid
    never-routed     frames, but no tools/call   -> a real routing finding
    search-only      calls, but no blast/graph   -> reached no resolver
    routed           at least one blast/graph

Usage:
    mechanism_table.py <results-root> <repo> --scenario <scenario.yaml> [--json]

    <results-root> is one model's question root, e.g.
    verticals/<v>/results/<model>/<scenario-version>
"""
import argparse
import glob
import json
import os
import re
import sys

import run_validity

REF_RE = re.compile(r'"ref"\s*:\s*"([^"\s:]+):(\d+)"')
BARE_REF_RE = re.compile(r'\b([\w./-]+\.\w{1,5}):(\d+)\b')
STRUCTURAL = ("sense_blast", "sense_graph")

REACH, IGNORED, FOUND_ANYWAY, MISSED = "reach", "ignored", "found-anyway", "missed"


def load_gold(scenario_path):
    """The scenario's gold rows as [{id, group, match:[paths]}]."""
    import yaml

    doc = yaml.safe_load(open(scenario_path))
    rows = doc.get("gold") or []
    return [
        {"id": r["id"], "group": r.get("group", ""), "match": list(r.get("match") or [])}
        for r in rows
        if r.get("id")
    ]


def _frames(io_path):
    """Every JSON-RPC frame mcp_tee logged, skipping any that did not parse."""
    out = []
    with open(io_path) as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                out.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return out


def _calls_and_results(frames):
    """(id -> tool name) for tools/call requests, and (id -> response text)."""
    calls, results = {}, {}
    for f in frames:
        msg = f.get("msg") or {}
        if f.get("dir") == "c2s" and msg.get("method") == "tools/call":
            calls[msg.get("id")] = ((msg.get("params") or {}).get("name")) or ""
        elif f.get("dir") == "s2c" and "result" in msg:
            content = (msg.get("result") or {}).get("content") or []
            results[msg.get("id")] = "".join(c.get("text", "") for c in content)
    return calls, results


def _refs_with_line(text):
    """Repo-relative paths this response returned WITH a line number."""
    paths = {m.group(1) for m in REF_RE.finditer(text)}
    if not paths:
        paths = {m.group(1) for m in BARE_REF_RE.finditer(text)}
    return paths


def read_run(run_dir):
    """One run's routing state, returned paths, and cited gold ids."""
    io_path = os.path.join(run_dir, "sense-io.jsonl")
    frames = _frames(io_path) if os.path.exists(io_path) else []
    calls, results = _calls_and_results(frames)

    returned = set()
    for cid in calls:
        returned |= _refs_with_line(results.get(cid, ""))

    if not frames or not _is_measurement(run_dir):
        routing = "harness-failure"
    elif not calls:
        routing = "never-routed"
    elif not any(t in STRUCTURAL for t in calls.values()):
        routing = "search-only"
    else:
        routing = "routed"

    cited = set()
    spath = os.path.join(run_dir, "scored.json")
    if os.path.exists(spath):
        try:
            scored = json.load(open(spath))
            for d in scored.get("gold_recall", {}).get("details", []):
                if d.get("cited"):
                    cited.add(d.get("id"))
        except (json.JSONDecodeError, KeyError, TypeError):
            pass

    return {
        "run": os.path.basename(run_dir),
        "routing": routing,
        "tools": sorted(set(calls.values())),
        "call_count": len(calls),
        "returned_paths": sorted(returned),
        "cited_ids": sorted(cited),
    }


def _is_measurement(run_dir):
    """Did this run measure the arm, per the one shared classifier?

    A crashed harness still writes the MCP handshake frames, so the log alone
    cannot tell "the model chose not to call Sense" from "the session died before
    it could". Two Kimi runs died that way and were published as a routing gap on
    our side, blanking a reach cell that held 18 sense-only answers. The runner
    already stamped the crash in run_meta; ask run_validity, the one classifier,
    rather than inventing a second rule (A DEAD SERVER IS NOT A MODEL CHOICE).
    """
    return run_validity.measured(os.path.join(run_dir, "scored.json"))


def _path_matches(gold_path, returned_paths):
    """Same suffix rule gold.py uses, so both sides agree on what a path is."""
    return any(
        gold_path == r or r.endswith("/" + gold_path) or gold_path.endswith("/" + r)
        for r in returned_paths
    )


def classify_run(run, gold):
    """Per gold row: which of the four cells this run landed in."""
    returned_paths, cited = set(run["returned_paths"]), set(run["cited_ids"])
    cells = {}
    for row in gold:
        was_returned = any(_path_matches(p, returned_paths) for p in row["match"])
        was_cited = row["id"] in cited
        if was_returned:
            cells[row["id"]] = REACH if was_cited else IGNORED
        else:
            cells[row["id"]] = FOUND_ANYWAY if was_cited else MISSED
    return cells


def _dominant(counts):
    """The cell a run is mostly in. Ties break toward the worse news."""
    order = [MISSED, IGNORED, FOUND_ANYWAY, REACH]
    return max(order, key=lambda c: (counts.get(c, 0), -order.index(c)))


def build(root, repo, gold):
    """The mechanism table for one model's cell: per run, plus the roll-up."""
    run_dirs = sorted(glob.glob(os.path.join(root, "sense", repo, "run-*")))
    runs = []
    for rd in run_dirs:
        run = read_run(rd)
        run["cells"] = classify_run(run, gold)
        run["counts"] = {c: list(run["cells"].values()).count(c)
                         for c in (REACH, IGNORED, FOUND_ANYWAY, MISSED)}
        run["dominant"] = _dominant(run["counts"])
        runs.append(run)

    measured = [r for r in runs if r["routing"] != "harness-failure"]
    disagreed = sorted(
        row["id"] for row in gold
        if len({r["cells"][row["id"]] for r in measured}) > 1
    )
    verdicts = {r["dominant"] for r in measured}
    return {
        "repo": repo,
        "root": root,
        "gold_rows": len(gold),
        "runs": runs,
        "measured_runs": len(measured),
        "routing": sorted({r["routing"] for r in measured}) or ["harness-failure"],
        "rows_disagreeing": disagreed,
        # A third run is bought by a flipped VERDICT, never by a flipped row: with
        # 23 gold rows a single row differs on almost every pair, so "any row" would
        # make run-3 unconditional. The per-row splits are still reported above.
        "verdict_split": len(verdicts) > 1,
        "dominant": sorted(verdicts)[0] if len(verdicts) == 1 else None,
    }


def render(table):
    """The plain-text table, for a human reading a run by hand."""
    out = [f"# mechanism: {table['repo']}  ({table['gold_rows']} gold rows)", ""]
    out.append(f"routing: {', '.join(table['routing'])}   "
               f"measured runs: {table['measured_runs']}")
    for run in table["runs"]:
        c = run["counts"]
        out.append(f"  {run['run']:6} {run['routing']:16} "
                   f"reach {c[REACH]:2}  ignored {c[IGNORED]:2}  "
                   f"found-anyway {c[FOUND_ANYWAY]:2}  missed {c[MISSED]:2}   "
                   f"-> {run['dominant']}")
    if table["rows_disagreeing"]:
        out.append("")
        out.append(f"rows that flipped between runs ({len(table['rows_disagreeing'])}): "
                   + ", ".join(table["rows_disagreeing"]))
    if table["verdict_split"]:
        out.append("VERDICT SPLIT: the runs disagree on the dominant cell; run a third.")
    return "\n".join(out)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("root", help="one model's question root")
    ap.add_argument("repo")
    ap.add_argument("--scenario", required=True, help="path to the scenario yaml")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args(argv[1:])

    gold = load_gold(args.scenario)
    if not gold:
        sys.exit(f"mechanism_table.py: no gold rows in {args.scenario}")
    table = build(args.root, args.repo, gold)
    if not table["runs"]:
        sys.exit(f"mechanism_table.py: no sense runs under {args.root}/sense/{args.repo}")
    print(json.dumps(table, indent=2, sort_keys=True) if args.json else render(table))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
