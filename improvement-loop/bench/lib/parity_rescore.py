#!/usr/bin/env python3
"""parity_rescore.py - what had an arm assembled at the OTHER arm's cost?

    parity_rescore.py <run_dir> <scenario.yaml> --seconds 276
    parity_rescore.py <run_dir> <scenario.yaml> --like <other_run_dir> [--mult 1.0]

WHY THIS EXISTS. The bench enforces a wall, and under matched budget the baseline's wall
is derived from its paired sense run. But every cell measured before that - and every
banked cell - ran both arms at one generous ceiling, so "who reaches more at equal cost"
was never answered for them. This answers it after the fact, from transcripts already on
disk, with no re-run: truncate an arm's session at a budget and score what it had by then.

WHY TIME AND NOT TOKENS. Tokens are the truer cost axis and the first choice, but they are
not reconstructible here: stream-json emits partial per-message usage that does not
reconcile with the session total (measured: 13 assistant messages summing to 80 output
tokens against a result total of 17,777). Wall time IS per-row (`timestamp` on assistant
and user rows), and it is the axis the harness actually enforces, so the truncation and
the constraint agree. Output tokens dominate billed here (38,411 of 38,454 on one measured
baseline), and they accrue with wall time, so time is a faithful stand-in.

WHAT IT DOES NOT DO. It never touches the source run. It reports gold recall only: the
`result` row is carried through unchanged, so the efficiency and token figures in the
truncated scored.json still describe the FULL session and must not be read. Both the full
and truncated copies are scored by the same invocation, so the comparison is like-for-like
even where an optional check (citation grounding, which needs a repo checkout) is skipped.
"""
import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
from datetime import datetime

LIB = os.path.dirname(os.path.abspath(__file__))


def _parse_ts(value):
    """ISO-8601 from the CLI transcript; tolerate the trailing Z form."""
    if not value:
        return None
    try:
        return datetime.fromisoformat(str(value).replace("Z", "+00:00"))
    except ValueError:
        return None


def load_rows(run_dir):
    path = os.path.join(run_dir, "transcript.json")
    with open(path) as fh:
        return [json.loads(line) for line in fh if line.strip()]


def wall_of(run_dir):
    """The recorded wall of a run, from its own run_meta.json."""
    with open(os.path.join(run_dir, "run_meta.json")) as fh:
        return int(json.load(fh)["wall_time_seconds"])


def truncate(rows, seconds):
    """Rows the session had produced within `seconds` of its first timestamped row.

    Rows with no timestamp (the leading system/init block) are always kept: they carry
    the session's setup, not its work, and dropping them makes the transcript unreadable
    to the scorer. The `result` row is kept for the same reason - see the module docstring
    for why its metrics must be ignored.
    """
    first = None
    for row in rows:
        ts = _parse_ts(row.get("timestamp"))
        if ts is not None:
            first = ts
            break
    if first is None:
        return list(rows), 0

    kept, dropped = [], 0
    for row in rows:
        ts = _parse_ts(row.get("timestamp"))
        if ts is None or (ts - first).total_seconds() <= seconds:
            kept.append(row)
        else:
            dropped += 1
    return kept, dropped


def score(run_dir, scenario, bench_dir):
    subprocess.run(
        [sys.executable, os.path.join(LIB, "scorer.py"), run_dir, scenario, bench_dir],
        capture_output=True, text=True, check=False,
    )
    with open(os.path.join(run_dir, "scored.json")) as fh:
        return json.load(fh)["gold_recall"]


def summarise(tag, recall):
    groups = " ".join(
        f"{name} {g['cited']}/{g['total']}"
        for name, g in sorted(recall.get("groups", {}).items())
    )
    return f"{tag:9s} cited {recall['cited']}/{recall['total']} = {recall['cited_recall']:.3f}   {groups}"


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("run_dir")
    ap.add_argument("scenario")
    ap.add_argument("--seconds", type=float, help="budget in seconds")
    ap.add_argument("--like", help="another run dir; use ITS wall as the budget")
    ap.add_argument("--mult", type=float, default=1.0, help="multiplier on --like (default 1.0)")
    ap.add_argument("--bench-dir", default="bench")
    args = ap.parse_args()

    if not args.seconds and not args.like:
        ap.error("give --seconds or --like")
    budget = args.seconds if args.seconds else wall_of(args.like) * args.mult

    rows = load_rows(args.run_dir)
    kept, dropped = truncate(rows, budget)
    actual = wall_of(args.run_dir)

    print(f"# parity rescore - {args.run_dir}")
    print(f"  ran {actual}s; budget {budget:.0f}s; dropped {dropped} of {len(rows)} rows\n")

    if dropped == 0:
        print("  the run finished inside the budget - nothing to truncate, recall is unchanged.")

    tmp = tempfile.mkdtemp(prefix="parity-")
    try:
        full = os.path.join(tmp, "full")
        cut = os.path.join(tmp, "cut")
        shutil.copytree(args.run_dir, full)
        shutil.copytree(args.run_dir, cut)
        with open(os.path.join(cut, "transcript.json"), "w") as fh:
            for row in kept:
                fh.write(json.dumps(row) + "\n")
        before = score(full, args.scenario, args.bench_dir)
        after = score(cut, args.scenario, args.bench_dir)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print(summarise("FULL", before))
    print(summarise("AT BUDGET", after))
    delta = after["cited_recall"] - before["cited_recall"]
    print(f"\n  cost of the shorter budget: {delta:+.3f} cited recall")
    return 0


if __name__ == "__main__":
    sys.exit(main())
