#!/usr/bin/env bash
# campaign-snapshot.sh - weekly, no-telemetry campaign tracker for a Sense vertical.
#
# Sense ships no product telemetry. The only honest signals come from GitHub:
#   - release asset download_count  -> TRUE installs (install.sh pulls release binaries)
#   - traffic views + referrers     -> who looked, and which platform sent them (14d ROLLING -> we persist it)
#   - stars                         -> social proof (track velocity, not the count)
#   - clones                        -> BOT/CI NOISE. captured but flagged, never a headline.
#
# It appends to two CSVs (machine source-of-truth, deduped by date) and (re)renders a
# human-readable CAMPAIGN-TRACKING.md whose AUTO block is regenerated and whose
# "## Observations" section is PRESERVED across runs.
#
# Usage:
#   bash improvement-loop/bench/lib/campaign-snapshot.sh \
#        --vertical improvement-loop/verticals/<key> \
#        [--repo luuuc/sense] [--articles ARTICLES.md] [--title "Rails vertical"]
set -euo pipefail

REPO="luuuc/sense"
VERTICAL=""
ARTICLES="ARTICLES.md"
TITLE=""

while [ $# -gt 0 ]; do
  case "$1" in
    --repo)     REPO="$2"; shift 2 ;;
    --vertical) VERTICAL="$2"; shift 2 ;;
    --articles) ARTICLES="$2"; shift 2 ;;
    --title)    TITLE="$2"; shift 2 ;;
    -h|--help)  grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

command -v gh >/dev/null || { echo "need: gh" >&2; exit 1; }
command -v jq >/dev/null || { echo "need: jq" >&2; exit 1; }
[ -d "$VERTICAL" ] || { echo "no such vertical dir: $VERTICAL" >&2; exit 1; }
[ -z "$TITLE" ] && TITLE="$(basename "$VERTICAL" | sed 's/^[0-9]*-//; s/-/ /g')"

TODAY="$(date -u +%Y-%m-%d)"
METRICS="$VERTICAL/metrics.csv"
REFCSV="$VERTICAL/referrers.csv"
DOC="$VERTICAL/CAMPAIGN-TRACKING.md"

echo "fetching $REPO ..." >&2

# --- pull from GitHub -------------------------------------------------------
DOWNLOADS="$(gh api "repos/$REPO/releases" --paginate 2>/dev/null | jq -s 'add | [.[].assets[]?.download_count] | add // 0')"
STARS="$(gh api "repos/$REPO" 2>/dev/null | jq -r '.stargazers_count // 0')"
read -r V_UNIQ V_TOT < <(gh api "repos/$REPO/traffic/views"  2>/dev/null | jq -r '"\(.uniques // 0) \(.count // 0)"')
read -r C_UNIQ C_TOT < <(gh api "repos/$REPO/traffic/clones" 2>/dev/null | jq -r '"\(.uniques // 0) \(.count // 0)"')

# referrers -> tmp tsv: referrer \t uniques \t count
REF_TSV="$(gh api "repos/$REPO/traffic/popular/referrers" 2>/dev/null \
  | jq -r '.[] | "\(.referrer)\t\(.uniques)\t\(.count)"')"
TOP_REF="$(printf '%s\n' "$REF_TSV" | head -1 | cut -f1)"
[ -z "${TOP_REF:-}" ] && TOP_REF="-"

# per-release downloads (tag \t dl \t date), most recent first, top 10
REL_TSV="$(gh api "repos/$REPO/releases" 2>/dev/null \
  | jq -r '.[:10][] | "\(.tag_name)\t\([.assets[]?.download_count]|add // 0)\t\(.published_at[:10])"')"

# --- append to metrics.csv (dedupe today's row) -----------------------------
MHEAD="date,total_downloads,stars,views_unique_14d,views_total_14d,clones_unique_14d,clones_total_14d,top_referrer"
NEWROW="$TODAY,$DOWNLOADS,$STARS,$V_UNIQ,$V_TOT,$C_UNIQ,$C_TOT,$TOP_REF"
tmp="$(mktemp)"; body="$(mktemp)"
{
  if [ -f "$METRICS" ]; then tail -n +2 "$METRICS" | grep -v "^$TODAY," || true; fi
  echo "$NEWROW"
} | grep -v '^[[:space:]]*$' | sort -t, -k1,1 > "$body"
{ echo "$MHEAD"; cat "$body"; } > "$tmp"
mv "$tmp" "$METRICS"; rm -f "$body"

# previous snapshot (latest date that is not today) for deltas
PREV_ROW="$(tail -n +2 "$METRICS" | grep -v "^$TODAY," | tail -1 || true)"
PREV_DL="$(printf '%s' "$PREV_ROW" | cut -d, -f2)"; PREV_DL="${PREV_DL:-}"
PREV_ST="$(printf '%s' "$PREV_ROW" | cut -d, -f3)"; PREV_ST="${PREV_ST:-}"
delta() { # $1 now $2 prev -> "(+N)" / "(=)" / "(first)"
  if [ -z "$2" ]; then echo "(first)"; return; fi
  d=$(( $1 - $2 )); if [ "$d" -gt 0 ]; then echo "(+$d)"; elif [ "$d" -lt 0 ]; then echo "($d)"; else echo "(=)"; fi
}
D_DL="$(delta "$DOWNLOADS" "$PREV_DL")"
D_ST="$(delta "$STARS" "$PREV_ST")"

# --- append to referrers.csv (dedupe today's rows) --------------------------
RHEAD="date,referrer,uniques,count"
tmp="$(mktemp)"
{
  echo "$RHEAD"
  if [ -f "$REFCSV" ]; then tail -n +2 "$REFCSV" | grep -v "^$TODAY," || true; fi
  if [ -n "${REF_TSV:-}" ]; then printf '%s\n' "$REF_TSV" | awk -F'\t' -v d="$TODAY" 'NF>=3{print d","$1","$2","$3}'; fi
} > "$tmp"
mv "$tmp" "$REFCSV"

# --- helpers for rendering --------------------------------------------------
# map a referrer host to a friendly platform (relays land here: t.co=X, lnkd.in=LinkedIn ...)
platform() {
  case "$1" in
    t.co)                         echo "X / Twitter" ;;
    lnkd.in|*linkedin.com)        echo "LinkedIn" ;;
    *medium.com)                  echo "Medium" ;;
    dev.to)                       echo "dev.to" ;;
    *ycombinator.com|*hn.*)       echo "Hacker News" ;;
    *reddit.com)                  echo "Reddit" ;;
    luuuc.github.io)              echo "Docs site" ;;
    github.com)                   echo "GitHub" ;;
    Google|*google.com)           echo "Search" ;;
    bing.com|duckduckgo.com)      echo "Search" ;;
    *)                            echo "" ;;
  esac
}

# preserve the manual Observations section if the doc already exists
OBS=""
if [ -f "$DOC" ] && grep -q '^## Observations' "$DOC"; then
  # capture from the Observations heading up to (not incl.) the auto Run log footer
  OBS="$(awk '/^## Observations/{f=1} /^## Run log/{f=0} f' "$DOC" | awk 'NF{p=NR} {a[NR]=$0} END{for(i=1;i<=p;i++)print a[i]}')"
fi
if [ -z "$OBS" ]; then
  OBS=$'## Observations\n\n_Manual notes - preserved across runs. Jot the causal reads here:\nwhich post/relay drove a view spike, which referrer converted, what stalled._\n\n- '
fi

# --- render CAMPAIGN-TRACKING.md --------------------------------------------
tmp="$(mktemp)"
{
cat <<EOF
# Campaign tracking - ${TITLE}

_Last updated: ${TODAY} (UTC) · repo \`${REPO}\`_

> **Internal, no-telemetry tracker.** Sense ships no product telemetry, so these are GitHub-side
> proxies only. Read them in this order of trust:
>
> - **Installs = release downloads** - the truest signal (\`install.sh\` pulls release binaries, so \`curl | sh\` counts here).
> - **Views + Referrers** - the campaign dial: did a post move unique views, and which platform sent them. **14-day rolling on GitHub, so this file persists it.**
> - **Stars** - social proof; watch the *velocity* around a publish, not the absolute count.
> - **Clones** - ⚠️ inflated by CI/mirrors/bots. Captured for completeness, **never a headline**.
> - **Retention** - invisible without telemetry. Only inbound proxies (issues, discussions, repeat doc referrers).
>
> Regenerate: \`bash improvement-loop/bench/lib/campaign-snapshot.sh --vertical ${VERTICAL}\`
> Everything between the AUTO markers is overwritten each run; the **Observations** section is preserved.

<!-- AUTO:BEGIN - regenerated by campaign-snapshot.sh; do not edit by hand -->

## At a glance

| Metric | Value | Δ since last snapshot |
|---|---|---|
| **Installs** (release downloads, all-time) | ${DOWNLOADS} | ${D_DL} |
| Stars | ${STARS} | ${D_ST} |
| Unique views (14d) | ${V_UNIQ} | rolling |
| Top referrer (14d) | ${TOP_REF} | rolling |
| Clones (14d, ⚠️ noise) | ${C_UNIQ} uniq / ${C_TOT} total | rolling |

## Publish log

_Synced from \`${ARTICLES}\`. One canonical piece per row; relays (LinkedIn / X / etc.) surface in **Referrer captures** below._

| Date | Piece |
|---|---|
EOF

# publish log from ARTICLES.md: dated lines with the title after a dash separator.
# The separator is matched via an escaped literal, not typed in this source: ARTICLES.md
# is INPUT we do not control, so the pattern must still match an em-dash there.
DASHES="$(printf '\xe2\x80\x94')-"
if [ -f "$ARTICLES" ]; then
  grep -E '^- [0-9]{4}-[0-9]{2}-[0-9]{2} ' "$ARTICLES" \
    | sed -E "s/^- ([0-9-]+) [${DASHES}]+ (.*)\$/\\1\t\\2/" \
    | while IFS=$'\t' read -r d rest; do
        # strip markdown links [text](url) -> text, collapse spaces
        clean="$(printf '%s' "$rest" | sed -E 's/\[([^]]*)\]\([^)]*\)/\1/g; s/  +/ /g')"
        printf '| %s | %s |\n' "$d" "$clean"
      done | sort -r
else
  printf '| - | _no %s found_ |\n' "$ARTICLES"
fi

cat <<EOF

## Referrer captures

_GitHub keeps only 14 days; each snapshot is persisted here so you can watch platforms (incl. relays) appear and grow._

EOF
# group referrers.csv by date, newest first
tail -n +2 "$REFCSV" | awk -F, '{print $1}' | sort -ru | while read -r d; do
  [ -z "$d" ] && continue
  echo "**${d}**"
  echo
  echo "| Referrer | Platform | Unique | Views |"
  echo "|---|---|---|---|"
  awk -F, -v d="$d" '$1==d{print}' "$REFCSV" | sort -t, -k3,3 -nr | while IFS=, read -r _ ref u c; do
    plat="$(platform "$ref")"
    printf '| %s | %s | %s | %s |\n' "$ref" "$plat" "$u" "$c"
  done
  echo
done

cat <<EOF
## Per-release downloads

_Which versions people actually install._

| Release | Downloads | Published |
|---|---|---|
EOF
if [ -n "${REL_TSV:-}" ]; then
  printf '%s\n' "$REL_TSV" | awk -F'\t' '{printf "| %s | %s | %s |\n", $1, $2, $3}'
fi

cat <<EOF
<!-- AUTO:END -->

EOF
printf '%s\n' "$OBS"

# --- Run log footer: compact, newest-first trend, one line per snapshot ------
cat <<EOF

## Run log

_Auto, newest first. One line per snapshot - the trend at a glance._

EOF
tail -n +2 "$METRICS" | sort -t, -k1,1 | awk -F, '
  { d[NR]=$1; dl[NR]=$2; st[NR]=$3; vu[NR]=$4; tr[NR]=$8; n=NR }
  END {
    for (i=n; i>=1; i--) {
      if (i>1) {
        x=dl[i]-dl[i-1]; ddl=(x>0?"(+"x")":(x<0?"("x")":"(=)"));
        y=st[i]-st[i-1]; dst=(y>0?" (+"y")":(y<0?" ("y")":""));
      } else { ddl="(first)"; dst=""; }
      printf "- **%s** · installs %s %s · stars %s%s · views %s (14d) · top %s\n",
             d[i], dl[i], ddl, st[i], dst, vu[i], tr[i];
    }
  }'
} > "$tmp"
mv "$tmp" "$DOC"

echo "wrote: $DOC" >&2
echo "       $METRICS" >&2
echo "       $REFCSV" >&2
