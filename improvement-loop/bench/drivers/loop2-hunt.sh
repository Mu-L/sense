#!/usr/bin/env bash
# loop2-hunt.sh - Loop 2 end to end, no human in it (docs/loops/02-repo-admission.md).
#
# Before this driver, Loop 2 had no automation: an agent wrote hub-ranking SQL by hand,
# picked anchors by eye, and hand-wrote slate.json. The bars were mechanical but
# the HUNT that fed them was not, so a re-run could not reproduce its own slate.
# This is that missing half.
#
#   pool -> clone -> size -> index -> rank anchors -> gate every survivor
#        -> memorization probe on the survivors -> compose -> verify
#
# THE POOL is declared, not discovered: verticals/<v>/pool.txt, one
# `repo-key|git-url|framework?` per line. Discovery-by-API was considered and
# left out - a pool the loop invents each session is a pool whose "exhausted"
# claim cannot be checked. A declared file is diffable, and widening it is an
# edit someone can review.
#
#   bash bench/drivers/loop2-hunt.sh php-laravel                 # dry run, composes nothing
#   bash bench/drivers/loop2-hunt.sh php-laravel --write         # writes repos.txt/pins/slate.json
#   bash bench/drivers/loop2-hunt.sh php-laravel --top 6         # anchors gated per repo
#   SKIP_PROBE=1 bash bench/drivers/loop2-hunt.sh php-laravel    # $0: no model calls, bar 5 unrun
#
# Bar 5 costs subscription calls (`claude -p`, unmetered), one per anchor that
# clears the other bars. SKIP_PROBE=1 leaves every cell at PENDING-BAR-5, which
# NEVER admits - an unrun bar is not a passed bar.
#
# Knobs: SENSE_CLONES (clone root), SENSE_BIN, TOP (anchors/repo, default 24),
# MIN_DEPS. TOP=8 was too shallow and it cost a slate: snipe-it's admitted
# anchors (`Presentable`, `Searchable`) rank ~#18-21 because its contracts are
# sparse and Eloquent models - which all die on K7 - crowd the top. A repo's
# depth is not its rank.
# Idempotent: existing clones and fresh indexes are skipped, gate cells are
# overwritten in place. Re-running is the cheap path, not the expensive one.

set -uo pipefail
BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
cd "$IL_ROOT" || exit 1

VERTICAL="${1:-}"
[ -z "$VERTICAL" ] && { echo "usage: loop2-hunt.sh <vertical> [--write] [--top N]" >&2; exit 64; }
shift

WRITE=""; TOP="${TOP:-24}"; MIN_DEPS="${MIN_DEPS:-12}"
while [ $# -gt 0 ]; do
  case "$1" in
    --write) WRITE="--write"; shift ;;
    --top) TOP="$2"; shift 2 ;;
    --min-deps) MIN_DEPS="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 64 ;;
  esac
done

VDIR="$IL_ROOT/verticals/$VERTICAL"
POOL="$VDIR/pool.txt"
CELLS="$VDIR/.gate-cells"
CLONES="${SENSE_CLONES:-$HOME/Developer/luuuc/oss/sense-benchmark/sense}"
BASELINE="$(dirname "$CLONES")/baseline"
SENSE_BIN="${SENSE_BIN:-$(command -v sense || echo "$HOME/.local/bin/sense")}"

[ -d "$VDIR" ] || { echo "no such vertical: $VDIR" >&2; exit 66; }
[ -f "$POOL" ] || { echo "no pool: $POOL (one 'key|url|framework?' per line)" >&2; exit 66; }
mkdir -p "$CELLS"

frameworks=""
echo "## [pool] $POOL"
while IFS='|' read -r key url isfw; do
  case "$key" in ''|\#*) continue ;; esac
  [ -n "${isfw:-}" ] && frameworks="$frameworks,$key"
  for arm_root in "$CLONES" "$BASELINE"; do
    if [ ! -d "$arm_root/$key" ]; then
      echo "   clone $key -> $(basename "$arm_root")"
      git clone --depth 1 -q "$url" "$arm_root/$key" || echo "   CLONE FAILED: $key"
    fi
  done
done < "$POOL"

# --- size gate first: a small repo cannot win, so never pay to index one -------
echo "## [size] dropping small repos before paying for an index"
KEEP=""
while IFS='|' read -r key url isfw; do
  case "$key" in ''|\#*) continue ;; esac
  [ -d "$CLONES/$key" ] || continue
  n=$(find "$CLONES/$key" -type f \( -name '*.php' -o -name '*.py' -o -name '*.rb' \
        -o -name '*.go' -o -name '*.ts' -o -name '*.js' \) \
        ! -path '*/vendor/*' ! -path '*/node_modules/*' ! -path '*/tests/*' 2>/dev/null | wc -l | tr -d ' ')
  if [ "$n" -lt 1000 ]; then
    echo "   skip $key ($n source files = small; the small slot was removed 2026-07-20)"
  else
    echo "   keep $key ($n source files)"
    KEEP="$KEEP $key"
  fi
done < "$POOL"
[ -z "$KEEP" ] && { echo "pool has no non-small repo - widen pool.txt" >&2; exit 1; }

echo "## [index] ensure-index (rebuilds only what the scan fingerprint stales)"
# shellcheck disable=SC2086
bash bench/lib/ensure-index.sh $KEEP 2>&1 | grep -E '^\[' || true

echo "## [gate] rank anchors, then gate each one"
for key in $KEEP; do
  clone="$CLONES/$key"
  slot=$(python3 - "$clone" <<'EOF'
import sys, os, sqlite3
db = os.path.join(sys.argv[1], ".sense", "index.db")
n = 0
if os.path.exists(db):
    con = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
    n = con.execute("SELECT count(*) FROM sense_files WHERE path NOT LIKE '%vendor/%' "
                    "AND path NOT LIKE '%node_modules/%' AND path NOT LIKE '%test%' "
                    "AND path NOT LIKE '%spec%'").fetchone()[0]
print("big" if n >= 4000 else "medium" if n >= 1000 else "small")
EOF
)
  case ",$frameworks," in *",$key,"*) slot="framework" ;; esac
  python3 bench/lib/anchor_rank.py "$clone" --top "$TOP" --min-deps "$MIN_DEPS" --tsv 2>/dev/null |
  while IFS=$'\t' read -r sym file; do
    [ -z "$sym" ] && continue
    cell="$CELLS/$key.$sym.json"
    memo="$CELLS/memo.$key.$sym.json"
    # Gate FIRST, probe only what survives. Bar 5 is the one bar that costs a
    # model call, and most anchors die on a $0 seam kill - probing before gating
    # spent ~5 calls for every 1 that could matter.
    # A memo only counts if the probe actually RAN (see memorization_probe.py):
    # a failed probe is never written, so a missing memo means "retry", not "pass".
    memo_arg=""; [ -f "$memo" ] && memo_arg="--memorization $memo"
    # shellcheck disable=SC2086
    v=$(python3 bench/lib/admission_gate.py "$clone" "$sym" --file "$file" \
            --slot "$slot" --sense "$SENSE_BIN" $memo_arg --json "$cell" 2>&1 |
        grep -oE 'ADMISSION: [A-Z0-9-]+')
    if [ -z "${SKIP_PROBE:-}" ] && [ ! -f "$memo" ] && [ "$v" = "ADMISSION: PENDING-BAR-5" ]; then
      if ! python3 bench/lib/memorization_probe.py "$clone" "$sym" --file "$file" \
              --json "$memo" >/dev/null 2>&1; then
        printf '   %-14s %-28s %s\n' "$key" "$sym" "PROBE FAILED (bar 5 unrun; re-run to retry)"
        continue
      fi
      v=$(python3 bench/lib/admission_gate.py "$clone" "$sym" --file "$file" \
              --slot "$slot" --sense "$SENSE_BIN" --memorization "$memo" \
              --json "$cell" 2>&1 | grep -oE 'ADMISSION: [A-Z0-9-]+')
    fi
    printf '   %-14s %-28s %s\n' "$key" "$sym" "${v:-GATE FAILED}"
  done
done

echo "## [compose]"
python3 bench/lib/compose_slate.py "$VERTICAL" --cells "$CELLS" --clones "$CLONES" \
        --frameworks "${frameworks#,}" $WRITE
rc=$?

if [ -n "$WRITE" ]; then
  echo "## [verify]"
  python3 bench/lib/slate_check.py "$VERTICAL"
  rc=$?
  echo "## [next] write the loop2/slate LEDGER entry, then Loop 3"
fi
exit $rc
