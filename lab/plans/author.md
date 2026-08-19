---
phase: author
reads: slate.md
writes: scenario.draft.yaml
emits: [DRAFT, NO-ANCHOR]
wall: 30m
---

# author

## Task

Pick this repository's contract, write the two-step probe scenario that asks for its scattered
dependents, and hand-audit its gold, so the next phase can measure a real baseline against it.

## Scope

You write one scenario and its gold. **Out of scope:** running either arm, the seven-step
expansion, the paid bench, editing a script, any other repository. You do not decide whether the
scenario discriminates; a two-arm run does that next, and it is spawned by the binary.

If a script here is broken, write one line naming it in `notes` and stop. Do not fix it.

## Run

`slate.md` names the repository and the anchor if one is carried. Every rejection
so far is beside it, oldest first, and **all of them are read, not only the last one.**

Reading only the latest read is how six attempts oscillated between "the plain search got
everything" and "neither arm got anything" without ever landing in between.

0. **Fewer than about five runs per arm on disk? Step 1 has no input you may trust, and you say
   so.** At zero there is no citation ranking at all; below five the ordering is noise by step
   1's own measurement — a row that read as a perfect discriminator at n=2 was 4 of 5 by n=5.
   Both cases are the same case, and the threshold is the law's rather than any one repository's.

   Nothing stands in for the ranking. A hand estimate of which rows are free is exactly the
   substitute A GOLD ROW SHOULD COST A READ forbids from a phase that runs before the two-arm
   bench. Write one line in the yaml header saying the free-row class is **unmeasured rather
   than empty**, and carry on from step 2.

   **Half of DECIDE rule 7 is skipped, not all of it.** Its free-and-unreachable-by-citation
   half has no input. Its other half — a row the blast payload does not carry is dead weight —
   needs no run history, and step 5 binds unconditionally: every `dependents` row you write must
   appear in what the arm is shown.

   *(Added by the first live run, jellyfin cycle 1, at n=0. Keyed to five rather than
   to zero in council review, so the case that run never saw — two or three recorded runs — is
   covered too.)*

1. **What the arms have ACTUALLY cited, per gold row, across every run on disk.** Rarest-cited
   first, scoped to the HEADLINE model's results root and not to every root: mixing model
   generations reorders the middle and hides the split, measured 2026-08-03, because a weak
   model's near-total failure lifts rows the headline model finds easily.

   Three classes, and only one of them is worth gold:

   - **baseline low, sense high** — the discriminator. Build the group from these.
   - **cited by every run, both arms** — free. Each one hands the baseline a row before it does
     any work. Ten such rows put a floor of 0.43 under the baseline on the measured mastodon
     gold, which no rewording can undo.
   - **0 of N on BOTH arms** — hard or unreachable. Check the row against the blast payload
     before keeping it: absent from the payload means the tool cannot serve it and the row can
     only ever score zero. Two mastodon spec rows sat at 0 of 12 across two models for exactly
     that reason.

   **n matters here and the ranking says what it has.** At n=2 one row looked like a perfect
   discriminator (baseline 0 of 2) and was 4 of 5 by n=5. Do not re-gold off fewer than about
   five runs per arm; below that the ordering is noise.

2. **The retention listing, for the KIND of question you may ask.** A LISTING, never a gate. It
   reports anchors whose dependents HOLD the contract rather than call it. A non-empty ring puts
   the strongest measured question kind on the menu; an empty one takes it off, and says nothing
   else about the repository.

3. **Candidate anchors**, as a LISTING. Never a gate, never a ranking you defer to.

4. **The seam profile, for three candidates.** This is the one number that has separated wins
   from ties across three verticals.

   `PRECISION` is dependents divided by grep hits. **LOWER IS BETTER**, and the calibration is on
   the record: a banked +1.00 cell profiled at 0.078 (37 dependents in 474 hits); a deliberate
   control tie profiled at 0.78 (29 dependents in 37 files, near-transcription); a dead cell sat
   near 0.94 and handed its baseline 17 of 18 rows in one grep. Record `SCATTER` beside it. A
   precision ABOVE 1.00 means false caller edges, not darkness: check a sample by hand before
   believing it.

5. **What the arm is actually SHOWN**, for the leading candidate, at MCP defaults. Pass no
   `min_confidence`. The shown set is cap-limited, so raising it does not add rows, it changes
   which rows win the same slots. **Every dependents row you write must appear here.**

6. **Open every candidate row and pin the line that actually touches the contract.** The blast
   payload gives the enclosing definition, not the dependency line.

7. **Memorization, per candidate row, on a famous repository.** Cut what comes back recited. A
   famous repository is not disqualified; a memorized row is.

8. After writing the yaml and the rubric, audit the gold: `sense-lab validate -scenario <yaml>
   -checkout <clone> -commit <pin>`. It resolves every row against the pinned checkout, reports
   what would be quarantined, and flags the rows a covering grep for the anchor already prints.

   **`stamp` and `verify` do not exist**, and neither does a rubric check, a gold-confidence
   check or a prompt render. They are named for cycle 07 and are not built. Until they are, the
   gold audit is `validate` plus opening every row by hand, which DO NOT below already requires.

   *(Corrected by the first live run, jellyfin cycle 1: the step named five absent
   things — `stamp`, `verify`, a rubric check, a gold-confidence check and a prompt render — and
   the Done-when list below required their artifacts, so the phase could not report itself done
   by its own criteria.)*

## Decide

The KIND of question first, then the contract, then every gold row.

### The kind of question

**Read the repository's answer-forms page FIRST if it exists.** Each stack answers differently and
the per-repository page is where that is written down, with the n under every claim. It carries
three things this shared plan cannot: the forms already MEASURED to fail here (do not re-buy
them), the forms measured to win here, and the mechanisms killed by a run so they are not
re-proposed.

Measured 2026-08-12: php-laravel spent 36 attempts across 3 repositories and the plain arm's
floor was the binding constraint in every one, because the form being asked for — "every place
that calls or holds X" — is one regex in PHP, where the same form banks +0.775 in Ruby. Same
plan, same laws, opposite outcome, and nothing in the plan said so.

That page ORDERS what to try. It never gates a draft, and a form absent from it is not
forbidden; it is unmeasured, and saying so in the header is the whole obligation.

A search prints occurrences. So a question whose true answer IS a list of occurrences is one the
plain arm can print, and no wording makes it otherwise. Two kinds have been measured.

- **"Find everything that uses X."** An occurrence list, and the evidence cuts BOTH ways, so read
  it before choosing. It is the shape of one repository's banked win (baseline 7 of 23, sense 19
  of 23, +0.53), so the kind is not disqualifying. It is also the shape of five straight failures
  on the same repository, where the plain arm scored 0.81, 0.16, 0.09, 1.00, 1.00 and the gap
  never opened. The banked one sat at step 4 of a 7-step session; the five failures were asked
  cold. If you write this kind, say in the header what makes yours the first sort and not the
  second.
- **"What HOLDS X, and what stays alive when X goes away."** Not who calls it: who keeps a handle
  on it, through a field, an embedded value or a wiring the call sites never name. The strongest
  measured kind in this bench: +0.58, +0.67, +0.80 and +1.00 on four cells, on gold the plain arm
  could not print because the link is established in a file the dependent never mentions.
  **Available only where the retention listing is non-empty.** Measured 2026-08-03: zero rings on
  either Ruby repository swept, top twelve anchors each, so in a stack with no ring this kind is
  off the menu and proposing it anyway writes an unanswerable question.

You are not limited to these two. What you may NOT do is ship an occurrence list and hope. Write
one sentence in the yaml header: **what the answer to this question is, that is not a list of the
places a name appears.** If you cannot write that sentence, you have kind one.

Do not test this with a grep. No pre-run census, no coverage count: the run is what decides, and
the loop has been wrong about this before in both directions.

### Then the contract, the question, and every gold row

1. **Take the repository's central contract** — the model or type everything hangs off, the thing
   a maintainer would actually rework. Not a clever corner.
2. **Rank the candidates by seam profile, lowest precision first.** Precision RANKS; it never
   kills. An anchor is not rejected here for anything a grep prints. The two-arm run in the next
   phase is what rejects.
3. **Frame it as a teardown audit**: "what depends on this before I rework it". Recurring, real,
   and it forces enumeration rather than a single lookup.
4. **Name the MECHANISM that carries the dependents** and write it into the axis: an association,
   a shared concern, a value passed as a constructor argument, a narrowed interface retained in a
   field, duck-typed dispatch, a derived query. This sentence is the scenario's whole thesis and
   the next phase measures it.
5. **Split the pool.** The contract itself and its obvious write path are ANCHORS: both arms reach
   them and they do NOT score. The scattered residue is the `dependents` group and it decides
   everything.
6. **One row per FILE.** A group listing several symbols from one file rewards a single read.
7. **Drop what the ranking says is free or unreachable.** A row every run cites is a gift to the
   baseline; a row no run has ever cited, and that the blast payload does not carry, is dead
   weight that only dilutes the group. Both are measured facts from step 1, not estimates, and
   they are the one place a row may be cut without a run.
8. **The ask names the MECHANISM, never the INVENTORY.** The neutrality check reads tokens, so a
   list of functional categories passes it clean and still hands over the answer. Say HOW
   dependents hide, never WHERE they live. Steering off the anchors is fine.
9. **The prompt is neutral** — no paths, class names, counts, tool names, or answer shape — and
   **every step demands `file:line`.** Without it the task grades at mention level and both arms
   tie.

Prefer rows whose line can only be identified by opening the file. That is a preference at
authoring time and nothing more: it is a hand estimate of a baseline, and the next phase measures
the baseline for real. It may not kill a draft, and no per-row grep census belongs in this phase.

### Re-question: you were routed back with a rejection

**Keep the anchor. Change the question.** The precedent both ways: a tied `serialize_payload`
axis became a winning `Status` teardown audit on the same repository, and a dead
dispatch-tracing axis on a storage batch became a winning retention audit on the SAME TYPE.
Re-scouting a new contract throws away the only measured thing you own.

**Aim at the window, not at the opposite of the last failure.** The window is: the plain arm at
or below 0.50 AND our arm at least 0.50 above it. Both conditions at once. A rewrite that only
fixes the last cycle's complaint lands on the other side of the window and burns a cycle proving
it — measured, five cycles, 0.81 then 0.16 then 0.09 then 1.00 then 1.00.

So before drafting, write the trajectory down as a list and say which side of the window each
cycle fell on. Then state, in one line in the yaml header, **which side you are correcting from
and how far you intend to move**. A cycle that lands on the far side is not progress; it is the
same cycle mirrored.

Read all the carried rejections, not the last one. Each names the rows the plain arm took and the
route it used. Rows it has taken in EVERY cycle are the ones no wording has protected, and
re-golding onto rows it missed once is betting on a sloppy run. **Re-gold from what it has missed
CONSISTENTLY.**

Write the superseded question into the yaml header with the number that killed it. Nothing is
deleted: the previous draft stays on disk in the attempt history, and the question is rewritten
in place.

### Re-anchoring is a separate decision

`NO-ANCHOR` says an anchor is owed. It does not pick one, and it does not throw away the one that
failed. A sub-floor cell is a question the anchor could not carry, not necessarily a bad anchor,
so re-anchoring never happens as a side effect of a re-question.

## Precedent

- A banked +1.00 cell profiled at precision 0.078: 37 dependents in 474 grep hits.
- A deliberate control tie profiled at 0.78: 29 dependents in 37 files, near-transcription.
- A dead cell near 0.94 handed its baseline 17 of 18 rows in one grep, in nine tool calls and 157
  seconds of a 480 second wall.
- Ten free rows put a floor of 0.43 under the baseline on the measured mastodon gold.
- Five cycles on one repository: 0.81, 0.16, 0.09, 1.00, 1.00. The question moved; the window
  was never hit.
- php-laravel, 2026-08-12: 36 attempts across 3 repositories, the plain arm's floor binding in
  every one, on a form that banks +0.775 in Ruby.
- Zero retention rings on either Ruby repository swept, 2026-08-03, top twelve anchors each.

## Artifact

`scenario.draft.yaml`: a TWO-step scenario with `name`, `repo`, `contract_symbol`,
`contract_file`, `description`, `steps`, `scoring`, `gold`. A header comment carries the seam
profile (precision, scatter, hit counts) and the axis sentence.

    step 1  "Map the <contract> contract"                        orientation; its gold is the non-scoring anchors
    step 2  "Audit every dependent of the <contract> contract"    THE discriminator

The rubric carries exactly two top-level keys, `audience` and `steps`, one rubric step per
scenario step, names matching verbatim and in order, each declaring `map_quality`, `specificity`,
`justification` and `uncertainty`. It carries no `repo` key.

A gold row is:

    - {id: d:<slug>, group: dependents, match: [<path fragment>], relation: "<path>:<line> <the exact expression> - one phrase on HOW it uses the contract"}

The verdict is `DRAFT` or `NO-ANCHOR`, with one line of notes carrying the chosen anchor's
precision.

`NO-ANCHOR` only after three candidates were profiled and none can yield twelve dependent files
across six areas from its shown blast set. It is a property of the pool, never of what a grep
prints: `NO-ANCHOR` reached from a token grep is a report about grep, not about the repository.

## Done when

- The scenario has two steps and the rubric has two matching steps in order.
- `dependents` holds at least twelve rows across at least six areas, one file each.
- No `dependents` row appears in the `contract` group.
- The prompt shows no path, symbol, count or tool name. No render command exists; the prompt is
  `description` plus each `steps[].prompt` and two fixed boilerplate sentences
  (`lab/internal/scenario/scenario.go:48-57`), so the check is performed by reading those fields
  in the yaml.
- `sense-lab validate` reports that it resolved every gold row at the pinned commit — the count,
  not the word "skipped" — and quarantines none. **Read the report, not only the exit code:**
  `validatecmd.go:64-69` prints `resolving skipped` and continues with a nil resolver when the
  checkout cannot be opened, and `:77-80` exits non-zero only on a quarantine, so a wrong
  `-checkout` or a bad pin produces a green run that verified nothing.
- Every `dependents` row's `relation` carries a `path:line` and the exact expression standing at
  that line, and `validate` resolves all of them. That is the hand-read leaving a trace; a bullet
  asserting the author was diligent is not checkable and is not one.

  *(The rubric check, the gold-confidence check and the stamp sheet were removed from this list
  by the first live run: the commands they name do not exist, so the bullets could never be
  satisfied. Restore them when cycle 07 builds the commands.*

  *A fourth bullet requiring no `dependents` row inside the covering grep was written and then
  removed in the same run's council review. It rebuilt a retired concept — NO GREP SCREEN IS
  A GATE, IN ANY FORM — in the one place the shape cannot reach, and it is unsatisfiable on the
  shipped corpus: on discourse, 19 of 23 gold rows lie inside a grep for `Category`, so at least
  8 of its 12 `dependents` rows would fail it. `goldcheck.go:68` refuses to quarantine on that
  reason for exactly this reason. Record the covering grep in the yaml header as an input to the
  step 4 ranking; never let it decide.)*
- The seam profile is in the yaml header and the anchor's precision is in the notes.
- The yaml header names the ANSWER FORM this draft uses and its line on the repository's
  answer-forms page: measured-to-win, under test, or unmeasured. A form that page records as
  MEASURED TO FAIL needs the sentence saying what is different this time, or it is the same cycle
  re-bought.
- The yaml header carries the one sentence saying what the answer is that is NOT a list of the
  places a name appears. If the honest answer is "it is a list of occurrences", say so in the
  notes rather than dressing it up.
- On a re-question: the header carries the trajectory of every previous cycle, which side of the
  window each fell on, and which side this draft corrects from.

## Do not

- Do not kill a draft with a grep census, a coverage count or an import dump. The run kills.
- Do not take a credit from a script tally; open the file and read the line, every row.
- Do not delete anything on a re-entry. The anchor stays and the question is rewritten in place.
- Do not spawn a subagent. Only the binary spawns.

## Questioned, not changed

**The RUN steps name what to compute, not which script to invoke.** The old plan named a specific
command per step — the citation ranking, the retention sweep, the anchor listing, the seam profile,
the shown-set probe, the memorization probe, the gold audit — and those scripts have not been
ported into this tree. Naming a path that does not exist would be worse than naming the
computation, because a plan is followed literally. **Cycle 07 binds each step to its command, and
until it does this plan is followed by hand.** Recorded rather than quietly dropped: the exact
commands are the part of this plan most likely to be lost in the move.

## Verdict

Write `verdict.json` in this phase's directory, beside the artifact:

```json
{"phase": "author", "repo": "<repo id>", "cycle": <n>, "verdict": "<one of the emitted>",
 "table": "<why, in one or two sentences>",
 "anchor": "<the symbol this attempt is anchored on>"}
```

The crank reads that file and nothing else to route. A verdict left in prose is a verdict only a
person can act on, and the loop this phase belongs to runs unattended.
