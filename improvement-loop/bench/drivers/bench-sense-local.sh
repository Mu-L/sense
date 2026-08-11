#!/usr/bin/env bash
# bench-sense-local.sh - sense-only dev iteration loop for the bench.
#
# Purpose:
#   Tight feedback loop while developing sense itself. Rebuilds the host
#   `sense` binary from the current working tree, runs the full 6-scenario
#   bench against the host clones, and emits the same scored/judged/report
#   artifacts the docker sweep produces - but free, against the local
#   claude CLI subscription instead of API credit.
#
#   Run it after every meaningful change to sense, then diff the outputs
#   below to see whether the change improved or regressed scores:
#
#     bench/results/sense/<repo>/scored.json
#       └─ deterministic metrics (cost, latency, tool-call counts,
#          rubric hits) - best for cheap A/B between iterations.
#     bench/results/sense/<repo>/judged.json
#       └─ LLM-judge scores per rubric criterion - noisier but captures
#          quality dimensions the deterministic checks can't.
#     bench/results/report.md / report.json
#       └─ aggregated view; the place to eyeball direction of travel.
#
#   run_meta.json carries `auth_mode: subscription_cli` so these dev runs
#   are distinguishable from any API-billed runs sitting in results/.
#
# Default flow:
#   1. rebuild sense → /path-to/the-user/.local/bin/sense
#   2. full `sense setup` (every detected tool) in each selected repo
#      (regenerates CLAUDE.md, .mcp.json, .claude/skills+agents+hooks,
#      AGENTS.md, etc. from the new binary; idempotent)
#   3. `sense scan --dir <repo> -embed` in each selected repo
#      (rebuilds .sense/ so MCP queries see new indexer behavior)
#   4. for each scenario: cd into the pinned-commit host clone, run
#      `claude -p <prompt>` against the OAuth subscription, capture
#      transcript.json + run_meta.json + claude.log
#   5. score.sh → judge.sh (--via-cli) → report.sh (md + json)
#
#   --no-build skips steps 1-3 ("trust current state"). --no-setup and
#   --no-scan skip step 2 or 3 individually (e.g. for tight MCP-layer
#   iteration where the index would be identical).
#
# Prereqs:
#   - $SENSE_BENCH_ROOT/sense/<repo>/.git at the pinned commit, with
#     full `sense setup` already applied (.mcp.json, .claude/, CLAUDE.md,
#     AGENTS.md present).
#   - `claude` CLI logged into a Pro/Max subscription (`claude /login`).
#   - Go toolchain on PATH (for the rebuild step).
#
# Caveats vs. the docker sweep:
#   - Pro/Max 5h rate windows will throttle a full 6-scenario sweep with
#     --runs > 1. Failed sessions land with claude_exit_code != 0; rerun
#     later to overwrite.
#   - Not a substitute for the docker sweep when publishing numbers -
#     host PATH, MCP servers, and shell hooks aren't isolated.
#   - Refresh cost: a default-flow run reindexes every selected repo,
#     so nextjs alone adds ~20min. Pass --no-scan when iterating on the
#     MCP/tool layer to keep the inner loop tight.
#
# History:
#   Host mode was removed from the multi-tool sweep because
#   Docker is canonical for cross-tool reproducibility. This script
#   restores host mode for sense-only iteration, where the docker rebuild
#   cycle (~minutes per change) is too slow for tight loops.

set -euo pipefail

BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_ROOT="$(cd "$BENCH_DIR/../.." && pwd)"
# Path law: remember whether the operator pinned RESULTS_DIR before we resolve it,
# so the model-scoping default below never clobbers an explicit override.
_RESULTS_DIR_PRESET="${RESULTS_DIR:-}"
# Resolves SCENARIOS_DIR + RESULTS_DIR for the global or VERTICAL bench.
source "$BENCH_DIR/lib/bench-paths.sh"
LIB_DIR="$BENCH_DIR/lib"

SENSE_BENCH_ROOT="${SENSE_BENCH_ROOT:-$(cd "$PROJECT_ROOT/.." && pwd)/sense-benchmark}"
TOOLS_CSV="sense"   # default: sense-only (backward compatible); --tool overrides
BASELINE_MCP="$LIB_DIR/baseline-mcp.json"   # empty MCP config for the baseline arm

SENSE_INSTALL_PATH="${SENSE_INSTALL_PATH:-$HOME/.local/bin/sense}"

FILTER_REPOS=""
DRY_RUN=false
NUM_RUNS=1
MODEL=""                  # no default: --model is required (ids live in verticals/<key>/arms.txt); override with --model
SESSION_TIMEOUT=""
FORCE=0
DO_BUILD=true
DO_SETUP=true
DO_SCAN=true

usage() {
  cat <<EOF
Usage: bench-sense-local.sh [--tool baseline,sense] [--repo r1,r2] [--runs N]
                            [--model MODEL] [--timeout SECS] [--no-build]
                            [--no-setup] [--no-scan] [--dry-run] [--force]

Local subscription bench loop for sense and/or baseline. Runs each selected
tool's host clones via the claude CLI subscription (no Docker, no API key),
then score → judge → report. Default --tool is sense (the historical
sense-only dev loop). With --tool baseline,sense it runs both arms and the
report shows the baseline-vs-sense comparison. The sense arm rebuilds the
binary and refreshes per-repo setup + indexes; the baseline arm runs the
bare clone with MCP forced empty. Writes to bench/results/<tool>/.

Default per-iteration refresh (after build, before run):
  - full \`sense setup\` (every detected tool) in each selected repo
    (regenerates CLAUDE.md, .mcp.json, .claude/skills/, .claude/agents/,
    .claude/settings.json hook block from the freshly-built binary).
  - \`sense scan --dir <repo> -embed\` in each selected repo
    (rebuilds the .sense/ index so MCP query results reflect the new
    binary's indexer behavior).

Options:
  --tool      Comma-separated tools to run (baseline, sense). Default: sense.
              baseline = bare clone, no MCP; sense = built binary + index.
  --no-build  Skip the rm + go build of $SENSE_INSTALL_PATH AND the
              follow-on setup + scan steps. Use when you know everything
              is already current.
  --no-setup  Skip just the per-repo \`sense setup\` refresh. Use when
              your change only touches the indexer/MCP layer and you
              don't want setup templates to change between runs.
  --no-scan   Skip just the per-repo \`sense scan -embed\` rebuild. Use
              for MCP-layer iterations where the index would be identical;
              saves ~20min on nextjs alone.
  --repo      Comma-separated repo filter (e.g. flask,gin)
  --runs      Runs per scenario for variance estimation (default: 1)
  --model     Session model id, REQUIRED - there is no default, because a
              hardcoded one outlives the model. Take it from the vertical's
              arms.txt (verticals/<key>/arms.txt). Anthropic ids carry no tag
              colon; ids with one (e.g. deepseek-v4-pro:cloud) route to the
              local Ollama daemon (host via \$OLLAMA_BASE_URL, default
              http://localhost:11434).
  --timeout   Per-session wall-clock seconds (default: scorer.py TIME_CEILINGS).
              NEVER raise this to rescue an arm that ran out of clock:
              can't-finish-at-budget is a RESULT, not an invalid run. See the
              CAN'T-FINISH-AT-BUDGET block in lib/scorer.py -> TIME_CEILINGS.
  --dry-run   Show what would run without launching claude
  --force     Forward to judge.sh (re-judge even if up-to-date)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tool)     TOOLS_CSV="$2"; shift 2 ;;
    --repo)     FILTER_REPOS="$2"; shift 2 ;;
    --runs)     NUM_RUNS="$2"; shift 2 ;;
    --model)    MODEL="$2"; shift 2 ;;
    --timeout)  SESSION_TIMEOUT="$2"; shift 2 ;;
    --no-build) DO_BUILD=false; DO_SETUP=false; DO_SCAN=false; shift ;;
    --no-setup) DO_SETUP=false; shift ;;
    --no-scan)  DO_SCAN=false; shift ;;
    --dry-run)  DRY_RUN=true; shift ;;
    --force)    FORCE=1; shift ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done
[ -n "$MODEL" ] || { echo "bench-sense-local.sh: --model is required (ids live in verticals/<key>/arms.txt)" >&2; exit 64; }

# Path law (write-side, forward-only): a VERTICAL run is ALWAYS model-scoped, so
# results land at verticals/<v>/results/<model>/<arm>/<repo>. Defaulting BENCH_MODEL
# from the session model here is what prevents a model-less landing (the dolt
# incident, 2026-07). Global runs (no VERTICAL) are single-model by design and skip
# this. An operator who pinned BENCH_MODEL or RESULTS_DIR still wins.
if [[ -n "${VERTICAL:-}" && -z "${BENCH_MODEL:-}" && -z "$_RESULTS_DIR_PRESET" ]]; then
  BENCH_MODEL="$MODEL"; unset RESULTS_DIR; source "$BENCH_DIR/lib/bench-paths.sh"
fi

# Resolve --tool into an ordered, validated list. Only baseline and sense are
# host-runnable on the subscription; other tools need their docker images.
# Default (sense) preserves the historical sense-only behavior.
TOOLS=()
while IFS= read -r _t; do
  [[ -z "$_t" ]] && continue
  case "$_t" in
    baseline|sense) TOOLS+=("$_t") ;;
    *) echo "Unknown --tool '$_t' (subscription path supports: baseline, sense)" >&2; exit 1 ;;
  esac
done < <(echo "$TOOLS_CSV" | tr ',' '\n')
[[ ${#TOOLS[@]} -gt 0 ]] || { echo "No tools selected" >&2; exit 1; }
# MATCHED BUDGET: the sense arm ALWAYS runs first when both are selected, because the
# baseline's wall is derived from it (see matched_budget_timeout). This reorders a
# user-supplied "baseline,sense" rather than rejecting it - the pairing is the point.
_reordered=()
for _t in "${TOOLS[@]}"; do [[ "$_t" == sense ]] && _reordered+=("$_t"); done
for _t in "${TOOLS[@]}"; do [[ "$_t" != sense ]] && _reordered+=("$_t"); done
TOOLS=("${_reordered[@]}")
TOOLS_CSV="$(IFS=,; echo "${TOOLS[*]}")"   # normalized (dedup not needed; user-ordered)

# THE BASELINE'S WALL IS THE SENSE ARM'S WALL PLUS A MARGIN. A fixed equal wall measures
# "who reaches more given generous time"; this measures "given the time it takes WITH the
# tool, can you get there without it" - the question a user actually has. It adapts per
# repo, per model and per scenario with no hand-tuned table, and it recomputes whenever
# the scenario changes, because the lookup is keyed on scenario_version.
#
# THE ONE WAY THIS COULD LIE is a sense arm that dies early: a short wall would hand the
# baseline a rigged budget and manufacture a win. So the derivation accepts only VALID,
# non-watchdogged sense runs, and when there are none the baseline does not run at all -
# the cell voids instead of scoring. A win can never be bought by failing fast.
MATCHED_BUDGET_MULT="${MATCHED_BUDGET_MULT:-1.2}"

# THE PAIRING TABLE. Run N of the baseline is held to the wall of run N of the sense arm,
# not to an aggregate: this is a PAIRED comparison, and pairing each run with its own
# partner is what makes "the time it took with the tool" mean anything per row. Entries are
# "<scenario>|<run_idx>|<wall>|<valid>", appended by the sense arm as each run finishes.
# A flat indexed array, because bash 3.2 has no associative arrays.
SENSE_RUNS=()

sense_run_record() {  # <scenario> <run_idx> <wall> <valid>
  SENSE_RUNS+=("$1|$2|$3|$4")
}

# sense_run_lookup <scenario> <run_idx> - echo "<wall> <valid>" for that run, or nothing.
sense_run_lookup() {
  # THE LAST RECORD FOR A (scenario, run_idx) WINS. The table appends, and a retried sense
  # arm re-records the same key, so returning the FIRST match handed the baseline the
  # attempt that failed and skipped it even though the retry had just succeeded - measured:
  # discourse run-5 finished cleanly in 459s and its baseline was skipped anyway.
  # Echo-only, never a non-zero return: the caller reads this inside a `paired=$(...)`
  # assignment under `set -e`, where a failing substitution would kill the run.
  local e hit=""
  for e in "${SENSE_RUNS[@]:-}"; do
    case "$e" in
      "$1|$2|"*) hit="${e#"$1|$2|"}" ;;
    esac
  done
  [ -n "$hit" ] && echo "$hit" | tr '|' ' '
  return 0
}

WANT_SENSE=false
for _t in "${TOOLS[@]}"; do [[ "$_t" == sense ]] && WANT_SENSE=true; done

# Explicit: subscription mode. Don't let .env's BENCHMARK_ANTHROPIC_API_KEY
# leak in as ANTHROPIC_API_KEY - claude prefers the key over OAuth and we'd
# silently bill the wrong wallet. We do not source lib/load-env.sh here.
unset ANTHROPIC_API_KEY BENCHMARK_ANTHROPIC_API_KEY

command -v claude >/dev/null || { echo "claude CLI not found in PATH" >&2; exit 1; }

# Provider routing. Anthropic model ids carry no tag colon (claude-opus-5,
# claude-fable-5); Ollama ids always do (deepseek-v4-pro:cloud, qwen3.5:9b).
# A colon in $MODEL therefore routes the SESSION to the local Ollama daemon's
# Anthropic-compatible endpoint instead of the subscription. The override is
# applied only inside the session subshell (below), so judge.sh keeps running
# on the Anthropic OAuth subscription - the pinned opus-4-7 judge must not move.
OLLAMA_BASE_URL="${OLLAMA_BASE_URL:-http://localhost:11434}"
case "$MODEL" in
  *:*) PROVIDER=ollama ;;
  *)   PROVIDER=anthropic ;;
esac
if [[ "$PROVIDER" == ollama ]] && ! $DRY_RUN; then
  curl -fsS "$OLLAMA_BASE_URL/api/version" >/dev/null 2>&1 \
    || { echo "Ollama daemon unreachable at $OLLAMA_BASE_URL (model '$MODEL' needs it; run 'ollama serve')" >&2; exit 1; }
fi

# macOS doesn't ship `timeout`; the docker entrypoint never hit this
# because containers run Linux. Prefer GNU `timeout`, then `gtimeout`
# from coreutils, else fall back to a pure-bash watchdog so the script
# has no brew dependency.
bash_timeout() {
  local secs=$1; shift
  "$@" &
  local pid=$!
  ( sleep "$secs"; kill -TERM "$pid" 2>/dev/null; sleep 5; kill -KILL "$pid" 2>/dev/null ) &
  local watchdog=$!
  local rc=0
  wait "$pid" 2>/dev/null || rc=$?
  kill "$watchdog" 2>/dev/null
  wait "$watchdog" 2>/dev/null
  # SIGTERM-killed → report 124 to match GNU timeout's exit code.
  [[ $rc -eq 143 ]] && rc=124
  return $rc
}
if command -v timeout >/dev/null; then
  TIMEOUT_CMD=(timeout)
elif command -v gtimeout >/dev/null; then
  TIMEOUT_CMD=(gtimeout)
else
  TIMEOUT_CMD=(bash_timeout)
fi

matches_filter() {
  local value="$1" filter="$2"
  [[ -z "$filter" ]] && return 0
  echo "$filter" | tr ',' '\n' | grep -qx "$value"
}

timestamp() { date +%Y-%m-%dT%H:%M:%S; }
# Underscored so the `log` macOS system command (in /usr/bin) can't shadow
# it through any caller path. Bash function lookup beats PATH lookup, but
# our own ordering-of-definition has bitten us once already.
_log() { echo "[$(timestamp)] [sense-local] $*" >&2; }

# Auto-memory isolation. Claude Code keeps per-project "auto memory" under
# ~/.claude/projects/<flattened-git-root>/memory/MEMORY.md, keyed off the git
# repo root (here the clone dir) with /, . and _ collapsed to -. That path
# persists across runs, so one session's note (e.g. "Sense tools ARE available,
# call ToolSearch to surface them") would leak into every later session in the
# same repo and bias adoption. We wipe it before each session AND set
# CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 in the session env so nothing reads or
# writes it. repo_dir is always under $SENSE_BENCH_ROOT, so this can never
# touch a real project's memory.
session_memory_dir() {
  local repo_dir="$1"
  printf '%s/.claude/projects/%s/memory' "$HOME" "$(printf '%s' "$repo_dir" | sed 's#[/._]#-#g')"
}

# Build sense from the current working tree. Pattern is `rm` then
# `go build -o` (NOT `cp` over an existing binary - that breaks macOS
# code signing and the resulting binary fails to launch). Install path
# matches /Users/luc/.local/bin/sense which takes precedence over
# ~/go/bin in this user's PATH.
build_sense() {
  _log "Building sense → $SENSE_INSTALL_PATH"
  mkdir -p "$(dirname "$SENSE_INSTALL_PATH")"
  rm -f "$SENSE_INSTALL_PATH"
  ( cd "$PROJECT_ROOT" && go build -o "$SENSE_INSTALL_PATH" ./cmd/sense )
  "$SENSE_INSTALL_PATH" --version >&2
}

# Build + binary checks are sense-arm concerns; skip entirely for baseline-only.
if $WANT_SENSE; then
  if $DO_BUILD && ! $DRY_RUN; then
    build_sense
  fi
  command -v sense >/dev/null || { echo "sense binary not found in PATH (built to $SENSE_INSTALL_PATH - is its dir on PATH?)" >&2; exit 1; }
  installed_sense="$(command -v sense)"
  if [[ "$installed_sense" != "$SENSE_INSTALL_PATH" ]]; then
    _log "WARN: PATH resolves to $installed_sense, not $SENSE_INSTALL_PATH - bench will use the PATH one"
  fi
fi

compute_session_budget() {
  local repo="$1"
  python3 -c "
import sys; sys.path.insert(0, '$LIB_DIR')
from scorer import BUDGET_PER_REPO, DEFAULT_BUDGET_USD
print(BUDGET_PER_REPO.get('$repo', DEFAULT_BUDGET_USD))
"
}

compute_session_timeout() {
  local repo="$1"
  if [[ -n "$SESSION_TIMEOUT" ]]; then echo "$SESSION_TIMEOUT"; return; fi
  python3 -c "
import sys; sys.path.insert(0, '$LIB_DIR')
from scorer import TIME_CEILINGS, DEFAULT_TIME_CEILING
print(max(300, TIME_CEILINGS.get('$repo', DEFAULT_TIME_CEILING)))
"
}

# Discover scenarios (filtered by --repo if given).
scenarios=()
scenario_files=()
for scenariofile in "$SCENARIOS_DIR"/*.yaml; do
  [[ -f "$scenariofile" ]] || continue
  # A rubric is NOT a scenario. Discovery used to accept any *.yaml carrying a `repo:`
  # key, so a rubric that happened to declare one was handed to scenario.py as a
  # scenario and killed the whole run on "missing required field: name". It only
  # worked because one rubric happened to omit the field - that is luck, not a rule.
  [[ "$scenariofile" == *.rubric.yaml ]] && continue
  name="$(basename "$scenariofile" .yaml)"
  repo=$(python3 -c "import yaml; print(yaml.safe_load(open('$scenariofile'))['repo'])" 2>/dev/null || echo "")
  [[ -z "$repo" ]] && continue
  matches_filter "$repo" "$FILTER_REPOS" || continue
  scenarios+=("$name")
  scenario_files+=("$scenariofile")
done

[[ ${#scenarios[@]} -gt 0 ]] || { echo "No scenarios matched" >&2; exit 1; }

scenario_repo_file() {
  local name="$1" idx
  for idx in "${!scenarios[@]}"; do
    [[ "${scenarios[$idx]}" == "$name" ]] && { echo "${scenario_files[$idx]}"; return; }
  done
}

total_runs=$((${#TOOLS[@]} * ${#scenarios[@]} * NUM_RUNS))
_log "Local bench: [${TOOLS_CSV}] x ${#scenarios[@]} scenarios x $NUM_RUNS runs = $total_runs sessions"
_log "Auth: claude CLI subscription (ANTHROPIC_API_KEY explicitly unset)"
_log "Repos under: $SENSE_BENCH_ROOT/<tool>/"

if $DRY_RUN; then
  echo ""
  echo "=== DRY RUN ==="
  i=0
  for tool in "${TOOLS[@]}"; do
    for s in "${scenarios[@]}"; do
      f=$(scenario_repo_file "$s")
      r=$(python3 -c "import yaml; print(yaml.safe_load(open('$f'))['repo'])")
      for k in $(seq 1 $NUM_RUNS); do
        i=$((i+1))
        echo "  [$i/$total_runs] tool=$tool scenario=$s repo=$r run=$k"
      done
    done
  done
  exit 0
fi

SENSE_VERSION=""
# Sense-binary provenance: which commit built the binary under test, whether the
# tree was dirty, and the exact release tag if any. PROJECT_ROOT is the Sense repo
# (see line 68). These make a dev/side-effect run self-describing instead of a bare
# "0.0.0-dev"; the release TAG (not the raw ref) is the final-data query's match key.
SENSE_REF=""; SENSE_DIRTY="false"; SENSE_RELEASE=""; SENSE_RELEASE_EXACT="true"
if $WANT_SENSE; then
  SENSE_VERSION="$(sense --version 2>/dev/null | head -1 || echo "")"
  SENSE_REF="$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "")"
  [[ -n "$(git -C "$PROJECT_ROOT" status --porcelain 2>/dev/null)" ]] && SENSE_DIRTY="true"
  # The release TAG is the final-data match key, so it must ALWAYS be populated.
  # `describe --exact-match` returns EMPTY the moment HEAD moves past the tag -- even
  # by a bench-only commit -- which is exactly how every cross-agent sense run on the
  # go board ended up with `sense_release: null` and got rejected as "not-release".
  # Fall back to the nearest reachable tag and record whether it was exact.
  SENSE_RELEASE="$(git -C "$PROJECT_ROOT" describe --tags --exact-match 2>/dev/null || echo "")"
  SENSE_RELEASE_EXACT="true"
  if [[ -z "$SENSE_RELEASE" ]]; then
    SENSE_RELEASE="$(git -C "$PROJECT_ROOT" describe --tags --abbrev=0 2>/dev/null || echo "")"
    SENSE_RELEASE_EXACT="false"
  fi
fi
# Human-meaning provenance, env-fed and null-safe: why this binary exists (pitch,
# one-line purpose, link). Absent -> null; the run stays identified by SENSE_REF.
SENSE_PITCH="${SENSE_PITCH:-}"; SENSE_PURPOSE="${SENSE_PURPOSE:-}"; SENSE_LINK="${SENSE_LINK:-}"

# Resolve the unique set of selected repos once - both setup and scan
# loops walk it, and a repo is shared across runs only by its scenario,
# not by its index in the scenarios array.
selected_repos=()
for s in "${scenarios[@]}"; do
  f=$(scenario_repo_file "$s")
  r=$(python3 -c "import yaml; print(yaml.safe_load(open('$f'))['repo'])")
  case " ${selected_repos[*]:-} " in *" $r "*) ;; *) selected_repos+=("$r") ;; esac
done

# Per-repo full `sense setup` (every detected tool). Regenerates CLAUDE.md,
# .mcp.json, .claude/skills/, .claude/agents/, .claude/settings.json, AGENTS.md
# hook block from the just-built binary. Idempotent - safe to re-run.
# Catches setup-template regressions and keeps all repos on the same
# version of the integration surface for fair comparison.
if $DO_SETUP && $WANT_SENSE; then
  _log "Refreshing sense setup in selected repos (all detected tools)"
  for r in "${selected_repos[@]}"; do
    d="$SENSE_BENCH_ROOT/sense/$r"
    [[ -d "$d/.git" ]] || { _log "  SKIP setup: clone missing at $d"; continue; }
    _log "  setup $r ..."
    # Full setup (no --tools): configures every detected tool so the shared
    # sense clone is ready for whichever harness runs it (claude/codex/opencode).
    # Scoping to one tool is what silently left the codex arm un-set-up; each
    # tool reads only its own file, so full setup never cross-contaminates.
    ( cd "$d" && sense setup >/dev/null )
  done
fi

# Per-repo `sense scan --dir <repo> -embed`. Rebuilds the .sense/
# index so MCP query results reflect the freshly-built indexer. Slow
# (nextjs ~20min) but the user opted into this as default - skip with
# --no-scan when iterating on the MCP/tool-description layer.
if $DO_SCAN && $WANT_SENSE; then
  _log "Rescanning selected repos with the freshly-built sense"
  for r in "${selected_repos[@]}"; do
    d="$SENSE_BENCH_ROOT/sense/$r"
    [[ -d "$d/.git" ]] || { _log "  SKIP scan: clone missing at $d"; continue; }
    _log "  scanning $r ..."
    t0=$(date +%s)
    sense scan --dir "$d" -embed
    _log "  done $r in $(( $(date +%s) - t0 ))s"
  done
fi

# Run loop - inlined host runner. Mirrors bench/global/docker/entrypoint.sh:81-140
# so the artifacts it writes (transcript.json, run_meta.json, claude.log)
# are byte-compatible with what score.sh / judge.sh / report.sh expect.
run_num=0
passed=0
failed=0
# THE ARM LOOP IS INNERMOST, so the arms INTERLEAVE: sense run-1, baseline run-1, sense
# run-2, baseline run-2. It used to be outermost, which put every sense run in the first
# half of the batch and every baseline run in the second - arm perfectly confounded with
# time-of-run, so any drift (rate limiting, machine load, a model-side change) landed on
# one arm only. Pairs now run back to back and share their conditions. It also means a
# complete pair exists after two runs instead of after the whole matrix, and the baseline
# reads a pairing-table entry written seconds earlier rather than across the batch.
for scenario_name in "${scenarios[@]}"; do
    scenario_file=$(scenario_repo_file "$scenario_name")
    # Scenario version = sha256 of the scored files (yaml + rubric sibling). Scoped
    # to the bytes that get scored, so unrelated repo edits do not move it; distinct
    # across every reshape (dolt v1..v3.2) that a shared filename otherwise hid.
    # ONE recipe, in lib: vertical-loop.sh matches validation runs on this value, and
    # two copies of a hash are how the stamper and the matcher stop agreeing.
    rubric_file="${scenario_file%.yaml}.rubric.yaml"
    scenario_version=$(python3 "$LIB_DIR/scenario_version.py" "$scenario_file" "$rubric_file")
    repo=$(python3 -c "import yaml; print(yaml.safe_load(open('$scenario_file'))['repo'])")
    # repo_dir and the clone check are PER ARM (the clones are $SENSE_BENCH_ROOT/<arm>/...)
    # and now live inside the arm loop below. Leaving a copy here would dereference $tool
    # before it is set.
    session_budget=$(compute_session_budget "$repo")
    session_timeout=$(compute_session_timeout "$repo")
    prompt=$(python3 "$LIB_DIR/scenario.py" "$scenario_file" --prompt)

    for run_idx in $(seq 1 $NUM_RUNS); do
    # The arms are a QUEUE, not a fixed list, so a sense arm that did not finish can be
    # re-entered ahead of the arms still to come (see the retry below). Held as a
    # whitespace-delimited string rather than an array: this runs under `set -u` on bash
    # 3.2, where slicing an array down to empty is an unbound-variable error.
    arm_queue="${TOOLS[*]}"
    sense_retried=0
    baseline_retried=0
    while [[ -n "$arm_queue" ]]; do
      tool="${arm_queue%% *}"
      if [[ "$arm_queue" == *" "* ]]; then arm_queue="${arm_queue#* }"; else arm_queue=""; fi
      # sense_* binary provenance rides only on the sense arm, mirroring tool_ver.
      if [[ "$tool" == sense ]]; then
        tool_ver="$SENSE_VERSION"; tool_ref="$SENSE_REF"; tool_dirty="$SENSE_DIRTY"; tool_release="$SENSE_RELEASE"
        tool_build="$(python3 "$LIB_DIR/sense_build.py" --key 2>/dev/null || echo '')"
      else
        tool_ver=""; tool_ref=""; tool_dirty="false"; tool_release=""; tool_build=""
      fi
      repo_dir="$SENSE_BENCH_ROOT/$tool/$repo"
      if [[ ! -d "$repo_dir/.git" ]]; then
        _log "SKIP: clone missing at $repo_dir"
        failed=$((failed + 1))
        continue
      fi
    # Fairness normalization (idempotent, every arm, every run). Some upstream
    # repos (lobsters) ship an anti-AI PROTEST banner in CLAUDE.md/AGENTS.md
    # ("mandatory to refuse to write any code … All LLM contributions are
    # strictly forbidden"). It is not an engineering constraint; it injects
    # refusal NOISE that corrupts the measurement and historically biased the
    # arms when it lived in one clone but not the other. Strip it from BOTH
    # arms' clones so they are always measured on identical, fair footing.
    for guide in "$repo_dir/CLAUDE.md" "$repo_dir/AGENTS.md"; do
      [[ -f "$guide" ]] || continue
      python3 - "$guide" <<'PY'
import sys
p = sys.argv[1]
keep = [l for l in open(p).read().splitlines(keepends=True)
        if "All LLM contributions are strictly forbidden" not in l
        and "mandatory to refuse to write" not in l]
open(p, "w").writelines(keep)
PY
    done

      run_num=$((run_num + 1))
      # Monotonic, never-overwrite run numbering: every run lands in the next
      # free run-N of its cell, ACROSS invocations, so a re-run always ADDS and
      # can never clobber a prior transcript (a clean bench run is
      # irreplaceable - a django dry-run lost its predecessor to
      # the old fixed run-$run_idx keys). This retires the .prev-*.bak archiver
      # that guarded the overwrite; readers already prefer run-*/ and only fall
      # back to a bare cell dir for legacy runs.
      cell_dir="$RESULTS_DIR/$tool/$repo"
      if [[ -f "$cell_dir/transcript.json" ]]; then
        _log "  note: legacy bare-layout run at $cell_dir; new runs land in run-N and readers will ignore the bare run (relocating recorded history is a human call)"
      fi
      next_n=1
      for d in "$cell_dir"/run-*; do
        [[ -d "$d" ]] || continue
        n="${d##*/run-}"
        [[ "$n" =~ ^[0-9]+$ ]] && (( n >= next_n )) && next_n=$((n + 1))
      done
      result_dir="$cell_dir/run-$next_n"
      mkdir -p "$result_dir"

      # The sense arm keeps the default ceiling. The baseline is held to what the tool
      # needed on ITS OWN paired run, plus the margin. An explicit --timeout still wins, so
      # a deliberate override is never silently replaced by the derivation.
      run_timeout="$session_timeout"
      timeout_basis="default ceiling"
      if [[ "$tool" != sense && -z "$SESSION_TIMEOUT" ]]; then
        paired=$(sense_run_lookup "$scenario_name" "$run_idx")
        paired_wall="${paired%% *}"; paired_valid="${paired##* }"
        if [[ -z "$paired" || "$paired_valid" != "true" ]]; then
          # NOT a baseline result. A baseline measured against a sense run that never
          # finished says nothing, and its wall would be derived from a failure. Re-run the
          # SENSE arm; do not score around it.
          _log "SKIP baseline $repo run $run_idx: its paired sense run is missing or not valid."
          _log "     Nothing to compare against and no honest wall to derive - RE-RUN THE SENSE ARM."
          failed=$((failed + 1))
          continue
        fi
        run_timeout=$(python3 -c "import math,sys; print(int(math.ceil(float(sys.argv[1])*float(sys.argv[2]))))" "$paired_wall" "$MATCHED_BUDGET_MULT")
        timeout_basis="matched budget: paired sense run ${paired_wall}s x $MATCHED_BUDGET_MULT"
        _log "  matched budget: baseline wall = ${run_timeout}s (paired sense run ${paired_wall}s)"
      fi
      export BENCH_TIMEOUT_BASIS="$timeout_basis"

      _log "[$run_num/$total_runs] tool=$tool scenario=$scenario_name repo=$repo timeout=${run_timeout}s"

      claude_args=(
        -p "$prompt"
        --verbose
        --output-format stream-json
        --permission-mode bypassPermissions
        --disallowed-tools "Agent"
      )
      # Both arms run under --strict-mcp-config so the operator's GLOBAL personal
      # MCP servers (claude.ai Gmail/Calendar/Drive) cannot leak in. Baseline gets
      # an empty config (no-code-intel control); the sense arm gets ONLY the
      # clone's own .mcp.json (the sense server written by `sense setup`). Without
      # this isolation the sense arm loaded 37 tools across 4 servers, which (a)
      # pollutes the measurement and (b) inflated the tool list enough to push the
      # Sense tools into deferred/ToolSearch loading, tanking adoption.
      if [[ "$tool" == baseline ]]; then
        claude_args+=(--strict-mcp-config --mcp-config "$BASELINE_MCP")
      else
        # Sense-arm MCP goes through the capture shim (bench/lib/mcp_tee.py): a
        # byte-transparent tee of every MCP request + full response into the
        # run's sense-io.jsonl (raw material for the misuse/budget-trim/loss
        # analyses). Capture lives ONLY in the bench: we derive a per-run config
        # here instead of ever editing the clone's .mcp.json, and nothing in the
        # product references the shim. SENSE_IO_CAPTURE=0 reverts to direct.
        if [[ "${SENSE_IO_CAPTURE:-1}" == 1 ]]; then
          python3 - "$repo_dir/.mcp.json" "$result_dir" "$LIB_DIR/mcp_tee.py" <<'PY'
import json, sys
src, outdir, shim = sys.argv[1], sys.argv[2], sys.argv[3]
cfg = json.load(open(src))
s = cfg["mcpServers"]["sense"]
orig = [s["command"]] + list(s.get("args", []))
s["command"] = sys.executable
s["args"] = [shim, "--log", f"{outdir}/sense-io.jsonl", "--"] + orig
json.dump(cfg, open(f"{outdir}/mcp-tee.json", "w"), indent=2)
PY
          claude_args+=(--strict-mcp-config --mcp-config "$result_dir/mcp-tee.json")
        else
          claude_args+=(--strict-mcp-config --mcp-config "$repo_dir/.mcp.json")
        fi
      fi
      # No --max-budget-usd: it bills against API quota, not subscription.
      # Wall-clock ceiling via `timeout` is the only enforcement we get here.
      [[ -n "$MODEL" ]] && claude_args+=(--model "$MODEL")

      # Blank-memory start: wipe any auto-memory a previous session persisted
      # for this clone (see session_memory_dir). Belt-and-suspenders alongside
      # CLAUDE_CODE_DISABLE_AUTO_MEMORY below - guarantees a clean start even if
      # the env var name ever changes.
      mem_dir="$(session_memory_dir "$repo_dir")"
      if [[ -d "$mem_dir" ]]; then
        _log "  wiping stale auto-memory: $mem_dir"
        rm -rf "$mem_dir"
      fi

      timestamp_iso=$(date -u +%Y-%m-%dT%H:%M:%SZ)
      start=$(date +%s)
      set +e
      (
        cd "$repo_dir"
        # Ollama models: point claude at the local daemon's Anthropic-compatible
        # endpoint, scoped to THIS subshell only so judge.sh stays on the
        # subscription. Anthropic models fall through to the unset-key OAuth path.
        if [[ "$PROVIDER" == ollama ]]; then
          export ANTHROPIC_BASE_URL="$OLLAMA_BASE_URL"
          export ANTHROPIC_AUTH_TOKEN="ollama"
          export ANTHROPIC_API_KEY=""
        fi
        # Blank slate: never read or write Claude Code auto-memory, so each
        # session starts with no carried-over per-project notes.
        export CLAUDE_CODE_DISABLE_AUTO_MEMORY=1
        # IS_SANDBOX=1 lets bypassPermissions through without root, matching
        # how the docker entrypoint runs as root inside the container.
        # $run_timeout, NOT $session_timeout: under matched budget the baseline's wall is
        # derived per run, and enforcing the default here while run_meta.json recorded the
        # derived number would make the artifact lie about the conditions it ran under.
        IS_SANDBOX=1 "${TIMEOUT_CMD[@]}" "$run_timeout" claude "${claude_args[@]}"
      ) > "$result_dir/transcript.json" 2> "$result_dir/claude.log"
      rc=$?
      set -e
      end=$(date +%s)
      wall=$((end - start))
      repo_commit=$(git -C "$repo_dir" rev-parse --short HEAD 2>/dev/null || echo "")

      SENSE_BUILD_KEY="$tool_build" \
      python3 - "$tool" "$repo" "$scenario_name" "$wall" "$session_budget" \
                  "$timestamp_iso" "$tool_ver" "$repo_commit" \
                  "$MODEL" "$rc" "$PROVIDER" \
                  "$tool_ref" "$tool_dirty" "$tool_release" \
                  "$SENSE_PITCH" "$SENSE_PURPOSE" "$SENSE_LINK" \
                  "$scenario_version" "$scenario_file" "$run_timeout" \
                  "$LIB_DIR" \
                  > "$result_dir/run_meta.json" <<'PY'
import json, os, sys
(tool, repo, scen, wall, budget, ts, ver, commit, model, rc, provider,
 sref, sdirty, srel, spitch, spurpose, slink,
 scen_ver, scen_file, timeout, lib_dir) = sys.argv[1:22]
sys.path.insert(0, lib_dir)
from run_validity import WATCHDOG_CODES
rc = int(rc)
meta = {
    "tool": tool, "repo": repo, "scenario": scen,
    "wall_time_seconds": int(wall),
    "session_timeout_seconds": int(timeout),
    "timeout_basis": os.environ.get("BENCH_TIMEOUT_BASIS") or "default ceiling",
    "max_budget_usd": float(budget),
    "timestamp": ts,
    "tool_version": ver or None,
    "repo_commit": commit or None,
    "model": model or None,
    "provider": provider,
    "claude_exit_code": rc,
    "auth_mode": "ollama_cli" if provider == "ollama" else "subscription_cli",
    # Provenance: run_meta is the on-disk source of record. sense_* ride on the
    # sense arm only; the release TAG (not sense_ref) is the final-data match key.
    "sense_ref": sref or None,
    "sense_dirty": sdirty == "true",
    "sense_release": srel or None,
    "sense_build_key": os.environ.get("SENSE_BUILD_KEY") or None,
    # the validation run is unscored by law: a x1 unscored number
    # is a sample and may not settle a win, a tie, or an article. The results root already
    # isolates it; this says so on the artifact itself, for anything copied out of the tree.
    "scoring": os.environ.get("BENCH_SCORING", "1") != "0",
    # ^ the EXPIRY key: sha256 of the binary. sense_ref+sense_dirty cannot separate
    # two dirty trees on one commit, which is what a Loop 7 spike-rebuild is
    # (lib/sense_build.py).
    "sense_pitch": spitch or None,
    "sense_purpose": spurpose or None,
    "sense_link": slink or None,
    "scenario_version": scen_ver or None,
    "scenario_file": scen_file or None,
    # OBSERVATION, not verdict. `valid` used to be `rc == 0`, which voided every
    # run the wall clock cut short -- a failed exam is not an invalid exam
    # (standing ruling; lib/scorer.py -> TIME_CEILINGS). run_meta is written
    # BEFORE scoring, so the answer evidence that separates "truncated a real
    # answer" from "never reached synthesis" does not exist yet. Record the exit
    # code and let lib/run_validity.classify_run judge at read time; only a
    # non-watchdog failure is knowable here.
    "watchdog_kind": WATCHDOG_CODES.get(rc),
    "valid": None if rc in WATCHDOG_CODES else rc == 0,
    "void_reason": None if rc == 0 or rc in WATCHDOG_CODES else "harness_crash",
}
if provider == "ollama":
    # claude can't know ollama.com pricing; its cost_usd is an anthropic-rate
    # estimate, not the real bill. Flag it so downstream/articles don't quote it.
    meta["cost_usd_note"] = "claude-estimated at anthropic rates; real bill is on ollama.com"
if rc != 0 and rc not in WATCHDOG_CODES:
    meta["error"] = "claude_session_failed"
print(json.dumps(meta, indent=2))
PY

      # Register this run in the pairing table so the baseline's run N can derive its wall
      # from sense's run N. `valid` is read back off the artifact just written rather than
      # recomputed here: run_meta is the source of record, and two derivations of the same
      # fact are how a harness starts disagreeing with itself. A watchdogged run is NOT
      # valid for this purpose - its wall is where the clock stopped it, not what the work
      # took, and deriving a baseline budget from it would understate the task.
      if [[ "$tool" == sense ]]; then
        run_valid=$(python3 -c "
import json, sys
d = json.load(open(sys.argv[1]))
print('true' if (d.get('valid') is True and not d.get('watchdog_kind')) else 'false')
" "$result_dir/run_meta.json")
        sense_run_record "$scenario_name" "$run_idx" "$wall" "$run_valid"
        # ONE RETRY FOR A SENSE ARM THAT DID NOT FINISH. The baseline derives its wall from
        # its paired sense run, so a watchdogged or crashed sense run takes the baseline
        # down with it and the whole cell is lost - measured: discourse run-1 hit the clock
        # at 481s, the baseline was skipped for want of an honest wall, and validate halted
        # on a pair that did not exist. The retry re-enters the sense arm AHEAD of the arms
        # still queued, lands in the next free run-N (nothing is overwritten) and re-records
        # the pairing entry for this run_idx, so the baseline pairs with the run that
        # finished. ONE retry, never a loop: cannot-finish-at-budget is a RESULT, and a
        # scenario that needs three attempts is saying something we must not average away.
        if [[ "$run_valid" != true && $sense_retried -eq 0 ]]; then
          sense_retried=1
          arm_queue="sense${arm_queue:+ $arm_queue}"
          _log "  sense arm did not finish (valid=$run_valid) - RETRYING the sense arm once"
          _log "     A second failure stands as the result; the watchdog is not raised."
          park_superseded "$result_dir"
        fi
      else
        # ONE RETRY FOR A BASELINE THE HARNESS VOIDED - AND THE PREDICATE IS NOT THE SENSE
        # ONE. The sense arm retries on `valid != true` OR a watchdog, because its wall
        # budgets the baseline and a censored wall budgets nothing. The baseline retries
        # ONLY on `valid: false`, the five artifact classes where the harness (not the arm)
        # decided the outcome. A watchdogged baseline is VALID - `truncated_at_ceiling` and
        # `never_reached_synthesis` are the arm's own result and the WIN CONDITION - so it
        # is never re-run: that would delete the very outcome the bench exists to measure.
        # Measured 2026-08-10: php-laravel/coolify's baseline died on a provider api_error
        # at 447s of its 550s wall with zero answer text, nothing retried it, and the phase
        # tried to judge a pair with one arm in it.
        base_valid=$(python3 -c "
import json, sys
d = json.load(open(sys.argv[1]))
print('true' if d.get('valid') is not False else 'false')
" "$result_dir/run_meta.json")
        if [[ "$base_valid" != true && $baseline_retried -eq 0 ]]; then
          baseline_retried=1
          arm_queue="$tool${arm_queue:+ $arm_queue}"
          _log "  baseline arm VOID (valid=false) - the harness decided this, not the arm."
          _log "     RETRYING the baseline once. A second void does not stand as a result:"
          _log "     the phase halts on an incomplete pair and asks for human direction."
          park_superseded "$result_dir"
        fi
      fi

      if [[ $rc -eq 0 ]]; then
        _log "  done (wall=${wall}s)"
        passed=$((passed + 1))
      elif [[ $rc -eq 124 || $rc -eq 125 || $rc -eq 126 ]]; then
        # Out of clock is a RESULT, not a harness failure - never call it FAIL
        # here, that framing is what got a 38k-char answer reported as "no
        # result". Class is derived at read time (lib/run_validity.py).
        _log "  wall (exit=$rc, wall=${wall}s) - ran out of clock; scored as a result"
        passed=$((passed + 1))
      else
        _log "  FAIL (exit=$rc, wall=${wall}s) - see $result_dir/claude.log"
        failed=$((failed + 1))
      fi

      # Post-run agent survey (sense arm, clean runs only): one --resume turn
      # asking the agent to rate Sense with transcript-citable evidence
      # (lib/survey_prompt.md). Fires AFTER run_meta.json (wall time final);
      # writes survey.json, NEVER transcript.json, so the scored artifact stays
      # byte-identical. Resumes on the clone's PLAIN .mcp.json (not the tee
      # shim) so a stray survey-turn sense call can never pollute
      # sense-io.jsonl. A survey failure never fails the run. survey_verify.py
      # stamps each cited instance verified/confabulated against the transcript
      # and appends to the model root's surveys.jsonl.
      if [[ "$tool" == sense && $rc -eq 0 ]]; then
        survey_sid=$(grep -oE '"session_id":"[^"]+"' "$result_dir/transcript.json" | head -1 | cut -d'"' -f4 || true)
        if [[ -n "$survey_sid" && -f "$LIB_DIR/survey_prompt.md" ]]; then
          _log "  survey turn (post-scoring artifact)"
          survey_args=(
            -p "$(cat "$LIB_DIR/survey_prompt.md")"
            --resume "$survey_sid"
            --verbose
            --output-format stream-json
            --permission-mode bypassPermissions
            --disallowed-tools "Agent"
            --strict-mcp-config --mcp-config "$repo_dir/.mcp.json"
          )
          [[ -n "$MODEL" ]] && survey_args+=(--model "$MODEL")
          set +e
          (
            cd "$repo_dir"
            if [[ "$PROVIDER" == ollama ]]; then
              export ANTHROPIC_BASE_URL="$OLLAMA_BASE_URL"
              export ANTHROPIC_AUTH_TOKEN="ollama"
              export ANTHROPIC_API_KEY=""
            fi
            export CLAUDE_CODE_DISABLE_AUTO_MEMORY=1
            IS_SANDBOX=1 "${TIMEOUT_CMD[@]}" 300 claude "${survey_args[@]}"
          ) > "$result_dir/survey.json" 2>> "$result_dir/claude.log" \
            || _log "  WARN: survey turn failed (run unaffected)"
          VERTICAL="${VERTICAL:-}" python3 "$LIB_DIR/survey_verify.py" --run "$result_dir" \
            --append "$RESULTS_DIR/surveys.jsonl" \
            || _log "  WARN: survey parse/verify failed (run unaffected)"
          set -e
        else
          _log "  WARN: no session_id in transcript.json - survey skipped"
        fi
      fi
    done
    done
  done

_log ""
_log "=== Run phase complete: $passed passed, $failed failed ==="
_log ""

# Downstream pipeline. score/judge are filtered to the selected tools so we
# don't rescore/rejudge other tools' results sitting in bench/results/. report
# always renders the full tree (same as bench.sh), so any other rows from
# prior sweeps stay visible alongside.
SCORE_JUDGE_ARGS=(--tool "$TOOLS_CSV")
[[ -n "$FILTER_REPOS" ]] && SCORE_JUDGE_ARGS+=(--repo "$FILTER_REPOS")

JUDGE_ARGS=("${SCORE_JUDGE_ARGS[@]}" --via-cli)
[[ "$FORCE" == "1" ]] && JUDGE_ARGS+=(--force)

bash "$BENCH_DIR/score.sh"  "${SCORE_JUDGE_ARGS[@]}"
bash "$BENCH_DIR/judge.sh"  "${JUDGE_ARGS[@]}"
bash "$BENCH_DIR/report.sh" --md
bash "$BENCH_DIR/report.sh" --json
