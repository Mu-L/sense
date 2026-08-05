#!/usr/bin/env python3
"""The cycle 2 board: one repo's scenario, measured across every arm.

Three jobs, all read-only over what is already on disk:

  eligible   which banked cells are owed a board
  gate       is this machine allowed to bench them
  assemble   the numbers JSON a board renders from

THE HEADLINE COLUMN IS READ, NEVER RE-RUN. The scenario earned its way here by
winning on the headline arm, and that cell is already in banked.jsonl with its
runs, its groups and its billed tokens. Re-running it would spend the most
constrained subscription to reproduce a number we hold, so the board reuses it
and stamps the date per column instead of per page.

EVERY NUMBER IS WITHIN ONE MODEL. A column is that model's sense arm against
that model's own baseline. Absolute scores are NOT comparable across models -
they run different harnesses at different budgets - so the board carries deltas
and the replication count, never a ranking. cost_parity.py owns the parity call;
this file carries raw billed tokens and does not become a rival answer to it.

THE VERSION GATE EXISTS BECAUSE THE COLUMNS MUST BE THE SAME PRODUCT. The
headline column was banked at one sense build. If the confirmation arms run at
another, the board quietly compares two products and reads as a model
difference. Refuse loudly instead: same build for every column, or no board.

A COLUMN THAT NEVER CALLED SENSE IS NOT A COLUMN ABOUT SENSE. mechanism_table.py
splits never-routed from harness-failure from routed; this file carries that
split into the replication count so an arm that ignored Sense cannot be reported
as Sense failing to help it.

Usage:
    board.py eligible  <vertical-dir> --headline <model>
    board.py gate      <vertical-dir> --repo <repo> --headline <model>
    board.py assemble  <vertical-dir> --repo <repo> --headline <model> [--arms "a b"]
"""
import argparse
import glob
import json
import os
import re
import subprocess
import sys

import banked
import mechanism_table

THRESHOLD = 0.50
VERSION_DIR = re.compile(r"^[0-9a-f]{16}$")


def short_version(scenario_version):
    """The bare hex of a `sha256:...` scenario version - the board's identity."""
    return (scenario_version or "").split(":")[-1]


def report_path(vertical_dir, repo, scenario_version):
    """One board per (repo, question). A second one is a redo, not a new page."""
    return os.path.join(vertical_dir, "reports",
                        f"{repo}-{short_version(scenario_version)}.md")


def banked_rows(vertical_dir):
    path = os.path.join(vertical_dir, "banked.jsonl")
    if not os.path.exists(path):
        return []
    rows = []
    for line in open(path):
        line = line.strip()
        if line:
            rows.append(json.loads(line))
    return rows


def eligible(vertical_dir, headline):
    """Banked WINs on the headline arm that have no board yet."""
    out = []
    for row in banked_rows(vertical_dir):
        if row.get("verdict") != "WIN" or row.get("model") != headline:
            continue
        if os.path.exists(report_path(vertical_dir, row["repo"], row["scenario_version"])):
            continue
        out.append(row)
    return out


def installed_sense_version():
    try:
        out = subprocess.run(["sense", "--version"], capture_output=True, text=True,
                             timeout=30)
    except (OSError, subprocess.SubprocessError):
        return None
    return (out.stdout or "").strip().splitlines()[0] if out.stdout.strip() else None


def gate(vertical_dir, repo, headline, installed=None):
    """Refuse a board whose columns would not be the same Sense build."""
    rows = [r for r in banked_rows(vertical_dir)
            if r["repo"] == repo and r.get("model") == headline
            and r.get("verdict") == "WIN"]
    if not rows:
        return {"ok": False, "reason": f"no banked WIN for {repo} on {headline}"}
    row = rows[-1]
    want = row.get("sense_version")
    have = installed if installed is not None else installed_sense_version()
    if not have:
        return {"ok": False, "reason": "cannot read `sense --version`", "banked": want}
    if have != want:
        return {"ok": False, "banked": want, "installed": have,
                "reason": "installed Sense differs from the banked headline column; "
                          "the board would compare two products"}
    return {"ok": True, "sense_version": want,
            "scenario_version": row["scenario_version"]}


def arm_root(vertical_dir, model, scenario_version):
    """One arm's question root, mirroring bench-paths.sh."""
    return os.path.join(vertical_dir, "results",
                        model.replace("/", "_").replace(":", "_"),
                        short_version(scenario_version))


def _scored(root, arm, repo):
    """Every scored run for one arm, in run order."""
    out = []
    for path in sorted(glob.glob(os.path.join(root, arm, repo, "run-*", "scored.json"))):
        try:
            out.append(json.load(open(path)))
        except (OSError, ValueError):
            continue
    return out


def _mean(values):
    return round(sum(values) / len(values), 2) if values else None


def session(root, repo):
    """What the work actually cost, per arm: time, tokens, money, tool calls.

    A delta with no cost beside it is half a result. These come from the metrics
    block the scorer already writes, so the board reports what was billed rather
    than a second opinion about it.
    """
    out = {}
    for arm in ("baseline", "sense"):
        runs = [s.get("metrics") or {} for s in _scored(root, arm, repo)]
        if not runs:
            continue
        out[arm] = {
            "wall_time_seconds": _mean([m.get("wall_time_seconds", 0) for m in runs]),
            "token_total_billed": _mean([m.get("token_total_billed", 0) for m in runs]),
            "cost_usd": _mean([m.get("cost_usd", 0) for m in runs]),
            "tool_calls": _mean([m.get("tool_calls", 0) for m in runs]),
            "grep_count": _mean([m.get("grep_count", 0) for m in runs]),
            "read_count": _mean([m.get("read_count", 0) for m in runs]),
            "mcp_count": _mean([m.get("mcp_count", 0) for m in runs]),
        }
    return out


def _cited_ids(root, arm, repo):
    """Every gold id this arm cited in ANY of its runs."""
    seen = set()
    for scored in _scored(root, arm, repo):
        for d in (scored.get("gold_recall", {}) or {}).get("details", []):
            if d.get("cited"):
                seen.add(d.get("id"))
    return seen


def coverage(root, repo, gold):
    """The 38 answers split by WHICH ARM reached them. The value picture.

    Without a baseline in it, a breakdown of the Sense run alone reads as "Sense
    supplied about half" when the truth is that a chunk of those answers exist
    only because Sense was there. The split that shows the value honestly is by
    arm, not by provenance inside one arm:

        both          the model would have got these anyway
        sense_only    it never reached these without Sense, in any run
        baseline_only reached without Sense but not with; noise, and shown
        neither       reached by no arm; the honest remainder
    """
    base, sense = _cited_ids(root, "baseline", repo), _cited_ids(root, "sense", repo)
    if not gold:
        return {}
    ids = {row["id"] for row in gold}
    base, sense = base & ids, sense & ids
    return {
        "both": len(base & sense),
        "sense_only": len(sense - base),
        "baseline_only": len(base - sense),
        "neither": len(ids - base - sense),
        "total": len(ids),
    }


def sense_only_reach(root, repo):
    """Answers the Sense arm reached that its own baseline never reached.

    The headline of the whole programme: not "scored higher" but "found what it
    could not otherwise find". Mirrors the sense-only reach in efficiency.py -
    cited by the Sense arm in at least one run, cited by the baseline in none.
    """
    return sorted(_cited_ids(root, "sense", repo) - _cited_ids(root, "baseline", repo))


def _column(root, repo, model, gold, source):
    """One model's column: its own pair, plus why the number is what it is."""
    row = banked.build_row(root, repo) if os.path.isdir(root) else None
    if row is None:
        return {"model": model, "source": source, "measured": False,
                "reason": "no scored runs under this arm's root"}
    mech = mechanism_table.build(root, repo, gold) if gold else {}
    return {
        "model": model,
        "source": source,
        "measured": True,
        "runs": row["runs"],
        "overall": row["overall"],
        "groups": row["groups"],
        "best_group_delta": row["best_group_delta"],
        "billed_tokens": row["billed_tokens"],
        "sense_version": row.get("sense_version"),
        "recorded_at": row.get("recorded_at"),
        "session": session(root, repo),
        "sense_only_reach": sense_only_reach(root, repo),
        "coverage": coverage(root, repo, gold),
        "routing": mech.get("routing", []),
        "mechanism": {k: mech[k] for k in
                      ("runs", "measured_runs", "rows_disagreeing", "verdict_split",
                       "dominant", "gold_rows") if k in mech},
    }


def _replication(columns, threshold=THRESHOLD):
    """Counted, never ranked - and an arm that ignored Sense is not counted at all."""
    routed, replicated, never_routed, search_only, not_measured = [], [], [], [], []
    for col in columns:
        if not col.get("measured"):
            not_measured.append(col["model"])
            continue
        states = col.get("routing") or []
        if states == ["never-routed"]:
            never_routed.append(col["model"])
            continue
        if states == ["search-only"]:
            search_only.append(col["model"])
            continue
        routed.append(col["model"])
        if col["best_group_delta"] >= threshold:
            replicated.append(col["model"])
    return {"routed": routed, "replicated": replicated, "never_routed": never_routed,
            "search_only": search_only, "not_measured": not_measured,
            "threshold": threshold}


def question(scenario_path):
    """The task the models were given, verbatim.

    A public board has to show the actual ask, not a paraphrase: a reader cannot
    judge a result without seeing the question that produced it, and paraphrasing
    it here would let the page drift from what the models were sent.
    """
    if not scenario_path or not os.path.exists(scenario_path):
        return {}
    import yaml

    doc = yaml.safe_load(open(scenario_path))
    return {
        "name": doc.get("name", ""),
        "description": (doc.get("description") or "").strip(),
        "contract_symbol": doc.get("contract_symbol", ""),
        "contract_file": doc.get("contract_file", ""),
        "steps": [{"name": s.get("name", ""), "prompt": (s.get("prompt") or "").strip()}
                  for s in (doc.get("steps") or [])],
    }


def assemble(vertical_dir, repo, headline, arms, scenario_path=None):
    """The numbers JSON. Nothing downstream may print a figure absent from here."""
    banked_row = next((r for r in reversed(banked_rows(vertical_dir))
                       if r["repo"] == repo and r.get("model") == headline
                       and r.get("verdict") == "WIN"), None)
    if banked_row is None:
        raise SystemExit(f"board.py: no banked WIN for {repo} on {headline}")
    version = banked_row["scenario_version"]
    gold = mechanism_table.load_gold(scenario_path) if scenario_path else []

    head = _column(arm_root(vertical_dir, headline, version), repo, headline, gold,
                   source="banked")
    if head.get("measured"):
        head["recorded_at"] = banked_row.get("recorded_at")
    columns = [head] + [
        _column(arm_root(vertical_dir, m, version), repo, m, gold, source="benched")
        for m in arms
    ]
    return {
        "repo": repo,
        "vertical": os.path.basename(os.path.normpath(vertical_dir)),
        "scenario_version": version,
        "sense_version": banked_row.get("sense_version"),
        "gold_rows": len(gold),
        "headline": headline,
        "question": question(scenario_path),
        "columns": columns,
        "replication": _replication(columns[1:]),
    }


def _vdir(args):
    return os.path.normpath(args.vertical_dir)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = ap.add_subparsers(dest="cmd", required=True)
    for name in ("eligible", "gate", "assemble"):
        p = sub.add_parser(name)
        p.add_argument("vertical_dir")
        p.add_argument("--headline", required=True)
        if name != "eligible":
            p.add_argument("--repo", required=True)
        if name == "assemble":
            p.add_argument("--arms", default="", help="space-separated confirmation arms")
            p.add_argument("--scenario", default=None)
    args = ap.parse_args(argv[1:])

    if args.cmd == "eligible":
        rows = eligible(_vdir(args), args.headline)
        for r in rows:
            print(f"{r['repo']}\t{r['scenario_version']}\t{r['sense_version']}")
        return 0 if rows else 1

    if args.cmd == "gate":
        res = gate(_vdir(args), args.repo, args.headline)
        print(json.dumps(res, indent=2, sort_keys=True))
        return 0 if res["ok"] else 1

    out = assemble(_vdir(args), args.repo, args.headline,
                   args.arms.split(), args.scenario)
    print(json.dumps(out, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
