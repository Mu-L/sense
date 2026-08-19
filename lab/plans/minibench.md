---
phase: minibench
reads: scenario.draft.yaml
writes: minibench.md
emits: [PROCEED, REQUESTION, NO-ANCHOR]
wall: 40m
---

# minibench

## Task

Read the unscored two-arm run the binary just produced against the two-step probe scenario, and
rule on one thing: is this question worth expanding into a full session.

## Scope

You read a run that already exists and issue one verdict. **Out of scope:** re-running anything,
expanding the scenario, editing the yaml or its gold, re-scoring, raising a wall, the paid bench,
any other repository.

You do not re-author. A routed `REQUESTION` sends the credit table back to the authoring phase,
which owns the rewrite.

**Both arms are guaranteed to be MEASUREMENTS.** Run validity is mechanical, not a judgment: the
runner classifies every run, retries a void arm once and parks the void one, and the phase halts
rather than reaching you if a pair is still incomplete. So the three verdicts below are always
issuable, and none of them is a comment on arm health.

If you are ever handed a void arm anyway, that is a defect in the binary: **write it down and
STOP.** Do not invent a fourth verdict, and do not average a harness artifact into a number.

**Out of clock is not void.** An arm that hit its wall is valid, it is the arm's own result, and
for the baseline it is the win condition.

## Run

1. **How many runs are you reading, and which are valid.** The cell may hold one pair or two: a
   baseline that landed near the bar is re-run once before you are spawned. Print that first and
   take the numbers from it.

   **The number you rule on is the MEAN over VALID runs of that arm, never run 1 alone.**
   Within-arm spread on one cell is 0.077 to 0.250 of the group — one to three gold rows — so a
   single run is a draw, not a reading: one recorded cell read +0.538 on one run and +0.423 on
   two. A run that is not valid is NOT averaged in and NOT counted: a dead half-pair once
   averaged a 0.0 with a 0.944 and manufactured a phantom +0.528 cell.

   If only one valid baseline run exists anyway, the second pair failed. Say so in the artifact
   and rule on n=1 with that stated.

2. **The credit table, both arms, per gold item.** This is your one mechanical input.

3. **Arm health, before believing any floored number.** Wall clock against the wall, exit code,
   whether the arm was cut off. A cut arm produces a false result in either direction.

4. **What the sense arm did with its tools.**

5. **How the BASELINE assembled its answer** — the route, not the number. Quote the route in your
   artifact. One search that returned the candidate set, followed by reads of files the ask named
   by function, means the question measured the PROMPT and not the contract. The precedent: nine
   tool calls, one `grep -rn "Setting\."`, 17 of 18 dependents, 157 seconds of a 480 second wall.

6. **What the sense arm cited that was never returned to it.** Scope the mine to THIS cycle's
   pair; a repository-wide root would blend in the cycles this question already replaced.

## Decide

**Both arms must pass, and the bar is arithmetic.** Quote the `dependents` group figures from the
credit table and compare them in writing.

- **`PROCEED`** — the baseline holds **at or below 0.50** of `dependents` AND the sense arm beats
  it by **at least +0.50**. Both conditions, quoted. This is the shape of every banked win on the
  record: a mini run at baseline 0 of 5 against sense 5 of 5 preceded a +1.00 cell.
- **`REQUESTION`** — the baseline holds **above 0.50**. The question does not discriminate however
  clean the scenario looks, because recall caps at 1.00 and a baseline at B caps the delta at
  `1.00 − B`. The lever is the QUESTION, not the anchor: name, in your artifact, the rows the
  baseline took and the route it used, so the next draft can ask something that route cannot
  answer.
- **`REQUESTION`** — the sense arm did not reach either. It never called `sense_blast`, dropped
  what came back, or never reached synthesis inside the wall. A question the instrument cannot
  answer is not a measurement of the baseline. Say which of the three it was.
- **`NO-ANCHOR`** — only when the shown blast set itself cannot carry twelve dependent files,
  which is a defect in the authoring phase, not a result. Everything else routes back for a
  rewrite.

**Cannot-finish-at-budget IS a result.** If an arm ran out of wall, that is the finding, not an
obstacle. Never propose raising the wall.

Every claim in the artifact is quoted output. A number you did not read out of the credit table or
a run record does not go in the file.

This cell is one or two runs per arm and **unscored**: it may not settle a win, a tie or a loss,
and its number may never be cited anywhere. It decides one thing, which is whether this question
gets a full session. State the n you ruled on, per arm: a delta carries the run count that
produced it or it is not a delta.

## Precedent

- Within-arm spread on one cell: 0.077 to 0.250 of the group, one to three gold rows.
- One cell read +0.538 on a single run and +0.423 over two.
- A dead half-pair averaged 0.0 with 0.944 and manufactured a phantom +0.528 cell.
- The baseline route that killed a scenario: nine tool calls, one `grep -rn "Setting\."`, 17 of 18
  dependents, 157 seconds of a 480 second wall.
- A mini run at baseline 0 of 5 against sense 5 of 5 preceded a +1.00 banked cell.

## Artifact

`minibench.md`, five headings:

    # Verdict           PROCEED, REQUESTION or NO-ANCHOR, and the sentence that decides it
    # Credit table      quoted, with the sense-only rows, the shared rows, the neither rows
    # The two numbers   baseline dependents recall vs 0.50, and the delta vs +0.50, with the n per arm
    # Baseline route    how it assembled the set, quoted
    # If REQUESTION     the rows the baseline took, and what the next question must defeat

## Done when

- Both arms' transcripts were read from disk, not inferred from an exit code.
- The credit table output is quoted.
- The baseline's `dependents` recall is stated as a number and compared to 0.50 in writing.
- The delta is stated as a number and compared to +0.50 in writing.
- Arm health is quoted per arm, and no floored score is believed without it.
- The n ruled on is stated per arm.
- On `REQUESTION`, every row the baseline cited is listed by id.

## Do not

- Do not re-author, re-gold or edit the yaml. Name the lever; the authoring phase pulls it.
- Do not propose raising a wall, extending a budget, or re-running to get a better sample.
- Do not spawn a subagent. Only the binary spawns.

## Questioned, not changed

**The RUN steps name what to compute, not which script to invoke**, because those scripts are not
ported into this tree yet and a plan that names a path which does not exist is worse than one that
names the computation. Cycle 07 binds each step to its command. See `author.md` for the full note.

## Verdict

Write `verdict.json` in this phase's directory, beside the artifact:

```json
{"phase": "minibench", "repo": "<repo id>", "cycle": <n>, "verdict": "<one of the emitted>",
 "table": "<why, in one or two sentences>",
 "anchor": "<the symbol this attempt is anchored on>"}
```

The crank reads that file and nothing else to route. A verdict left in prose is a verdict only a
person can act on, and the loop this phase belongs to runs unattended.
