#!/usr/bin/env bash
# vertical-loop.sh - the mechanical per-repo loop of the vertical bench (manifesto
# docs/loops/01-repo-authoring.md .. 03-repo-diagnosis.md), with the human gates kept load-bearing.
#
# It chains the FREE/mechanical steps and PAUSES where someone must act. The line
# goal.md draws: Loop A (the product-gap detectors) and the run
# mechanics are automated; Loop B (the scenario + the tie diagnosis) is DRAFTED and
# tuned by the AI agent running this loop, and the human REVIEWS it adversarially,
# asynchronously (anomalies / inconsistencies). The human is the integrity anchor,
# not the author. The script cannot write a scenario itself, so at the scout pause it
# hands control back to the AI agent to author <repo>.yaml + <repo>.rubric.yaml +
# gold, then resumes once those exist. The only true HUMAN decision is the cost
# NOTE: loops 1-3 have had NO human gate since 2026-07-31; --yes is retained as a
# no-op so existing invocations keep working.
#
# Phases (a state file resumes at the next one; re-run to advance):
#   index      ensure-index.sh                                    [auto]
#   scout      seam_hunt.py --propose  ->  GATE: draft+review scenario+gold
#   preflight  render prompt + loopA preflight (resolve_oracle)   [auto]
#   validate   both arms x1, UNSCORED (BENCH_VALIDATION=1)        [auto]
#   bench      runs-variance.sh Opus x2 (PAID)                    [auto]
#   report     pergroup.py verdict; WIN -> harvest, else          -> Loop 3
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
VDIR="$BENCH_DIR/../verticals/$VERTICAL"
SCEN_DIR="$VDIR/scenarios"
STATE="$VDIR/.loop-state.json"
PHASES=(index scout preflight validate bench report harvest done)

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

if [ "$STATUS" = 1 ]; then echo "[$VERTICAL/$REPO] phase: $(state_get "$REPO" || echo index)"; exit 0; fi
if [ "$RESET" = 1 ]; then state_set "$REPO" index; echo "[$VERTICAL/$REPO] reset to phase 'index'"; exit 0; fi

# yaml field reader (contract_symbol / contract_file), no yaml dep needed
yaml_field() { grep -E "^$1:" "$YAML" 2>/dev/null | head -1 | sed -E "s/^$1:[[:space:]]*//" | tr -d '"'"'"; }

gate() { # message... - record we're parked at the current phase and stop
  echo ""
  echo "==================== PAUSE - ACTION NEEDED ===================="
  printf '%s\n' "$@"
  echo "=============================================================="
  echo "Re-run: bash bench/drivers/vertical-loop.sh $REPO   (resumes at this phase)"
  exit 0
}

# ---- phase implementations (echo the next phase on success, or gate+exit) ----
do_index() {
  echo "## [index] ensure $REPO index matches the current scan engine"
  bash "$LIB/ensure-index.sh" "$REPO" || { echo "[index] FAILED - fix the index before continuing"; exit 1; }
  NEXT=scout
}

do_scout() {
  if [ -f "$YAML" ] && [ -f "$RUBRIC" ]; then
    echo "## [scout] scenario present ($REPO.yaml + .rubric.yaml) - advancing"
    NEXT=preflight; return
  fi
  echo "## [scout] propose the contract + candidate gold (advisory)"
  local sym="$SYMBOL"; [ -z "$sym" ] && sym="$(yaml_field contract_symbol)"
  local fh="$FILE_HINT"; [ -z "$fh" ] && fh="$(yaml_field contract_file)"
  if [ -n "$sym" ]; then
    local fflag=(); [ -n "$fh" ] && fflag=(--file "$fh")
    # ${arr[@]+...} guards the empty-array case under `set -u` on bash 3.2 (macOS).
    python3 "$LIB/seam_hunt.py" "$CLONE" "$sym" --conf 0.7 --propose ${fflag[@]+"${fflag[@]}"} 2>&1 || true
  else
    echo "  (no --symbol given and no scenario yet; pick the central abstraction from"
    echo "   repos.md, then: vertical-loop.sh $REPO --symbol <Sym> [--file <path>])"
  fi
  gate \
    "LOOP 1 AUTHORING - the agent running this loop authors these now; no human review:" \
    "  - $YAML            (7 neutral steps, audit step forces per-dep file:line)" \
    "  - $RUBRIC   (matching rubric)" \
    "  - gold: + contract_symbol:/contract_file: in the yaml (curate the candidate above)" \
    "Guidance: docs/loops/01-repo-authoring.md + docs/scenarios/crafting.md." \
    "Before it leaves Loop 1: adversary probe, scenario.py --prompt, audit_scenarios.py," \
    "gold_confidence_check.py at 0.3 AND 0.7, and the per-dependency hand audit."
}

do_preflight() {
  echo "## [preflight] render prompt (leak check) + Loop-A resolve_oracle (\$0)"
  [ -f "$YAML" ] || { echo "[preflight] $YAML missing - back to scout"; state_set "$REPO" scout; exit 1; }
  echo "---- rendered agent prompt (verify it leaks NO paths/symbols/counts/tool names, manifesto §13) ----"
  python3 "$LIB/scenario.py" "$YAML" --prompt 2>&1 | sed 's/^/  /' || true
  echo "---- Loop-A preflight (gold must be default-blast-retrievable, manifesto §10) ----"
  bash "$LIB/loopA-scan.sh" preflight "$VERTICAL" "$REPO" 2>&1 || true

  # The COST GATE was REMOVED 2026-07-31: loops 1-3 have no human gate. Runs go
  # through a subscription by default, so what a cycle spends is quota against the
  # weekly reset. The leak check and the oracle above are what decide now.
  NEXT=bench
}

do_validate() {
  # Loop 2's validation run (docs/loops/02-repo-run.md): both arms, ONE run each,
  # unscored, before anything paid. BENCH_VALIDATION=1 routes it to a separate
  # results root and stamps "scoring": false, so no scorer can ever see it.
  local vdir_out="$VDIR/results/validation"
  if [ -d "$vdir_out" ] && [ -n "$(find "$vdir_out" -name 'transcript.json' 2>/dev/null | head -1)" ]; then
    echo "## [validate] a validation run already exists for this scenario - not re-running"
    echo "   (delete $vdir_out to force a fresh one)"
    NEXT=bench; return
  fi
  echo "## [validate] unscored validation run, both arms x1 (BENCH_VALIDATION=1)"
  BENCH_VALIDATION=1 VERTICAL="$VERTICAL" MODELS="$MODELS" RUNS=1 \
    bash "$BENCH_DIR/drivers/runs-variance.sh" "$REPO" || {
      echo "[validate] the validation run FAILED - that is a result, not an obstacle."
      echo "           Read the transcripts before re-running (02-repo-run.md)."; exit 1; }
  echo "## [validate] done. Read it before paying:"
  echo "   RESULTS_DIR=$vdir_out python3 bench/lib/credit_table.py $REPO"
  echo "   If the baseline assembled the set, DO NOT PAY - go back to Loop 1 authoring."
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
  if echo "$out" | grep -q '^VERDICT: WIN'; then
    echo "  -> WIN (discriminator >= +0.50). Banking Loop-A harvest."
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
      "SIX CYCLES WITH NO MOVEMENT - swap this repo (docs/loops/03-repo-diagnosis.md)." \
      "The slot takes its OWN declared backup from slate.json, not the next slate repo." \
      "Write the swap dossier to LEDGER.md as loop3/$REPO/swap, then reset:" \
      "  bash bench/drivers/vertical-loop.sh <backup-repo> --reset"
  fi
  gate \
    "BELOW +0.50 - hand the cell to Loop 3 diagnosis (docs/loops/03-repo-diagnosis.md)." \
    "1. STRUGGLE READ (every run): spawn the bench-struggle-read agent on the table above." \
    "   It returns scenario material - the rows the baseline missed and what it spent instead." \
    "2. TAXONOMY (sub-floor only): spawn bench-evaluator; branches in order, cheapest first." \
    "   \$0 gold-retarget re-scores without re-benching:" \
    "     python3 bench/lib/scorer.py <run_dir> $YAML bench" \
    "   Branch 2 (re-author + re-bench) is the only paid lever and needs 1/3/4 exhausted." \
    "Then re-enter Loop 1: bash bench/drivers/vertical-loop.sh $REPO --phase scout"
}

do_harvest() {
  echo "## [harvest] Loop-A transcript_miss over the paid transcripts (\$0, advisory)"
  bash "$LIB/loopA-scan.sh" harvest "$VERTICAL" "$HEADLINE_MODEL" 2>&1 || true
  echo ""
  echo "## $REPO - Definition of Done (manifesto §14), confirm by hand:"
  echo "   [ ] discriminator >= +0.50 (favored +0.80) on Opus 4.8 x$RUNS"
  echo "   [ ] Sense adopted its tools (mcp_count>0), no hallucinated cites, baseline floor legit"
  echo "   [ ] efficiency reported, scenario human-readable + leak-free, article matches the numbers"
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
