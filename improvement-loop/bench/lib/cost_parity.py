#!/usr/bin/env python3
"""cost_parity.py -- did the sense arm reach at HELD cost, or at a premium?

    RESULTS_DIR=<results/<model>> python3 cost_parity.py <repo> [threshold]

Prints a machine-greppable verdict line and always exits 0 - this is a ROUTING
signal, not a gate:

    COST_PARITY: PASS ratio=1.03 baseline=177757 sense=183090
    COST_PARITY: MISS ratio=1.26 baseline=177757 sense=223525

## WHY THIS EXISTS

`help-the-ai.md` defines the win as reach "at held-or-better billed-token cost".
Reach had an arbiter (`pergroup.py`); cost had none, so a cell could win the
headline while quietly costing 30% more and nothing in the loop would say so.

Worse, the miss had no lane. On 2026-08-01 the rails cell won its discriminator
(+0.56) at 1.30x cost, and the session read that as a MEASUREMENT question ("which
token axis is honest?"), which routes to the STOPPER lane and halts everything
downstream - so Loop 5 harvest never ran and the product question ("why is Sense's
context 27% bigger, and can we trim it?") was never asked. Same observation, two
lanes, and the framing picked the one that stops rather than the one that learns.

**Costing more is a product finding, not a defect in the bench.** This script
makes the routing mechanical so it no longer depends on how a session frames it:
a MISS on a winning cell goes to harvest with `context_cost_audit.py`.

## THE AXIS

`token_total_priced` - every BILLED token in input-token equivalents (scorer.py
`priced_tokens`). NOT `token_total_billed`, which is the uncached remainder and
misses ~97% of what is paid for.
"""
import glob
import json
import os
import statistics
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from scorer import priced_tokens  # noqa: E402

# Tolerance on "held-or-better". The definition says held, i.e. 1.00; a few
# percent is run-to-run noise, not a premium. 10% is wide enough to absorb that
# and far below the 26-30% the rails cell showed, so a real premium cannot hide
# under it.
DEFAULT_THRESHOLD = 1.10


def run_priced(path):
    """Priced tokens for one scored run, recomputing for cells scored before
    `token_total_priced` existed so old results stay comparable."""
    with open(path) as fh:
        m = (json.load(fh) or {}).get("metrics") or {}
    if m.get("token_total_priced"):
        return m["token_total_priced"]
    return priced_tokens({
        "input_tokens": m.get("token_input_uncached", 0),
        "output_tokens": m.get("token_output", 0),
        "cache_read_input_tokens": m.get("token_cache_read", 0),
        "cache_creation_input_tokens": m.get("token_cache_write", 0),
    })


def arm_mean(results_dir, arm, repo):
    runs = sorted(glob.glob(os.path.join(results_dir, arm, repo, "run-*", "scored.json")))
    vals = [run_priced(p) for p in runs]
    vals = [v for v in vals if v]
    return statistics.mean(vals) if vals else None


def verdict(baseline, sense, threshold):
    if not baseline or not sense:
        return None, "COST_PARITY: SKIP (no scored runs on both arms)"
    ratio = sense / baseline
    tag = "PASS" if ratio <= threshold else "MISS"
    return ratio, (f"COST_PARITY: {tag} ratio={ratio:.2f} "
                   f"baseline={baseline:.0f} sense={sense:.0f}")


def main(argv):
    if len(argv) < 2:
        raise SystemExit("usage: cost_parity.py <repo> [threshold]  (RESULTS_DIR must be set)")
    repo = argv[1]
    threshold = float(argv[2]) if len(argv) > 2 else DEFAULT_THRESHOLD
    results_dir = os.environ.get("RESULTS_DIR")
    if not results_dir:
        raise SystemExit("cost_parity: RESULTS_DIR must be set")
    ratio, line = verdict(arm_mean(results_dir, "baseline", repo),
                          arm_mean(results_dir, "sense", repo), threshold)
    print(line)
    if ratio and ratio > threshold:
        print(f"   Sense reached at a {(ratio - 1) * 100:.0f}% premium, not at parity.")
        print("   This is a PRODUCT finding for Loop 5 harvest, NOT a stopper:")
        print(f"     RESULTS_DIR={results_dir} python3 bench/lib/context_cost_audit.py {repo}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
