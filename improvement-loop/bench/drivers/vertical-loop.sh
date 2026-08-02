#!/usr/bin/env bash
# vertical-loop.sh - the mechanical per-repo loop of the vertical bench.
#
# THE SCRIPT OWNS THE ORDER, THE STATE, THE GATES AND EVERY SPAWN. It never judges.
# Where a phase needs judgment it spawns a headless agent on one file from plans/,
# and refuses to advance until that agent's artifact is on disk, its verifier exits 0
# and its verdict JSON parses (`require_verdict`, backed by lib/verdict_check.py).
# An exit code is a claim; the artifact is the fact. That guard used to exist once,
# by hand, in do_validate, because that defect happened to bite there - it bites once
# per phase otherwise.
#
# A phase agent never chooses the next phase and never spawns its own judge: the
# adversary probe is a separate `probe` phase spawned from here, so the author of a
# shape cannot grade it. --yes is retained as a no-op (loops 1-3 have no human gate).
#
# Phases (a state file resumes at the next one; re-run to advance):
#   index      ensure-index.sh                                    [auto]
#   scout      AGENT plans/01-scout.md   -> shape.md              [SHAPE|NO-AXIS]
#   probe      AGENT adversary probe on the shape, Sense forbidden [DISCLAIMED|ASSEMBLED]
#   curate     AGENT plans/01-curate.md  -> yaml+rubric+gold, then COMMIT them
#   preflight  render prompt + loopA preflight (resolve_oracle)   [auto]
#   validate   both arms x1 UNSCORED, then AGENT plans/02-validate.md  [PAY|DO-NOT-PAY]
#   bench      runs-variance.sh Opus x2 (PAID)                    [auto]
#   report     pergroup.py verdict; WIN -> harvest, else diagnosis
#   harvest    loopA-scan.sh harvest (mine the paid transcripts)  [auto] -> done
#
# Usage:
#   bash bench/drivers/vertical-loop.sh <repo>                 # run from saved phase
#   bash bench/drivers/vertical-loop.sh <repo> --symbol ProductVariant --file product/models.py
#   bash bench/drivers/vertical-loop.sh <repo> --yes          # accepted, no-op since 2026-07-31
#   bash bench/drivers/vertical-loop.sh <repo> --phase scout  # force one phase
#   bash bench/drivers/vertical-loop.sh <repo> --reset        # back to index
#   bash bench/drivers/vertical-loop.sh --status              # all repos' phases
#
# Env: VERTICAL (REQUIRED, no default), MODELS (default claude-opus-4-8),
#      RUNS (default 2), SENSE_CLONES (default ~/Developer/luuuc/oss/sense-benchmark/sense).
set -uo pipefail

BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$BENCH_DIR/.."
LIB="$BENCH_DIR/lib"
# VERTICAL is REQUIRED and has no default. It used to default to python-django, a
# vertical that has never had a directory, so an unset VERTICAL sent every path in
# this script at a tree that does not exist.
: "${VERTICAL:?VERTICAL must be set (a key from verticals.txt, e.g. VERTICAL=ruby-rails)}"
source "$BENCH_DIR/lib/arms.sh"
# The headline arm by default; the id lives only in verticals/<key>/arms.txt.
MODELS="${MODELS:-$(arms_models "$VERTICAL" headline)}"
[ -n "$MODELS" ] || { echo "no headline arm for VERTICAL=$VERTICAL (see verticals/$VERTICAL/arms.txt)" >&2; exit 1; }
export BENCH_JUDGE_MODEL="${BENCH_JUDGE_MODEL:-$(arms_judge "$VERTICAL")}"
HEADLINE_MODEL="${MODELS%% *}"      # first model = the headline arm the verdict reads
RUNS="${RUNS:-2}"
CLONES="${SENSE_CLONES:-$HOME/Developer/luuuc/oss/sense-benchmark/sense}"
# One derivation of the vertical dir, used everywhere below. verticals/ is a SIBLING of
# bench/ (bench-paths.sh says so): before this was factored out, two sites here still used
# the pre-move $BENCH_DIR/verticals and silently read a directory that cannot exist - the
# preflight gate found no scenario and pergroup.py found no runs.
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
VDIR="$IL_ROOT/verticals/$VERTICAL"
SCEN_DIR="$VDIR/scenarios"
PLANS="$IL_ROOT/plans"
STATE="$VDIR/.loop-state.json"
PHASES=(index scout probe curate preflight validate bench report harvest done)

# ---- args -------------------------------------------------------------------
REPO=""; SYMBOL=""; FILE_HINT=""; YES=0; FORCE_PHASE=""; STATUS=0; RESET=0
while [ $# -gt 0 ]; do
  case "$1" in
    --symbol) SYMBOL="$2"; shift 2 ;;
    --file)   FILE_HINT="$2"; shift 2 ;;
    --phase)  FORCE_PHASE="$2"; shift 2 ;;
    --yes|-y) YES=1; shift ;;
    --status) STATUS=1; shift ;;
    --reset)  RESET=1; shift ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    -*) echo "unknown flag: $1" >&2; exit 64 ;;
    *)  REPO="$1"; shift ;;
  esac
done

# ---- state helpers (JSON map repo -> phase) ---------------------------------
state_get() { # repo -> phase ("" if unknown)
  python3 -c "import json,sys
try: d=json.load(open('$STATE'))
except Exception: d={}
print(d.get('$1',''))"
}
state_set() { # repo phase
  python3 -c "import json,os
p='$STATE'
try: d=json.load(open(p))
except Exception: d={}
d['$1']='$2'
os.makedirs(os.path.dirname(p),exist_ok=True)
json.dump(d,open(p,'w'),indent=2,sort_keys=True); open(p,'a').write('\n')"
}
# Loop 3's six-cycle swap rule needs two more facts per repo. They ride on
# suffixed keys ("<repo>#cycle", "<repo>#credits") so the repo->phase map that
# every other reader expects keeps its shape.
state_dump() {
  python3 -c "import json
try: d=json.load(open('$STATE'))
except Exception: d={}
print('\n'.join(f'  {k:16} {v}' for k,v in sorted(d.items())) or '  (none)')"
}

if [ "$STATUS" = 1 ] && [ -z "$REPO" ]; then
  echo "[$VERTICAL] phases:"; state_dump; exit 0
fi
[ -z "$REPO" ] && { echo "usage: vertical-loop.sh <repo> [--symbol S] [--file F] [--yes] [--phase P] [--reset]" >&2; exit 64; }

CLONE="$CLONES/$REPO"
YAML="$SCEN_DIR/$REPO.yaml"
RUBRIC="$SCEN_DIR/$REPO.rubric.yaml"
# Where a phase agent writes its verdict, and where the design-time ($0, pre-scenario)
# artifacts live. Both are under results/, which is private by policy - the scenario is
# the only thing this driver commits.
LOOPDIR="$VDIR/results/loop/$REPO"
DRYRUN="$VDIR/results/dryrun/$REPO"
# Where the unscored validation run lands, mirroring bench-paths.sh: the results root
# is model-scoped and BENCH_VALIDATION=1 appends /validation to it.
VALIDATION_DIR="$VDIR/results/$(printf '%s' "$HEADLINE_MODEL" | tr '/:' '__')/validation"

if [ "$STATUS" = 1 ]; then echo "[$VERTICAL/$REPO] phase: $(state_get "$REPO" || echo index)"; exit 0; fi
if [ "$RESET" = 1 ]; then state_set "$REPO" index; echo "[$VERTICAL/$REPO] reset to phase 'index'"; exit 0; fi

gate() { # message... - record we're parked at the current phase and stop
  echo ""
  echo "==================== PAUSE - ACTION NEEDED ===================="
  printf '%s\n' "$@"
  echo "=============================================================="
  echo "Re-run: bash bench/drivers/vertical-loop.sh $REPO   (resumes at this phase)"
  exit 0
}

# ---- headless agents --------------------------------------------------------
# Every agent in this system is spawned HERE. A phase agent is told so in its plan,
# and runs with Agent disallowed, so it cannot delegate its own grading.
#
# PLAN_MODEL overrides the operator model; unset means the CLI default. It is
# deliberately NOT $MODELS: that is the model being MEASURED, and the two roles
# drifting into one is how a bench ends up grading itself.
headless() { # <cwd> <out.json> <prompt> [extra claude args...]
  local cwd="$1" out="$2" prompt="$3"; shift 3
  mkdir -p "$(dirname "$out")"
  (
    cd "$cwd" || exit 1
    export CLAUDE_CODE_DISABLE_AUTO_MEMORY=1
    IS_SANDBOX=1 claude -p "$prompt" \
      --output-format json \
      --permission-mode bypassPermissions \
      --disallowed-tools "Agent" \
      ${PLAN_MODEL:+--model "$PLAN_MODEL"} \
      "$@"
  ) > "$out" 2> "${out%.json}.log"
}

# spawn_plan <plan-file> - hand one plan to a headless agent, laws first.
# The laws are PREPENDED rather than linked: a plan that ends in "see the laws" is
# the documentation-as-runtime-prompt bug this split exists to remove.
# The agent's transcript is named for the PHASE, not the plan file: require_verdict
# points a stuck operator at "<phase>.agent.log", and a name that does not match is a
# dead end at exactly the moment someone is debugging.
# archive_dryrun <mv|cp> - move the previous shape/probe pair aside under a shared index,
# before anything rewrites either. A re-run must never overwrite the run it disagrees with:
# the results tree is gitignored, so an overwrite is the only copy, and a probe without the
# shape it ran against cannot be re-scored by probe_score.py. The argument is what happens to
# the SHAPE - `mv` when the scout is about to rewrite it, `cp` when only the probe is.
archive_dryrun() {
  local how="$1" n=1
  [ -f "$DRYRUN/adversary-probe.md" ] || [ -f "$DRYRUN/shape.md" ] || return 0
  while [ -f "$DRYRUN/shape.$n.md" ] || [ -f "$DRYRUN/adversary-probe.$n.md" ]; do
    n=$((n + 1))
  done
  [ -f "$DRYRUN/shape.md" ] && "$how" "$DRYRUN/shape.md" "$DRYRUN/shape.$n.md"
  [ -f "$DRYRUN/adversary-probe.md" ] &&
    mv "$DRYRUN/adversary-probe.md" "$DRYRUN/adversary-probe.$n.md"
  echo "## [$PHASE] archived the previous shape/probe pair under index $n"
}

spawn_plan() {
  local plan="$PLANS/$1" name="$PHASE"
  [ -f "$plan" ] || { echo "[$PHASE] missing plan: $plan" >&2; exit 1; }
  mkdir -p "$LOOPDIR" "$DRYRUN"
  echo "## [$PHASE] spawning the phase agent on plans/$1"
  local prompt
  prompt="$(cat "$PLANS/laws.md"; echo; cat "$plan"; cat <<EOF

# CONTEXT (resolved; these are also exported in your shell)

    VERTICAL = $VERTICAL
    REPO     = $REPO
    CLONE    = $CLONE
    VDIR     = $VDIR
    YAML     = $YAML
    RUBRIC   = $RUBRIC
    RDIR     = $VALIDATION_DIR
    SYMBOL   = ${SYMBOL:-(none - pick the contract yourself)}
    FILE     = ${FILE_HINT:-(none)}

Work from $IL_ROOT. Write the artifact and the verdict JSON, then stop.
EOF
)"
  VERTICAL="$VERTICAL" REPO="$REPO" CLONE="$CLONE" VDIR="$VDIR" \
  YAML="$YAML" RUBRIC="$RUBRIC" RDIR="$VALIDATION_DIR" \
    headless "$IL_ROOT" "$LOOPDIR/$name.agent.json" "$prompt"
  echo "   agent transcript: $LOOPDIR/$name.agent.json (stderr in $name.agent.log)"
}

# require_verdict <phase> <allowed-csv> [verifier...] - the guard. Sets VERDICT.
# No advance unless the JSON parses, names this phase and repo, carries an allowed
# verdict, its named artifact EXISTS, and any verifier passed here exits 0.
require_verdict() {
  local phase="$1" allowed="$2"; shift 2
  VERDICT="$(python3 "$LIB/verdict_check.py" "$LOOPDIR/$phase.verdict.json" \
      --phase "$phase" --repo "$REPO" --allow "$allowed" --root "$IL_ROOT")" || {
    echo "[$phase] no usable verdict on disk - NOT advancing."
    echo "         Read $LOOPDIR/$phase.agent.log, then re-run this phase."
    exit 1; }
  if [ $# -gt 0 ]; then
    "$@" || { echo "[$phase] verdict says $VERDICT but its verifier FAILED - NOT advancing."; exit 1; }
  fi
  echo "## [$phase] verdict: $VERDICT"
}

# invalidate <phase>... - drop a phase's verdict so re-entry RE-SPAWNS instead of
# re-approving what was just rejected. Without this, every routed lever loops: the
# probe kills a shape, the driver sends it back to scout, and scout finds its own
# stale verdict on disk and waves the same dead shape through.
invalidate() {
  local p
  for p in "$@"; do rm -f "$LOOPDIR/$p.verdict.json"; done
}

# have_verdict <phase> <allowed-csv> - true when a usable verdict is already on disk.
# Phases are idempotent on resume: a park re-enters the phase, and re-spawning would
# overwrite a good artifact with a fresh guess.
have_verdict() {
  VERDICT="$(python3 "$LIB/verdict_check.py" "$LOOPDIR/$1.verdict.json" \
      --phase "$1" --repo "$REPO" --allow "$2" --root "$IL_ROOT" 2>/dev/null)"
}

# ---- phase implementations (echo the next phase on success, or gate+exit) ----
do_index() {
  echo "## [index] ensure $REPO index matches the current scan engine"
  bash "$LIB/ensure-index.sh" "$REPO" || { echo "[index] FAILED - fix the index before continuing"; exit 1; }
  NEXT=scout
}

do_scout() {
  if [ -f "$YAML" ] && [ -f "$RUBRIC" ]; then
    echo "## [scout] scenario present ($REPO.yaml + .rubric.yaml)"
    # A draft found on disk is NOT a draft that was audited. This once short-circuited
    # straight to preflight, so an unaudited gold (and a comment citing transcripts that
    # did not exist) sailed through untouched. The per-dependency hand audit is
    # authoring's load-bearing check and nobody downstream catches it.
    if ! python3 "$LIB/gold_audit.py" verify "$YAML"; then
      echo "[scout] authoring does not advance until this gold is hand-audited."
      exit 1
    fi
    echo "## [scout] gold audited - advancing"
    NEXT=preflight; return
  fi
  # No seam_hunt here: the plan tells the agent to run it, and running it twice would
  # print a listing into a terminal nobody is reading. --symbol/--file ride along as
  # operator hints in the agent's context block. Choosing the anchor is the agent's
  # judgment, which is exactly why this phase exists.
  have_verdict scout "SHAPE,NO-AXIS" || { archive_dryrun mv; spawn_plan 01-scout.md; }
  require_verdict scout "SHAPE,NO-AXIS"
  if [ "$VERDICT" = NO-AXIS ]; then
    # NO-AXIS is the one routed lever that parks at the phase that PRODUCED it, so unlike
    # SHAPE it must not be idempotent. Without this the gate below is unachievable by the
    # command it prints: the re-run finds this verdict on disk, have_verdict succeeds, the
    # spawn is skipped and the same park replays forever - a parked repo could never be
    # re-scouted, and a re-run under NEW rules silently replays a verdict authored under the
    # old ones. Every other routed lever (probe, curate, validate) already invalidates.
    invalidate scout
    gate \
      "NO AXIS proposed for $REPO. This is a routed lever, never a loss." \
      "Read $DRYRUN/shape.md for the anchors that were tried, then either:" \
      "  - re-run this phase to RE-SCOUT. The verdict just shown has been cleared, so the" \
      "    next run re-spawns the scout and archives this shape/probe pair beside it." \
      "  - or swap the slot for its OWN declared backup in slate.json (never the next" \
      "    slate repo):  bash bench/drivers/vertical-loop.sh <backup-repo> --reset"
  fi
  # A shape missing a heading is not a shape. The probe is spawned against the
  # headline ask, so an absent one silently probes nothing.
  local h
  for h in "Contract" "Axis" "Headline ask" "Periphery pool" "Anchors" "Import battery"; do
    grep -qE "^# +$h" "$DRYRUN/shape.md" || {
      echo "[scout] shape.md carries no '$h' heading - NOT advancing."; exit 1; }
  done
  NEXT=probe
}

do_probe() {
  # The design-time kill, $0, and it is spawned HERE so the author of the shape never
  # grades it. Same TOOLS as the baseline arm - the clone, no Sense - but not the same
  # conditions: it answers one question with no watchdog while the arm answers a seven-step
  # session against a wall, so it is deliberately STRONGER than the arm and this gate is
  # deliberately strict. Measured on mastodon, an unleaked probe pinned <=7 of 16 gold rows
  # where the benched baseline pinned 2.5. Strict is fine; unreversible is not, which is why
  # the kill is scored, the record is archived and no axis is closed by prose.
  if ! have_verdict probe "DISCLAIMED,ASSEMBLED"; then
    local ask
    ask="$(awk '/^# +Headline ask/{f=1;next} /^# /{f=0} f' "$DRYRUN/shape.md")"
    [ -n "$(printf '%s' "$ask" | tr -d '[:space:]')" ] || {
      echo "[probe] the shape carries an empty headline ask - back to scout."
      state_set "$REPO" scout; exit 1; }
    # A re-run must not overwrite the run it disagrees with, and the next probe must not READ
    # it. The Disclaimed section of a prior probe is a list of the answers it missed, and a
    # probe working down that list holds a key the benched baseline never gets: measured on
    # mastodon, the checklist was worth 8 of 16 gold rows and it is the whole difference
    # between a bench-able shape and a NO-AXIS. The shape is copied, not moved - this probe
    # still needs it.
    archive_dryrun cp
    echo "## [probe] spawning the adversary probe in the baseline clone (Sense forbidden)"
    headless "$CLONE" "$LOOPDIR/probe.agent.json" "$(cat <<EOF
You are an adversary. Your job is to BEAT a benchmark scenario before it is written,
using nothing but what is in this repository clone. There is no code-intelligence
server available to you and you may not install one: grep, find, read and the shell
are what you have. Answer this in full, at the bar stated.

--- THE ASK ---
$ask
--- END ---

GRADING BAR: every location you report is an exact path:line. A filename alone scores
nothing. Be exhaustive - a missed dependent is a regression shipped.

WORK ONLY FROM THIS CLONE. Do not open anything under the directory you are told to write
to: it holds earlier probes of this repo, and the Disclaimed section of an earlier probe is
a list of the answers it could not find. Working down that list hands you a key the benched
baseline never gets, which is the one way this gate can lie about a scenario.

When you are done, write these two files (absolute paths, outside this clone):

1. $DRYRUN/adversary-probe.md, with exactly these headings:
   # Result       SURVIVES or ASSEMBLED, one sentence
   # Method       what you searched, in what order, and how many tool calls it took
   # Covered      every path:line you are confident in
   # Disclaimed   VERBATIM and specific: what you could NOT establish, what you never
                  looked at, and where you know your answer is thin. This section is
                  the most valuable thing you produce. "I pinned the class line and
                  nothing else" is the shape of it.

2. $LOOPDIR/probe.verdict.json:
   {"phase":"probe","repo":"$REPO","verdict":"ASSEMBLED",
    "artifact":"verticals/$VERTICAL/results/dryrun/$REPO/adversary-probe.md",
    "notes":"one line"}

   ASSEMBLED means you believe you produced the answer at path:line and a scenario
   built on this ask would not discriminate. DISCLAIMED means you did not - name the
   axis you could not reach. Do not flatter yourself in either direction: an overclaim
   kills a good scenario, an underclaim ships a bad one.
EOF
)" --strict-mcp-config --mcp-config "$LIB/baseline-mcp.json"
  fi
  require_verdict probe "DISCLAIMED,ASSEMBLED"
  # The verdict is a CLAIM and the score is the fact. The probe never sees the pool it is
  # graded against, so "ASSEMBLED" means it believes it was exhaustive - the belief this
  # bench exists to measure. probe_score.py intersects its citations with the shape's pool
  # and applies the same arithmetic as pay_ceiling.py, at $0 instead of after a pair.
  local rc
  python3 "$LIB/probe_score.py" "$DRYRUN/adversary-probe.md" "$DRYRUN/shape.md"; rc=$?
  if [ "$rc" -ge 2 ]; then
    echo "[probe] the probe could not be scored against the pool - NOT advancing."
    echo "         An unscored probe is not a passed probe. Fix the headings and re-run."
    exit 1
  fi
  if [ "$rc" = 1 ]; then
    invalidate scout probe
    state_set "$REPO" scout
    gate \
      "THE PROBE COVERED THE POOL - this shape cannot clear the floor." \
      "Its Method section is the assembly route; a shape reaching the same rows the same" \
      "way is out. Archived probes are evidence of assembly COST, never a ban list -" \
      "only a scored kill closes an axis. Re-run to author a different one."
  fi
  [ "$VERDICT" = ASSEMBLED ] && echo "## [probe] the probe claimed ASSEMBLED and the score" \
    "does not support it - shape survives, calibration noted."
  NEXT=curate
}

do_curate() {
  have_verdict curate "GOLD,NO-AXIS" || spawn_plan 01-curate.md
  require_verdict curate "GOLD,NO-AXIS"
  if [ "$VERDICT" = NO-AXIS ]; then
    invalidate scout probe curate
    state_set "$REPO" scout
    gate \
      "The probe disclaimed no gold-able axis on this shape. Back to scout." \
      "Read $DRYRUN/adversary-probe.md - the disclaimer IS the axis, so an empty one" \
      "means the ask was answerable without Sense."
  fi
  # The per-dependency hand audit is the load-bearing check and nobody downstream
  # catches it, so it is re-run HERE rather than taken on the agent's word.
  python3 "$LIB/gold_audit.py" verify "$YAML" || {
    echo "[curate] the verdict says GOLD but the hand audit is not finished - NOT advancing."
    exit 1; }
  # And the rubric, against the judge's own contract. The gold had two gates and the
  # rubric had none, so a rubric authored in an invented shape passed authoring, passed
  # preflight, and died at judge time - with both arms already spent.
  python3 "$LIB/rubric_check.py" "$YAML" || {
    echo "[curate] the rubric does not satisfy the judge - NOT advancing."; exit 1; }
  commit_scenario
  NEXT=preflight
}

# The scenario is the ONE thing this driver commits: validate and report write into
# results/, which is private by policy. It is committed here, after the audit passes,
# so a benched cell always has its scenario in history at the version it ran.
commit_scenario() {
  local root files
  root="$(cd "$IL_ROOT/.." && pwd)"
  files=("$YAML" "$RUBRIC" "$SCEN_DIR/$REPO.gold-audit.json")
  git -C "$root" add -- "${files[@]}" >/dev/null 2>&1 || return 0
  git -C "$root" diff --cached --quiet -- "${files[@]}" && return 0
  git -C "$root" commit -q -m "bench($VERTICAL): author the $REPO scenario with hand-audited gold" \
    -- "${files[@]}" && echo "## [curate] scenario committed"
}

do_preflight() {
  echo "## [preflight] render prompt (leak check) + Loop-A resolve_oracle (\$0)"
  [ -f "$YAML" ] || { echo "[preflight] $YAML missing - back to scout"; state_set "$REPO" scout; exit 1; }
  echo "---- rendered agent prompt (verify it leaks NO paths/symbols/counts/tool names, manifesto §13) ----"
  python3 "$LIB/scenario.py" "$YAML" --prompt 2>&1 | sed 's/^/  /' || true
  echo "---- Loop-A preflight (gold must be default-blast-retrievable, manifesto §10) ----"
  bash "$LIB/loopA-scan.sh" preflight "$VERTICAL" "$REPO" 2>&1 || true
  # MCP IS THE ONLY SURFACE (campaign-laws): a check that queries Sense through the CLI
  # measures a surface no arm runs. Blocking - a wrong instrument is worse than no check.
  echo "---- mcp-only law ----"
  python3 "$LIB/mcp_only_check.py" || { state_set "$REPO" scout; exit 1; }
  # The judge's rubric contract, checked BEFORE the money rather than at judge time.
  # A scenario authored before this gate existed reaches preflight without ever having
  # been checked, so preflight runs it too - not only curate.
  echo "---- rubric contract ----"
  python3 "$LIB/rubric_check.py" "$YAML" || {
    echo "[preflight] the rubric does not satisfy the judge. Nothing runs on a scenario"
    echo "            whose answers cannot be judged."; exit 1; }

  # The COST GATE was REMOVED 2026-07-31: loops 1-3 have no human gate. Runs go
  # through a subscription by default, so what a cycle spends is quota against the
  # weekly reset. The leak check and the oracle above are what decide now.
  #
  # NEXT=validate, not bench: the unscored both-arms validation run is the spend-time
  # go/no-go (the validation run is the spend-time go/no-go). This said
  # NEXT=bench until 2026-08-01, which made `validate` unreachable from the chain -
  # it could only be entered by hand with --phase validate, so every driver-run cycle
  # went straight from preflight to the paid bench with no unscored check.
  NEXT=validate
}

do_validate() {
  # The validation run: both arms, ONE run each,
  # unscored, before anything paid. BENCH_VALIDATION=1 routes it to a separate
  # results root and stamps "scoring": false, so no scorer can ever see it.
  # Look wherever a validation transcript can land: RESULTS_DIR is model-scoped, so the
  # tree is results/<model>/validation/, not results/validation/. Probing the latter
  # never finds an existing run, so every entry re-spends.
  #
  # AND SCOPE IT TO THIS REPO. The probe used to match any `*/validation/*` transcript
  # in the vertical, so the FIRST repo to run a validation satisfied it for every repo
  # after it: mastodon skipped its own unscored go/no-go and would have entered the PAID
  # bench on rails' evidence. The runner writes <results>/<model>/validation/<tool>/<repo>/run-N.
  local vdir_out="$VDIR/results"
  validation_runs() {
    find "$vdir_out" -path "*/validation/*/$REPO/*" -name 'transcript.json' 2>/dev/null | head -1
  }
  if [ -n "$(validation_runs)" ]; then
    echo "## [validate] a validation run already exists for $REPO - not re-running"
    echo "   (delete $VALIDATION_DIR to force a fresh one)"
  else
    echo "## [validate] unscored validation run, both arms x1 (BENCH_VALIDATION=1)"
    BENCH_VALIDATION=1 VERTICAL="$VERTICAL" MODELS="$MODELS" RUNS=1 \
      bash "$BENCH_DIR/drivers/runs-variance.sh" "$REPO" || {
        echo "[validate] the validation run FAILED - that is a result, not an obstacle."
        echo "           Read the transcripts before re-running."; exit 1; }
    # POST-CONDITION, not just the exit code: the runner has returned 0 with the run
    # actually failed, and this phase advanced to the PAID bench on it. An exit code is
    # a claim; the transcript on disk is the fact. Generalised into require_verdict for
    # every agent phase - this one guards a runner, so it stays here too.
    if [ -z "$(validation_runs)" ]; then
      echo "[validate] the runner exited 0 but wrote NO transcript for $REPO - treating as a FAILED"
      echo "           validation. Nothing advances to a paid bench on an absent artefact."
      exit 1
    fi
  fi
  # ARITHMETIC BEFORE JUDGMENT. pergroup declares WIN when a group's delta reaches the
  # floor, and recall caps at 1.00, so a group whose baseline already sits at B can never
  # deliver more than 1.00-B. If no group can reach the floor, there is no judgment left
  # to make and no agent is spawned. Measured: a cell whose baseline held 10 of 16
  # scattered rows discriminated clearly (+0.31) and was still incapable of a WIN
  # (ceiling +0.375) - the judgment phase correctly answered "does it discriminate?" and
  # had no reason to ask "can it clear the bar?". A floor is not a judgment call.
  echo "## [validate] arithmetic ceiling (can any group still reach the floor?)"
  if ! RESULTS_DIR="$VALIDATION_DIR" python3 "$LIB/pay_ceiling.py" "$REPO" 0.50; then
    invalidate scout probe curate validate
    state_set "$REPO" scout
    gate \
      "DO NOT PAY on $REPO - no gold group can reach +0.50 however well the sense arm" \
      "does. This is arithmetic on the validation pair, not a judgment." \
      "The lever is the GOLD, not the wall: re-gold from what the baseline MISSED." \
      "The scenario is COMMITTED, so authoring will not re-enter until it is out of the" \
      "way. Move it aside, then re-run to resume at scout:" \
      "  git rm -q $YAML $RUBRIC $SCEN_DIR/$REPO.gold-audit.json"
  fi
  # The pay decision. It is the last thing standing between a scenario and the money,
  # and nothing reviews it afterwards, so it is a judgment phase and gets an agent.
  have_verdict validate "PAY,DO-NOT-PAY" || spawn_plan 02-validate.md
  require_verdict validate "PAY,DO-NOT-PAY"
  if [ "$VERDICT" = DO-NOT-PAY ]; then
    invalidate scout probe curate validate
    state_set "$REPO" scout
    gate \
      "DO NOT PAY on $REPO - the validation run says this scenario does not discriminate." \
      "The read is in $LOOPDIR/validate.md, with the lever for the next draft." \
      "The scenario is COMMITTED, so authoring will not re-enter until it is out of the" \
      "way. Move it aside, then re-run to resume at scout:" \
      "  git rm -q $YAML $RUBRIC $SCEN_DIR/$REPO.gold-audit.json" \
      "This step is deliberately yours: deleting a benched scenario is a git decision," \
      "and the version that ran stays in history either way."
  fi
  NEXT=bench
}

do_bench() {
  echo "## [bench] VERTICAL=$VERTICAL MODELS='$MODELS' RUNS=$RUNS runs-variance.sh $REPO"
  VERTICAL="$VERTICAL" MODELS="$MODELS" RUNS="$RUNS" \
    bash "$BENCH_DIR/drivers/runs-variance.sh" "$REPO" || { echo "[bench] FAILED"; exit 1; }
  NEXT=report
}

do_report() {
  echo "## [report] axis panel refresh (observational, all banked cells)"
  python3 "$LIB/panel.py" 2>&1 | sed 's/^/  /' || true
  echo "## [report] per-group cited-recall verdict (headline arm: $HEADLINE_MODEL)"
  # RESULTS_DIR mirrors bench-paths.sh: verticals/<name>/results/<sanitized-model>.
  local msan rdir out
  msan="$(printf '%s' "$HEADLINE_MODEL" | tr '/:' '__')"
  rdir="$VDIR/results/$msan"
  out="$(RESULTS_DIR="$rdir" python3 "$LIB/pergroup.py" "$REPO" 0.50 2>&1)"
  echo "$out" | sed 's/^/  /'

  # Cost parity, on EVERY cell. Reach had an arbiter and cost had none, so a
  # cell could win the headline at a 30% premium and nothing would say so.
  # Printed before the verdict routes, because a parity MISS is a product
  # finding for harvest - never a reason to halt (see cost_parity.py).
  echo "## [report] cost parity (priced tokens, sense vs baseline)"
  local cost_out
  cost_out="$(RESULTS_DIR="$rdir" python3 "$LIB/cost_parity.py" "$REPO" 2>&1)"
  echo "$cost_out" | sed 's/^/  /'

  if echo "$out" | grep -q '^VERDICT: WIN'; then
    echo "  -> WIN (discriminator >= +0.50). Banking Loop-A harvest."
    if echo "$cost_out" | grep -q '^COST_PARITY: MISS'; then
      echo "  -> COST-PARITY MISS on a winning cell. Sense is NOT reaching parity."
      echo "     This is a PRODUCT finding: harvest owes an answer to WHY the sense"
      echo "     arm's context is larger, and what can be trimmed. Do NOT treat it"
      echo "     as a measurement defect and stop - that is the 2026-08-01 error."
      echo "     Required in harvest: RESULTS_DIR=$rdir python3 bench/lib/context_cost_audit.py $REPO"
    fi
    state_set "$REPO#cycle" 0
    NEXT=harvest; return
  fi
  # The credit table is Loop 3's one mechanical input, and its fingerprint is the
  # movement detector: a re-authored scenario that moves no row has not moved the
  # cell, however different its prose.
  local fp prev cycle
  fp="$(RESULTS_DIR="$rdir" python3 "$LIB/credit_table.py" "$REPO" --fingerprint 2>/dev/null)"
  prev="$(state_get "$REPO#credits")"
  cycle="$(state_get "$REPO#cycle")"; [ -z "$cycle" ] && cycle=0
  if [ -n "$fp" ] && [ "$fp" != "$prev" ]; then
    cycle=1
    echo "  -> credit table MOVED ($prev -> $fp): cycle counter reset to 1"
  else
    cycle=$((cycle + 1))
    echo "  -> credit table UNCHANGED ($fp): cycle $cycle of 6"
  fi
  state_set "$REPO#cycle" "$cycle"
  [ -n "$fp" ] && state_set "$REPO#credits" "$fp"

  echo ""
  RESULTS_DIR="$rdir" python3 "$LIB/credit_table.py" "$REPO" 2>&1 | sed 's/^/  /' || true

  if [ "$cycle" -ge 6 ]; then
    gate \
      "SIX CYCLES WITH NO MOVEMENT - swap this repo." \
      "The slot takes its OWN declared backup from slate.json, not the next slate repo." \
      "Write the swap dossier to LEDGER.md as loop3/$REPO/swap, then reset:" \
      "  bash bench/drivers/vertical-loop.sh <backup-repo> --reset"
  fi
  gate \
    "BELOW +0.50 - this cell goes to diagnosis. The loop cannot record a loss." \
    "1. STRUGGLE READ (every run): spawn the bench-struggle-read agent on the table above." \
    "   It returns scenario material - the rows the baseline missed and what it spent instead." \
    "2. TAXONOMY (sub-floor only): spawn bench-evaluator; branches in order, cheapest first." \
    "   \$0 gold-retarget re-scores without re-benching:" \
    "     python3 bench/lib/scorer.py <run_dir> $YAML bench" \
    "   Branch 2 (re-author + re-bench) is the only paid lever and needs 1/3/4 exhausted." \
    "Then re-enter authoring: bash bench/drivers/vertical-loop.sh $REPO --phase scout"
}

do_harvest() {
  echo "## [harvest] Loop-A transcript_miss over the paid transcripts (\$0, advisory)"
  bash "$LIB/loopA-scan.sh" harvest "$VERTICAL" "$HEADLINE_MODEL" 2>&1 || true
  echo ""
  # The product half. Printing the instruction in `report` was not enough - an
  # instruction nobody executes is how the budget-trim audit sat deferred from
  # 2026-07-30 to 2026-08-01 while a 26% premium went unexplained. Harvest RUNS it.
  local msan rdir
  msan="$(printf '%s' "$HEADLINE_MODEL" | tr '/:' '__')"
  rdir="$VDIR/results/$msan"
  echo "## [harvest] context cost audit - WHY the sense arm costs what it costs"
  RESULTS_DIR="$rdir" python3 "$LIB/cost_parity.py" "$REPO" 2>&1 | sed 's/^/  /' || true
  RESULTS_DIR="$rdir" python3 "$LIB/context_cost_audit.py" "$REPO" 2>&1 | sed 's/^/  /' || true
  echo ""
  echo "## $REPO - Definition of Done (manifesto §14), confirm by hand:"
  echo "   [ ] discriminator >= +0.50 (favored +0.80) on the headline arm x$RUNS"
  echo "   [ ] Sense adopted its tools (mcp_count>0), no hallucinated cites, baseline floor legit"
  echo "   [ ] efficiency reported, scenario human-readable + leak-free, article matches the numbers"
  echo "   [ ] COST PARITY: either PASS above, or a MISS carrying a named trim candidate"
  echo "       and a Loop 7 pitch. A premium with no lever is an unfinished harvest."
  NEXT=done
}

# ---- driver: run phases from the current one until a gate stops us ----------
PHASE="${FORCE_PHASE:-$(state_get "$REPO")}"; [ -z "$PHASE" ] && PHASE=index
echo "[$VERTICAL/$REPO] entering at phase '$PHASE' (models='$MODELS' runs=$RUNS)"

while :; do
  NEXT=""
  # Each phase prints its own progress and EITHER sets NEXT (advance) OR calls
  # gate()/exit (stop). Output flows straight to the terminal (no capture).
  case "$PHASE" in
    index)     do_index ;;
    scout)     do_scout ;;
    probe)     do_probe ;;
    curate)    do_curate ;;
    preflight) do_preflight ;;
    validate)  do_validate ;;
    bench)     do_bench ;;
    report)    do_report ;;
    harvest)   do_harvest ;;
    done)      echo "[$VERTICAL/$REPO] phase 'done' - nothing to do (use --reset to rerun)"; exit 0 ;;
    *)         echo "unknown phase '$PHASE'" >&2; exit 64 ;;
  esac
  [ -z "$NEXT" ] && { echo "[$VERTICAL/$REPO] phase '$PHASE' set no next phase - stopping" >&2; exit 1; }
  state_set "$REPO" "$NEXT"
  [ -n "$FORCE_PHASE" ] && { echo "[$VERTICAL/$REPO] phase '$PHASE' done; next is '$NEXT' (--phase forced single step)"; exit 0; }
  PHASE="$NEXT"
  [ "$PHASE" = done ] && { echo "[$VERTICAL/$REPO] all phases complete."; exit 0; }
done
