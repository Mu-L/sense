#!/usr/bin/env bash
# loop1-bootstrap.sh - Loop 1 end to end (docs/loops/01-vertical-bootstrap.md).
#
#   preflight -> stamp -> evaluate -> record -> return
#
# Loop 1 is a SCRIPT, not an agent: every step is mechanical. It stamps the
# vertical, verifies the stamp with a separate evaluator, writes the
# loop1/scaffold LEDGER entry from facts it just measured, and returns a result
# JSON the caller can branch on. No model is in the path.
#
# CONTRACT, so this is callable from anywhere (an orchestrator, CI, a headless
# agent): human-readable progress goes to STDERR, the result JSON is the only
# thing on STDOUT. Exit 0 means BOOTSTRAPPED and the JSON says so; any non-zero
# exit still prints a JSON with the failure reason.
#
#   bash bench/drivers/loop1-bootstrap.sh <key> --lang <lang> [--title "T"]
#                                         [--framework <name>] [--reason "..."]
#
#   bash bench/drivers/loop1-bootstrap.sh php-laravel --lang php --title "PHP / Laravel"
#   result=$(bash bench/drivers/loop1-bootstrap.sh go --lang go) && echo "$result" | jq .next
#
# Loop 1 never touches repos: no clone root, no GitHub token, no sense binary is
# needed here. Its only environment is this repo and python3, and the preflight
# says so out loud rather than failing three steps later.

set -uo pipefail
BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
REPO_ROOT="$(cd "$IL_ROOT/.." && pwd)"
cd "$IL_ROOT" || exit 1

say() { printf '%s\n' "$*" >&2; }

# A single place that emits the return value, so every exit path returns one.
emit() {  # emit <status> <exit-code> [key] [lang] [note]
  python3 - "$@" <<'PY'
import json, sys
status, code = sys.argv[1], int(sys.argv[2])
key  = sys.argv[3] if len(sys.argv) > 3 else ""
lang = sys.argv[4] if len(sys.argv) > 4 else ""
note = sys.argv[5] if len(sys.argv) > 5 else ""
out = {"loop": "1-vertical-bootstrap", "status": status, "vertical": key,
       "lang": lang, "note": note}
if status == "BOOTSTRAPPED":
    out["vertical_dir"] = f"verticals/{key}"
    out["next"] = {"loop": "2-repo-admission",
                   "command": f"bash bench/drivers/loop2-hunt.sh {key} --write",
                   "needs": "verticals/%s/pool.txt - Loop 2 writes it; Loop 1 never touches repos" % key}
print(json.dumps(out, indent=1, sort_keys=True))
PY
  exit "$2"
}

KEY=""; LANG_ARG=""; TITLE=""; FRAMEWORK=""; REASON=""
while [ $# -gt 0 ]; do
  case "$1" in
    --lang)      LANG_ARG="$2"; shift 2 ;;
    --title)     TITLE="$2"; shift 2 ;;
    --framework) FRAMEWORK="$2"; shift 2 ;;
    --reason)    REASON="$2"; shift 2 ;;
    -h|--help)   sed -n '2,24p' "$0" >&2; exit 0 ;;
    -*) say "unknown flag: $1"; emit USAGE 64 ;;
    *)  KEY="$1"; shift ;;
  esac
done
[ -z "$KEY" ] && { say "usage: loop1-bootstrap.sh <key> --lang <lang> [--title T]"; emit USAGE 64; }
[ -z "$LANG_ARG" ] && { say "missing --lang (the internal/extract/<lang> dir name)"; emit USAGE 64 "$KEY"; }
[ -z "$TITLE" ] && TITLE="$KEY"
# php-laravel -> laravel. The framework model and resolver are named for it.
[ -z "$FRAMEWORK" ] && case "$KEY" in *-*) FRAMEWORK="${KEY#*-}" ;; *) FRAMEWORK="$KEY" ;; esac

# --- preflight: name what is missing, do not discover it three steps later ----
say "## [preflight]"
missing=""
command -v python3 >/dev/null 2>&1 || missing="$missing python3"
command -v git     >/dev/null 2>&1 || missing="$missing git"
[ -d "$REPO_ROOT/internal/extract" ] || missing="$missing internal/extract(not-the-sense-repo)"
[ -f "$BENCH_DIR/lib/bootstrap_check.py" ] || missing="$missing bench/lib/bootstrap_check.py"
[ -f "$BENCH_DIR/drivers/new-vertical.sh" ] || missing="$missing bench/drivers/new-vertical.sh"
if [ -n "$missing" ]; then
  say "   MISSING:$missing"
  emit PREFLIGHT-FAILED 70 "$KEY" "$LANG_ARG" "missing:$missing"
fi
say "   ok - python3, git, sense repo at $REPO_ROOT"

EXTRACT_DIR="$REPO_ROOT/internal/extract/$LANG_ARG"
if [ ! -d "$EXTRACT_DIR" ]; then
  say "   extractor NOT READY: no internal/extract/$LANG_ARG"
  say "   this is the loop's documented failure path: it is a sequencing decision"
  say "   (park the lane, or make the gap a Loop 7 item first), never a bootstrap task."
  emit EXTRACTOR-NOT-READY 65 "$KEY" "$LANG_ARG" "no internal/extract/$LANG_ARG"
fi
EXTRACT_FILES=$(find "$EXTRACT_DIR" -maxdepth 1 -name '*.go' ! -name '*_test.go' 2>/dev/null | wc -l | tr -d ' ')
if [ "$EXTRACT_FILES" -eq 0 ]; then
  say "   extractor NOT READY: internal/extract/$LANG_ARG has no production files"
  emit EXTRACTOR-NOT-READY 65 "$KEY" "$LANG_ARG" "internal/extract/$LANG_ARG is empty"
fi
say "   extractor: $EXTRACT_FILES production file(s) in internal/extract/$LANG_ARG"

# --- stamp --------------------------------------------------------------------
say "## [stamp]"
bash "$BENCH_DIR/drivers/new-vertical.sh" "$KEY" --title "$TITLE" >&2 || {
  emit STAMP-FAILED 71 "$KEY" "$LANG_ARG" "new-vertical.sh failed"; }

# --- evaluate: a separate pass, never the stamping step's self-report ----------
say "## [evaluate]"
python3 "$BENCH_DIR/lib/bootstrap_check.py" "$KEY" --lang "$LANG_ARG" --strict >&2
check_rc=$?
if [ "$check_rc" -ne 0 ]; then
  emit NOT-BOOTSTRAPPED "$check_rc" "$KEY" "$LANG_ARG" "bootstrap_check.py --strict failed"
fi

# --- record: the LEDGER entry, written from facts just measured ---------------
LEDGER="$IL_ROOT/verticals/$KEY/LEDGER.md"
if [ -f "$LEDGER" ] && grep -q "loop1/scaffold" "$LEDGER"; then
  say "## [record] loop1/scaffold already in LEDGER.md, not duplicating"
else
  say "## [record] appending loop1/scaffold to LEDGER.md"
  [ -z "$REASON" ] && REASON="a new vertical was opened for this stack"
  KEY="$KEY" LANG_ARG="$LANG_ARG" TITLE="$TITLE" FRAMEWORK="$FRAMEWORK" \
  REASON="$REASON" EXTRACT_DIR="$EXTRACT_DIR" REPO_ROOT="$REPO_ROOT" \
  IL_ROOT="$IL_ROOT" python3 "$BENCH_DIR/lib/loop1_ledger.py" >&2 || {
    emit RECORD-FAILED 72 "$KEY" "$LANG_ARG" "could not write the LEDGER entry"; }
fi

say "## [done] BOOTSTRAPPED - next is Loop 2, which writes pool.txt itself"
emit BOOTSTRAPPED 0 "$KEY" "$LANG_ARG" ""
