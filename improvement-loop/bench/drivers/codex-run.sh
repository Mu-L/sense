#!/usr/bin/env bash
# codex-run.sh runs the Rails-vertical bench through the Codex CLI agent
# (GPT-5.x on the ChatGPT subscription) instead of the Claude CLI.
#
# Single-prompt over the 7-step scenario (the trustworthy path, same as
# bench-sense-local.sh): renders all steps into one prompt, runs `codex exec
# --json`, normalizes the JSONL into the canonical transcript scorer.py reads
# (via lib/parse-codex-result.py), then score -> judge (--via-cli) -> report.
# Writes to bench/results/{baseline,sense}/<repo>/ so the existing
# score/judge/report/snapshot pipeline runs unchanged.
#
#   bash bench/drivers/codex-run.sh --tool baseline,sense --repo ruby_llm
#   bash bench/drivers/codex-run.sh --repo discourse --model gpt-5.6
#
# Sense reaches Codex through TWO channels and we report which it used:
#   - MCP: registered on the sense arm via `-c mcp_servers.sense=...`
#   - CLI: the `sense` binary on PATH, which GPT-5.x tends to prefer
# (see channels.json per arm). Arm isolation: the BASELINE arm runs with the
# sense binary's dir stripped from PATH (and no MCP), so it cannot reach Sense
# by either channel (the contamination risk called out for Codex).
#
# Prereqs: clones at $SENSE_BENCH_ROOT/{baseline,sense}/<repo>; sense arm
# already `sense scan`-ed; `codex` logged in (`codex login`); `sense` on PATH.
# Judge stays claude-opus-4-7 (set in judge.py); it runs on the Claude
# subscription, untouched by this script.

set -uo pipefail

BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_ROOT="$(cd "$BENCH_DIR/../.." && pwd)"
# Path law: remember whether the operator pinned RESULTS_DIR before we resolve it,
# so the model-scoping default below never clobbers an explicit override.
_RESULTS_DIR_PRESET="${RESULTS_DIR:-}"
# Resolves SCENARIOS_DIR + RESULTS_DIR for the global or VERTICAL bench.
source "$BENCH_DIR/lib/bench-paths.sh"
# Subscription-throttle pacing for this METERED arm (default-on; BENCH_THROTTLE_PACING=0
# = exact pre-pacing behavior). codex exec is single-shot (no retry loop), so the
# exponential backoff does not apply here; inter-session spacing, the per-plan lock,
# the cooldown (gated in runs-variance) and the health log do. The opus runner never
# sources this.
source "$BENCH_DIR/lib/throttle-pacing.sh"
LIB_DIR="$BENCH_DIR/lib"
SENSE_BENCH_ROOT="${SENSE_BENCH_ROOT:-$(cd "$PROJECT_ROOT/.." && pwd)/sense-benchmark}"

TOOLS_CSV="baseline,sense"; REPO=""; MODEL=""; SANDBOX="read-only"   # --model required
SESSION_TIMEOUT=""; KEEP_RAW=0
while [[ $# -gt 0 ]]; do case "$1" in
  --tool) TOOLS_CSV="$2"; shift 2;;
  --repo) REPO="$2"; shift 2;;
  --model) MODEL="$2"; shift 2;;
  --sandbox) SANDBOX="$2"; shift 2;;
  --timeout) SESSION_TIMEOUT="$2"; shift 2;;
  --keep-raw) KEEP_RAW=1; shift;;
  -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0;;
  *) echo "unknown arg: $1" >&2; exit 1;;
esac; done
[ -n "$MODEL" ] || { echo "codex-run.sh: --model is required (ids live in verticals/<key>/arms.txt)" >&2; exit 64; }
[[ -n "$REPO" ]] || { echo "need --repo" >&2; exit 1; }

# Path law (write-side, forward-only): a VERTICAL run is ALWAYS model-scoped, so
# results land at verticals/<v>/results/<model>/<arm>/<repo>. Defaulting BENCH_MODEL
# from the session model prevents a model-less landing. Global runs (no VERTICAL)
# skip this. An operator who pinned BENCH_MODEL or RESULTS_DIR still wins.
if [[ -n "${VERTICAL:-}" && -z "${BENCH_MODEL:-}" && -z "$_RESULTS_DIR_PRESET" ]]; then
  BENCH_MODEL="$MODEL"; unset RESULTS_DIR; source "$BENCH_DIR/lib/bench-paths.sh"
fi

command -v codex >/dev/null || { echo "codex CLI not found in PATH" >&2; exit 1; }
command -v sense >/dev/null || { echo "sense not found in PATH (needed for the sense arm)" >&2; exit 1; }

# Don't let a stray API key bill the wrong wallet; Codex uses its own auth.json.
unset ANTHROPIC_API_KEY BENCHMARK_ANTHROPIC_API_KEY

# macOS ships no `timeout`; prefer GNU, then gtimeout, else no ceiling. The
# seconds get baked into TO once SECS is known (below), so the invocation stays
# `"${TO[@]}" codex …`; on macOS TO=(env) is a no-op prefix (no ceiling).
TIMEOUT_BIN=""
if command -v timeout >/dev/null; then TIMEOUT_BIN=timeout
elif command -v gtimeout >/dev/null; then TIMEOUT_BIN=gtimeout; fi

# Baseline isolation: a PATH with the sense binary's directory removed, so the
# control arm cannot call `sense` (CLI channel); Codex can use the CLI, and
# `sense` lives on the host PATH globally.
SENSE_BIN_DIR="$(dirname "$(command -v sense)")"
SCRUBBED_PATH="$(printf '%s' "$PATH" | tr ':' '\n' | grep -vFx "$SENSE_BIN_DIR" | paste -sd: -)"

# Per-repo lock so two sessions can bench DIFFERENT repos concurrently and only
# ever serialize on the SAME repo's clones+results. If a sweep parent already holds
# this repo's lock (exported BENCH_PACE_LOCK_HELD), this is a no-op. Released on exit.
pace_lock_acquire "repo-$REPO"

SCEN="$SCENARIOS_DIR/$REPO.yaml"
[[ -f "$SCEN" ]] || { echo "no scenario $SCEN" >&2; exit 1; }
SCEN_NAME=$(python3 -c "import yaml;print(yaml.safe_load(open('$SCEN'))['name'])")
PROMPT=$(python3 "$LIB_DIR/scenario.py" "$SCEN" --prompt)
SVER="$(sense --version 2>/dev/null | head -1 || echo '')"
# Provenance (parity with bench-sense-local.sh): run_meta is the on-disk source of
# record. sense_* describe the binary under test (git on PROJECT_ROOT, the Sense
# repo), gated to the sense arm in the emitter. scenario_version is the sha256 of
# the scored files (yaml + rubric sibling); pitch/purpose/link are env-fed.
SENSE_REF="$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo '')"
SENSE_DIRTY="false"; [[ -n "$(git -C "$PROJECT_ROOT" status --porcelain 2>/dev/null)" ]] && SENSE_DIRTY="true"
# The release TAG is the final-data match key, so it must ALWAYS be populated.
# `describe --exact-match` returns EMPTY the moment HEAD moves past the tag -- even
# by a bench-only commit -- which is exactly how every cross-agent sense run on the
# go board ended up with `sense_release: null` and got rejected as "not-release".
# Fall back to the nearest reachable tag and record whether it was exact.
SENSE_RELEASE="$(git -C "$PROJECT_ROOT" describe --tags --exact-match 2>/dev/null || echo '')"
SENSE_RELEASE_EXACT="true"
if [[ -z "$SENSE_RELEASE" ]]; then
  SENSE_RELEASE="$(git -C "$PROJECT_ROOT" describe --tags --abbrev=0 2>/dev/null || echo '')"
  SENSE_RELEASE_EXACT="false"
fi
SENSE_PITCH="${SENSE_PITCH:-}"; SENSE_PURPOSE="${SENSE_PURPOSE:-}"; SENSE_LINK="${SENSE_LINK:-}"
RUBRIC="${SCEN%.yaml}.rubric.yaml"
SCEN_VER=$(python3 - "$SCEN" "$RUBRIC" <<'PY'
import hashlib, os, sys
h = hashlib.sha256()
for p in sys.argv[1:]:
    if os.path.exists(p):
        with open(p, "rb") as f:
            h.update(f.read())
print("sha256:" + h.hexdigest()[:16])
PY
)

# ⚠️ --timeout is for pinning a ceiling, NEVER for rescuing an arm that ran out
# of clock. Can't-finish-at-budget is a RESULT: the arm failed the exam, the exam
# is not invalid (a standing rule). An rc=124 that never REACHED
# synthesis is a real failure to keep and report; only a cut MID-DELIVERY on a
# metered sub is an artifact worth re-running. Classify from the transcript
# (assistant-text chars vs tool calls), not the exit code. Rule: lib/scorer.py
# -> TIME_CEILINGS.
if [[ -n "$SESSION_TIMEOUT" ]]; then SECS_CEILING="$SESSION_TIMEOUT"; else
  SECS_CEILING=$(python3 -c "import sys;sys.path.insert(0,'$LIB_DIR');from scorer import TIME_CEILINGS,DEFAULT_TIME_CEILING;print(max(600,TIME_CEILINGS.get('$REPO',DEFAULT_TIME_CEILING)))")
fi
MATCHED_BUDGET_MULT="${MATCHED_BUDGET_MULT:-1.2}"

IFS=',' read -ra TOOLS <<< "$TOOLS_CSV"
# MATCHED BUDGET: the sense arm ALWAYS runs first when both are selected, because the
# baseline's wall is derived from it. This reorders a user-supplied "baseline,sense"
# rather than rejecting it - the pairing is the point, not a pacing preference, so it
# is structural here and not left to BENCH_SENSE_FIRST.
_reordered=()
for _t in "${TOOLS[@]}"; do [[ "$_t" == sense ]] && _reordered+=("$_t"); done
for _t in "${TOOLS[@]}"; do [[ "$_t" != sense ]] && _reordered+=("$_t"); done
TOOLS=("${_reordered[@]}")
TOOLS_CSV="$(IFS=,; echo "${TOOLS[*]}")"
arm_idx=0
# The arms are a QUEUE, not a fixed list, so a sense arm that did not finish can be
# re-entered ahead of the arms still to come (see the retry below). Held as a
# whitespace-delimited string rather than an array: this runs under `set -u` on bash
# 3.2, where slicing an array down to empty is an unbound-variable error.
arm_queue="${TOOLS[*]}"
sense_retried=0; sense_wall=""; sense_valid=""
while [[ -n "$arm_queue" ]]; do
  tool="${arm_queue%% *}"
  if [[ "$arm_queue" == *" "* ]]; then arm_queue="${arm_queue#* }"; else arm_queue=""; fi
  repo_dir="$SENSE_BENCH_ROOT/$tool/$REPO"
  [[ -d "$repo_dir/.git" ]] || { echo "[codex] SKIP $tool: clone missing at $repo_dir" >&2; continue; }

  # THE BASELINE'S WALL IS THE SENSE ARM'S WALL PLUS A MARGIN. A fixed equal wall measures
  # "who reaches more given generous time"; this measures "given the time it takes WITH the
  # tool, can you get there without it" - the question a user actually has. An explicit
  # --timeout still wins, so a deliberate override is never silently replaced.
  #
  # THE ONE WAY THIS COULD LIE is a sense arm that dies early: a short wall would hand the
  # baseline a rigged budget and manufacture a win. So the derivation accepts only VALID,
  # non-watchdogged sense runs, and when there is none the baseline does not run at all -
  # the cell voids instead of scoring. A win can never be bought by failing fast.
  SECS="$SECS_CEILING"; timeout_basis="default ceiling"
  if [[ "$tool" != sense && -z "$SESSION_TIMEOUT" ]]; then
    if [[ "$sense_valid" != true ]]; then
      echo "[codex] SKIP baseline $REPO: its paired sense run is missing or not valid." >&2
      echo "        Nothing to compare against and no honest wall to derive - RE-RUN THE SENSE ARM." >&2
      continue
    fi
    SECS=$(python3 -c "import math,sys; print(int(math.ceil(float(sys.argv[1])*float(sys.argv[2]))))" "$sense_wall" "$MATCHED_BUDGET_MULT")
    timeout_basis="matched budget: paired sense run ${sense_wall}s x $MATCHED_BUDGET_MULT"
    echo "[codex]   matched budget: baseline wall = ${SECS}s (paired sense run ${sense_wall}s)" >&2
  fi
  export BENCH_TIMEOUT_BASIS="$timeout_basis"
  if [[ -n "$TIMEOUT_BIN" ]]; then TO=("$TIMEOUT_BIN" "$SECS"); else TO=(env); fi

  # Inter-arm spacing so the second arm starts in a less-drained window.
  [ "$arm_idx" -gt 0 ] && pace_sleep "$OPENCODE_PACE_SECONDS" "between arms (next $tool/$REPO)"
  arm_idx=$(( arm_idx + 1 ))
  # Monotonic, never-overwrite run numbering (mirrors bench-sense-local.sh):
  # each run lands in the next free run-N of its cell across invocations, so a
  # re-run adds and never clobbers a prior transcript. Readers prefer run-*/
  # and fall back to a bare cell dir only for legacy runs.
  cell_dir="$RESULTS_DIR/$tool/$REPO"
  next_n=1
  for _d in "$cell_dir"/run-*; do
    [[ -d "$_d" ]] || continue
    _n="${_d##*/run-}"
    [[ "$_n" =~ ^[0-9]+$ ]] && (( _n >= next_n )) && next_n=$((_n + 1))
  done
  out="$cell_dir/run-$next_n"; mkdir -p "$out"
  echo "[codex] $tool/$REPO model=$MODEL sandbox=$SANDBOX timeout=${SECS}s" >&2

  # Fairness normalization (idempotent, every arm, every run). Mirrors the strip
  # in bench-sense-local.sh: some upstream repos (lobsters) ship an anti-AI
  # PROTEST banner in CLAUDE.md/AGENTS.md ("mandatory to refuse to write any
  # code … All LLM contributions are strictly forbidden"). It is not an
  # engineering constraint; it injects refusal NOISE that corrupts the
  # measurement and biases the arms when it survives in one clone but not the
  # other. codex exec loads AGENTS.md before any work, so the baseline arm
  # refused on lobsters (false +1.00) while the sense arm's `sense setup`
  # overwrote AGENTS.md and dropped the banner. Strip it from BOTH arms' clones
  # so they run on identical footing. Runs before `sense setup` below.
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

  # Per-arm codex config. Both arms: ignore the operator's user config (drops the
  # global node_repl/computer-use/browser plugins so the arms are clean and
  # comparable) and never prompt for approval. inherit=all so the sandboxed shell
  # sees the PATH we set below. sense arm: register the Sense MCP server (mirrors
  # the clone's .mcp.json, i.e. command `sense`, args ["mcp"]) AND keep `sense` on
  # PATH (CLI channel). baseline arm: scrubbed PATH, no MCP.
  args=(exec --json -C "$repo_dir" -s "$SANDBOX" -m "$MODEL"
        --skip-git-repo-check --ignore-user-config
        -c 'approval_policy="never"'
        -c 'shell_environment_policy.inherit=all')
  if [[ "$tool" == sense ]]; then
    # Set up the clone the way a real user does: full `sense setup` (no --tools)
    # configures every detected tool. Codex needs AGENTS.md (the routing prose
    # `codex exec` loads before any work); without it the only steering is the
    # MCP serverInstructions blob, which GPT-5.x ignores in `codex exec`, so the
    # arm reaches Sense 0 times even though the MCP server is registered. We
    # deliberately do NOT scope to --tools codex-cli: the scoped form is what
    # silently left this arm un-set-up, and each tool reads only its own file
    # (codex→AGENTS.md, Claude→CLAUDE.md, Cursor→.cursorrules) with identical
    # guidance text, so a full setup never cross-contaminates. Baseline stays
    # isolated by its own clone + scrubbed PATH, untouched by this.
    ( cd "$repo_dir" && sense setup >/dev/null 2>&1 ) \
      || echo "[codex]   WARN: sense setup failed" >&2
    # MCP through the capture shim (byte-transparent tee of every request +
    # full response → $out/sense-io.jsonl; see bench/lib/mcp_tee.py). Bench-only
    # interposition; SENSE_IO_CAPTURE=0 reverts to the direct registration.
    if [[ "${SENSE_IO_CAPTURE:-1}" == 1 ]]; then
      args+=(-c 'mcp_servers.sense.command="python3"'
             -c "mcp_servers.sense.args=[\"$LIB_DIR/mcp_tee.py\",\"--log\",\"$out/sense-io.jsonl\",\"--\",\"sense\",\"mcp\"]")
    else
      args+=(-c 'mcp_servers.sense.command="sense"' -c 'mcp_servers.sense.args=["mcp"]')
    fi
    run_path="$PATH"
  else
    run_path="$SCRUBBED_PATH"
  fi

  raw="$out/codex-raw.jsonl"
  ts_iso=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  start=$(date +%s)
  ( cd "$repo_dir" && PATH="$run_path" "${TO[@]}" codex "${args[@]}" "$PROMPT" ) \
      > "$raw" 2> "$out/codex.log"
  rc=$?
  wall=$(( $(date +%s) - start ))

  python3 "$LIB_DIR/parse-codex-result.py" "$raw" --channels-json "$out/channels.json" \
      > "$out/transcript.json" 2>> "$out/codex.log" || echo "[codex] parse failed ($tool)" >&2
  # Keep claude.log present so downstream tools that glance at it don't choke.
  cp "$out/codex.log" "$out/claude.log" 2>/dev/null || true
  [[ "$KEEP_RAW" == 1 ]] || rm -f "$raw"

  nmcp=$(python3 -c "import json;print(json.load(open('$out/channels.json'))['channels']['mcp_sense'])" 2>/dev/null || echo 0)
  ncli=$(python3 -c "import json;print(json.load(open('$out/channels.json'))['channels']['cli_sense'])" 2>/dev/null || echo 0)
  if [[ "$tool" == sense ]]; then
    if [[ $((nmcp + ncli)) -gt 0 ]]; then echo "[codex]   sense used: mcp=$nmcp cli=$ncli (valid)" >&2
    else echo "[codex]   *** INVALID: sense arm reached Sense 0 times (mcp=0 cli=0) ***" >&2; fi
  fi

  commit=$(git -C "$repo_dir" rev-parse --short HEAD 2>/dev/null || echo "")
  # sense_* binary provenance rides only on the sense arm, mirroring ver.
  ver=""; ref=""; dirty="false"; release=""; rel_exact="true"
  if [[ "$tool" == sense ]]; then
    ver="$SVER"; ref="$SENSE_REF"; dirty="$SENSE_DIRTY"; release="$SENSE_RELEASE"; rel_exact="$SENSE_RELEASE_EXACT"
  fi
  python3 - "$tool" "$REPO" "$SCEN_NAME" "$wall" "$MODEL" "$commit" "$ver" "$rc" \
              "$ts_iso" "$SECS" "$ref" "$dirty" "$release" \
              "$SENSE_PITCH" "$SENSE_PURPOSE" "$SENSE_LINK" \
              "$SCEN_VER" "$SCEN" "$LIB_DIR" "$rel_exact" > "$out/run_meta.json" <<'PY'
import json, os, sys
(tool, repo, scen, wall, model, commit, ver, rc,
 ts, timeout, ref, dirty, release,
 pitch, purpose, link, scen_ver, scen_file, lib_dir, rel_exact) = sys.argv[1:21]
sys.path.insert(0, lib_dir)
from run_validity import WATCHDOG_CODES
rc = int(rc)
meta = {
    "tool": tool, "repo": repo, "scenario": scen,
    "wall_time_seconds": int(wall),
    "session_timeout_seconds": int(timeout),
    "timeout_basis": os.environ.get("BENCH_TIMEOUT_BASIS") or "default ceiling",
    "timestamp": ts,
    "model": model,
    "repo_commit": commit or None, "tool_version": ver or None,
    "harness": "codex", "provider": "codex",
    "auth_mode": "subscription_cli", "mode": "single_prompt",
    "codex_exit_code": rc,
    "cost_usd_note": "codex runs on a ChatGPT subscription; per-token cost not meaningful",
    # Provenance: run_meta is the on-disk source of record. sense_* ride on the
    # sense arm only; the release TAG (not sense_ref) is the final-data match key.
    "sense_ref": ref or None,
    "sense_dirty": dirty == "true",
    "sense_release": release or None,
    "sense_release_exact": (rel_exact == "true") if release else None,
    "sense_pitch": pitch or None,
    "sense_purpose": purpose or None,
    "sense_link": link or None,
    "scenario_version": scen_ver or None,
    "scenario_file": scen_file or None,
    # the validation run is unscored by law; the results root
    # already isolates it, this says so on the artifact itself.
    "scoring": os.environ.get("BENCH_SCORING", "1") != "0",
    # OBSERVATION, not verdict - see bench-sense-local.sh for the full rule.
    # A run the wall clock cut short FAILED the exam; the exam still counts, and
    # run_meta is written before the answer evidence that separates truncated
    # from never-synthesised exists. lib/run_validity.py judges at read time.
    "watchdog_kind": WATCHDOG_CODES.get(rc),
    "valid": None if rc in WATCHDOG_CODES else rc == 0,
    "void_reason": None if rc == 0 or rc in WATCHDOG_CODES else "harness_crash",
}
if rc != 0 and rc not in WATCHDOG_CODES:
    meta["error"] = "codex_session_failed"
print(json.dumps(meta, indent=2))
PY
  echo "[codex]   $tool rc=$rc wall=${wall}s" >&2

  if [[ "$tool" == sense ]]; then
    sense_wall="$wall"
    sense_valid=$(python3 -c "
import json, sys
d = json.load(open(sys.argv[1]))
print('true' if (d.get('valid') is True and not d.get('watchdog_kind')) else 'false')
" "$out/run_meta.json")
    # ONE RETRY FOR A SENSE ARM THAT DID NOT FINISH. The baseline derives its wall from
    # its paired sense run, so a watchdogged or crashed sense run takes the baseline down
    # with it and the whole cell is lost. The retry re-enters the sense arm AHEAD of the
    # arms still queued and lands in the next free run-N, so nothing is overwritten and
    # the baseline pairs with the run that finished. ONE retry, never a loop:
    # cannot-finish-at-budget is a RESULT, and a scenario that needs three attempts is
    # saying something we must not average away.
    if [[ "$sense_valid" != true && $sense_retried -eq 0 ]]; then
      sense_retried=1
      arm_queue="sense${arm_queue:+ $arm_queue}"
      echo "[codex]   sense arm did not finish (valid=$sense_valid) - RETRYING the sense arm once" >&2
      echo "[codex]      A second failure stands as the result; the watchdog is not raised." >&2
    fi
  fi
  # Throttle-health line per session. codex exec yields no per-stream token/answer
  # counts here, so otok/achars are '-'; class is derived from the exit code.
  [ "$rc" -eq 0 ] && hclass=ok || hclass=session_failed
  pace_health_log "$REPO" "$tool" "$wall" "-" "-" "1" "$hclass"

  # Post-run agent survey (sense arm, clean runs only; process: loops doc
  # 08-agent-survey.md). Resumes the SAME codex session with
  # lib/survey_prompt.md, normalizes through parse-codex-result.py so
  # survey_verify.py reads the canonical stream-json shape. Plain (non-tee)
  # MCP registration so a stray survey-turn sense call never pollutes
  # sense-io.jsonl. Fires AFTER run_meta.json; writes survey.json, NEVER
  # transcript.json; a survey failure never fails the run.
  if [[ "$tool" == sense && $rc -eq 0 ]]; then
    survey_sid=$(grep -oE '"session_id": *"[^"]+"' "$out/transcript.json" | head -1 | cut -d'"' -f4 || true)
    if [[ -n "$survey_sid" ]]; then
      echo "[codex]   survey turn (post-scoring artifact)" >&2
      survey_raw="$out/survey-raw.jsonl"
      # NB: `codex exec resume` takes a SUBSET of `codex exec`'s flags. It has no
      # -C/--cd (the working root comes from the `cd "$repo_dir"` below) and no
      # -s/--sandbox: `codex exec resume --help` lists only -c, -i, -m,
      # --skip-git-repo-check, --ignore-user-config, --json, -o. Passing -s made
      # clap read it as the PROMPT positional and kill the turn with
      # "tip: to pass '-s' as a value, use '-- -s'", so every survey since has been
      # empty. The sandbox posture is carried by -c sandbox_mode instead, which
      # keeps the survey turn as read-only as the scored turn it resumes.
      survey_args=(exec resume "$survey_sid" --json -m "$MODEL"
                   -c "sandbox_mode=\"$SANDBOX\""
                   --skip-git-repo-check --ignore-user-config
                   -c 'approval_policy="never"'
                   -c 'shell_environment_policy.inherit=all'
                   -c 'mcp_servers.sense.command="sense"' -c 'mcp_servers.sense.args=["mcp"]')
      ( cd "$repo_dir" && PATH="$run_path" "${TO[@]}" codex "${survey_args[@]}" "$(cat "$LIB_DIR/survey_prompt.md")" ) \
          > "$survey_raw" 2>> "$out/codex.log" || echo "[codex]   WARN: survey turn failed (run unaffected)" >&2
      python3 "$LIB_DIR/parse-codex-result.py" "$survey_raw" > "$out/survey.json" 2>> "$out/codex.log" \
          || echo "[codex]   WARN: survey parse failed" >&2
      [[ "$KEEP_RAW" == 1 ]] || rm -f "$survey_raw"
      VERTICAL="${VERTICAL:-}" python3 "$LIB_DIR/survey_verify.py" --run "$out" \
          --append "$RESULTS_DIR/surveys.jsonl" \
          || echo "[codex]   WARN: survey verify failed (run unaffected)" >&2
    else
      echo "[codex]   WARN: no session_id in transcript.json -- survey skipped" >&2
    fi
  fi
done

SJ=(--tool "$TOOLS_CSV" --repo "$REPO")
bash "$BENCH_DIR/score.sh"  "${SJ[@]}"
bash "$BENCH_DIR/judge.sh"  "${SJ[@]}" --via-cli
bash "$BENCH_DIR/report.sh" --md
echo "[codex] done, see bench/results/{${TOOLS_CSV}}/$REPO/ (channels.json per arm)" >&2
