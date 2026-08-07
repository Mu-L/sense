#!/usr/bin/env bash
# cycle2-board.sh - put a proved question to the other models and publish the page.
#
# THE SCRIPT OWNS THE ORDER, THE STATE, THE GATES AND EVERY SPAWN. It never judges.
# Cycle 2 starts where cycle 1 stops: a banked WIN on the headline arm. It re-runs
# nothing of that win, it never re-authors the question, and it publishes what it
# measures. Cycle 1 cannot record a loss; this one can, and must.
#
# Most of this cycle is deterministic and needs no agent. The eligibility query, the
# build gate, the mechanism table, the numbers and the page itself are all same-input
# same-bytes. Two judgments are left, and each gets a plan:
#
#   validity  did these runs MEASURE anything (plans/.../01-bench.md)
#   read      what does the result mean to a reader (plans/.../02-report.md)
#
# Phases (a state file resumes at the next one; re-run to advance):
#   gate       eligibility + same-Sense-build check                    [auto]
#   bench      runs-variance.sh, confirmation arms x2 (PAID, paced)    [auto]
#   validity   AGENT 01-bench.md            [BOARD|RERUN]
#   assemble   board.py assemble -> numbers.json                       [auto]
#   render     render_board.py -> a staged page in the loop dir          [auto]
#   read       AGENT 02-report.md           [READING]
#   publish    board_check.py, then the page is copied into reports/    [gate]
#
# A RERUN verdict re-enters `bench` for the named arms ONLY, up to RERUN_MAX times.
# An arm that never called Sense is never re-run to get a better number: that is the
# result, and 01-bench.md is told so.
#
# Usage:
#   VERTICAL=ruby-rails bash bench/drivers/cycle2-board.sh <repo>
#   VERTICAL=ruby-rails bash bench/drivers/cycle2-board.sh --eligible
#   VERTICAL=ruby-rails bash bench/drivers/cycle2-board.sh <repo> --phase render
#   VERTICAL=ruby-rails bash bench/drivers/cycle2-board.sh <repo> --reset
#   VERTICAL=ruby-rails bash bench/drivers/cycle2-board.sh --status
#
# Env: VERTICAL (REQUIRED), RUNS (default 2), RERUN_MAX (default 2), PLAN_MODEL.
set -uo pipefail

BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$BENCH_DIR/.."
LIB="$BENCH_DIR/lib"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
: "${VERTICAL:?VERTICAL must be set (a key from verticals.txt, e.g. VERTICAL=ruby-rails)}"
source "$LIB/arms.sh"

VDIR="$IL_ROOT/verticals/$VERTICAL"
PLANS="$IL_ROOT/plans/cycle-2-bench-across-models"
STATE="$VDIR/.cycle2-state.json"
PHASES=(gate bench validity assemble render read publish done)
RUNS="${RUNS:-2}"
RERUN_MAX="${RERUN_MAX:-2}"

HEADLINE="$(arms_models "$VERTICAL" headline)"
ARMS="$(arms_models "$VERTICAL" confirmation)"
[ -n "$HEADLINE" ] || { echo "no headline arm for VERTICAL=$VERTICAL" >&2; exit 1; }
[ -n "$ARMS" ] || { echo "no confirmation arms for VERTICAL=$VERTICAL" >&2; exit 1; }
export BENCH_JUDGE_MODEL="${BENCH_JUDGE_MODEL:-$(arms_judge "$VERTICAL")}"

REPO=""; FORCE_PHASE=""; STATUS=0; RESET=0; ELIGIBLE=0
while [ $# -gt 0 ]; do
  case "$1" in
    --phase)    FORCE_PHASE="$2"; shift 2 ;;
    --status)   STATUS=1; shift ;;
    --reset)    RESET=1; shift ;;
    --eligible) ELIGIBLE=1; shift ;;
    -h|--help)  sed -n '2,34p' "$0"; exit 0 ;;
    -*) echo "unknown flag: $1" >&2; exit 64 ;;
    *)  REPO="$1"; shift ;;
  esac
done

state_get() { python3 -c "import json
try: d=json.load(open('$STATE'))
except Exception: d={}
print(d.get('$1',''))"; }
state_set() { python3 -c "import json,os
p='$STATE'
try: d=json.load(open(p))
except Exception: d={}
d['$1']='$2'
os.makedirs(os.path.dirname(p),exist_ok=True)
json.dump(d,open(p,'w'),indent=2,sort_keys=True); open(p,'a').write('\n')"; }

if [ "$ELIGIBLE" = 1 ]; then
  python3 "$LIB/board.py" eligible "$VDIR" --headline "$HEADLINE"
  exit $?
fi
if [ "$STATUS" = 1 ] && [ -z "$REPO" ]; then
  echo "[$VERTICAL] cycle 2 phases:"
  python3 -c "import json
try: d=json.load(open('$STATE'))
except Exception: d={}
print('\n'.join(f'  {k:16} {v}' for k,v in sorted(d.items())) or '  (none)')"
  exit 0
fi
[ -z "$REPO" ] && { echo "usage: cycle2-board.sh <repo> [--phase P] [--reset] [--eligible]" >&2; exit 64; }

YAML="$VDIR/scenarios/$REPO.yaml"
LOOPDIR="$VDIR/results/cycle2/$REPO"
NUMBERS="$LOOPDIR/numbers.json"
mkdir -p "$LOOPDIR"

if [ "$STATUS" = 1 ]; then echo "[$VERTICAL/$REPO] phase: $(state_get "$REPO" || echo gate)"; exit 0; fi
if [ "$RESET" = 1 ]; then state_set "$REPO" gate; state_set "$REPO#reruns" 0
  echo "[$REPO] reset to gate"; exit 0; fi

# The question id, and therefore the page's name, comes from the banked cell. It is
# read once here so every phase below agrees on which question it is working.
VERSION="$(python3 -c "import json,sys
sys.path.insert(0,'$LIB')
import board
rows=[r for r in board.banked_rows('$VDIR')
      if r['repo']=='$REPO' and r.get('model')=='$HEADLINE' and r.get('verdict')=='WIN']
print(rows[-1]['scenario_version'] if rows else '')")"
[ -n "$VERSION" ] || { echo "[$REPO] no banked WIN on $HEADLINE - nothing for cycle 2 to do."; exit 1; }
BOARD="$VDIR/reports/$REPO-${VERSION#sha256:}.md"
# The page is WRITTEN in the private loop dir and only COPIED to reports/ at publish.
# board.py eligible keys off the published path, so rendering straight into reports/
# would drop the cell out of the queue before its page was fit to ship, and an
# abandoned run would leave a half-written board standing as the finished one.
BOARD_STAGE="$LOOPDIR/board.md"

# STATUS.md is the page a resuming session reads and nothing else re-renders it: cycle 1
# re-renders at every transition for exactly this reason. Rendering is $0, read-only and
# write-only in the other direction (it decides nothing), so it must never take the loop
# down - hence the `|| true`.
advance() {
  state_set "$REPO" "$1"
  bash "$LIB/render-status.sh" "$VERTICAL" >/dev/null 2>&1 || true
  echo "## [$REPO] -> $1"
}

# spawn_plan <plan-file> - hand one plan to a headless agent, laws first. The laws are
# PREPENDED, never linked: a plan that ends in "see the laws" is the bug the split of
# plans from docs exists to remove.
spawn_plan() {
  local plan="$PLANS/$1" phase="$2"
  [ -f "$plan" ] || { echo "[$phase] missing plan: $plan" >&2; exit 1; }
  echo "## [$phase] spawning the phase agent on ${PLANS#"$IL_ROOT/"}/$1"
  local prompt
  prompt="$(cat "$PLANS/laws.md"; echo; cat "$plan"; cat <<EOF

# CONTEXT (resolved; these are also exported in your shell)

    VERTICAL = $VERTICAL
    VDIR     = $VDIR
    REPO     = $REPO
    HEADLINE = $HEADLINE
    ARMS     = $ARMS
    YAML     = $YAML
    NUMBERS  = $NUMBERS
    BOARD    = $BOARD_STAGE
    LOOPDIR  = $LOOPDIR
EOF
)"
  (
    cd "$IL_ROOT" || exit 1
    export VERTICAL VDIR REPO HEADLINE ARMS YAML NUMBERS LOOPDIR
    export BOARD="$BOARD_STAGE"
    export CLAUDE_CODE_DISABLE_AUTO_MEMORY=1
    IS_SANDBOX=1 claude -p "$prompt" \
      --output-format json \
      --permission-mode bypassPermissions \
      --disallowed-tools "Agent" \
      ${PLAN_MODEL:+--model "$PLAN_MODEL"}
  ) > "$LOOPDIR/$phase.agent.json" 2> "$LOOPDIR/$phase.agent.log"
}

# require_verdict <phase> <allowed-csv> - no advance unless the verdict JSON parses,
# names this phase and repo, and carries an allowed verdict. An exit code is a claim;
# the artifact is the fact.
require_verdict() {
  local phase="$1" allowed="$2"
  VERDICT="$(python3 "$LIB/verdict_check.py" "$LOOPDIR/$phase.verdict.json" \
      --phase "$phase" --repo "$REPO" --allow "$allowed" --root "$IL_ROOT")" || {
    echo "[$phase] no usable verdict on disk - NOT advancing."
    echo "         Read $LOOPDIR/$phase.agent.log, then re-run this phase."
    exit 1; }
  echo "## [$phase] verdict: $VERDICT"
}

do_gate() {
  echo "## [gate] $REPO @ ${VERSION#sha256:}  arms: $ARMS"
  python3 "$LIB/board.py" gate "$VDIR" --repo "$REPO" --headline "$HEADLINE" || {
    echo "[gate] REFUSED. Re-index at the banked Sense build, or re-bank the headline."
    exit 1; }
  advance bench
}

do_bench() {
  # Which arms to run: everything on a first pass, only the named ones on a re-entry.
  local todo="$ARMS" runs="$RUNS" keep="${KEEP_RUNS:-0}" start=1 rerun=0
  if [ -f "$LOOPDIR/validity.verdict.json" ]; then
    todo="$(python3 -c "import json
d=json.load(open('$LOOPDIR/validity.verdict.json'))
print(' '.join(d.get('rerun') or []))" 2>/dev/null)"
    [ -n "$todo" ] || todo="$ARMS"
    rerun=1
  fi
  # A RERUN buys ONE more run per named arm, filed BESIDE the existing ones.
  # This used to pass KEEP_RUNS=0, which made runs-variance hit its "REFUSING to
  # wipe a cell that holds scored runs" gate and `continue` - so a RERUN verdict
  # spent three validity spawns and produced no runs at all, then published. The
  # append knobs (START_RUN + KEEP_RUNS) already existed; the driver just never
  # used them. START_RUN clears the runs already on disk: RUNS base plus the
  # rerun attempt, which do_validity has already incremented.
  if [ "$rerun" = 1 ]; then
    local n; n="$(state_get "$REPO#reruns")"; n="${n:-0}"
    runs=1; keep=1; start=$((RUNS + n))
  fi
  echo "## [bench] VERTICAL=$VERTICAL RUNS=$runs START_RUN=$start arms: $todo"
  local before after
  before="$(_count_runs $todo)"
  MODELS="$todo" RUNS="$runs" KEEP_RUNS="$keep" START_RUN="$start" \
    bash "$BENCH_DIR/drivers/runs-variance.sh" "$REPO" || {
      echo "[bench] FAILED"; exit 1; }
  after="$(_count_runs $todo)"
  # A rerun that added no run directory did NOT happen. Advancing anyway is what
  # let a board claim a third run it never bought. Counting is the check rather
  # than looking for run-$start by name: the metered runners pick their own next
  # free slot per arm, so the new run's number is not START_RUN's to predict.
  if [ "$rerun" = 1 ] && [ "$after" -le "$before" ]; then
    echo "[bench] RERUN added no run for any of: $todo (still $after on disk)" >&2
    echo "        Nothing was appended, so the board would repeat the same numbers." >&2
    exit 1
  fi
  advance validity
}

# How many run-N directories the named arms hold for this repo, both arms summed.
# The model-id -> directory rule is bench-paths.sh's (`tr '/:' '__'`); the glob
# spans the scenario-version segment between the model root and the arm.
_count_runs() {
  local m msan d n=0
  for m in "$@"; do
    msan="$(printf '%s' "$m" | tr '/:' '__')"
    for d in "$VDIR/results/$msan"/*/baseline/"$REPO"/run-* \
             "$VDIR/results/$msan"/*/sense/"$REPO"/run-*; do
      [ -d "$d" ] && n=$((n + 1))
    done
  done
  echo "$n"
}

do_validity() {
  rm -f "$LOOPDIR/validity.verdict.json"
  spawn_plan 01-bench.md validity
  require_verdict validity "BOARD,RERUN"
  if [ "$VERDICT" = "RERUN" ]; then
    local n; n="$(state_get "$REPO#reruns")"; n="${n:-0}"
    if [ "$n" -ge "$RERUN_MAX" ]; then
      echo "## [validity] RERUN again at attempt $n, the ceiling. Building the board"
      echo "              as it stands; the reason is in $LOOPDIR/bench.md."
      advance assemble; return
    fi
    state_set "$REPO#reruns" "$((n + 1))"
    advance bench; return
  fi
  advance assemble
}

do_assemble() {
  python3 "$LIB/board.py" assemble "$VDIR" --repo "$REPO" --headline "$HEADLINE" \
    --arms "$ARMS" --scenario "$YAML" > "$NUMBERS" || {
      echo "[assemble] FAILED"; exit 1; }
  python3 -c "import json
d=json.load(open('$NUMBERS'))
print('## [assemble] columns:', ', '.join(
  f\"{c['model']}={'ok' if c.get('measured') else 'not-measured'}\" for c in d['columns']))
print('## [assemble] replication:', json.dumps(d['replication']))"
  advance render
}

do_render() {
  mkdir -p "$(dirname "$BOARD")"
  python3 "$LIB/render_board.py" "$NUMBERS" -o "$BOARD_STAGE" || {
    echo "[render] FAILED"; exit 1; }
  echo "## [render] staged at ${BOARD_STAGE#"$IL_ROOT/"}"
  advance read
}

do_read() {
  rm -f "$LOOPDIR/read.verdict.json"
  spawn_plan 02-report.md read
  require_verdict read "READING"
  advance publish
}

do_publish() {
  python3 "$LIB/board_check.py" "$BOARD_STAGE" "$NUMBERS" \
    --verdict "$LOOPDIR/read.verdict.json" || {
      echo "[publish] REFUSED. The page is NOT publishable as written."
      echo "          Re-run --phase read; the agent rewrites its own section."
      state_set "$REPO" read
      exit 1; }
  mkdir -p "$(dirname "$BOARD")"
  cp "$BOARD_STAGE" "$BOARD"
  # The page says what we measured; the ledger says why we ran it and what it taught,
  # and it is where a later session asks whether this cell is still live. Appended once
  # per (repo, question) - the key is the guard, exactly as bootstrap/scaffold is.
  local ledger="$VDIR/LEDGER.md" key="cycle2/$REPO/board"
  if [ -f "$ledger" ] && grep -q "| $key |" "$ledger"; then
    echo "## [publish] $key already in LEDGER.md, not duplicating"
  else
    local before after
    before="$(python3 "$LIB/ledger_check.py" "$VERTICAL" 2>&1 | grep -c "^  - " || true)"
    { echo; python3 "$LIB/cycle2_ledger.py" "$NUMBERS" \
        --date "$(date -u +%Y-%m-%d)"; } >> "$ledger"
    # Gate on the DELTA, never on the whole file: the ledger carries pre-existing
    # findings from before the schema tightened, and holding a publish hostage to
    # history would block every board forever. What this cycle ADDS must be clean.
    after="$(python3 "$LIB/ledger_check.py" "$VERTICAL" 2>&1 | grep -c "^  - " || true)"
    if [ "$after" -gt "$before" ]; then
      echo "[publish] the ledger entry ADDED $((after - before)) schema finding(s):"
      python3 "$LIB/ledger_check.py" "$VERTICAL" 2>&1 | grep "$key" | sed 's/^/          /'
      exit 1
    fi
    echo "## [publish] $key appended to LEDGER.md ($after finding(s), unchanged)"
  fi
  echo "## [publish] ${BOARD#"$IL_ROOT/"} stands. It is tracked, so it ships with the repo."
  advance done
}

PHASE="${FORCE_PHASE:-$(state_get "$REPO")}"
PHASE="${PHASE:-gate}"
case " ${PHASES[*]} " in *" $PHASE "*) ;; *) echo "unknown phase: $PHASE" >&2; exit 64 ;; esac

while :; do
  case "$PHASE" in
    gate)     do_gate ;;
    bench)    do_bench ;;
    validity) do_validity ;;
    assemble) do_assemble ;;
    render)   do_render ;;
    read)     do_read ;;
    publish)  do_publish ;;
    done)     echo "## [$REPO] done: ${BOARD#"$IL_ROOT/"}"; exit 0 ;;
  esac
  [ -n "$FORCE_PHASE" ] && exit 0
  PHASE="$(state_get "$REPO")"
done
