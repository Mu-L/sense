#!/usr/bin/env bash
# render-status.sh - regenerate a vertical's STATUS.md from disk. Readability
# artifact per ledger.md: humans read it, loops NEVER do (write-only law).
# Every section is recomputed from the authoritative sources (results tree,
# next-steps.md, repos.md); the file is replaced wholesale so it can never drift
# into hand-edited state.
#
# NO MATRIX SECTION. It used to embed report-matrix.sh, which labels whatever
# directory it finds as a model: after the one-root-per-question migration that
# published scenario-version hashes (and, at another root, "minibench" and
# "validation") in the model column with real numbers beside them. Removed
# 2026-08-04 rather than repaired - nothing reads the matrix (no plan references
# it), and the cross-model surface it exists for gets designed against the
# versioned tree when the bench-all-models plan is written.
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

banked_section() {
  # THE SCORES, not the inventory. Every decision-bearing number in this loop is
  # print-only (pergroup, pay_ceiling, cost_parity), so until banked.jsonl existed a
  # reader of STATUS.md could see that a repo was `done` and never learn what it
  # scored. Rendered from the index, never recomputed here: one derivation of a
  # verdict, and `banked.py rebuild` is what refreshes it from disk.
  local banked="$DOC_DIR/banked.jsonl"
  if [ ! -s "$banked" ]; then
    echo "_no banked cells yet (\`python3 bench/lib/banked.py rebuild verticals/${KEY}\`)_"
    return
  fi
  python3 - "$banked" <<'PY'
import json, sys

rows = []
for line in open(sys.argv[1]):
    line = line.strip()
    if line:
        try:
            rows.append(json.loads(line))
        except ValueError:
            continue
if not rows:
    print("_banked.jsonl holds no readable rows_")
    raise SystemExit(0)
print("| repo | verdict | overall Δ | best group | runs | scenario |")
print("|---|---|---|---|---|---|")
for r in sorted(rows, key=lambda x: (x.get("repo") or "")):
    groups = r.get("groups") or {}
    best = max(groups.items(), key=lambda kv: kv[1].get("delta", 0), default=(None, {}))
    runs = r.get("runs") or {}
    ver = (r.get("scenario_version") or "").replace("sha256:", "")[:16]
    print("| %s | %s | %+.2f | %s %+.2f | %s/%s | `%s` |" % (
        r.get("repo"), r.get("verdict"),
        (r.get("overall") or {}).get("delta", 0.0),
        best[0] or "-", best[1].get("delta", 0.0),
        runs.get("baseline", 0), runs.get("sense", 0), ver))
print()
print("_Rows are re-derivable from the results tree: `banked.py rebuild verticals/<key>`._")
PY
}

cells_section() {
  if [ -d "$RESULTS" ]; then
    local found=0
    while IFS= read -r cell; do
      found=1
      local runs
      runs=$(find "$cell" -mindepth 1 -maxdepth 1 -type d -name 'run-*' | wc -l | tr -d ' ')
      echo "- \`${cell#"$RESULTS"/}\` - $runs run(s)"
    # Any depth, not a pinned one: cells used to sit at <model>/<arm>/<repo>/run-N
    # and the one-root-per-question migration added a <version> segment, so a
    # -maxdepth 4 walk reported "no run cells on disk yet" over a full results
    # tree. The printed path carries whatever segments are actually there.
    done < <(find "$RESULTS" -type d -name 'run-*' -exec dirname {} \; | sort -u)
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
#
# The phase ALONE is not the position, and reading it as one has already cost a session. A
# repo parked at `scout` looks identical whether the scout will re-run or whether a stored
# verdict decides without spawning anything: `have_verdict` is idempotent, so a phase holding
# a verdict skips its agent entirely. That session re-ran the loop, watched the driver replay
# a two-day-old NO-AXIS, and reported it as a fresh result. So the verdict on disk renders
# beside the phase, and the cell says which way the next run goes.
# Cycle 2: where each cell is on the road to a published board, and which pages
# already stand. STATUS.md is where a resuming session looks; before this it could
# only see cycle 1 and read as though nothing else existed.
cycle2_section() {
  local st="$IL_ROOT/verticals/$KEY/.cycle2-state.json"
  local reports="$IL_ROOT/verticals/$KEY/reports"
  if [ ! -f "$st" ] && [ ! -d "$reports" ]; then
    echo "_no cycle 2 run yet. Eligible cells:_ \`bash bench/drivers/cycle2-board.sh --eligible\`"
    return
  fi
  if [ -f "$st" ]; then
    python3 -c "
import json, sys
d = json.load(open(sys.argv[1]))
rows = [(k, v) for k, v in sorted(d.items()) if '#' not in k]
if not rows:
    print('_no cell has entered cycle 2 yet._')
else:
    print('| repo | phase | re-runs |')
    print('|---|---|---|')
    for k, v in rows:
        print(f'| {k} | {v} | {d.get(k + chr(35) + \"reruns\", 0)} |')
" "$st"
    echo
  fi
  if [ -d "$reports" ] && [ -n "$(ls -A "$reports" 2>/dev/null)" ]; then
    echo "Published boards:"
    echo
    for f in "$reports"/*.md; do
      [ -f "$f" ] || continue
      echo "- \`reports/$(basename "$f")\`"
    done
  else
    echo "_no board published yet._"
  fi
  echo
  # State without an action is why a resuming session stalls: the cycle 1 section has
  # carried its resume line since it was written, and this one read as a report.
  if [ -f "$st" ] && python3 -c "
import json,sys
d=json.load(open(sys.argv[1]))
sys.exit(0 if [k for k,v in d.items() if '#' not in k and v!='done'] else 1)" "$st" 2>/dev/null; then
    echo "Resume: \`VERTICAL=${KEY} bash bench/drivers/cycle2-board.sh <repo>\`  (repo keys above)"
  else
    echo "Start:  \`VERTICAL=${KEY} bash bench/drivers/cycle2-board.sh --eligible\`, then"
    echo "\`VERTICAL=${KEY} bash bench/drivers/cycle2-board.sh <repo>\` for one of them."
  fi
}

loop_position_section() {
  local st="$IL_ROOT/verticals/$KEY/.loop-state.json"
  if [ ! -f "$st" ]; then
    echo "_no \`.loop-state.json\` yet - the loop has never been run for this vertical._"
    return
  fi
  python3 -c "
import glob,json,os,sys
d=json.load(open(sys.argv[1]))
loopdir=sys.argv[2]
root=sys.argv[3]
results=sys.argv[4]
key=sys.argv[5]
# The phases that spawn an agent and store its verdict; the rest are automatic.
AGENT_PHASES=('author','minibench','expand','validate')

def parked_rounds(repo):
    # A PARK IS INVISIBLE IN THE STATE FILE, and that cost a session 6 paid cycles.
    # At AUTH_CYCLE_MAX the driver writes back phase=author and authcycle=0 - byte-identical
    # to a repo that has never authored - archives the round ledger as cycles.N.jsonl and
    # writes handoff.md for a human. So the pickup file rendered 'the next run RE-SPAWNS the
    # author agent' with a resume command under it, over a repo that had already spent 24
    # attempts and was waiting on a decision. Re-running is what the park exists to prevent,
    # and nothing on this page said so.
    # handoff.md has ONE spawn path in the driver (the ceiling), so its presence means a park
    # happened; authcycle==0 is what distinguishes 'still parked' from 'a human re-ran it'.
    h=os.path.join(loopdir,repo,'handoff.md')
    if not os.path.exists(h):
        return 0
    if str(d.get(repo+chr(35)+'authcycle','')) != '0':
        return 0
    return len(glob.glob(os.path.join(results,'dryrun',repo,'cycles.*.jsonl')))

p={k:v for k,v in d.items() if '#' not in k}
if not p:
    print('_state file present, no repo phases recorded._')
    raise SystemExit(0)
parked={k:n for k,n in ((k,parked_rounds(k)) for k in p) if n}
print('| repo | phase | verdict on disk |'); print('|---|---|---|')
for k in sorted(p):
    ph=p[k]
    if k in parked:
        cell='**PARKED** - awaiting YOUR decision, do not resume'
    elif ph not in AGENT_PHASES:
        cell='- (no agent at this phase)'
    else:
        try:
            with open(os.path.join(loopdir,k,ph+'.verdict.json')) as fh:
                data=json.load(fh)
            v=data.get('verdict','?')
            # verdict_check.py refuses a verdict whose artifact is gone, and the driver
            # then re-spawns. A measuring phase's verdict outlives its read the moment a
            # later re-entry archives it, so this is the realistic case, not a corner.
            art=data.get('artifact')
            if art and os.path.exists(os.path.join(root,art)):
                cell='\`%s\` stored - the %s agent will NOT re-run' % (v,ph)
            else:
                cell='\`%s\` stored but its artifact is GONE - the %s agent re-runs' % (v,ph)
        except (OSError,ValueError):
            cell='none - the next run RE-SPAWNS the %s agent' % ph
    print('| %s | %s | %s |' % (k,ph,cell))
print()
for k in sorted(parked):
    n=parked[k]
    print('**%s is PARKED** after %d round(s) of authoring - roughly %d attempts - without a'
          % (k,n,n*6))
    print('payable question. The loop stopped ON PURPOSE and handed the call up to you.')
    print()
    print('- READ FIRST: \`%s\`' % os.path.relpath(os.path.join(loopdir,k,'handoff.md'),
                                                   os.path.join(root,'verticals',key)))
    print('- the numbers behind it: \`results/dryrun/%s/cycles.*.jsonl\`' % k)
    print('- re-running spends another 6 cycles and is a DECISION, not a resume. The other')
    print('  option is swapping the slot for its own declared backup in slate.json.')
    print()
live=[k for k in sorted(p) if k not in parked]
if live:
    print('Resume - run the line for the repo you are taking:')
    for k in live:
        print('- \`VERTICAL=$KEY bash bench/drivers/vertical-loop.sh %s\`' % k)
else:
    print('_No repo is resumable: every repo above is parked on a human decision._')
" "$st" "$RESULTS/loop" "$IL_ROOT" "$RESULTS" "$KEY"
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
  echo "## Cycle 2: cross-model boards (\`.cycle2-state.json\`, \`reports/\`)"
  echo
  cycle2_section
  echo
  echo "## Banked cells (\`verticals/${KEY}/banked.jsonl\`)"
  echo
  banked_section
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
