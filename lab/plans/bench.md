---
phase: bench
reads: scenario.yaml
writes: cells.json
emits: [AUTO]
---

# bench

## Task

Run the paid cells: every arm the campaign declares, against the validated scenario, at the real
wall, with both subjects of each cell run under one supervising process.

## Scope

You run what preflight planned. **Out of scope:** authoring, re-questioning, scoring, judging, any
cell the pay call did not clear.

## Decide

Nothing is judged here, and one thing is refused.

**A cell is never split across processes.** Both arms are launched by the same supervising
process, because killing a session between the two arms does not merely cost time: the finished arm
is burned, it can never be paired, and the money spent on it is gone. Catching a half-pair after
the fact is not preventing it.

**An interruption is recorded, not raised.** A cell stopped part way writes a record naming the
arms that finished and can never be paired, and the arms that have no result at all. Without that
record nothing on disk names the burned arm, and a later pass would pair it.

**Every arm gets its declared runs.** The recorded same-cell spread reaches 0.250 against a bar of
0.50, so one run is a sample of a distribution whose spread is half the bar. A cell short of its
replicates is not a measurement and the gates refuse to publish it.

## Precedent

- Recorded same-cell spread: 0.077 to 0.250 of the group, against a bar of 0.50.
- A half-pair whose finished arm was burned because its partner never ran, and the earlier
  instrument's response, which was a function that detected it afterwards.

## Artifact

`cells.json`: every cell, its arms, their run directories, whether the cell is complete, and by
name the arms that are burned or have no result.

## Done when

- Every planned job ran or is named as unusable, and nothing is silently absent.
- Each cell's record says whether it is complete, and names its burned and unusable arms.
- A run that hit its wall is recorded as its own outcome and not as a failure. It is a result, and
  reporting it as success would let a stalled arm pass unnoticed.

## Do not

- Do not raise a wall to rescue a stalled arm. Cannot-finish-at-budget is a result.
- Do not re-run an arm to get a better number. One retry, never a loop, and a replaced run is
  never scored.
- Do not spawn a subagent. Only the binary spawns.
