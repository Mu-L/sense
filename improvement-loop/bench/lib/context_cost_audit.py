#!/usr/bin/env python3
"""context_cost_audit.py -- WHY does the sense arm cost more, and what can be trimmed?

    RESULTS_DIR=<results/<model>> python3 context_cost_audit.py <repo>

Reads `sense-io.jsonl` (every MCP request/response, captured by mcp_tee.py) plus
both arms' `scored.json`, and answers the question a COST_PARITY miss raises:
Sense put bytes into the agent's context - which tool put them there, how many,
and what did that actually cost once the context was re-read on every later turn?

## WHY THIS EXISTS

This is the budget-trim audit. `03-repo-diagnosis.md` recorded it as *"Missing,
deferred by decision 2026-07-30: stays a hand check"*, and named the cost itself:
*"the check most able to falsify 'send the RIGHT info' remains the easiest one to
skip."* It was skipped. On 2026-08-01 the rails cell won at a 26% priced-token
premium and the loop had no instrument to ask why, so the finding went to the
stopper lane and Loop 5 never got it.

## THE MULTIPLIER, WHICH IS THE WHOLE POINT

A tool response is injected ONCE and then re-read as cached input on EVERY
subsequent turn. So the cost of a fat response is not its size, it is its size
times the turns that follow it. That is why the arms' OUTPUT tokens are near
parity while cache-read diverges: the premium is context carried, not answer
produced. Trimming a response early in the session is worth a multiple of the
bytes removed - which is what makes trimming a product lever rather than
cosmetics.

Token estimates are chars/4 and are approximate BY DESIGN: this ranks trim
candidates, it does not price a cell. `cost_parity.py` prices the cell.
"""
import collections
import glob
import json
import os
import sys

CHARS_PER_TOKEN = 4


def parse_io(path):
    """(tool name, response chars, turn index) per MCP tool call, in order."""
    names, sizes = {}, {}
    with open(path) as fh:
        lines = fh.readlines()
    for line in lines:
        line = line.strip()
        if not line:
            continue
        try:
            rec = json.loads(line)
        except json.JSONDecodeError:
            continue
        msg = rec.get("msg") or {}
        if rec.get("dir") == "c2s" and msg.get("method") == "tools/call":
            names[msg.get("id")] = (msg.get("params") or {}).get("name", "?")
        elif rec.get("dir") == "s2c" and "result" in msg:
            chars = sum(len(b.get("text", "") or "")
                        for b in (msg["result"].get("content") or []))
            sizes[msg.get("id")] = chars
    return [(names[i], sizes.get(i, 0), n)
            for n, i in enumerate(sorted(k for k in names))]


def metric_mean(results_dir, arm, repo, key):
    vals = []
    for p in sorted(glob.glob(os.path.join(results_dir, arm, repo, "run-*", "scored.json"))):
        with open(p) as fh:
            m = (json.load(fh) or {}).get("metrics") or {}
        if m.get(key) is not None:
            vals.append(m[key])
    return sum(vals) / len(vals) if vals else 0


def report(results_dir, repo):
    io_files = sorted(glob.glob(os.path.join(results_dir, "sense", repo, "run-*", "sense-io.jsonl")))
    if not io_files:
        raise SystemExit(f"context_cost_audit: no sense-io.jsonl under {results_dir}/sense/{repo}")

    per_tool = collections.defaultdict(lambda: {"calls": 0, "chars": 0, "max": 0})
    biggest, injected_total = [], 0
    for f in io_files:
        calls = parse_io(f)
        for name, chars, turn in calls:
            t = per_tool[name]
            t["calls"] += 1
            t["chars"] += chars
            t["max"] = max(t["max"], chars)
            injected_total += chars
            # Turns that follow this call re-read it as cached input.
            biggest.append((chars, name, os.path.basename(os.path.dirname(f)),
                            max(0, len(calls) - 1 - turn)))
    runs = len(io_files)
    injected_mean = injected_total / runs

    print(f"## context cost audit - {repo}  ({runs} sense run(s))")
    print()
    print("### what Sense injected, by tool")
    print(f"{'tool':<22}{'calls':>7}{'~tokens':>10}{'share':>8}{'largest call':>14}")
    total_tok = injected_total / CHARS_PER_TOKEN
    for name, t in sorted(per_tool.items(), key=lambda kv: -kv[1]["chars"]):
        tok = t["chars"] / CHARS_PER_TOKEN
        print(f"{name:<22}{t['calls']:>7}{tok:>10,.0f}{tok / total_tok:>7.0%}"
              f"{t['max'] / CHARS_PER_TOKEN:>14,.0f}")
    print(f"{'TOTAL':<22}{sum(t['calls'] for t in per_tool.values()):>7}{total_tok:>10,.0f}{'100%':>8}")
    print()

    cr_b = metric_mean(results_dir, "baseline", repo, "token_cache_read")
    cr_s = metric_mean(results_dir, "sense", repo, "token_cache_read")
    delta = cr_s - cr_b
    inj_tok = injected_mean / CHARS_PER_TOKEN
    print("### injection vs what it actually cost")
    print(f"   injected per run (est)      {inj_tok:>12,.0f} tokens")
    print(f"   cache-read delta vs baseline{delta:>12,.0f} tokens")
    if inj_tok > 0 and delta > 0:
        print(f"   RE-READ MULTIPLIER          {delta / inj_tok:>12.1f}x")
        print("   Every token injected was paid for ~this many times, because the")
        print("   context is re-read on each later turn. Trimming N tokens from an")
        print("   early response saves roughly N x this.")
    print()

    print("### trim candidates (largest single responses, and turns left to re-read)")
    print(f"{'~tokens':>10}{'re-reads':>10}  {'tool':<20}{'run'}")
    for chars, name, run, after in sorted(biggest, reverse=True)[:8]:
        print(f"{chars / CHARS_PER_TOKEN:>10,.0f}{after:>10}  {name:<20}{run}")
    print()
    print("Hand the biggest rows to Loop 5: for each, is every field in that")
    print("response load-bearing for the answer the agent gave? Fields returned")
    print("and never cited are the trim, and they cost their size x the re-reads.")
    return 0


def main(argv):
    if len(argv) < 2:
        raise SystemExit("usage: context_cost_audit.py <repo>  (RESULTS_DIR must be set)")
    results_dir = os.environ.get("RESULTS_DIR")
    if not results_dir:
        raise SystemExit("context_cost_audit: RESULTS_DIR must be set")
    return report(results_dir, argv[1])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
