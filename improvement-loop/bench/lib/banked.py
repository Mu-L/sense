#!/usr/bin/env python3
"""banked.py - the durable index of resolved cells for one vertical.

WHY THIS EXISTS. Every decision-bearing number in this loop is print-only:
`pergroup.py`, `pay_ceiling.py`, `cost_parity.py` and `credit_table.py` write no
files at all. The discourse WIN (+0.54 overall, dependents +0.71) therefore lived
only in a terminal buffer, and the per-root `report.json` that IS written keeps
`avg_cited_recall` with no per-group breakdown - so the one axis every verdict
turns on is absent from the stored summary. A verdict nobody can re-read is a
verdict the next session has to re-derive or take on trust.

WHAT IT IS NOT. It is not a source of truth: `scored.json` is, and this index is
re-derivable from it at any time (`--rebuild`). It is not a second opinion either
- the verdict is passed IN by the phase that made it (`--verdict`), and only a
rebuild of history has to re-apply the threshold, which it marks as such in
`verdict_source`. Two derivations of one fact are how a harness starts disagreeing
with itself.

WHERE IT LIVES. `verticals/<key>/banked.jsonl`, beside LEDGER.md and STATUS.md -
the human-and-loop-facing layer, not the run tree. (Checked: both `reporter.py`
and `matrix.py` skip non-directories, so a file under results/ would not break
them - this is about where a reader looks, not about breakage.)

  RESULTS_DIR=<paid root> banked.py record <repo> --out <file> [--verdict WIN]
  banked.py rebuild <vertical-dir>              # re-derive every cell from disk

Rows are keyed by (repo, model, scenario_version): recording the same cell twice
replaces its row rather than appending a second one.
"""
import argparse
import glob
import json
import os
import re
import sys

import run_validity

THRESHOLD = 0.50
# A scenario-version segment: 16 hex chars, the one-root-per-question directory.
VERSION_DIR = re.compile(r"^[0-9a-f]{16}$")


def _runs(root, arm, repo):
    """This arm's MEASUREMENT runs, via the one shared gate.

    This used to carry its own copy of the glob and lean on scored.json's
    `failed` alone, which made it the fourth instrument answering "which runs
    count" its own way - the exact split run_validity.measured_runs exists to
    end. The two agree on today's cells; a run whose crash the scorer did not
    flag is where they would not.
    """
    return run_validity.measured_runs(os.path.join(root, arm, repo))


def collect(root, arm, repo):
    """group -> [(cited, total), ...] per surviving run, plus per-run overalls.

    Mirrors pergroup.collect, including its one judgement call: a run that did not
    MEASURE the arm is skipped, never blended in as a real 0.0 (that manufactured a
    false loss once already). test_banked.py pins the two against each other.

    A run the wall clock cut short is NOT one of those: a failed exam is still an
    exam, so `truncated_at_ceiling` and `never_reached_synthesis` keep their real
    0.0 (run_validity's standing rule). Only harness artifacts drop out. The
    `failed` check below is scored.json's own stamp, kept as a second line behind
    _runs rather than as the only one.
    """
    by_group, overall, tokens = {}, [], []
    for path in _runs(root, arm, repo):
        with open(path) as fh:
            data = json.load(fh)
        if data.get("failed"):
            continue
        gold = data.get("gold_recall", {}) or {}
        overall.append(gold.get("cited_recall", 0.0))
        for name, g in (gold.get("groups", {}) or {}).items():
            by_group.setdefault(name, []).append([g.get("cited", 0), g.get("total", 0)])
        billed = (data.get("metrics", {}) or {}).get("token_total_billed")
        if isinstance(billed, (int, float)):
            tokens.append(billed)
    return by_group, overall, tokens


def _mean(pairs):
    """Summed cited over summed total, which is how pergroup reads a group."""
    cited = sum(c for c, _ in pairs)
    total = sum(t for _, t in pairs)
    return (cited / total) if total else 0.0


def cell(root, repo, threshold=THRESHOLD):
    """The facts for one cell, or None when either arm has no surviving run."""
    bg, bo, btok = collect(root, "baseline", repo)
    sg, so, stok = collect(root, "sense", repo)
    if not bo or not so:
        return None
    groups = {}
    for name in sorted(set(bg) | set(sg)):
        bpairs, spairs = bg.get(name, []), sg.get(name, [])
        bmean, smean = _mean(bpairs), _mean(spairs)
        groups[name] = {
            "baseline": bpairs, "sense": spairs,
            "baseline_mean": round(bmean, 4), "sense_mean": round(smean, 4),
            "delta": round(smean - bmean, 4),
        }
    bmean, smean = sum(bo) / len(bo), sum(so) / len(so)
    return {
        "runs": {"baseline": len(bo), "sense": len(so)},
        "overall": {
            "baseline": [round(x, 4) for x in bo], "sense": [round(x, 4) for x in so],
            "baseline_mean": round(bmean, 4), "sense_mean": round(smean, 4),
            "delta": round(smean - bmean, 4),
        },
        "groups": groups,
        # Raw per-run billed tokens, NOT a ratio: cost_parity.py owns the parity
        # call and this index must not become a rival answer to it.
        "billed_tokens": {"baseline": btok, "sense": stok},
        "threshold": threshold,
        "best_group_delta": round(max((g["delta"] for g in groups.values()), default=0.0), 4),
    }


def verdict_for(row, threshold=THRESHOLD):
    """The threshold rule, applied ONLY when rebuilding history."""
    return "WIN" if row["best_group_delta"] >= threshold else "NOT-YET"


def _provenance(root, repo):
    """Model, scenario version and sense build, read off a run's own run_meta.

    `sense_build_key` is carried alongside the version LABEL because the label is not the
    build: every dirty working-tree binary between two releases reports the same
    `sense 1.13.5 (...)` string, so a board gated on the label alone can compare two
    different products and say they match. The runners have stamped the key since
    lib/sense_build.py landed; nothing read it back until the cycle-2 gate did.
    `sense_dirty` rides along for the same reason - a win banked off an uncommitted tree
    is reproducible only by the person who has that tree."""
    out = {"model": None, "scenario_version": None, "sense_version": None,
           "sense_build_key": None, "sense_dirty": None}
    metas = sorted(glob.glob(os.path.join(root, "*", repo, "run-*", "run_meta.json")))
    for path in metas:
        try:
            with open(path) as fh:
                meta = json.load(fh)
        except (OSError, ValueError):
            continue
        out["model"] = out["model"] or meta.get("model")
        out["scenario_version"] = out["scenario_version"] or meta.get("scenario_version")
        out["sense_version"] = out["sense_version"] or meta.get("tool_version") or meta.get("sense_version")
        out["sense_build_key"] = out["sense_build_key"] or meta.get("sense_build_key")
        if out["sense_dirty"] is None:
            out["sense_dirty"] = meta.get("sense_dirty")
        # `sense_dirty` is legitimately False, so `all()` would keep scanning for it.
        if all(out[k] is not None for k in
               ("model", "scenario_version", "sense_version", "sense_build_key")):
            break
    # The directory is authoritative for the version when run_meta predates it.
    tail = os.path.basename(os.path.normpath(root))
    if not out["scenario_version"] and VERSION_DIR.match(tail):
        out["scenario_version"] = "sha256:" + tail
    return out


def upsert(path, row):
    """Replace the row for this (repo, model, scenario_version), else append."""
    key = (row.get("repo"), row.get("model"), row.get("scenario_version"))
    rows = []
    if os.path.exists(path):
        with open(path) as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    old = json.loads(line)
                except ValueError:
                    continue
                if (old.get("repo"), old.get("model"), old.get("scenario_version")) != key:
                    rows.append(old)
    rows.append(row)
    os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
    with open(path, "w") as fh:
        for r in rows:
            fh.write(json.dumps(r, sort_keys=True) + "\n")
    return rows


def build_row(root, repo, verdict=None, recorded_at=None, threshold=THRESHOLD):
    row = cell(root, repo, threshold)
    if row is None:
        return None
    row["repo"] = repo
    row.update(_provenance(root, repo))
    row["root"] = root
    if verdict:
        row["verdict"], row["verdict_source"] = verdict, "driver"
    else:
        row["verdict"], row["verdict_source"] = verdict_for(row, threshold), "rebuilt"
    row["recorded_at"] = recorded_at
    return row


def paid_roots(vertical_dir):
    """Every one-root-per-question PAID root under a vertical.

    Skips `validation/` and `minibench/` (unscored by construction), and the
    doubled `validation/<v>/validation/<v>` and `minibench/<v>/minibench/<v>`
    directories the pre-2026-08-04 path bug left on disk.
    """
    results = os.path.join(vertical_dir, "results")
    for model in sorted(glob.glob(os.path.join(results, "*"))):
        if not os.path.isdir(model):
            continue
        for root in sorted(glob.glob(os.path.join(model, "*"))):
            if not os.path.isdir(root):
                continue
            if not VERSION_DIR.match(os.path.basename(root)):
                continue
            yield root


def cmd_record(args):
    root = os.environ.get("RESULTS_DIR")
    if not root:
        sys.exit("banked.py record: RESULTS_DIR must point at the paid root")
    row = build_row(root, args.repo, args.verdict, args.at)
    if row is None:
        sys.exit(f"banked.py: no scored pair for {args.repo} under {root} - nothing to bank")
    upsert(args.out, row)
    print(f"banked: {args.repo} {row['verdict']} overall {row['overall']['delta']:+.2f} "
          f"best group {row['best_group_delta']:+.2f} -> {args.out}")
    return 0


def cmd_rebuild(args):
    out = args.out or os.path.join(args.vertical_dir, "banked.jsonl")
    written = 0
    for root in paid_roots(args.vertical_dir):
        for arm_dir in sorted(glob.glob(os.path.join(root, "baseline", "*"))):
            if not os.path.isdir(arm_dir):
                continue
            repo = os.path.basename(arm_dir)
            row = build_row(root, repo, None, args.at)
            if row is None:
                continue
            upsert(out, row)
            written += 1
            print(f"  {repo:12} {row['verdict']:8} overall {row['overall']['delta']:+.2f} "
                  f"best group {row['best_group_delta']:+.2f}  {os.path.basename(root)}")
    print(f"rebuilt {written} cell(s) -> {out}")
    return 0


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = ap.add_subparsers(dest="cmd", required=True)
    rec = sub.add_parser("record", help="bank one cell (RESULTS_DIR = its paid root)")
    rec.add_argument("repo")
    rec.add_argument("--out", required=True)
    rec.add_argument("--verdict", help="the verdict the phase already made")
    rec.add_argument("--at", help="timestamp to stamp the row with")
    rec.set_defaults(func=cmd_record)
    reb = sub.add_parser("rebuild", help="re-derive every paid cell from disk")
    reb.add_argument("vertical_dir")
    reb.add_argument("--out")
    reb.add_argument("--at")
    reb.set_defaults(func=cmd_rebuild)
    args = ap.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
