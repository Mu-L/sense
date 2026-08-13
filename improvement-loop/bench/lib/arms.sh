#!/usr/bin/env bash
# arms.sh - read a vertical's arms (the tools/LLMs it benches) from ONE file.
#
# The single source of truth is verticals/<key>/arms.txt, stamped from docs/arms.default.txt.
# Nothing else names a model id: drivers, one-pagers and the manifesto all resolve through
# here, so superseding a release is one edit instead of a grep across the tree.
#
#   source "$BENCH_DIR/lib/arms.sh"
#   arms_models "$VERTICAL"                 # every arm, headline first
#   arms_models "$VERTICAL" headline        # just the headline id
#   arms_models "$VERTICAL" confirmation    # the confirmation set
#   arms_runs   "$VERTICAL" <model-id>      # that arm's run count (default 2)
#   arms_judge  "$VERTICAL"                 # the pinned judge id
#
# Read-only, no side effects. A missing arms.txt is an error the caller must handle: a
# silent fallback to a hardcoded id is the rot this file exists to remove.

arms_file() {
  local v="${1:?arms_file: vertical key required}"
  echo "${BENCH_DIR:?arms.sh: BENCH_DIR must be set}/../verticals/$v/arms.txt"
}

# _arms_rows <vertical> [role] -> "role model runs" per line, comments and blanks stripped
_arms_rows() {
  local f; f="$(arms_file "$1")"
  [ -f "$f" ] || { echo "arms.sh: no arms file at $f" >&2; return 1; }
  awk -v want="${2:-}" '
    /^[[:space:]]*(#|$)/ { next }
    { if (want == "" || $1 == want) print $1, $2, ($3 == "" ? 2 : $3) }
  ' "$f"
}

arms_models() {
  local rows; rows="$(_arms_rows "$1" "${2:-}")" || return 1
  # headline first, then confirmation; the judge is never an arm under test
  { echo "$rows" | awk '$1=="headline"    {print $2}'
    echo "$rows" | awk '$1=="confirmation"{print $2}'
    [ "${2:-}" = "judge" ] && echo "$rows" | awk '$1=="judge"{print $2}'
  } | awk 'NF' | tr '\n' ' ' | sed 's/ $//'
}

arms_runs() {
  local rows; rows="$(_arms_rows "$1")" || return 1
  echo "$rows" | awk -v m="${2:?arms_runs: model id required}" '$2==m {print $3; found=1} END{if(!found) print 2}' | head -1
}

arms_judge() { _arms_rows "$1" judge | awk '{print $2}' | head -1; }
