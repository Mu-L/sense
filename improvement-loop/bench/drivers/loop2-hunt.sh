#!/usr/bin/env bash
# loop2-hunt.sh - Loop 2 end to end, no human in it (docs/loops/02-repo-admission.md).
#
#   pool -> clone -> screen -> compose -> pin -> index the admitted -> verify
#
# The screens are repo-level facts: does it declare this stack, is it maintained,
# is it big enough, is it used. Nothing here opens an index or picks an anchor.
# Loop 2 used to run a seven-bar seam gate over every anchor of every candidate;
# backtested against the four banked go wins it rejected 4 of 4, so it was
# retired. What Sense can win on is Loop 3's question, asked against a scenario
# that exists.
#
# THE POOL is declared, not discovered: verticals/<v>/pool.txt, one
# `repo-key|git-url|framework?` per line, with a `# stack: <manifest>:<needle>`
# directive on top. Discovery-by-API is left out on purpose - a pool the loop
# invents each session is a pool whose "exhausted" claim cannot be checked. A
# declared file is diffable, and widening it is an edit someone can review.
#
#   bash bench/drivers/loop2-hunt.sh php-laravel            # dry run, composes nothing
#   bash bench/drivers/loop2-hunt.sh php-laravel --write    # writes repos.txt/pins/slate.json
#   NO_API=1 bash bench/drivers/loop2-hunt.sh php-laravel   # skip gh; maintained+used UNRUN
#
# The maintained and used screens need `gh`. NO_API=1 leaves them UNRUN, which
# NEVER admits - an unrun screen is not a passed screen.
#
# Knobs: SENSE_CLONES (clone root), SENSE_BIN.
# Idempotent: existing clones are skipped, screen cells are overwritten in place,
# indexes rebuild only when their scan fingerprint stales. Re-running is cheap.

set -uo pipefail
BENCH_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IL_ROOT="$(cd "$BENCH_DIR/.." && pwd)"
cd "$IL_ROOT" || exit 1

VERTICAL="${1:-}"
[ -z "$VERTICAL" ] && { echo "usage: loop2-hunt.sh <vertical> [--write]" >&2; exit 64; }
shift

WRITE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --write) WRITE="--write"; shift ;;
    *) echo "unknown arg: $1" >&2; exit 64 ;;
  esac
done

VDIR="$IL_ROOT/verticals/$VERTICAL"
POOL="$VDIR/pool.txt"
CELLS="$VDIR/.screen-cells"
CLONES="${SENSE_CLONES:-$HOME/Developer/luuuc/oss/sense-benchmark/sense}"
BASELINE="$(dirname "$CLONES")/baseline"

[ -d "$VDIR" ] || { echo "no such vertical: $VDIR" >&2; exit 66; }
[ -f "$POOL" ] || { echo "no pool: $POOL (one 'key|url|framework?' per line)" >&2; exit 66; }
mkdir -p "$CELLS"

# The stack marker is the pool's own header, so the in-vertical screen is
# declared next to the candidates it screens.
STACK_MARKER="$(sed -n 's/^# *stack: *//p' "$POOL" | head -1)"
[ -z "$STACK_MARKER" ] && {
  echo "pool has no '# stack: <manifest>:<needle>' header - the in-vertical screen cannot run" >&2
  exit 65; }
echo "## [pool] $POOL  (stack marker: $STACK_MARKER)"

frameworks=""
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

echo "## [screen] four repo-level facts per candidate"
api_flag=""; [ -n "${NO_API:-}" ] && api_flag="--no-api"
ADMITTED=""
while IFS='|' read -r key url isfw; do
  case "$key" in ''|\#*) continue ;; esac
  [ -d "$CLONES/$key" ] || continue
  # shellcheck disable=SC2086
  v=$(python3 bench/lib/repo_screen.py "$CLONES/$key" --key "$key" --url "$url" \
        --stack "$STACK_MARKER" --json "$CELLS/$key.json" $api_flag 2>&1 |
      grep -oE 'SCREEN: [A-Z]+')
  printf '   %-18s %s\n' "$key" "${v:-SCREEN FAILED}"
  [ "$v" = "SCREEN: ADMIT" ] && ADMITTED="$ADMITTED $key"
done < "$POOL"
[ -z "$ADMITTED" ] && { echo "no repo cleared the screens - widen pool.txt (try-harder law)" >&2; exit 1; }

echo "## [compose]"
python3 bench/lib/compose_slate.py "$VERTICAL" --cells "$CELLS" --clones "$CLONES" \
        --frameworks "${frameworks#,}" $WRITE
rc=$?

if [ -n "$WRITE" ] && [ $rc -eq 0 ]; then
  # Index the SLATE only. The screens never needed an index, so a pool of 28 no
  # longer pays for 28 indexes to admit 4.
  echo "## [index] ensure-index over the composed slate"
  slate_repos=$(grep -v '^#' "$VDIR/repos.txt" | tr '\n' ' ')
  # shellcheck disable=SC2086
  bash bench/lib/ensure-index.sh $slate_repos 2>&1 | grep -E '^\[' || true

  echo "## [verify]"
  python3 bench/lib/slate_check.py "$VERTICAL"
  rc=$?
  echo "## [next] write the loop2/slate LEDGER entry, then Loop 3"
fi
exit $rc
