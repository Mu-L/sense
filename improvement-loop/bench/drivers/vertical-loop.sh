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
# A phase agent never chooses the next phase and never spawns its own judge: the phase
# that measures a scenario is never the phase that wrote it. --yes is retained as a
# no-op (loops 1-3 have no human gate).
#
# AUTHORING IS A CYCLE, NOT A LINE, AND IT RUNS ITSELF. The first measurement in this
# loop is a REAL BASELINE ARM against a two-step probe scenario, not a screen and not a
# simulated adversary: every cheaper substitute this bench built (token-cover battery,
# import screen, scored adversary probe, hand-run read-cost census) was measured wrong,
# and each killed a repo a benched scenario then won.
#
# Every routed lever - NO-ANCHOR, REQUESTION, DO-NOT-PAY - re-enters `author` WITH the
# credit table that rejected it and WITHOUT deleting anything: the anchor stays and the
# question is rewritten in place. No pause. It parks for a human only after
# AUTH_CYCLE_MAX (6) cycles on one repo, and hands the next run a fresh budget.
#
# Phases (a state file resumes at the next one; re-run to advance):
#   index      ensure-index.sh                                    [auto]
#   author     AGENT plans/01-author.md    -> 2-step yaml+rubric+gold  [DRAFT|NO-ANCHOR]
#   minibench  both arms x1 UNSCORED on the 2-step scenario, then
#              AGENT plans/02-minibench.md      [PROCEED|REQUESTION|NO-ANCHOR]
#   expand     AGENT plans/03-expand.md    -> 7-step scenario, then COMMIT it
#   preflight  render prompt + loopA preflight (resolve_oracle)   [auto]
#   validate   both arms x1 UNSCORED, then AGENT plans/04-validate.md  [PAY|DO-NOT-PAY]
#   bench      runs-variance.sh Opus x2 (PAID)                    [auto]
#   report     pergroup.py verdict; WIN -> harvest, else diagnosis
#   harvest    loopA-scan.sh harvest (mine the paid transcripts)  [auto] -> done
#   handoff    not in the chain - AGENT plans/05-handoff.md, spawned only when the
#              authoring cycle hits its ceiling, to hand the human a readable page
#
# Usage:
#   bash bench/drivers/vertical-loop.sh <repo>                 # run from saved phase
#   bash bench/drivers/vertical-loop.sh <repo> --symbol ProductVariant --file product/models.py
#   bash bench/drivers/vertical-loop.sh <repo> --yes          # accepted, no-op since 2026-07-31
#   bash bench/drivers/vertical-loop.sh <repo> --phase author # force one phase
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
PHASES=(index author minibench expand preflight validate bench report harvest done)

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
    -h|--help) sed -n '2,45p' "$0"; exit 0 ;;
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
# Where the unscored runs land, mirroring bench-paths.sh: the results root is
# model-scoped, BENCH_VALIDATION=1 appends /validation and BENCH_MINIBENCH=1 appends
# /minibench. Two roots, not one: both write <root>/<arm>/<repo>/run-1, so a shared root
# would have the seven-step pair overwrite the two-step pair at the same path.
MODEL_RESULTS="$VDIR/results/$(printf '%s' "$HEADLINE_MODEL" | tr '/:' '__')"
VALIDATION_DIR="$MODEL_RESULTS/validation"
MINIBENCH_DIR="$MODEL_RESULTS/minibench"
# The results root the phase agent is handed as $RDIR. Set per phase; the two measuring
# phases read different trees and handing either the wrong one grades the wrong run.
RDIR="$VALIDATION_DIR"

if [ "$STATUS" = 1 ]; then echo "[$VERTICAL/$REPO] phase: $(state_get "$REPO" || echo index)"; exit 0; fi
if [ "$RESET" = 1 ]; then
  state_set "$REPO" index
  # The authoring budget is per attempt-at-this-repo, so a deliberate reset clears it too.
  state_set "$REPO#authcycle" 0
  echo "[$VERTICAL/$REPO] reset to phase 'index' (authoring cycle counter cleared)"
  exit 0
fi

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
# archive_read <name> - move a measuring phase's previous read aside under an index,
# called by that phase before it writes a new one. A re-run must never overwrite the run
# it disagrees with: the results tree is gitignored, so an overwrite is the only copy.
# These reads are what a re-question is written against - they name, per row, which gold
# the baseline took and the route it took it by - so losing one costs the next draft
# exactly the information that would have improved it.
#
# NOT called by the authoring phase, deliberately: plans/01-author.md opens by reading
# minibench.md to decide it is in a re-question cycle. Archiving before authoring would
# hide the credit table from the phase that exists to answer it.
archive_read() {
  local name="$1" n=1
  [ -f "$LOOPDIR/$name.md" ] || return 0
  while [ -f "$DRYRUN/$name.$n.md" ]; do n=$((n + 1)); done
  mkdir -p "$DRYRUN"
  mv "$LOOPDIR/$name.md" "$DRYRUN/$name.$n.md"
  echo "## [$PHASE] archived the previous $name read as $name.$n.md"
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
    RDIR     = $RDIR
    SYMBOL   = ${SYMBOL:-(none - pick the contract yourself)}
    FILE     = ${FILE_HINT:-(none)}

Work from $IL_ROOT. Write the artifact and the verdict JSON, then stop.
EOF
)"
  VERTICAL="$VERTICAL" REPO="$REPO" CLONE="$CLONE" VDIR="$VDIR" \
  YAML="$YAML" RUBRIC="$RUBRIC" RDIR="$RDIR" \
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
# mini-bench rejects a question, the driver sends it back to author, and author finds
# its own stale verdict on disk and waves the same rejected question through.
invalidate() {
  local p
  for p in "$@"; do rm -f "$LOOPDIR/$p.verdict.json"; done
}

# ---- the authoring cycle -----------------------------------------------------
# AUTHORING RUNS ITSELF. A rejected question re-enters authoring immediately, with the
# credit table that rejected it, up to AUTH_CYCLE_MAX attempts on this repo. Parking at
# every routed lever made a human the loop's scheduler: ten mastodon cycles were driven
# by hand in one day, which is how the same wrong screen survived ten re-derivations
# instead of being contradicted by a run. The per-repo phases have no human gate.
#
# The bound IS the spend control: one cycle is one author agent plus one unscored
# two-arm pair, so six is roughly an hour of wall and a bounded token bill.
#
# It uses its own counter. `<repo>#cycle` belongs to the PAID diagnosis loop in
# do_report, which resets on credit-table MOVEMENT; conflating the two would let a paid
# cycle spend an authoring attempt and a re-author reset the swap rule.
AUTH_CYCLE_MAX="${AUTH_CYCLE_MAX:-6}"

# record_cycle <n> <label> <verdict> - append one row to the authoring ledger.
# THE RUNS DO NOT SURVIVE. runs-variance.sh wipes <root>/<arm>/<repo> before each run, so
# every cycle overwrites the last cycle's scored.json at the same path. Without this the
# handoff at the ceiling could only re-narrate the agents' prose, and a summary of six
# attempts has to stand on numbers. One line per cycle, written while the run is still on
# disk, quoting the discriminator group both arms actually scored.
record_cycle() {
  local n="$1" label="$2" verdict="$3" root="$VALIDATION_DIR"
  [ "$label" = minibench ] && root="$MINIBENCH_DIR"
  python3 - "$LOOPDIR/cycles.jsonl" "$n" "$label" "$verdict" "$YAML" "$root" "$REPO" <<'PY'
import json, os, sys
out, n, phase, verdict, yaml_path, root, repo = sys.argv[1:8]

def deps(arm):
    p = os.path.join(root, arm, repo, "run-1", "scored.json")
    try:
        with open(p) as fh:
            g = json.load(fh).get("gold_recall", {}).get("groups", {})
    except (OSError, ValueError):
        return None
    d = g.get("dependents")
    return None if d is None else d.get("cited_recall")

# `cycle` is the in-round counter the ceiling uses; `attempt` counts every measured try
# on this repo across rounds, so a summary can never confuse two attempts numbered 1.
try:
    prior = sum(1 for _ in open(out))
except OSError:
    prior = 0
row = {"cycle": int(n), "attempt": prior + 1, "phase": phase, "verdict": verdict,
       "baseline_dependents": deps("baseline"), "sense_dependents": deps("sense")}
try:
    import yaml
    y = yaml.safe_load(open(yaml_path))
    row["title"] = y.get("name")
    row["contract"] = y.get("contract_symbol")
    steps = y.get("steps") or []
    row["steps"] = len(steps)
    # THE TITLE IS NOT THE QUESTION. Two cycles with opposite outcomes (baseline 0.81 and
    # baseline 0.16) carried the same `name:`, so a handoff table keyed on it shows six
    # identical rows. What varies per cycle is the discriminator step's prompt, which is
    # the ask itself.
    if steps:
        row["ask"] = " ".join((steps[-1].get("prompt") or "").split())[:400]
except Exception:
    pass
os.makedirs(os.path.dirname(out), exist_ok=True)
with open(out, "a") as fh:
    fh.write(json.dumps(row) + "\n")
PY
}

# requeue_author <label> <why...> - route a rejected question back to authoring.
# Nothing is deleted: the scenario, its gold and the read that rejected it all stay on
# disk, and 01-author.md opens by reading them. The anchor stays.
requeue_author() {
  local label="$1"; shift
  local n; n="$(state_get "$REPO#authcycle")"; [ -z "$n" ] && n=0
  n=$((n + 1))
  # Record BEFORE the run is wiped by the next cycle, and before anything is invalidated.
  record_cycle "$n" "$label" "${VERDICT:-$label}"
  invalidate author minibench expand validate
  state_set "$REPO" author
  if [ "$n" -ge "$AUTH_CYCLE_MAX" ]; then
    # Park, and hand the next run a FRESH budget. A counter left at the ceiling makes
    # the printed re-run command a lie: it would re-enter, increment to 7 and park again
    # forever - the same trap the old NO-AXIS park had before it learned to invalidate.
    state_set "$REPO#authcycle" 0
    # A CEILING IS A DECISION HANDED UP, NOT A DEAD END. The repo is not dead and the
    # anchor may be fine; what has run out is the loop's own judgment, so it writes the
    # human a plain-language read of all $AUTH_CYCLE_MAX attempts and stops. Spawned
    # HERE, like every other agent: the phases that produced these cycles do not get to
    # summarise themselves.
    PHASE=handoff
    have_verdict handoff "HANDOFF" || spawn_plan 05-handoff.md
    require_verdict handoff "HANDOFF"
    invalidate handoff
    # Close the round's ledger AFTER the handoff has read it. The counter restarts at 1 for
    # the next round, so a shared file would carry two rows numbered 1 and the summary could
    # not tell which attempt it was reading. Archived, never deleted: 01-author.md globs the
    # archives too, because the useful history is every attempt ever made on this repo.
    local ln=1
    while [ -f "$DRYRUN/cycles.$ln.jsonl" ]; do ln=$((ln + 1)); done
    [ -f "$LOOPDIR/cycles.jsonl" ] && {
      mkdir -p "$DRYRUN"
      mv "$LOOPDIR/cycles.jsonl" "$DRYRUN/cycles.$ln.jsonl"
      echo "## [handoff] closed the round ledger as cycles.$ln.jsonl"; }
    gate \
      "We tried $AUTH_CYCLE_MAX attempts on $REPO without a payable question." \
      "Sharing a summary with you so you can take the next step." \
      "" \
      "  READ THIS:  $LOOPDIR/handoff.md" \
      "" \
      "It is written in plain words: what each attempt asked, what the scores were," \
      "what worked, what did not, and a recommendation. Nothing has been deleted -" \
      "every scenario and every read is still on disk, and the numbers behind the" \
      "summary are in $LOOPDIR/cycles.jsonl." \
      "" \
      "Re-run to spend another $AUTH_CYCLE_MAX cycles, or swap the slot for its OWN" \
      "declared backup in slate.json (never the next slate repo):" \
      "  bash bench/drivers/vertical-loop.sh <backup-repo> --reset"
  fi
  state_set "$REPO#authcycle" "$n"
  echo ""
  echo "## [$label] authoring cycle $n of $AUTH_CYCLE_MAX - re-entering author, no pause"
  printf '   %s\n' "$@"
  NEXT=author
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
  NEXT=author
}

# runs_for <root> <scenario_version> - the first transcript under <root> for THIS repo at
# THIS scenario version, or nothing. Two scopings, both of them paid for:
#   REPO, because the probe used to match any */validation/* transcript in the vertical,
#   so the first repo to run one satisfied it for every repo after it and mastodon would
#   have entered the PAID bench on rails' evidence.
#   SCENARIO VERSION, because a re-authored question inherited the previous question's
#   pair, so the loop read a ceiling off a baseline that had answered a different
#   scenario. run_meta.json has carried scenario_version all along; it gated nothing.
runs_for() {
  python3 - "$1" "$REPO" "$2" <<'PY'
import json, os, sys
root, repo, want = sys.argv[1], sys.argv[2], sys.argv[3]
for dirpath, _, filenames in os.walk(root):
    if "run_meta.json" not in filenames or "transcript.json" not in filenames:
        continue
    if repo not in dirpath.split(os.sep):
        continue
    try:
        with open(os.path.join(dirpath, "run_meta.json")) as fh:
            meta = json.load(fh)
    except (OSError, ValueError):
        continue
    if meta.get("scenario_version") != want:
        continue
    print(os.path.join(dirpath, "transcript.json"))
    break
PY
}

# archive_scenario <yaml> - keep the scored bytes of this scenario version forever.
#
# WHY. run_meta.json records the version AND the scenario_file, but that path is the LIVE
# one, which the next authoring cycle overwrites. So a number on disk knows the identity of
# the question that produced it and cannot reach its gold. Recovering which question produced
# the twelve scored mastodon runs meant brute-forcing ten .bak files crossed with two rubrics.
# Content-addressed and append-only, so re-archiving an unchanged pair is a no-op.
archive_scenario() {
  python3 "$LIB/scenario_archive.py" add "$1" --store "$(dirname "$1")/.versions" >/dev/null \
    || echo "[warn] could not archive $1 - its gold will not be recoverable by version" >&2
}

# next_run <root> - the run index a new cycle should file under, i.e. one past the
# highest run-N already on disk for this repo under either arm.
#
# WHY THIS EXISTS. An authoring cycle used to re-run at the SAME run-1 and wipe the
# previous cycle's transcripts to get there. cycles.jsonl kept the FIGURES and $DRYRUN
# kept the agent's READ, but the runs those were derived from were gone, so no finding
# from a cycle could be re-checked afterwards - and every real finding this loop has
# produced came out of a transcript, not a number. Appending instead costs disk and
# nothing else: runs_for() selects a cycle's own pair by scenario_version, so two
# cycles' runs coexist in one root without being blended by anything downstream.
next_run() {
  local root="$1" max=0 n d
  for d in "$root"/*/"$REPO"/run-*; do
    [ -d "$d" ] || continue
    n="${d##*/run-}"
    case "$n" in *[!0-9]*) continue ;; esac
    [ "$n" -gt "$max" ] && max="$n"
  done
  echo $((max + 1))
}

# scenario_steps <yaml> - how many steps the scenario declares (0 if unreadable).
scenario_steps() {
  python3 -c "import sys,yaml
try: print(len(yaml.safe_load(open(sys.argv[1]))['steps']))
except Exception: print(0)" "$1" 2>/dev/null
}

do_author() {
  # THE HAND-AUTHORED ENTRY. A scenario written by a human outside this loop enters here
  # and skips straight to the phase that measures it. It is gated on the gold audit and
  # nothing else: a draft found on disk is NOT a draft that was audited, and this once
  # short-circuited to preflight so an unaudited gold sailed through untouched.
  #
  # NOT taken mid-cycle. minibench.md on disk means a measured run already happened and
  # this entry is a RE-QUESTION, whose whole job is to rewrite the scenario that is
  # sitting right there. Taking the door then would wave the just-rejected question back
  # through against its own credit table.
  if [ -f "$YAML" ] && [ -f "$RUBRIC" ] && [ ! -f "$LOOPDIR/minibench.md" ]; then
    echo "## [author] scenario present ($REPO.yaml + .rubric.yaml), no measured run yet"
    if ! python3 "$LIB/gold_audit.py" verify "$YAML"; then
      echo "[author] authoring does not advance until this gold is hand-audited."
      exit 1
    fi
    local n; n="$(scenario_steps "$YAML")"
    if [ "$n" -le 2 ]; then
      echo "## [author] gold audited, $n-step draft - to the mini-bench"
      NEXT=minibench
    else
      echo "## [author] gold audited, $n-step scenario - already expanded, to preflight"
      NEXT=preflight
    fi
    return
  fi
  # No seam_hunt here: the plan tells the agent to run it, and running it twice would
  # print a listing into a terminal nobody is reading. --symbol/--file ride along as
  # operator hints in the agent's context block. Choosing the anchor is the agent's
  # judgment, which is exactly why this phase exists.
  have_verdict author "DRAFT,NO-ANCHOR" || spawn_plan 01-author.md
  require_verdict author "DRAFT,NO-ANCHOR"
  if [ "$VERDICT" = NO-ANCHOR ]; then
    # NO-ANCHOR parks at the phase that PRODUCED it, so unlike DRAFT it must not be
    # idempotent: without this the re-run finds the verdict on disk, the spawn is skipped
    # and the same park replays forever, so a parked repo could never be re-authored and
    # a re-run under NEW rules would silently replay a verdict authored under the old ones.
    requeue_author author \
      "NO ANCHOR proposed: the shown blast set cannot carry twelve dependent files" \
      "across six areas on any of three candidates. It does NOT say the repo is dead," \
      "and nothing a grep prints may reach it - only a run kills a question here." \
      "The next cycle profiles different contracts."
    return
  fi
  # A two-step draft is the contract of this phase, and the mini-bench measures the
  # SECOND step. A draft that arrives with more steps has skipped the measurement the
  # expansion is supposed to earn.
  local n; n="$(scenario_steps "$YAML")"
  [ "$n" = 2 ] || {
    echo "[author] the verdict says DRAFT but $YAML declares $n steps, not 2 - NOT advancing."
    echo "         The probe scenario is two steps: map the contract, audit the dependents."
    exit 1; }
  python3 "$LIB/gold_audit.py" verify "$YAML" || {
    echo "[author] the verdict says DRAFT but the hand audit is not finished - NOT advancing."
    exit 1; }
  NEXT=minibench
}

do_minibench() {
  # THE FIRST MEASUREMENT IN THIS LOOP, and the only one before a seven-step scenario
  # exists. Both arms, ONE run each, unscored, at the cell's real wall, against the
  # two-step probe scenario. It replaces the simulated adversary probe, which never
  # estimated the benched arm in either direction: on the parked cell it reached 4 of 16
  # where the arm reached 10; on the banked cell it reached at most 7 of 16 where the arm
  # reached 2.5, and a leaked run reached 15 of 16 over that same arm. No correction
  # factor fits points that disagree by 2.5x one way and 0.36x the other. This costs
  # about the same and measures a real baseline instead of guessing at one.
  RDIR="$MINIBENCH_DIR"
  local scen_ver
  scen_ver="$(python3 "$LIB/scenario_version.py" "$YAML")" || {
    echo "[minibench] cannot compute the scenario version for $YAML - NOT advancing."
    echo "            A pair that cannot be tied to a scenario is not evidence."
    exit 1; }
  archive_scenario "$YAML"
  echo "## [minibench] scenario $scen_ver"
  if [ -n "$(runs_for "$MINIBENCH_DIR" "$scen_ver")" ]; then
    echo "## [minibench] a run already exists for this scenario version - not re-running"
    echo "   (delete $MINIBENCH_DIR to force a fresh one)"
  else
    echo "## [minibench] unscored two-arm run x1 (BENCH_MINIBENCH=1)"
    # APPEND, never wipe. This used to pass FORCE_WIPE=1 so a second cycle could run
    # at the same <root>/<arm>/<repo>/run-1 at all - the figures were in cycles.jsonl
    # and the read in $DRYRUN, so the runs looked like working space. They were not:
    # cycles 1-8 of the Account campaign are unrecoverable, and every finding worth
    # having this campaign came from reading a transcript. KEEP_RUNS=1 + START_RUN
    # files each cycle beside the last; runs_for() still selects this cycle's own pair
    # by scenario_version, so nothing downstream blends two questions.
    BENCH_MINIBENCH=1 KEEP_RUNS=1 START_RUN="$(next_run "$MINIBENCH_DIR")" \
      VERTICAL="$VERTICAL" MODELS="$MODELS" RUNS=1 \
      bash "$BENCH_DIR/drivers/runs-variance.sh" "$REPO" || {
        echo "[minibench] the run FAILED - that is a result, not an obstacle."
        echo "            Read the transcripts before re-running."; exit 1; }
    # POST-CONDITION, not just the exit code: the runner has returned 0 with the run
    # actually failed. An exit code is a claim; the transcript on disk is the fact.
    if [ -z "$(runs_for "$MINIBENCH_DIR" "$scen_ver")" ]; then
      echo "[minibench] the runner exited 0 but wrote NO transcript for $REPO - treating"
      echo "            as a FAILED run. Nothing advances on an absent artefact."
      exit 1
    fi
  fi
  have_verdict minibench "PROCEED,REQUESTION,NO-ANCHOR" ||
    { archive_read minibench; spawn_plan 02-minibench.md; }
  require_verdict minibench "PROCEED,REQUESTION,NO-ANCHOR"
  if [ "$VERDICT" = REQUESTION ]; then
    # KEEP THE ANCHOR, CHANGE THE QUESTION. Nothing is deleted and nothing is re-scouted:
    # the scenario, its gold and its credit table all stay on disk, and 01-author.md opens
    # by reading minibench.md to write the next question against the rows the baseline
    # actually took. Measured both ways - a tied serialize_payload axis became a winning
    # teardown audit on the same repo, and a dead dispatch axis became a winning retention
    # audit on the SAME TYPE.
    requeue_author minibench \
      "REQUESTION: this question does not discriminate. Never a loss." \
      "The read is in $LOOPDIR/minibench.md, with the rows the baseline took and the" \
      "route it took them by. The ANCHOR STAYS and the scenario stays on disk; the next" \
      "draft rewrites the question in place against that credit table."
    return
  fi
  if [ "$VERDICT" = NO-ANCHOR ]; then
    requeue_author minibench \
      "NO ANCHOR: the shown blast set cannot carry the discriminator group. That is an" \
      "authoring defect, not a result. The read is in $LOOPDIR/minibench.md and the next" \
      "cycle profiles a different contract."
    return
  fi
  NEXT=expand
}

do_expand() {
  have_verdict expand "SCENARIO,REQUESTION" || spawn_plan 03-expand.md
  require_verdict expand "SCENARIO,REQUESTION"
  if [ "$VERDICT" = REQUESTION ]; then
    requeue_author expand \
      "REQUESTION from the expansion - step 4 could not survive verbatim." \
      "$LOOPDIR/expand.agent.log names the row or the wording that broke. The anchor stays."
    return
  fi
  # THE MEASURED QUESTION MUST SURVIVE THE EXPANSION. Step 4 and the dependents gold are
  # what the mini-bench measured; move either and that number stops describing this
  # scenario, and the loop is back to expanding on a guess.
  local n; n="$(scenario_steps "$YAML")"
  [ "$n" = 7 ] || {
    echo "[expand] the verdict says SCENARIO but $YAML declares $n steps, not 7 - NOT advancing."
    exit 1; }
  # The per-dependency hand audit is the load-bearing check and nobody downstream
  # catches it, so it is re-run HERE rather than taken on the agent's word.
  python3 "$LIB/gold_audit.py" verify "$YAML" || {
    echo "[expand] the verdict says SCENARIO but the hand audit is not finished - NOT advancing."
    exit 1; }
  # And the rubric, against the judge's own contract. The gold had two gates and the
  # rubric had none, so a rubric authored in an invented shape passed authoring, passed
  # preflight, and died at judge time - with both arms already spent.
  python3 "$LIB/rubric_check.py" "$YAML" || {
    echo "[expand] the rubric does not satisfy the judge - NOT advancing."; exit 1; }
  commit_scenario
  NEXT=preflight
}

# The scenario is the ONE thing this driver commits: minibench, validate and report write
# into results/, which is private by policy. It is committed at EXPAND, after the audit
# passes and after the question has been measured, so a benched cell always has its
# scenario in history at the version it ran. The two-step draft is deliberately not
# committed: it is a probe, and half of them are rewritten.
commit_scenario() {
  local root files
  root="$(cd "$IL_ROOT/.." && pwd)"
  files=("$YAML" "$RUBRIC" "$SCEN_DIR/$REPO.gold-audit.json")
  git -C "$root" add -- "${files[@]}" >/dev/null 2>&1 || return 0
  git -C "$root" diff --cached --quiet -- "${files[@]}" && return 0
  git -C "$root" commit -q -m "bench($VERTICAL): author the $REPO scenario with hand-audited gold" \
    -- "${files[@]}" && echo "## [expand] scenario committed"
}

do_preflight() {
  echo "## [preflight] render prompt (leak check) + Loop-A resolve_oracle (\$0)"
  [ -f "$YAML" ] || { echo "[preflight] $YAML missing - back to author"; state_set "$REPO" author; exit 1; }
  echo "---- rendered agent prompt (verify it leaks NO paths/symbols/counts/tool names, manifesto §13) ----"
  python3 "$LIB/scenario.py" "$YAML" --prompt 2>&1 | sed 's/^/  /' || true
  echo "---- Loop-A preflight (gold must be default-blast-retrievable, manifesto §10) ----"
  bash "$LIB/loopA-scan.sh" preflight "$VERTICAL" "$REPO" 2>&1 || true
  # MCP IS THE ONLY SURFACE (campaign-laws): a check that queries Sense through the CLI
  # measures a surface no arm runs. Blocking - a wrong instrument is worse than no check.
  echo "---- mcp-only law ----"
  python3 "$LIB/mcp_only_check.py" || { state_set "$REPO" author; exit 1; }
  # The judge's rubric contract, checked BEFORE the money rather than at judge time.
  # A scenario authored before this gate existed reaches preflight without ever having
  # been checked, so preflight runs it too - not only expand.
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
  # RESULTS_DIR is model-scoped, so the tree is results/<model>/validation/, not
  # results/validation/. Probing the latter never finds an existing run, so every entry
  # re-spends. Scoping to repo and scenario version lives in runs_for; both were paid for.
  RDIR="$VALIDATION_DIR"
  local scen_ver
  scen_ver="$(python3 "$LIB/scenario_version.py" "$YAML")" || {
    echo "[validate] cannot compute the scenario version for $YAML - NOT advancing."
    echo "           A validation pair that cannot be tied to a scenario is not evidence."
    exit 1; }
  archive_scenario "$YAML"
  echo "## [validate] scenario $scen_ver"
  if [ -n "$(runs_for "$VALIDATION_DIR" "$scen_ver")" ]; then
    echo "## [validate] a validation run already exists for $REPO - not re-running"
    echo "   (delete $VALIDATION_DIR to force a fresh one)"
  else
    echo "## [validate] unscored validation run, both arms x1 (BENCH_VALIDATION=1)"
    # APPEND, never wipe - same reason as the mini-bench: a re-validation used to
    # destroy the pair the previous cycle was diagnosed from, and a diagnosis whose
    # transcripts are gone cannot be re-checked.
    BENCH_VALIDATION=1 KEEP_RUNS=1 START_RUN="$(next_run "$VALIDATION_DIR")" \
      VERTICAL="$VERTICAL" MODELS="$MODELS" RUNS=1 \
      bash "$BENCH_DIR/drivers/runs-variance.sh" "$REPO" || {
        echo "[validate] the validation run FAILED - that is a result, not an obstacle."
        echo "           Read the transcripts before re-running."; exit 1; }
    # POST-CONDITION, not just the exit code: the runner has returned 0 with the run
    # actually failed, and this phase advanced to the PAID bench on it. An exit code is
    # a claim; the transcript on disk is the fact. Generalised into require_verdict for
    # every agent phase - this one guards a runner, so it stays here too.
    if [ -z "$(runs_for "$VALIDATION_DIR" "$scen_ver")" ]; then
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
    requeue_author validate \
      "DO NOT PAY - no gold group can reach +0.50 however well the sense arm does." \
      "This is arithmetic on the validation pair, not a judgment. The lever is the" \
      "QUESTION: re-gold from what the baseline MISSED, on the SAME anchor. The scenario" \
      "stays on disk and stays committed; git holds the version that ran either way."
    return
  fi
  # The pay decision. It is the last thing standing between a scenario and the money,
  # and nothing reviews it afterwards, so it is a judgment phase and gets an agent.
  have_verdict validate "PAY,DO-NOT-PAY" ||
    { archive_read validate; spawn_plan 04-validate.md; }
  require_verdict validate "PAY,DO-NOT-PAY"
  if [ "$VERDICT" = DO-NOT-PAY ]; then
    # KEEP THE ANCHOR, CHANGE THE QUESTION - the same route the mini-bench takes. This
    # used to invalidate everything, rewind to a fresh contract hunt and print a `git rm`
    # for the scenario, which threw away the only measured thing the loop owned at the
    # exact moment it was most informative. Ten mastodon authoring cycles ran that way.
    requeue_author validate \
      "DO NOT PAY - the validation run says this scenario does not discriminate." \
      "The read is in $LOOPDIR/validate.md, with the rows the baseline took and the route" \
      "it took them by. The ANCHOR STAYS and the scenario stays on disk; the next draft" \
      "rewrites the question in place against that credit table."
    return
  fi
  # PAID from here. The authoring budget is spent only by questions that failed to
  # discriminate, so a payable one clears it.
  state_set "$REPO#authcycle" 0
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
    "Then re-enter authoring: bash bench/drivers/vertical-loop.sh $REPO --phase author"
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
    author)    do_author ;;
    minibench) do_minibench ;;
    expand)    do_expand ;;
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
