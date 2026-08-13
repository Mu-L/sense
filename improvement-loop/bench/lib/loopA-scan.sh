#!/usr/bin/env bash
# loopA-scan.sh - the Loop-A (PRODUCT-gap detection) pass over a vertical.
#
# Loop A ≠ Loop B. The bench (runs-variance.sh + pergroup/scorer/judge) is the
# value-PROOF instrument (Loop B). This is the gap-DETECTOR (Loop A): it is
# ADVISORY ONLY - it writes a report and NEVER gates a bench, blocks a sweep, or
# touches a score. Two free passes, no LLM, no new bench tokens:
#
#   preflight  resolve_oracle.py - live `sense` CLI vs the gold discriminator.
#              Run it at SCOUT time (after the index builds, while authoring gold):
#              catches ambiguous symbols needing --file, budget-cap eviction, and
#              gold that blast can't retrieve (manifesto §8.3/§10 "gold must be
#              default-blast-retrievable") - BEFORE you pay for a sweep.
#   harvest    transcript_miss.py - mines the ×N transcripts a sweep already paid
#              for (cited-not-returned / fallback-reads / empty returns). Run it
#              AFTER a sweep to bank the Loop-A signal for free.
#
# Output is appended (never overwritten) to verticals/<stack>/results/loopA-gaps.md
# so the product-gap log accumulates across that vertical's repos. Loop 7 reads
# the set with `verticals/*/results/loopA-gaps.md`, which still finds an archived
# vertical. It is NOT model-scoped: a resolution gap is a fact about the product,
# so it never sits under a BENCH_MODEL root. A FAIL in the oracle is
# a fix CANDIDATE for the NEXT vertical's pre-bench window, not a reason to stop -
# fixing the product mid-vertical invalidates that vertical's frozen numbers.
#
# Usage:
#   bash bench/lib/loopA-scan.sh preflight <stack> [repo]
#   bash bench/lib/loopA-scan.sh harvest   <stack> [model]
#   bash bench/lib/loopA-scan.sh both      <stack>           # preflight + harvest
set -uo pipefail

BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
LIB="$BENCH_DIR/lib"
MODE="${1:?usage: loopA-scan.sh <preflight|harvest|both> <stack> [repo|model]}"
STACK="${2:?usage: loopA-scan.sh <mode> <stack> [arg]}"
ARG="${3:-}"

# The stack IS the vertical key, so the log lives in that vertical's own home
# (bench-paths.sh: verticals/<name>/results/). Writing to $BENCH_DIR/results put
# a per-vertical file in the GLOBAL bench root, which is the competitor bench.
LOGDIR="$IL_ROOT/verticals/$STACK/results"
[ -d "$IL_ROOT/verticals/$STACK" ] || {
  echo "loopA-scan: no such vertical: verticals/$STACK" >&2; exit 66; }
mkdir -p "$LOGDIR"
LOG="$LOGDIR/loopA-gaps.md"
STAMP="$(date -u +%Y-%m-%dT%H:%MZ)"

append() { tee -a "$LOG"; }

run_preflight() {
  local repo_flag=()
  [ -n "$ARG" ] && repo_flag=(--repo "$ARG")
  {
    echo ""
    echo "## [$STAMP] PREFLIGHT resolve_oracle - stack=$STACK ${ARG:+repo=$ARG}"
    echo '```'
  } | append
  # advisory: capture exit but do NOT propagate as a wrapper failure.
  # ${arr[@]+...} guards the empty-array case under `set -u` on bash 3.2 (macOS).
  python3 "$LIB/resolve_oracle.py" --stack "$STACK" ${repo_flag[@]+"${repo_flag[@]}"} 2>&1 | append
  echo '```' | append
}

run_harvest() {
  local model_flag=()
  [ -n "$ARG" ] && model_flag=(--model "$ARG")
  {
    echo ""
    echo "## [$STAMP] HARVEST transcript_miss - stack=$STACK ${ARG:+model=$ARG}"
    echo '```'
  } | append
  python3 "$LIB/transcript_miss.py" --stack "$STACK" ${model_flag[@]+"${model_flag[@]}"} 2>&1 | append
  echo '```' | append
}

case "$MODE" in
  preflight) run_preflight ;;
  harvest)   run_harvest ;;
  both)      run_preflight; run_harvest ;;
  *) echo "unknown mode: $MODE (preflight|harvest|both)" >&2; exit 64 ;;
esac

echo ""
echo "Loop-A scan appended to $LOG (advisory - no bench was gated)."
