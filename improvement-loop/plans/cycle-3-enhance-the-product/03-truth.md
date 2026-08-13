# PLAN 03-truth

## TASK

Turn the worklist into ground truth: a fixture test per idiom that is RED on this branch
today, and the real-code probes that will judge the finished lane.

## SCOPE

You write tests and probe definitions. **Out of scope:** writing the extractor, the model,
the resolver rules or the detectors; making any test pass; editing another language's
files; touching `verticals/`. A test you can make pass while you are here is a test that
was never red.

The driver has already created and checked out `$BRANCH` in `$SENSE_ROOT`. Commit your test
files on it; do not create a second branch and do not switch away.

## RUN

`$KEY`, `$LANG`, `$FRAMEWORK`, `$WDIR`, `$SENSE_ROOT`, `$CLONES`, `$BRANCH` are exported.
Work from `improvement-loop/`.

1. The worklist you are turning into tests:

       cat "$WDIR/worklist.md"

2. How this repository writes an extractor test, before you write one. Read a lane that
   exists, both halves - the table-driven unit tests and the fixture corpus:

       ls "$SENSE_ROOT/internal/extract/php"
       sed -n 1,80p "$SENSE_ROOT/internal/extract/php/php_test.go"
       ls "$SENSE_ROOT/internal/extract/testdata" | head -30

3. The conventions and framework-model side, if the worklist has rows there:

       sed -n 1,60p "$SENSE_ROOT/internal/conventions/detectors_php.go"
       sed -n 1,60p "$SENSE_ROOT/internal/model/laravel.go"

4. Write the tests. One test per worklist row, named for the behaviour it asserts, in the
   namespaced file the row declared. Fixture source goes in the language's testdata
   directory, minimal and hand-written, reproducing the idiom you cited from the clone -
   never a paste of a real file.

5. Prove they are RED, and quote it:

       cd "$SENSE_ROOT" && go test ./internal/... 2>&1 | tail -40

   A test that errors because the package does not exist yet is red for the wrong reason:
   create the package with the minimum declaration it needs to COMPILE, and let the test
   fail on the assertion. Red means "the assertion failed", never "the build broke".

6. Write the real-code probes. For each worklist row, one row in `probes.json`: the corpus
   repo, the MCP call an agent would make, the substring that must appear in the result,
   and the `path:line` in the clone that proves the relationship is really there. Verify
   each cite by opening the file at that line - a probe whose expectation you have not
   read in the source is a guess, and it will pass or fail for reasons nobody can audit.

## DECIDE

One verdict.

- `TRUTH` - every worklist row has a test, `go test ./internal/...` fails on assertions and
  not on the build, and every probe row carries a verified `path:line`.
- `NO-REPRO` - a worklist row cannot be made to fail. Either the behaviour is already
  correct and `01-intake` mis-measured it, or the idiom cannot be expressed in a fixture.
  Say which, name the row, and drop it: the window continues on the rows that ARE red. Only
  when EVERY row is unreproducible does this verdict close the window, and then it closes
  it honestly.

**A fixture asserts one idiom.** A fixture carrying four idioms fails as one line and tells
the build phase nothing about which three still work.

**Assert the relationship, never the count.** `len(edges) == 7` breaks on every unrelated
improvement and says nothing about what was found; assert that the edge from A to B exists
with the kind it should have.

## ARTIFACT

Write `$WDIR/probes.json`:

    [
      {
        "row":    "<worklist row name>",
        "repo":   "<name from corpus.txt>",
        "call":   {"name": "sense_graph", "arguments": {"symbol": "<Sym>"}},
        "expect": "<substring that must appear in the returned JSON>",
        "source": "<path:line in the clone that proves the relationship exists>"
      }
    ]

Write `$WDIR/truth.md`, four headings:

    # Verdict          TRUTH or NO-REPRO, and the sentence that decides it
    # The tests        one row per worklist row: the test name, its file, what it asserts
    # Red, quoted      the go test output showing every new test failing on its assertion
    # Dropped rows     any NO-REPRO row, why, and whether it is a real behaviour or a
                       fixture limit

Then `$WDIR/truth.verdict.json`:

    {
      "phase":    "truth",
      "repo":     "<key>",
      "verdict":  "TRUTH" | "NO-REPRO",
      "artifact": "product-window/<key>/truth.md",
      "notes":    "one line: how many tests, how many red, how many rows dropped"
    }

Commit the tests and fixtures on `$BRANCH` with a conventional subject, then stop.

## DONE WHEN

- Every worklist row has either a test or a recorded reason it was dropped.
- `go test ./internal/...` output is quoted in `truth.md` and every new test fails on an
  assertion, not on a build error.
- `probes.json` parses, and every `source` cite was opened and read in the clone.
- The tests and fixtures are committed on `$BRANCH`.
- The verdict JSON exists and parses.

## DO NOT

- Do not make anything pass. Red is this phase's deliverable.
- Do not paste real repository files into testdata. Write the minimum fixture that carries
  the idiom.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
