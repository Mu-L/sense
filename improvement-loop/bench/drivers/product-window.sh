#!/usr/bin/env bash
# product-window.sh - the mechanical driver of cycle 3, the only cycle that touches
# Sense's own code.
#
# THE SCRIPT OWNS THE ORDER, THE STATE, THE GATES AND EVERY SPAWN. It never judges.
# Where a phase needs judgment it spawns a headless agent on one file from
# plans/cycle-3-enhance-the-product/, and refuses to advance until that agent's artifact
# is on disk and its verdict JSON parses (verdict_check.py). An exit code is a claim;
# the artifact is the fact.
#
# The unit of work is one VERTICAL KEY, never a repo: cycle 3 makes Sense able to read a
# stack, and it does that once for the stack, not once per repository.
#
# IT SHARES NOTHING WITH CYCLES 1 AND 2. It never reads or writes verticals/, never reads
# a scored run, never spawns a benched arm and never spends a paid token. Its whole input
# is product-window/<key>/request.json and its whole output is a git branch plus this
# window's pages. That isolation is what lets it run unattended: the only question it asks
# is "this relationship exists in the source; does Sense return it?", which is a fact.
#
# THE HUMAN GATE IS THE PULL REQUEST, AND IT IS THE ONLY ONE. The window runs to a
# committed local branch and a handoff page, then stops. Nothing is pushed, no PR is
# opened, no branch is deleted.
#
# Phases (a state file resumes at the next one; re-run to advance):
#   intake   AGENT 01-intake.md   -> worklist + corpus   [WORKLIST|ALREADY-READY|OUT-OF-SCOPE]
#   truth    branch, then AGENT 02-truth.md -> red tests + probes, then the RED gate
#                                                        [TRUTH|NO-REPRO]
#   build    control counts BEFORE, AGENT 03-build.md, then `make ci` as the gate
#                                                        [BUILD|CANNOT-BUILD]
#   prove    branch binary, re-index the corpus, RUN every probe over MCP, control counts
#            AFTER, then AGENT 04-prove.md                [PROVEN|REVERT]
#   handoff  AGENT 05-handoff.md -> the page a human reads before the PR   [HANDOFF]
#
# Usage:
#   bash bench/drivers/product-window.sh <key>              # run from saved phase
#   bash bench/drivers/product-window.sh <key> --phase build # force one phase
#   bash bench/drivers/product-window.sh <key> --reset       # back to intake
#   bash bench/drivers/product-window.sh --status            # every window's phase
#
# Env: SENSE_CLONES (default ~/Developer/luuuc/oss/sense-benchmark/sense),
#      PLAN_MODEL (the operator model for phase agents; unset means the CLI default).
set -uo pipefail

BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
SENSE_ROOT="$(cd "$IL_ROOT/.." && pwd)"
cd "$IL_ROOT" || exit 1
LIB="$BENCH_DIR/lib"
PLANS="$IL_ROOT/plans/cycle-3-enhance-the-product"
WROOT="$IL_ROOT/product-window"
STATE="$WROOT/.window-state.json"
CLONES="${SENSE_CLONES:-$HOME/Developer/luuuc/oss/sense-benchmark/sense}"
PHASES=(intake truth build prove handoff done)

# ---- args -------------------------------------------------------------------
KEY=""; FORCE_PHASE=""; STATUS=0; RESET=0
while [ $# -gt 0 ]; do
  case "$1" in
    --phase) FORCE_PHASE="$2"; shift 2 ;;
    --status) STATUS=1; shift ;;
    --reset)  RESET=1; shift ;;
    -h|--help) sed -n '2,38p' "$0"; exit 0 ;;
    -*) echo "unknown flag: $1" >&2; exit 64 ;;
    *)  KEY="$1"; shift ;;
  esac
done

# ---- state (JSON map key -> phase) ------------------------------------------
state_get() {
  python3 -c "import json
try: d=json.load(open('$STATE'))
except Exception: d={}
print(d.get('$1',''))"
}
state_set() {
  python3 -c "import json,os
p='$STATE'
try: d=json.load(open(p))
except Exception: d={}
d['$1']='$2'
os.makedirs(os.path.dirname(p),exist_ok=True)
json.dump(d,open(p,'w'),indent=2,sort_keys=True); open(p,'a').write('\n')"
}
state_dump() {
  python3 -c "import json
try: d=json.load(open('$STATE'))
except Exception: d={}
print('\n'.join(f'  {k:22} {v}' for k,v in sorted(d.items())) or '  (none)')"
}

if [ "$STATUS" = 1 ] && [ -z "$KEY" ]; then
  echo "product windows:"; state_dump; exit 0
fi
[ -z "$KEY" ] && { echo "usage: product-window.sh <key> [--phase P] [--reset] [--status]" >&2; exit 64; }

# ---- the stack, from the queue (the one place a key is defined) -------------
QLINE="$(grep "^$KEY|" "$IL_ROOT/verticals.txt" 2>/dev/null | head -1)"
[ -n "$QLINE" ] || { echo "'$KEY' is not in verticals.txt" >&2; exit 66; }
LANG="$(printf '%s' "$QLINE" | cut -d'|' -f2)"
FRAMEWORK="$(printf '%s' "$QLINE" | cut -d'|' -f3)"
TITLE="$(printf '%s' "$QLINE" | cut -d'|' -f4)"
WDIR="$WROOT/$KEY"
BRANCH="feat/$LANG-extractor"

if [ "$STATUS" = 1 ]; then
  # state_get exits 0 with empty output on an unknown key, so `|| echo` never fires here.
  p="$(state_get "$KEY")"; echo "[$KEY] phase: ${p:-intake}"; exit 0
fi
if [ "$RESET" = 1 ]; then state_set "$KEY" intake; echo "[$KEY] reset to phase 'intake'"; exit 0; fi

park() { # message... - stop here, leaving the state where it is
  echo ""
  echo "==================== WINDOW STOPPED ===================="
  printf '%s\n' "$@"
  echo "========================================================"
  exit 0
}

# ---- headless agents --------------------------------------------------------
# Every agent in this system is spawned HERE. A phase agent is told so in its plan and
# runs with Agent disallowed, so it cannot delegate its own judgment.
headless() { # <cwd> <out.json> <prompt>
  local cwd="$1" out="$2" prompt="$3"
  mkdir -p "$(dirname "$out")"
  (
    cd "$cwd" || exit 1
    export CLAUDE_CODE_DISABLE_AUTO_MEMORY=1
    IS_SANDBOX=1 claude -p "$prompt" \
      --output-format json \
      --permission-mode bypassPermissions \
      --disallowed-tools "Agent" \
      ${PLAN_MODEL:+--model "$PLAN_MODEL"}
  ) > "$out" 2> "${out%.json}.log"
}

# spawn_plan <plan-file> - hand one plan to a headless agent, laws first. The laws are
# PREPENDED rather than linked: a plan that ends in "see the laws" is the
# documentation-as-runtime-prompt bug this split exists to remove.
spawn_plan() {
  local plan="$PLANS/$1" name="$PHASE" prompt
  [ -f "$plan" ] || { echo "[$PHASE] missing plan: $plan" >&2; exit 1; }
  mkdir -p "$WDIR"
  echo "## [$PHASE] spawning the phase agent on ${PLANS#"$IL_ROOT/"}/$1"
  prompt="$(cat "$PLANS/laws.md"; echo; cat "$plan"; cat <<EOF

# CONTEXT (resolved; these are also exported in your shell)

    KEY        = $KEY
    LANG       = $LANG
    FRAMEWORK  = $FRAMEWORK
    TITLE      = $TITLE
    WDIR       = $WDIR
    SENSE_ROOT = $SENSE_ROOT
    CLONES     = $CLONES
    BRANCH     = $BRANCH

Work from $IL_ROOT. Write the artifact and the verdict JSON, then stop.
EOF
)"
  KEY="$KEY" LANG="$LANG" FRAMEWORK="$FRAMEWORK" TITLE="$TITLE" WDIR="$WDIR" \
  SENSE_ROOT="$SENSE_ROOT" CLONES="$CLONES" BRANCH="$BRANCH" \
    headless "$IL_ROOT" "$WDIR/$name.agent.json" "$prompt"
  echo "   agent transcript: $WDIR/$name.agent.json (stderr in $name.agent.log)"
}

# require_verdict <phase> <allowed-csv> - the guard. Sets VERDICT. No advance unless the
# JSON parses, names this phase and this key, carries an allowed verdict, and its named
# artifact EXISTS on disk.
require_verdict() {
  local phase="$1" allowed="$2"
  VERDICT="$(python3 "$LIB/verdict_check.py" "$WDIR/$phase.verdict.json" \
      --phase "$phase" --repo "$KEY" --allow "$allowed" --root "$IL_ROOT")" || {
    echo "[$phase] no usable verdict on disk - NOT advancing."
    echo "         Read $WDIR/$phase.agent.log, then re-run this phase."
    exit 1; }
  echo "## [$phase] verdict: $VERDICT"
}

# ---- the control: other languages must not move -----------------------------
# A per-language change cannot alter another language's index, and that is PROVEN, not
# assumed: the php holder-composition fix was verified php-only by rescanning a Go, a Ruby
# and a Python repo and finding identical counts.
#
# The control repos are cloned into the WINDOW's own tree, never into SENSE_CLONES. A
# control scan writes the clone's .sense, and the bench's clones carry indexes cycles 1
# and 2 read: rebuilding one of those with an unmerged binary would corrupt a frozen
# measurement. Counts are read over MCP, because the CLI diverges by design.
CONTROL_LIST="$BENCH_DIR/drivers/control-repos.txt"
CONTROL_DIR="$WDIR/control"

status_counts() { # <clone> <bin> -> "symbols edges", or "" when the probe fails
  SENSE_BIN="$2" python3 "$LIB/mcp_probe.py" "$1" \
    '[{"name":"sense_status","arguments":{}}]' 2>/dev/null | python3 -c '
import json, sys
for line in reversed(sys.stdin.read().splitlines()):
    line = line.strip()
    if line.startswith("{"):
        d = json.loads(line).get("index", {})
        print(d.get("symbols", ""), d.get("edges", ""))
        break
'
}

# record_control <label> <bin> - scan every control repo with THIS binary and record what
# it produced. Re-scanning is the whole point: reading an index the binary never rebuilt
# compares a number with itself and can only pass.
record_control() {
  local label="$1" bin="$2" repo url counts
  mkdir -p "$CONTROL_DIR"
  : > "$WDIR/control.$label"
  while IFS='|' read -r repo url; do
    case "$repo" in ''|\#*) continue ;; esac
    [ -d "$CONTROL_DIR/$repo" ] || git clone --depth 1 "$url" "$CONTROL_DIR/$repo" >/dev/null 2>&1
    (cd "$CONTROL_DIR/$repo" && "$bin" scan -dir . -rebuild) >> "$WDIR/control.scan.log" 2>&1
    counts="$(status_counts "$CONTROL_DIR/$repo" "$bin")"
    printf '%s %s\n' "$repo" "${counts:-UNRUN}" >> "$WDIR/control.$label"
  done < "$CONTROL_LIST"
  echo "## [$PHASE] control ($label, $(basename "$bin")):"; sed 's/^/     /' "$WDIR/control.$label"
}

# ---- phases -----------------------------------------------------------------
do_intake() {
  [ -f "$WDIR/request.json" ] || {
    echo "[intake] no request at $WDIR/request.json" >&2
    echo "         A window opens from bootstrap: bash bench/drivers/bootstrap-run.sh $KEY" >&2
    exit 1; }
  spawn_plan 01-intake.md
  require_verdict intake "WORKLIST,ALREADY-READY,OUT-OF-SCOPE"
  case "$VERDICT" in
    WORKLIST)
      [ -s "$WDIR/corpus.txt" ] || { echo "[intake] WORKLIST with no corpus.txt - NOT advancing." >&2; exit 1; }
      NEXT=truth ;;
    *) state_set "$KEY" done
       park "[$KEY] intake ruled $VERDICT. No product code is owed." \
            "Read: product-window/$KEY/worklist.md" ;;
  esac
}

do_truth() {
  # The branch is the driver's to create: a phase agent that picks its own branch name
  # writes commits nothing downstream can find.
  if [ -n "$(git -C "$SENSE_ROOT" status --porcelain)" ]; then
    park "[$KEY] $SENSE_ROOT has uncommitted changes." \
         "Cycle 3 commits on $BRANCH and will not start on a dirty tree."
  fi
  git -C "$SENSE_ROOT" rev-parse --verify "$BRANCH" >/dev/null 2>&1 \
    && git -C "$SENSE_ROOT" checkout "$BRANCH" \
    || git -C "$SENSE_ROOT" checkout -b "$BRANCH"
  echo "## [truth] on branch $BRANCH"

  spawn_plan 02-truth.md
  require_verdict truth "TRUTH,NO-REPRO"
  [ "$VERDICT" = "NO-REPRO" ] && { state_set "$KEY" done
    park "[$KEY] no worklist row could be made to fail." \
         "Read: product-window/$KEY/truth.md"; }

  # THE RED GATE. No repro, no code: if the suite is green here, the tests do not
  # reproduce anything and the build phase would be measuring nothing.
  echo "## [truth] the red gate: go test ./internal/... must FAIL"
  if (cd "$SENSE_ROOT" && go test ./internal/... > "$WDIR/truth.red.log" 2>&1); then
    echo "[truth] the suite is GREEN on a TRUTH verdict - NOT advancing."
    echo "        The tests reproduce nothing. Read $WDIR/truth.red.log."
    exit 1
  fi
  echo "   red, as required (log: product-window/$KEY/truth.red.log)"
  [ -s "$WDIR/probes.json" ] || { echo "[truth] no probes.json - NOT advancing." >&2; exit 1; }
  NEXT=build
}

do_build() {
  # BEFORE, with the installed binary: main's behaviour on three other languages, measured
  # while this branch still carries nothing but tests.
  record_control before "${SENSE_BIN:-sense}"
  spawn_plan 03-build.md
  require_verdict build "BUILD,CANNOT-BUILD"
  [ "$VERDICT" = "CANNOT-BUILD" ] && { state_set "$KEY" done
    park "[$KEY] the lane cannot be built inside the identity." \
         "Read: product-window/$KEY/build.md"; }

  # The repository's own gate, run by the driver and not taken from the agent's report:
  # build, coverage floor, lint, complexity ledger.
  echo "## [build] make ci"
  (cd "$SENSE_ROOT" && make ci > "$WDIR/build.ci.log" 2>&1) || {
    echo "[build] make ci FAILED on a BUILD verdict - NOT advancing."
    echo "        Read $WDIR/build.ci.log."
    exit 1; }
  echo "   green (log: product-window/$KEY/build.ci.log)"
  NEXT=prove
}

do_prove() {
  local bin="$WDIR/sense-branch" repo url
  echo "## [prove] building the branch binary"
  (cd "$SENSE_ROOT" && go build -o "$bin" ./cmd/sense) || {
    echo "[prove] the branch binary does not build - NOT advancing." >&2; exit 1; }

  mkdir -p "$WDIR/probes"
  while IFS='|' read -r repo url; do
    [ -n "$repo" ] || continue
    [ -d "$CLONES/$repo" ] || git clone --depth 1 "$url" "$CLONES/$repo" || continue
    echo "## [prove] re-indexing $repo with the branch binary"
    (cd "$CLONES/$repo" && "$bin" scan -dir . -rebuild) >> "$WDIR/prove.scan.log" 2>&1
  done < "$WDIR/corpus.txt"

  echo "## [prove] running every probe over MCP"
  SENSE_BIN="$bin" CLONES="$CLONES" WDIR="$WDIR" LIB="$LIB" python3 - <<'PY'
import json, os, subprocess, sys

wdir, clones, lib = os.environ["WDIR"], os.environ["CLONES"], os.environ["LIB"]
rows = json.load(open(os.path.join(wdir, "probes.json")))
out = os.path.join(wdir, "probes")
lines = []
for i, row in enumerate(rows, 1):
    name = row.get("row") or "row-%d" % i
    slug = "".join(c if c.isalnum() or c in "-_" else "-" for c in name)
    clone = os.path.join(clones, row["repo"])
    res = subprocess.run(
        [sys.executable, os.path.join(lib, "mcp_probe.py"), clone,
         json.dumps([row["call"]])],
        capture_output=True, text=True)
    body = res.stdout
    with open(os.path.join(out, slug + ".json"), "w") as fh:
        fh.write(body or res.stderr)
    verdict = "PASS" if row["expect"] in body else "MISS"
    lines.append("%-6s %-28s expect=%s source=%s"
                 % (verdict, name, row["expect"], row.get("source", "")))
with open(os.path.join(out, "summary.txt"), "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("\n".join(lines))
PY

  record_control after "$bin"
  {
    echo "repo  symbols edges (before)  symbols edges (after)"
    join -a1 -a2 "$WDIR/control.before" "$WDIR/control.after" 2>/dev/null \
      || { cat "$WDIR/control.before"; echo "--"; cat "$WDIR/control.after"; }
  } > "$WDIR/probes/control.txt"

  spawn_plan 04-prove.md
  require_verdict prove "PROVEN,REVERT"
  [ "$VERDICT" = "REVERT" ] && { state_set "$KEY" done
    park "[$KEY] the lane did not prove on real code." \
         "Read: product-window/$KEY/prove.md" \
         "The branch $BRANCH is left in place; delete it when you have read the page."; }
  NEXT=handoff
}

do_handoff() {
  spawn_plan 05-handoff.md
  require_verdict handoff "HANDOFF"
  state_set "$KEY" done
  park "[$KEY] the window is finished and stops before the pull request." \
       "" \
       "  page:   product-window/$KEY/handoff.md" \
       "  branch: $BRANCH (committed, not pushed)" \
       "" \
       "Read the page, then open the PR yourself (/pr). Nothing else is owed."
}

# ---- driver: run phases from the current one until a gate stops us ----------
PHASE="${FORCE_PHASE:-$(state_get "$KEY")}"; [ -z "$PHASE" ] && PHASE=intake
echo "[$KEY] entering at phase '$PHASE' ($TITLE, lang=$LANG, framework=$FRAMEWORK)"

while :; do
  NEXT=""
  case "$PHASE" in
    intake)  do_intake ;;
    truth)   do_truth ;;
    build)   do_build ;;
    prove)   do_prove ;;
    handoff) do_handoff ;;
    done)    echo "[$KEY] phase 'done' - nothing to do (use --reset to rerun)"; exit 0 ;;
    *)       echo "unknown phase '$PHASE'" >&2; exit 64 ;;
  esac
  [ -z "$NEXT" ] && { echo "[$KEY] phase '$PHASE' set no next phase - stopping" >&2; exit 1; }
  state_set "$KEY" "$NEXT"
  [ -n "$FORCE_PHASE" ] && { echo "[$KEY] phase '$PHASE' done; next is '$NEXT' (--phase forced single step)"; exit 0; }
  PHASE="$NEXT"
done
