---
phase: board
reads: harvest.json
writes: board.md
emits: [AUTO]
---

# board

## Task

Bank the confirmed win: write it onto the board with every figure it was confirmed at.

## Scope

You record a result that is already decided. **Out of scope:** re-scoring, re-reading, re-judging,
choosing what to publish, any cell that is not confirmed.

## Decide

Nothing. A cell reaches this phase only with a confirmed win behind it.

**Objective recall is the headline and the blind composite never leads.** A board ranked by the
composite published a +0.44 win as a tie while the document saying so was right for months. The
ordering is not a presentation choice.

**Cost is recorded and never gates.** Costing more is a product finding, not a stopper, so a cell
that wins its discriminator while spending more still wins. There is nowhere on this board for a
cost to refuse a result.

## Precedent

- The reporter that ranked by the blind composite and published a +0.44 win as a tie.
- A banked cell carrying a contract group at baseline 0.75, which is why gating on any group rather
  than the discriminator group would refuse a cell that was paid for and published.

## Artifact

`board.md`: the cell, the repository, the arms, the per-run figures, the spread, the routing state
of every arm, and the build the headline column was banked at.

## Done when

- Every figure on the board is quoted from the harvest record.
- The headline is objective recall.
- Every arm appears, including the ones that never routed.
- The build the win was banked at is recorded, so a later reader can tell whether a board is
  comparable to this one.

## Do not

- Do not re-round a figure, and do not write one that is not in the harvest record.
- Do not omit an arm because it did poorly. A never-routed arm is a finding.
- Do not spawn a subagent. Only the binary spawns.
