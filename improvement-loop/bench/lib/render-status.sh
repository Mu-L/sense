#!/usr/bin/env bash
# render-status.sh - regenerate a vertical's STATUS.md from disk. Readability
# artifact per ledger.md: humans read it, loops NEVER do (write-only law).
# Every section is recomputed from the authoritative sources (results tree,
# report-matrix.sh, next-steps.md, repos.md); the file is replaced wholesale so
# it can never drift into hand-edited state.
#
#   bash render-status.sh <vertical-key>
#   e.g.: bash render-status.sh laravel   -> writes docs/laravel/STATUS.md
set -euo pipefail

LIB_DIR="$(cd "$(dirname "$0")" && pwd)"
IL_ROOT="$(cd "$LIB_DIR/../.." && pwd)"
KEY="${1:?usage: render-status.sh <vertical-key>}"
DOC_DIR="$IL_ROOT/verticals/$KEY"
[ -d "$DOC_DIR" ] || { echo "render-status.sh: no vertical doc dir at $DOC_DIR" >&2; exit 1; }
OUT="$DOC_DIR/STATUS.md"
RESULTS="$DOC_DIR/results"
# Per-vertical, optional: the go campaign's next-steps.md is frozen in
# ZZ-go-vertical-legacy; fresh verticals run on STATUS.md + LEDGER.md alone.
NEXT_STEPS="$DOC_DIR/next-steps.md"
LEDGER="$DOC_DIR/LEDGER.md"
STAMP="$(date '+%Y-%m-%d %H:%M %Z')"

matrix_section() {
  # report-matrix.sh writes report.md into $RESULTS and cats it; we keep the file.
  if [ -d "$RESULTS" ] && VERTICAL="$KEY" bash "$IL_ROOT/bench/drivers/report-matrix.sh" >/dev/null 2>&1 \
     && [ -s "$RESULTS/report.md" ]; then
    # Numbers only: start at the report's Results section (methodology prose
    # stays in report.md), demote its headings under ours.
    if grep -q '^## Results' "$RESULTS/report.md"; then
      sed -n '/^## Results/,$p' "$RESULTS/report.md" | sed 's/^#/##/'
    else
      sed 's/^#/##/' "$RESULTS/report.md"
    fi
    echo
    echo "_Full report with methodology: \`verticals/${KEY}/results/report.md\`_"
  else
    echo "_matrix render unavailable (no results yet, or report-matrix.sh failed; source: $RESULTS)_"
  fi
}

cells_section() {
  if [ -d "$RESULTS" ]; then
    local found=0
    while IFS= read -r cell; do
      found=1
      local runs
      runs=$(find "$cell" -mindepth 1 -maxdepth 1 -type d -name 'run-*' | wc -l | tr -d ' ')
      echo "- \`${cell#"$RESULTS"/}\` - $runs run(s)"
    done < <(find "$RESULTS" -mindepth 4 -maxdepth 4 -type d -name 'run-*' -exec dirname {} \; | sort -u)
    [ "$found" -eq 1 ] || echo "_no run cells on disk yet_"
  else
    echo "_no results tree yet at ${RESULTS}_"
  fi
}

header_section() {
  # next-steps.md opens with an italic "_Last updated ..._" block: the maintained
  # one-paragraph state. Print from "_Last updated" through the line ending in "_".
  awk '/^_Last updated/{on=1; first=NR} on{print; if (/_$/ && (NR>first || /_.+_$/)) exit}' \
    "$NEXT_STEPS" 2>/dev/null || echo "_next-steps.md header not found_"
}

steps_section() {
  # Unchecked (☐) steps. Three corrections over a raw grep, each of which made the
  # old render misleading rather than merely terse:
  #  (1) a heading often spans lines, so its first line can stop mid-sentence
  #      (step 24 rendered as '...owner-authorized: "we changed the').
  #      Cut at the closing ** when it is on the line, else at the first ' (',
  #      else cap the length.
  #  (2) an unchecked step annotated OVERTAKEN/OBE is recorded history, not work.
  #      Unlabelled, the list reads as "step 1 is what to do next"; steps 1 and 2
  #      have been overtaken and still rendered as open.
  #  (3) LOOP BINDING (decision-errors "worked outside the loop"): a
  #      pickup that names the step but not the LOOP the step belongs to leaves the
  #      session to improvise a "how". Each step's block may carry a tag
  #          [Loop <N>: <driver>]      e.g. [Loop 4: bench/drivers/sweep.sh]
  #      searched across the WHOLE step block (heading + continuation lines). When
  #      present it renders beside the step; when an OPEN step has none, the render
  #      flags it so the step is bound to its loop + driver before it is executed,
  #      instead of hand-rolled. This is the mechanism that stops the recurring
  #      "started from STATUS, then drifted outside the loop" error.
  # OBE is read from the heading line only, where the file actually records it:
  # a missed label degrades to the old behaviour, a false one would mislead.
  local out
  out=$(awk '
    function emit(   tag) {
      if (num == "") return
      tag = ""
      if (match(block, /\[Loop[^]]*\]/)) tag = substr(block, RSTART, RLENGTH)
      printf "- step %s: %s", num, title
      if (obe) printf "  _[overtaken, recorded history, not work]_"
      else if (tag != "") printf "  `%s`", tag
      else printf "  ⚠ NO LOOP TAG - bind this step to its loop + driver before executing (do not improvise)"
      printf "\n"
      num = ""
    }
    /^[0-9]+\. \*\*/ {
      emit()                                    # close any open block first
      if ($0 ~ /^[0-9]+\. \*\*☐/) {
        num = $0; sub(/\..*$/, "", num)
        obe = ($0 ~ /OVERTAKEN|OBE/)
        title = $0
        sub(/^[0-9]+\. \*\*☐ */, "", title)
        if (title ~ /\*\*/) sub(/\*\*.*$/, "", title)
        else if (title ~ / \(/) sub(/ \(.*$/, "", title)
        else if (length(title) > 80) title = substr(title, 1, 80) "..."
        sub(/ +$/, "", title)
        block = $0
      }
      next
    }
    num != "" { block = block "\n" $0 }         # accumulate the open step block
    END { emit() }
  ' "$NEXT_STEPS" 2>/dev/null)
  if [ -n "$out" ]; then echo "$out"; else echo "_no unchecked steps found_"; fi
}

ledger_section() {
  if [ -f "$LEDGER" ]; then
    grep '^## ' "$LEDGER" | tail -1 | sed 's/^## /- latest entry: /'
    echo "- full narrative: \`LEDGER.md\` (this folder)"
  else
    echo "_no LEDGER.md yet_"
  fi
  local intake="$IL_ROOT/docs/decision-errors.md" open
  # A MISSING file must not render as a healthy zero. The file was absent for an unknown
  # stretch and this line kept printing "0 open incident(s)", which reads exactly like a
  # clean intake - the same dormancy shape as ledger_check rule 10 over an untracked
  # instrument. A check that cannot fire is worse than a short list.
  if [ ! -f "$intake" ]; then
    echo "- decision-errors intake: **MISSING** (\`../../docs/decision-errors.md\` does not exist - the count is unverifiable, not zero)"
  else
    # An open incident is an ENTRY bullet, so anchor on the bullet: prose that merely
    # mentions the phrase (the file's own header describes this very count) is not an
    # incident, and the template's "Status: open | protocolized" is not one either.
    open=$(grep -E '^[-*] .*Status: open' "$intake" | grep -cv 'open | protocolized') || open=0
    echo "- decision-errors intake: $open open incident(s) (\`../../docs/decision-errors.md\`)"
  fi
}

# Where each repo stands in the driver's phase machine. Without this the pickup file
# cannot answer "where am I?" - the state lives in .loop-state.json, and a session reading
# only STATUS.md would have to guess, or redo work already done.
loop_position_section() {
  local st="$IL_ROOT/verticals/$KEY/.loop-state.json"
  if [ ! -f "$st" ]; then
    echo "_no \`.loop-state.json\` yet - the loop has never been run for this vertical._"
    return
  fi
  python3 -c "
import json,sys
d=json.load(open(sys.argv[1]))
p={k:v for k,v in d.items() if '#' not in k}
if not p:
    print('_state file present, no repo phases recorded._')
else:
    print('| repo | phase |'); print('|---|---|')
    for k in sorted(p): print(f'| {k} | {p[k]} |')
    print()
    print('Resume: \`VERTICAL=$KEY bash bench/drivers/vertical-loop.sh <repo>\`')
" "$st"
}

{
  echo "# ${KEY} - STATUS (auto-rendered)"
  echo
  echo "> AUTO-RENDERED ${STAMP} by \`bench/lib/render-status.sh ${KEY}\`."
  echo "> Do not edit by hand; do not use for loop decisions (write-only law, \`ledger.md\`)."
  echo "> Position is authoritative ON DISK: the results tree, \`repos.md\`, \`LEDGER.md\`."
  echo
  if [ -f "$NEXT_STEPS" ]; then
    echo "## Where we are (next-steps.md header)"
    echo
    header_section
    echo
    echo "## Open steps (unchecked in next-steps.md)"
    echo
    echo "_Each step names the **Loop** it belongs to and its driver (\`[Loop N: driver]\`)._"
    echo "_Execute that loop's driver - do NOT hand-roll orchestration. A step tagged_"
    echo "_⚠ NO LOOP TAG must be bound to its loop + driver before you run it._"
    echo
    steps_section
    echo
    echo "_Authority: \`next-steps.md\` (this folder; read the newest annotations;_"
    echo "_later steps supersede earlier ones). Overtaken steps are labelled above._"
    echo
  fi
  echo "## Loop position (\`.loop-state.json\`)"
  echo
  loop_position_section
  echo
  echo "## Matrix (report-matrix.sh, VERTICAL=${KEY})"
  echo
  matrix_section
  echo
  echo "## Results cells on disk (\`verticals/${KEY}/results\`)"
  echo
  cells_section
  echo
  echo "## Ledger"
  echo
  ledger_section
  echo
  echo "## Slate"
  echo
  echo "- \`repos.md\` (this folder) - admitted repos, pins, and per-candidate verdicts"
} > "$OUT"

echo "Written $OUT" >&2
