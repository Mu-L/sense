# PLAN 04-build

## TASK

Write the product code that turns every red test from `03-truth` green, and leave the
repository's own gates green with it.

## SCOPE

You write Sense's code. **Out of scope:** editing the tests to fit the implementation,
touching another language's files, adding a command, an output format, a config knob or a
tool, refactoring code your rows do not require, `verticals/`, and the real-code proof -
`05-prove` owns that and it is not yours to pre-empt.

You are on `$BRANCH` in `$SENSE_ROOT`. Stay on it.

## RUN

`$KEY`, `$LANG`, `$FRAMEWORK`, `$WDIR`, `$SENSE_ROOT`, `$BRANCH` are exported. Work from
`improvement-loop/` and run the Go commands from `$SENSE_ROOT`.

1. What must go green, and what it must not cost:

       cat "$WDIR/truth.md"
       sed -n 1,120p "$SENSE_ROOT/CLAUDE.md"

2. The lane you are copying the shape of. Read it properly before writing - the walker, the
   harvest, the composition, the registration:

       ls "$SENSE_ROOT/internal/extract/php" "$SENSE_ROOT/internal/extract/rust"
       sed -n 1,60p "$SENSE_ROOT/internal/extract/langspec/langspec.go"

3. If the language already has a `langspec` entry, decide in writing whether this lane
   EXTENDS it or replaces it with a dedicated package, and say why. A dedicated package
   that re-implements what langspec already does correctly is the expensive wrong answer;
   a langspec entry stretched to carry framework dispatch is the other one.

4. Write the code, then the loop:

       cd "$SENSE_ROOT" && go test ./internal/... 2>&1 | tail -40

   until every test from `03-truth` passes and nothing else broke.

5. The repository's gates, both, quoted in your artifact:

       cd "$SENSE_ROOT" && make ci
       cd "$SENSE_ROOT" && make smoke

   `make ci` is build, coverage, lint and the complexity ledger. Over 15 cyclomatic or 30
   cognitive reds it, and `make ledger` enforces zero suppressions: decompose the function,
   you cannot annotate your way past it.

6. Coverage on what you touched, which is stricter than the repo floor: every file AND
   function you created or modified holds **above 94%**. Read it off the gate's own output,
   never estimate it.

## DECIDE

One verdict.

- `BUILD` - every test from `03-truth` is green, `make ci` is green, `make smoke` is green,
  and every touched file and function is above 94% line and function coverage.
- `CANNOT-BUILD` - the lane cannot be written inside the identity. Name what stopped it:
  the idiom needs a knob, or a generic detector, or a fifth tool, or the grammar does not
  expose the node. Say what a future window would need. This is a real ending; a lane
  forced through against the identity is worse than a parked one.

**Do not weaken a test to reach green.** If a test from `03-truth` is wrong, say so in the
artifact, name the row, and leave it red - `05-prove` reads this file and needs to know.
Editing the assertion to match the output is how a lane ships broken and looks proven.

**Do not lower a floor or grow an exception list.** Cover the gap. The coverage gate is
deny-by-default and it is not negotiable from inside this window.

## ARTIFACT

Write `$WDIR/build.md`, five headings:

    # Verdict          BUILD or CANNOT-BUILD, and the sentence that decides it
    # What was written one row per file created or modified, and which worklist row it serves
    # Green, quoted    the go test tail showing the new tests passing
    # The gates        make ci and make smoke output, quoted, plus the touched-set coverage
                       numbers read off the gate
    # Left undone      any test still red, any row not implemented, and why

Then `$WDIR/build.verdict.json`:

    {
      "phase":    "build",
      "repo":     "<key>",
      "verdict":  "BUILD" | "CANNOT-BUILD",
      "artifact": "product-window/<key>/build.md",
      "notes":    "one line: tests green, ci state, lowest touched-file coverage"
    }

Commit the implementation on `$BRANCH` with a conventional subject - a new language lane is
`feat`, and the scope is the language: `feat(csharp): ...`. Then stop.

## DONE WHEN

- Every test written by `03-truth` passes, or is named in `# Left undone` with its reason.
- `make ci` and `make smoke` output is quoted in `build.md`.
- The touched-set coverage figures are quoted, and every one is above 94%.
- Per-language namespacing holds: no generic file carries this language's heuristic.
- The implementation is committed on `$BRANCH`.
- The verdict JSON exists and parses.

## DO NOT

- Do not edit a test to make it pass, and do not lower a coverage floor or add a
  complexity suppression.
- Do not add a command, an output format, a config knob or a tool. The three lanes bind.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
