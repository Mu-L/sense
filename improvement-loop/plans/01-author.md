# PLAN 01-author

## TASK

Pick this repo's contract, write the two-step probe scenario that asks for its scattered
dependents, and hand-audit its gold - so the next phase can measure a real baseline against it.

## SCOPE

You write one scenario and its gold. **Out of scope:** running either arm, the seven-step
expansion, the paid bench, editing a script, any other repo. You do not decide whether the
scenario discriminates - a two-arm run does that next, and it is spawned by the driver.
If a script here is broken, write one line naming it in `notes` and stop; do not fix it.

## RUN

`$CLONE`, `$REPO`, `$VERTICAL`, `$VDIR`, `$YAML`, `$RUBRIC` are exported. Work from
`improvement-loop/`.

0. If `$VDIR/results/loop/$REPO/cycles.jsonl` exists you were routed back from measured runs.
   Read **EVERY** cycle before anything else, not only the last one:

       cat "$VDIR/results/dryrun/$REPO"/cycles.*.jsonl "$VDIR/results/loop/$REPO/cycles.jsonl" 2>/dev/null
       for f in "$VDIR/results/dryrun/$REPO"/minibench.*.md "$VDIR/results/loop/$REPO/minibench.md"; do
         [ -f "$f" ] && { echo "=== $f"; sed -n '/^# The two numbers/,$p' "$f"; }
       done

   Earlier rounds are archived as `cycles.<n>.jsonl` and count: `cycle` restarts each round,
   `attempt` does not. Then read the RE-QUESTION section at the end of DECIDE before anything
   else. Reading only
   the latest read is how six attempts oscillated between "the plain search got everything"
   and "neither arm got anything" without ever landing in between.

0b. The retention listing, for the KIND of question you may ask (see DECIDE):

       python3 bench/lib/ring_sweep.py "$CLONE" --top 12

   A LISTING, never a gate. It reports anchors whose dependents HOLD the contract rather than
   call it. A non-empty ring puts the strongest measured question kind on the menu; an empty
   one takes it off, and says nothing else about the repo.

1. Candidate anchors, as a LISTING - never a gate, never a ranking you defer to:

       python3 bench/lib/anchor_rank.py "$CLONE" --top 20

2. The seam profile, for three candidates. This is the one number that has separated wins from
   ties across three verticals:

       python3 bench/lib/seam_hunt.py "$CLONE" <Symbol> --propose [--file <path>]

   `PRECISION` is dependents divided by grep hits. **LOWER IS BETTER**, and the calibration is
   on the record: a banked +1.00 cell profiled at 0.078 (37 dependents in 474 hits); a
   deliberate control tie profiled at 0.78 (29 dependents in 37 files, near-transcription); a
   dead cell sat near 0.94 and handed its baseline 17 of 18 rows in one grep. Record
   `SCATTER` beside it. A precision ABOVE 1.00 means false caller edges, not darkness; check a
   sample by hand before believing it.

3. What the arm is actually SHOWN for the leading candidate, at MCP defaults:

       python3 bench/lib/mcp_probe.py "$CLONE" \
         '[{"name":"sense_blast","arguments":{"symbol":"<Symbol>"}}]'

   Pass no `min_confidence`. The shown set is cap-limited, so raising it does not add rows, it
   changes which rows win the same slots. Every dependents row you write must appear here.

4. Open every candidate row and pin the line that actually touches the contract. The blast
   payload gives the enclosing `def`, not the dependency line.

5. Memorization, per candidate row, on a famous repo:

       python3 bench/lib/memorization_probe.py "$CLONE" <Symbol> --json <out>.json

   Cut what comes back recited. A famous repo is not disqualified; a memorized row is.

6. After writing the yaml and the rubric, in this order:

       python3 bench/lib/scenario.py "$YAML" --prompt
       python3 bench/lib/rubric_check.py "$YAML"
       python3 bench/lib/gold_confidence_check.py "$YAML" <Symbol> --repo "$CLONE" --group dependents
       python3 bench/lib/gold_audit.py stamp "$YAML"
       python3 bench/lib/gold_audit.py verify "$YAML"

   `stamp` writes one TODO row per gold item; you replace each by opening the file and reading
   the credit. `verify` fails while any TODO remains.

## DECIDE

The KIND of question first, then the contract, then every gold row.

### The kind of question

A search prints occurrences. So a question whose true answer IS a list of occurrences is one
the plain arm can print, and no wording makes it otherwise. Two kinds have been measured.

- **"Find everything that uses X."** An occurrence list, and the evidence cuts BOTH ways, so
  read it before choosing. It is the shape of this repo's banked win (baseline 7 of 23, sense
  19 of 23, +0.53) - so the kind is not disqualifying. It is also the shape of five straight
  failures here, where the plain arm scored 0.81, 0.16, 0.09, 1.00, 1.00 and the gap never
  opened. The banked one sat at step 4 of a 7-step session, the five failures were asked cold.
  If you write this kind, say in the header what makes yours the first sort and not the second.
- **"What HOLDS X, and what stays alive when X goes away."** Not who calls it: who keeps a
  handle on it, through a field, an embedded value or a wiring the call sites never name. The
  strongest measured kind in this bench: +0.58, +0.67, +0.80 and +1.00 on four cells, on gold
  the plain arm could not print because the link is established in a file the dependent never
  mentions. **Available only where step 0b's ring is non-empty.** Measured 2026-08-03: zero
  rings on either Ruby repo swept, top twelve anchors each, so in a stack with no ring this
  kind is off the menu and proposing it anyway writes an unanswerable question.

You are not limited to these two. What you may NOT do is ship an occurrence list and hope.
Write one sentence in the yaml header: **what the answer to this question is, that is not a
list of the places a name appears.** If you cannot write that sentence, you have kind one.

Do not test this with a grep. No pre-run census, no coverage count: the run is what decides,
and the loop has been wrong about this before in both directions.

### Then the contract, the question, and every gold row

1. **Take the repo's central contract** - the model or type everything hangs off, the thing a
   maintainer would actually rework. Not a clever corner.
2. **Rank the candidates by seam profile, lowest precision first.** Precision RANKS; it never
   kills. An anchor is not rejected here for anything a grep prints - the two-arm run in the
   next phase is what rejects.
3. **Frame it as a teardown audit**: "what depends on this before I rework it". Recurring,
   real, and it forces enumeration rather than a single lookup.
4. **Name the MECHANISM that carries the dependents** and write it into the axis: an
   association, a shared concern, a value passed as a constructor argument, a narrowed
   interface retained in a field, duck-typed dispatch, a derived query. This sentence is the
   scenario's whole thesis and the next phase measures it.
5. **Split the pool.** The contract itself and its obvious write path are ANCHORS: both arms
   reach them and they do NOT score. The scattered residue is the `dependents` group and it
   decides everything.
6. **One row per FILE.** A group listing several symbols from one file rewards a single read.
7. **The ask names the MECHANISM, never the INVENTORY.** `scenario.py --prompt` reads tokens,
   so a list of functional categories passes it clean and still hands over the answer. Say HOW
   dependents hide, never WHERE they live. Steering off the anchors is fine.
8. **The prompt is neutral** - no paths, class names, counts, tool names, or answer shape - and
   **every step demands `file:line`.** Without it the task grades at mention level and both
   arms tie.

Prefer rows whose line can only be identified by opening the file. That is a preference at
authoring time and nothing more: it is a hand estimate of a baseline, and the next phase
measures the baseline for real. It may not kill a draft, and no per-row grep census belongs in
this phase.

### RE-QUESTION (you were routed back from measured runs)

**Keep the anchor. Change the question.** The precedent both ways: a tied `serialize_payload`
axis became a winning `Status` teardown audit on the same repo, and a dead dispatch-tracing
axis on a storage batch became a winning retention audit on the SAME TYPE. Re-scouting a new
contract throws away the only measured thing you own.

**Aim at the window, not at the opposite of the last failure.** The window is: the plain arm
at or below 0.50 AND our arm at least 0.50 above it. Both conditions at once. A rewrite that
only fixes the last cycle's complaint lands on the other side of the window and burns a cycle
proving it - measured, five cycles, 0.81 then 0.16 then 0.09 then 1.00 then 1.00.

So before drafting, write the trajectory down as a list and say which side of the window each
cycle fell on. Then state, in one line in the yaml header, **which side you are correcting
from and how far you intend to move**. A cycle that lands on the far side is not progress; it
is the same cycle mirrored.

Read all the reads, not the last one. The archived `minibench.N.md` files each name the rows
the plain arm took and the route it used. Rows it has taken in EVERY cycle are the ones no
wording has protected, and re-golding onto rows it missed once is betting on a sloppy run.
Re-gold from what it has missed CONSISTENTLY.

Write the superseded question into the yaml header with the number that killed it, and keep
the previous yaml as `$VDIR/scenarios/$REPO.yaml.<slug>.bak`.

## ARTIFACT

Write `$YAML`: a TWO-step scenario, `name`, `repo`, `contract_symbol`, `contract_file`,
`description`, `steps`, `scoring`, `gold`. A header comment carries the seam profile
(precision, scatter, hit counts) and the axis sentence.

    step 1  "Map the <contract> contract"      - orientation; its gold is the non-scoring anchors
    step 2  "Audit every dependent of the <contract> contract"  - THE discriminator

Write `$RUBRIC` with exactly two top-level keys, `audience` and `steps`, one rubric step per
scenario step, names matching verbatim and in order, each declaring `map_quality`,
`specificity`, `justification` and `uncertainty`. It carries no `repo:` key.

A gold row is:

    - {id: d:<slug>, group: dependents, match: [<path fragment>], relation: "<path>:<line> <the exact expression> - one phrase on HOW it uses the contract"}

Then write `$VDIR/results/loop/$REPO/author.verdict.json`:

    {
      "phase":    "author",
      "repo":     "<repo>",
      "verdict":  "DRAFT" | "NO-ANCHOR",
      "artifact": "verticals/<vertical>/scenarios/<repo>.yaml",
      "notes":    "one line, with the chosen anchor's precision in it"
    }

`NO-ANCHOR` only after three candidates were profiled and none can yield twelve dependent
files across six areas from its shown blast set. It is a property of the pool, never of what a
grep prints. `NO-ANCHOR` reached from a token grep is a report about grep, not about the repo.

## DONE WHEN

- `$YAML` has two steps and `$RUBRIC` has two matching steps in order.
- `dependents` holds at least twelve rows across at least six areas, one file each.
- No `dependents` row appears in the `contract` group.
- `scenario.py --prompt` shows no path, symbol, count or tool name in the rendered prompt.
- `rubric_check.py` exits 0.
- `gold_confidence_check.py --group dependents` exits 0.
- `gold_audit.py verify` exits 0: zero TODO rows, gold unchanged under a finished sheet.
- The seam profile is in the yaml header and the anchor's precision is in `notes`.
- The yaml header carries the one sentence saying what the answer is that is NOT a list of
  the places a name appears. If the honest answer is "it is a list of occurrences", say so in
  `notes` rather than dressing it up.
- On a re-question: the header carries the trajectory of every previous cycle, which side of
  the window each fell on, and which side this draft corrects from.
- The verdict JSON exists and parses.

## DO NOT

- Do not kill a draft with a grep census, a coverage count or an import dump. The run kills.
- Do not take a credit from a script tally; open the file and read the line, every row.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
