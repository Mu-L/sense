# PLAN 02-proposal

## TASK

Convene the council on the worklist, before any code exists, and rule on whether this is
the right change to make.

## SCOPE

You review an approach. **Out of scope:** writing tests, writing product code, creating a
branch, re-measuring what `01-intake` measured, re-scoping the worklist yourself, any
other stack. You are not the author and you are not the implementer.

This is the cheap point. The merge-time pass cannot redirect a wrong approach without
throwing away the work; this one costs a session and no code. That is the whole reason it
runs first.

## RUN

`$KEY`, `$LANG`, `$FRAMEWORK`, `$WDIR`, `$SENSE_ROOT` are exported. Work from
`$SENSE_ROOT`, which is where the project's own commands live.

1. What is being proposed, and what it rests on:

       cat "$WDIR/worklist.md"

2. The identity the change has to survive:

       sed -n 1,60p "$SENSE_ROOT/NON-GOALS.md"
       sed -n 1,40p "$SENSE_ROOT/CONTRIBUTING.md"

3. Convene the council on it:

       /council The worklist at improvement-loop/product-window/<key>/worklist.md proposes
       <n> changes to Sense so it can read <title>. Review the APPROACH before any code:
       the layer each row touches, the namespacing, the blast radius across other
       languages, and whether any row denatures the product.

   If the command is not available in this session, read `.claude/commands/council.md` and
   follow it inline. The council runs inline either way and never spawns seats.

## DECIDE

One verdict, and it is the council's synthesis, not your own opinion added on top.

- `PROCEED` - every row is inside the three lanes, lands in a namespaced file, and no seat
  raised an objection that survives to the end of the round. Carry the concerns that were
  raised and answered into the artifact anyway; the build phase reads them.
- `REWORK` - a row would need a config knob, a fifth tool, a new output format, a generic
  detector, or an abstraction the codebase does not have; or the approach trades one gap
  for two side effects. Name the row and the rule it breaks. The window stops here for a
  human, which is correct: a wrong approach caught before the branch exists is the
  cheapest outcome this cycle can produce.

**Name roles, never people.** The artifact writes "the Go seat", "the scope seat", "the
testing seat". A name in a bench artifact is a standing rule of this repository and a
linter enforces it on the committed surfaces.

**A concern is not a veto.** The synthesis distinguishes what blocks the approach from what
the implementer should carry. Recording every concern as blocking makes the pass useless
and it will be routed around.

## ARTIFACT

Write `$WDIR/proposal.md`, four headings:

    # Verdict        PROCEED or REWORK, and the sentence that decides it
    # The approach   what is being proposed, per row, in one line each
    # The round      per seat, by role: the assessment and any concern
    # Carried        concerns the build phase must hold, and what would answer each

Then `$WDIR/proposal.verdict.json`:

    {
      "phase":    "proposal",
      "repo":     "<key>",
      "verdict":  "PROCEED" | "REWORK",
      "artifact": "product-window/<key>/proposal.md",
      "notes":    "one line: how many rows cleared, and the blocking concern if any"
    }

## DONE WHEN

- The worklist was read in full before any seat spoke.
- Every worklist row is addressed by the round, by role.
- No person's name appears in the artifact.
- On `REWORK`, the row and the rule it breaks are both named.
- The verdict JSON exists and parses.

## DO NOT

- Do not write code, tests or a branch. Nothing is implemented in this phase.
- Do not re-scope the worklist. Route it back; the intake phase owns the rewrite.
- Do not spawn a subagent. The council runs inline, in this session.
