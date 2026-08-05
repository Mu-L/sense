#!/usr/bin/env bash
# bench-paths.sh - resolve RESULTS_DIR and SCENARIOS_DIR for the active bench.
#
# A "bench" is either the GLOBAL bench (baseline + competitors vs sense, across
# all language/framework repos) or a VERTICAL bench (baseline vs sense only, one
# language/framework's repos). Select a vertical with VERTICAL=<name> (e.g.
# ruby-rails); empty/unset means the global bench.
#
# A vertical bench is also MODEL-SCOPED: set BENCH_MODEL=<id> and each model gets
# its own bench root, so sweeping several models (opus-4-8, gpt-5.6, an
# ollama-cloud model, ...) never overwrites another's results. The id is
# sanitized (/ and : -> _) to a safe dir name. The
# global bench is deliberately single-model: BENCH_MODEL is ignored there.
#
#   GLOBAL :           bench/results/                        bench/scenarios/
#   VERTICAL:          verticals/<name>/results/             verticals/<name>/scenarios/
#   VERTICAL + model:  verticals/<name>/results/<model>/     verticals/<name>/scenarios/
#
# Each vertical is ONE self-contained home at the improvement-loop root:
# verticals/<name>/ holds repos.txt, scenarios/, results/, AND the vertical's own
# docs (README, repos.md, LEDGER, STATUS, articles/) together. The global bench keeps the legacy top-level
# results/ + scenarios/ roots (baseline + competitors vs sense).
#
# Source this AFTER setting BENCH_DIR (and optionally VERTICAL / BENCH_MODEL). It
# exports RESULTS_DIR and SCENARIOS_DIR so child scripts (run/score/judge/report)
# inherit the right roots. A pre-set RESULTS_DIR/SCENARIOS_DIR always wins, so an
# explicit override (or a parent that already resolved them) is never clobbered;
# multi-model loops `unset RESULTS_DIR` before re-sourcing to re-derive per model.
: "${BENCH_DIR:?bench-paths.sh: BENCH_DIR must be set before sourcing}"
_il_root="$(cd "$BENCH_DIR/.." && pwd)"   # verticals/ is a sibling of bench/
if [ -n "${VERTICAL:-}" ]; then
  _results_base="$_il_root/verticals/$VERTICAL/results"
  SCENARIOS_DIR="${SCENARIOS_DIR:-$_il_root/verticals/$VERTICAL/scenarios}"
else
  _results_base="$BENCH_DIR/results"
  SCENARIOS_DIR="${SCENARIOS_DIR:-$BENCH_DIR/scenarios}"
fi
if [ -n "${VERTICAL:-}" ] && [ -n "${BENCH_MODEL:-}" ]; then
  _msan="$(printf '%s' "$BENCH_MODEL" | tr '/:' '__')"
  RESULTS_DIR="${RESULTS_DIR:-$_results_base/$_msan}"
else
  RESULTS_DIR="${RESULTS_DIR:-$_results_base}"
fi
# BENCH_VALIDATION=1 -> the unscored validation run (plans/cycle-1-craft-the-scenario/04-validate.md). It is a
# measurement of whether the scenario is the right scenario, and its number may never
# settle anything. The isolation is the RESULTS ROOT, not a flag the scorer has to
# honour: pergroup.py/scorer.py walk RESULTS_DIR, so a validation run is invisible to
# them by construction and no measurement instrument had to change to make it so.
# BENCH_SCORING rides along for run_meta.json, so a file copied out of the tree still
# says what it is.
# BENCH_MINIBENCH=1 -> the two-step probe run (plans/cycle-1-craft-the-scenario/02-minibench.md), unscored like a
# validation run and isolated from it by its own root. It cannot share the validation
# tree: both write <root>/<arm>/<repo>/run-1, so the seven-step pair would overwrite the
# two-step pair at the same path and plans/cycle-1-craft-the-scenario/04-validate.md's session row would compare a
# run against itself. It takes precedence over BENCH_VALIDATION when both are set.
if [ "${BENCH_MINIBENCH:-0}" = 1 ]; then
  case "$RESULTS_DIR" in
    */minibench|*/minibench/*) : ;;
    *) RESULTS_DIR="$RESULTS_DIR/minibench" ;;
  esac
  BENCH_SCORING=0
elif [ "${BENCH_VALIDATION:-0}" = 1 ]; then
  # IDEMPOTENT: RESULTS_DIR is exported and this file is sourced once per driver in the
  # chain (vertical-loop -> runs-variance -> bench-sense-local -> reporter), so an
  # unconditional append nested it once per hop - runs landed in .../validation/validation
  # while the reporter looked in .../validation/validation/validation and died. The run
  # itself was fine; only the paths disagreed.
  #
  # The guard matches the SEGMENT, not the tail: one-root-per-question appends the
  # scenario version below this segment, so a tail-only test stopped seeing the
  # `validation` it had already added and nested a fresh copy per hop.
  case "$RESULTS_DIR" in
    */validation|*/validation/*) : ;;
    *) RESULTS_DIR="$RESULTS_DIR/validation" ;;
  esac
  BENCH_SCORING=0
else
  BENCH_SCORING=1
fi
# BENCH_SCENARIO_VERSION=<sha256:...> -> ONE ROOT PER QUESTION.
#
# Same principle as the validation/minibench split above, applied to the axis that broke
# next. A results root used to hold exactly one question because the authoring cycle wiped
# it between cycles; once runs began to ACCUMULATE (KEEP_RUNS), a root could hold two
# questions with different gold, and every reader that walks <root>/<arm>/<repo>/run-* was
# silently averaging them. Measured: pergroup.py pooled five runs of the previous scenario
# into a new cell and printed the OPPOSITE verdict (dependents +0.17 / NOT YET against the
# +0.70 the pair actually showed).
#
# Fifteen readers walk that path and one filters on scenario_version. Rather than teach
# fifteen instruments - six of them verdict-bearing, so six re-score diffs - the isolation
# is the DIRECTORY, exactly as it is for validation runs. Nothing downstream changes and
# every reader written later is correct for free.
#
# The segment sits ABOVE <arm> deliberately. A reader aimed at the parent finds no
# <arm>/<repo>/run-* under it and says "no scored runs" instead of returning a number
# averaged across two questions: this fails LOUDLY, where a per-reader filter fails quietly.
# Idempotent for the same reason the validation append is - this file is sourced once per
# driver hop.
if [ -n "${BENCH_SCENARIO_VERSION:-}" ]; then
  _sv="${BENCH_SCENARIO_VERSION#sha256:}"
  # Strip an existing version segment before appending, so a carried-over RESULTS_DIR at a
  # DIFFERENT version re-points instead of nesting one question's root inside another's.
  case "$RESULTS_DIR" in
    */[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) RESULTS_DIR="${RESULTS_DIR%/*}" ;;
  esac
  RESULTS_DIR="$RESULTS_DIR/$_sv"
fi
export RESULTS_DIR SCENARIOS_DIR VERTICAL BENCH_MODEL BENCH_SCORING BENCH_SCENARIO_VERSION
