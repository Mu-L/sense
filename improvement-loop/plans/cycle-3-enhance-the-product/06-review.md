# PLAN 06-review

## TASK

Convene the council on the finished diff and rule on one thing: did the change introduce
what it should, and nothing else.

## SCOPE

You review a diff that already exists and passed its gates. **Out of scope:** editing code,
editing tests, re-running the probes, re-scoping, re-litigating the approach - the proposal
pass settled that and reversing it here throws away the work rather than redirecting it.

Two passes, two questions. The proposal asked "is this the right change to make?"; this one
asks "did the change introduce what it should and nothing else?". Neither substitutes for
the other and neither ever demotes.

## RUN

`$KEY`, `$LANG`, `$WDIR`, `$SENSE_ROOT`, `$BRANCH` are exported. Work from `$SENSE_ROOT`.

1. What was approved, what was measured, and what the machine gates said:

       cat "$WDIR/proposal.md" "$WDIR/build.md" "$WDIR/prove.md"
       tail -8 "$WDIR/build.ci.log"
       cat "$WDIR/gates.txt"

   `gates.txt` carries the driver's own runs: `make ci`, the touched-set coverage gate and
   qlty. They are facts, already established. The council does not re-run them and does not
   review what a machine already checked.

2. The diff itself:

       git diff main.."$BRANCH"
       git log --oneline main.."$BRANCH"

3. Convene the council on it:

       /council The diff on <branch> adds <lang> support to Sense. Review the FINISHED CODE
       for new-bug introduction and side effects: correctness of the walker, what happens on
       shapes the tests do not cover, whether anything outside this language can be reached
       by it, and whether the tests would catch a regression.

   If the command is not available in this session, read `.claude/commands/council.md` and
   follow it inline. The council runs inline either way and never spawns seats.

## DECIDE

One verdict, and it is the council's synthesis.

- `PASS` - no seat found a defect in the diff that would ship a bug or reach outside the
  language. Concerns that are boundaries rather than defects are recorded and carried onto
  the handoff page, where the reader needs them.
- `REWORK` - a seat found a defect, an unhandled shape that a real repository will hit, or
  a reach outside this language. Name it at `path:line` in the diff. The window stops for a
  human.

**A boundary named by a test is not a defect.** The build phase pinned several deliberately.
Re-reading one as a bug is how a clean lane gets sent back for work that changes nothing.

**Name roles, never people**, in the artifact. A linter enforces it on the committed
surfaces of this repository.

## ARTIFACT

Write `$WDIR/review.md`, four headings:

    # Verdict        PASS or REWORK, and the sentence that decides it
    # The round      per seat, by role: the assessment and any concern
    # Defects        anything that would ship a bug, at path:line, or "none"
    # For the page   boundaries the handoff must state, in the reader's words

Then `$WDIR/review.verdict.json`:

    {
      "phase":    "review",
      "repo":     "<key>",
      "verdict":  "PASS" | "REWORK",
      "artifact": "product-window/<key>/review.md",
      "notes":    "one line: defects found, and the boundaries carried to the page"
    }

## DONE WHEN

- The whole diff was read, not only the files the artifacts describe.
- The machine gates were read from `gates.txt` and not re-run.
- Every defect is cited at `path:line` in the diff.
- No person's name appears in the artifact.
- The verdict JSON exists and parses.

## DO NOT

- Do not edit code or tests. Name the defect and route it.
- Do not re-open the approach the proposal pass settled.
- Do not spawn a subagent. The council runs inline, in this session.
