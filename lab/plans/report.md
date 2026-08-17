---
phase: report
reads: cells.json
writes: report.md
emits: [WIN, DIAGNOSIS]
---

# report

## Task

Read the benched cells and rule on one thing: is this a win, or is it a result to be handed up
with its numbers.

## Scope

You read runs that already exist and issue one verdict. **Out of scope:** re-authoring the
question, editing the scenario or its gold, re-scoring, running any arm, raising a wall, any other
repository.

You do not judge whether Sense did well. The numbers say that, without you.

## Run

1. **The build gate, before believing any column.** A non-zero exit means the installed Sense is
   not the build the headline column was banked at. Stop and say so: there is no board to build
   today.

2. **The numbers**, per arm: the measured flag, the routing state, the run count, and whether the
   two runs land in different dominant cells.

3. **Per arm, the mechanism table and its routing states, run by run.**

## Decide

One verdict for the run set. The recipe, applied per arm:

| what you see | what it is | what it earns |
|---|---|---|
| a harness failure in any run | the MCP server never came up: nothing was measured | re-run that arm |
| fewer than 2 measured sense runs | an incomplete pair | re-run that arm |
| the two runs land in different dominant cells | a split | re-run that arm, once, for a third run |
| never routed, or search only | the server was up and the model did not use it | KEEP. A finding, not a fault |
| routed, 2 measured runs | a measurement | KEEP |

**A never-routed arm is never re-run to get a better number. It is the result.**

Re-runs are bounded: an arm already re-run twice for the same reason is reported as it stands, with
the reason named. Never raise a wall or a budget to rescue one.

Then the verdict on the cell:

- **`WIN`** — the discriminator group's delta holds at or above +0.50 across BOTH runs, or recall
  ties and the sense arm is robustly cheaper. Quote the per-run numbers. It goes to harvest, where
  the confirmation checks are run mechanically.
- **`DIAGNOSIS`** — anything else. **The loop cannot record a loss**: it wins, it parks, or it
  hands up a swap with the numbers attached. A sub-floor cell is handed up with its diagnosis
  rather than quietly re-authored, because the cell was paid for and the finding belongs to
  whoever decides what changes next.

**Objective recall is the headline and the blind composite must never lead.** The precedent is
expensive: the retired instrument's reporter ranked by the blind composite anyway and published a
+0.44 win as a tie. The document was right for months while the table was wrong.

## Precedent

- A published +0.44 win reported as a tie, because the reporter ranked by the blind composite while
  the manifesto said objective recall leads.
- Ten of twenty paid cells in the retired instrument were arithmetically dead before they ran, at a
  baseline above 0.50.

## Artifact

`report.md`: one short section per arm, each carrying the arm's routing states, its measured run
count, and the one line of quoted output the call was based on. Then the verdict, and on a
diagnosis, what the numbers say and what it would take.

## Done when

- Every arm the campaign declares appears in the report.
- Every claim is quoted output.
- The discriminator delta is stated per run, not pooled, and compared to +0.50 in writing.
- An arm that never routed is reported as a finding, in our own words, and not dressed as a model
  failing.

## Do not

- Do not write a number you did not read out of the numbers, and do not re-round one.
- Do not edit the scenario, its gold, its rubric, or any script.
- Do not name a competing tool, promise a fix, or apologise for a gap. State it and move on.
- Do not spawn a subagent. Only the binary spawns.

## Questioned, not changed

The retired instrument split this phase in two: a validity verdict on whether the run set was
sound, and a separate reading written on top of an already-rendered board. Here the graph has one
report phase, and the reading of what the numbers mean sits in it. The split is recorded because
it had a reason — the reading was written after the figures were fixed, so no figure could be
chosen to suit the sentence — and that property is held here by the rule that every claim is
quoted output rather than by a phase boundary. Recorded rather than resolved.
