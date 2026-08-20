---
phase: harvest
reads: report.md
writes: harvest.json
emits: [WIN CONFIRMED, DoD FAIL]
wall: 30m
---

# harvest

## Task

Run the five mechanical checks against a reported win and confirm it, or bounce it.

## Scope

Five checks against on-disk verifier output, in order, each numbered in the verdict. **Out of
scope:** diagnosing a sub-floor cell, reading transcripts for causes, proposing levers,
re-running anything, editing anything.

**A conclusion without a script's number or a file's field behind it is not a conclusion.**

## Run

**The five checks. Run ALL, in order, and number each in the verdict.**

1. **Discriminator.** The per-group verdict reports a win: a gold-group delta at or above +0.50
   held across BOTH runs, or an efficiency-at-parity win where recall ties and the sense arm is
   robustly cheaper. Quote the per-run numbers.

2. **Sense adoption.** Every sense run made at least one MCP call. Quote each.

3. **Leak-free baseline.** Every baseline run made zero MCP calls: no Sense leaked into the control
   arm.

4. **Leak-free prompt.** Render the prompt and confirm no gold identifier appears in it verbatim.
   Identifiers the prompt names as given context are exempt only where the gold curation notes mark
   them as out of gold or shown.

5. **No hallucinated cites, and a legitimate baseline.** Spot-check at least two credited gold
   dependents per arm: the credited identifier must actually appear in that run's transcript, which
   is the guard against a basename matching by accident. Confirm every run scored without failing.

## Decide

**Confirm and stop.** When the five checks pass, write the verdict and end. Inventing problems in a
clean win means the reading is over-tuned.

**A run flag that does not move one of the five checks is not a finding.** A nonzero exit code on a
run that still scored, a constrained flag, noisy logs: a recorded win stands unless a check itself
fails.

**Sub-floor is not yours.** If the numbers are below the bar, this cell should never have reached
you: say so and stop. Diagnosis belongs to the report phase.

- **`WIN CONFIRMED`** — all five pass, with the numbers for each.
- **`DoD FAIL`** — one check fails. Name the check by number and the number or field that fails.
  It goes back for diagnosis and **never to the board**: a cell that failed its confirmation checks
  is not a result to publish.

## Precedent

The check that exists because of a specific failure is number 5: a credited gold dependent whose
identifier never appeared in the transcript, credited on a basename that matched something else.
Every check here is one that a recorded run needed.

## Artifact

`harvest.json`: the verdict, and the five checks with the number or field each was decided on.

## Done when

- All five checks ran, in order, and each is numbered in the verdict.
- Every check carries the number or field it was decided on, quoted.
- On a failure, exactly one check is named as the cause.

## Do not

- Do not fault-find a clean win.
- Do not diagnose a sub-floor cell.
- Do not spawn a subagent. Only the binary spawns.

## Questioned, not changed

**The RUN steps name what to compute, not which script to invoke**, because those scripts are not
ported into this tree yet and a plan that names a path which does not exist is worse than one that
names the computation. Cycle 07 binds each step to its command. See `author.md` for the full note.

## Verdict

Write `verdict.json` in this phase's directory, beside the artifact:

```json
{"phase": "harvest", "repo": "<repo id>", "cycle": <n>, "verdict": "<one of the emitted>",
 "table": "<why, in one or two sentences>"}
```

The crank reads that file and nothing else to route. A verdict left in prose is a verdict only a
person can act on, and the loop this phase belongs to runs unattended.
