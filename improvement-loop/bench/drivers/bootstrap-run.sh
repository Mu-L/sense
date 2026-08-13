#!/usr/bin/env bash
# bootstrap-run.sh - run bootstrap, and open a cycle 3 product window when it stops
# because Sense cannot read the stack yet.
#
# This is what you call instead of bench/bootstrap/run.sh. It adds exactly one thing:
# `EXTRACTOR-NOT-READY` stops being a dead end you read and becomes a window that runs.
#
# NOTHING IN bootstrap/ IS EDITED. run.sh already prints result JSON on stdout on every
# exit path and documents that `result=$(run.sh)` is the whole integration for an
# orchestrator - so the trigger is a reader, not a patch. That matters: bootstrap is a
# script other cycles' plans depend on, and changing one costs a full cycle to verify.
#
# The trigger fires on a fact about a DIRECTORY (`scaffold.sh` tests whether
# `internal/extract/<lang>/` exists with production files). It is not a measurement of
# support - a language can be partly served by internal/extract/langspec/<lang>.go with no
# such directory. Measuring what is actually missing is the window's first phase, not this
# script's job.
#
# Usage:
#   bash bench/drivers/bootstrap-run.sh                # next key in the queue
#   bash bench/drivers/bootstrap-run.sh csharp-aspnet  # a named key
#   bash bench/drivers/bootstrap-run.sh --re-hunt      # passed through
#
# After the window's branch is merged, re-run this same command: the extractor check
# passes and bootstrap continues into hunt and admit. One command, both halves.
set -uo pipefail

BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
cd "$IL_ROOT" || exit 1

RESULT="$(bash "$BENCH_DIR/bootstrap/run.sh" "$@")"; RC=$?
printf '%s\n' "$RESULT"

STATUS="$(printf '%s' "$RESULT" | python3 -c 'import json,sys
try: print(json.load(sys.stdin).get("status",""))
except Exception: print("")')"
[ "$STATUS" = "EXTRACTOR-NOT-READY" ] || exit "$RC"

KEY="$(printf '%s' "$RESULT" | python3 -c 'import json,sys
try: print(json.load(sys.stdin).get("vertical",""))
except Exception: print("")')"
[ -n "$KEY" ] || { echo "EXTRACTOR-NOT-READY with no vertical in the result JSON" >&2; exit "$RC"; }

QLINE="$(grep "^$KEY|" "$IL_ROOT/verticals.txt" | head -1)"
[ -n "$QLINE" ] || { echo "'$KEY' is not in verticals.txt" >&2; exit 66; }

WDIR="$IL_ROOT/product-window/$KEY"
mkdir -p "$WDIR"

# The request is written ONCE and never overwritten: a second bootstrap run on a window
# that is already mid-flight must resume it, not reset the record of why it opened.
if [ -f "$WDIR/request.json" ]; then
  echo "## [$KEY] a product window is already open; resuming it" >&2
else
  QLINE="$QLINE" RESULT="$RESULT" python3 - "$WDIR/request.json" <<'PY'
import json, os, sys
key, lang, framework, title = (os.environ["QLINE"].split("|") + [""] * 4)[:4]
json.dump({
    "opened_by": "bootstrap",
    "status": "EXTRACTOR-NOT-READY",
    "key": key, "lang": lang, "framework": framework, "title": title,
    "bootstrap_result": json.loads(os.environ["RESULT"]),
    "the_check": "bench/bootstrap/scaffold.sh: internal/extract/<lang>/ exists with production files",
    "not_a_measurement": ("the check is a fact about a directory, not about support - "
                          "01-intake measures what is actually missing"),
}, open(sys.argv[1], "w"), indent=2, sort_keys=True)
open(sys.argv[1], "a").write("\n")
PY
  echo "## [$KEY] opened a product window: product-window/$KEY/request.json" >&2
fi

exec bash "$BENCH_DIR/drivers/product-window.sh" "$KEY"
