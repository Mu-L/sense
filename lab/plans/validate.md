---
phase: validate
reads: scenario.yaml
writes: pay-call.md
emits: [PAY, DO-NOT-PAY]
wall: 25m
---

# validate

## Task

Read the unscored validation run the binary just produced against the full seven-step scenario,
at the real wall, and rule on one thing: does this cell go to the paid bench, or back to the
question.

## Scope

You read a run that already exists and issue one verdict. **Out of scope:** re-running anything,
running the paid bench, editing the scenario or its gold, re-scoring, raising a wall, any other
repository. You do not diagnose a paid loss; a separate phase owns that. You do not fix the
harness.

**Both arms are guaranteed to be MEASUREMENTS.** Run validity is mechanical, not a judgment: the
runner classifies every run, retries a void arm once and parks the void one, and this phase halts
rather than reaching you if a pair is still incomplete. So `PAY` and `DO-NOT-PAY` are always
issuable and neither is a comment on arm health.

If you are ever handed a void arm anyway, that is a defect in the binary: **write it down and
STOP.** Never spend on a pair you cannot read.

**Out of clock is not void.** An arm that hit its wall is valid, it is the arm's own result, and
for the baseline it is the win condition.

## Run

1. **The credit table, both arms, per gold item.** This is your one mechanical input.

2. **Arm health, before believing any floored number.** Wall clock against the wall, exit code,
   whether the arm was cut off. A cut arm produces a false loss.

3. **What the sense arm did with its tools.**

4. **What it cited but was never returned, and where it fell back.** Scope the mine to THIS
   validation cell; a repository-wide root would blend in cells this scenario already replaced.

5. **How the BASELINE assembled its answer** — the route, not the number.

6. **The session row.** The mini-bench measured this same discriminator step in ISOLATION; this
   run measures it after three steps of accumulated context and wall. Put both baseline
   `dependents` figures side by side and state which way the session moved it. One row per cell is
   how the loop learns whether the isolated step is a fair predictor of the session, and it is
   free.

## Decide

**The bar is +0.50 on the `dependents` group, not a visible gap.** A cell can discriminate clearly
and still be worth nothing: baseline 0.625 against sense 0.938 is a real, tool-driven gap of +0.31
and it is a LOSS, because the floor is +0.50. Quote the delta and compare it to 0.50 explicitly
before writing a verdict.

Three readings, and only one of them pays.

- **The baseline assembled too much of the set.** It cited the scattered residue at `path:line`,
  not just the anchors, and the group delta is under +0.50. **DO-NOT-PAY.** The lever is the
  QUESTION: name the rows the baseline took and the route it used, exactly as the mini-bench does,
  so the next draft can defeat that route on the same anchor.
- **Neither arm reached.** The sense arm never called `sense_blast`, dropped what came back, or
  never reached synthesis inside the wall. **DO-NOT-PAY.** A scenario the instrument cannot answer
  is not a measurement of the baseline.
- **The sense arm reached what the baseline did not, by at least the floor.** The gap is in
  `dependents`, at `path:line`, and the group delta is at or above +0.50. **PAY.**

**Cannot-finish-at-budget IS a result.** If the sense arm ran out of wall, that is the verdict, not
an obstacle. Never propose raising the wall to rescue it.

Every claim in the artifact is quoted output. A number you did not read out of the credit table or
a run record does not go in the file.

This run is unscored: it may not settle a win, a tie or a loss, and its number may never be cited
anywhere. It decides one thing, which is **whether money moves**.

## Precedent

- Baseline 0.625 against sense 0.938: a real, tool-driven +0.31 gap, and a loss, because the bar
  is +0.50.
- The arithmetic that makes the bar a screen and not a preference: `delta = sense − base` and
  `sense ≤ 1.00`, so `+0.50` REQUIRES `base ≤ 0.50`. Ten of twenty paid cells in the retired
  instrument were dead before they ran.

## Artifact

`pay-call.md`, five headings:

    # Verdict         PAY or DO-NOT-PAY, and the single sentence that decides it
    # Credit table    the sense-only rows, the shared rows, the neither rows
    # Arm health      wall clock and exit per arm, quoted
    # Session row     baseline dependents recall: isolated step vs full session
    # If DO-NOT-PAY   which of the three readings, the rows the baseline took, and what the
                      next question must defeat

A `DO-NOT-PAY` keeps the anchor and the scenario on disk. The lever is the question, and the
authoring phase rewrites it in place against this credit table.

## Done when

- Both arms' transcripts were read from disk, not inferred from an exit code.
- The credit table output is quoted.
- The `dependents` delta is stated as a number and compared to +0.50 in writing.
- Arm health is quoted per arm, and no floored score is believed without it.
- The session row carries both baseline figures and says which way the session moved it.
- On `DO-NOT-PAY`, every row the baseline cited is listed by id.

## Do not

- Do not pay on a hand-grep, a proxy, or a scenario that "looks strong". The run decides.
- Do not propose raising a wall, extending a budget, or re-running to get a better sample.
- Do not spawn a subagent. Only the binary spawns.

## Questioned, not changed

**The RUN steps name what to compute, not which script to invoke**, because those scripts are not
ported into this tree yet and a plan that names a path which does not exist is worse than one that
names the computation. Cycle 07 binds each step to its command. See `author.md` for the full note.

## Verdict

Write `verdict.json` in this phase's directory, beside the artifact:

```json
{"phase": "validate", "repo": "<repo id>", "cycle": <n>, "verdict": "<one of the emitted>",
 "table": "<why, in one or two sentences>"}
```

The crank reads that file and nothing else to route. A verdict left in prose is a verdict only a
person can act on, and the loop this phase belongs to runs unattended.
