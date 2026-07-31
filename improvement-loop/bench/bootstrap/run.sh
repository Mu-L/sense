#!/usr/bin/env bash
# run.sh - everything Loop 3 needs before it can start.
#
#   select -> scaffold -> hunt -> admit -> READY-FOR-LOOP
#
# This is scripted automation, not a loop: it converges on the first pass or it
# stops with a reason. Nothing here iterates and no model is in the path.
#
# SELECTION is a declared queue, not a decision: verticals.txt is walked top to
# bottom and the FIRST key whose verticals/<key>/ does not exist is taken. The
# human sets the order by editing that file. Pass a key explicitly to override.
#
#   bash bench/bootstrap/run.sh                  # next in the queue
#   bash bench/bootstrap/run.sh php-laravel      # a named vertical
#   bash bench/bootstrap/run.sh --re-hunt        # re-run the repo hunt
#
# Progress to STDERR, result JSON on STDOUT, every exit path returns one.

set -uo pipefail
BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
cd "$IL_ROOT" || exit 1

say() { printf '%s\n' "$*" >&2; }

emit() {  # emit <status> <exit-code> <key> <stage> <note>
  python3 - "$@" <<'PY'
import json, sys
a = sys.argv + [""] * 6
out = {"pipeline": "bootstrap", "status": a[1], "vertical": a[3],
       "stage": a[4], "note": a[5]}
if a[1] == "READY-FOR-LOOP":
    out["next"] = {"loop": "1-repo-authoring",
                   "reads": f"verticals/{a[3]}/slate.json"}
print(json.dumps(out, indent=1, sort_keys=True))
PY
  exit "$2"
}

KEY=""; REHUNT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --re-hunt) REHUNT="--force"; shift ;;
    -h|--help) sed -n '2,20p' "$0" >&2; exit 0 ;;
    -*) say "unknown flag: $1"; emit USAGE 64 "" select "" ;;
    *)  KEY="$1"; shift ;;
  esac
done

QUEUE="$IL_ROOT/verticals.txt"

# --- 0. preflight: the binary must BE the latest release ----------------------
# Everything downstream builds indexes, and an index built by an older binary is
# indistinguishable from a fresh one once it is on disk.
say "## [preflight] sense version"
if ! bash "$BENCH_DIR/bootstrap/sense-version-check.sh" >&2; then
  emit SENSE-STALE 69 "" preflight "installed sense is not the latest release"
fi

# --- 1. select ----------------------------------------------------------------
say "## [select]"
if [ -z "$KEY" ]; then
  [ -f "$QUEUE" ] || { say "   no queue at verticals.txt"; emit NO-QUEUE 66 "" select "verticals.txt missing"; }
  skipped=""
  while IFS='|' read -r k lang fw title; do
    case "$k" in ''|\#*) continue ;; esac
    if [ -d "$IL_ROOT/verticals/$k" ]; then
      skipped="$skipped $k"
      continue
    fi
    KEY="$k"; LANG_ARG="$lang"; FRAMEWORK="$fw"; TITLE="$title"
    break
  done < "$QUEUE"
  [ -n "$skipped" ] && say "   already stamped, skipping:$skipped"
  if [ -z "$KEY" ]; then
    say "   every queued vertical already exists - add a line to verticals.txt"
    emit QUEUE-EXHAUSTED 3 "" select "every key in verticals.txt has a directory"
  fi
else
  # An explicit key still has to be IN the queue: the queue is the declaration.
  line=$(grep -v '^#' "$QUEUE" | grep "^$KEY|" | head -1)
  [ -z "$line" ] && { say "   '$KEY' is not in verticals.txt"; emit NOT-QUEUED 66 "$KEY" select "add it to verticals.txt first"; }
  LANG_ARG=$(echo "$line" | cut -d'|' -f2)
  FRAMEWORK=$(echo "$line" | cut -d'|' -f3)
  TITLE=$(echo "$line" | cut -d'|' -f4)
fi
say "   $KEY (lang=$LANG_ARG framework=$FRAMEWORK)"

# The stack profile is a hand-written bootstrap prerequisite, checked here the
# same way the queue is - BEFORE the stamp, so a bad profile never leaves a
# half-created vertical behind.
say "## [prerequisite] stacks/$KEY.conf"
if ! python3 "$BENCH_DIR/bootstrap/stack_profile_check.py" "$KEY" >&2; then
  say "   write or fix it before re-running - format: stacks/README.md"
  emit NO-STACK-PROFILE 66 "$KEY" prerequisite "stacks/$KEY.conf missing or invalid"
fi

# --- 2. scaffold --------------------------------------------------------------
say "## [scaffold]"
b=$(bash "$BENCH_DIR/bootstrap/scaffold.sh" "$KEY" --lang "$LANG_ARG" \
      --framework "$FRAMEWORK" --title "$TITLE" \
      --reason "taken from the verticals.txt queue by run.sh")
brc=$?
if [ $brc -ne 0 ]; then
  st=$(printf '%s' "$b" | python3 -c "import json,sys; print(json.load(sys.stdin)['status'])" 2>/dev/null)
  emit "${st:-BOOTSTRAP-FAILED}" "$brc" "$KEY" bootstrap "scaffold.sh returned ${st:-non-zero}"
fi

# --- 3. hunt ------------------------------------------------------------------
say "## [hunt] declared queries from stacks/$KEY.conf"
command -v gh >/dev/null 2>&1 || {
  say "   gh not installed - the hunt and two of the admission screens need it"
  emit PREFLIGHT-FAILED 70 "$KEY" hunt "gh missing"; }
gh auth status >/dev/null 2>&1 || {
  say "   gh is not authenticated (run: gh auth login)"
  emit PREFLIGHT-FAILED 70 "$KEY" hunt "gh not authenticated"; }
# shellcheck disable=SC2086
python3 "$BENCH_DIR/bootstrap/hunt.py" "$KEY" $REHUNT || {
  emit HUNT-FAILED 71 "$KEY" hunt "hunt.py failed"; }

# --- 4. admit -----------------------------------------------------------------
say "## [admit]"
bash "$BENCH_DIR/bootstrap/admit.sh" "$KEY" --write >&2
arc=$?
if [ $arc -ne 0 ]; then
  emit SLATE-INCOMPLETE "$arc" "$KEY" admit \
    "admit.sh could not compose a standing slate; widen stacks/$KEY.conf hunt queries and re-run with --re-hunt"
fi

say "## [done] READY-FOR-LOOP"
emit READY-FOR-LOOP 0 "$KEY" done ""
