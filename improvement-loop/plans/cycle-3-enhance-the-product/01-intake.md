# PLAN 01-intake

## TASK

Measure what Sense actually cannot do for this stack, decide whether it is in scope, and
write the worklist plus the corpus the rest of the window is proven on.

## SCOPE

You measure and you scope. **Out of scope:** writing product code, writing tests, creating
a branch, stamping a vertical, hunting or pinning repos, touching `verticals/`, any other
stack. You do not fix what you find here - later phases own that.

**The exit code that opened this window is a claim about a directory, not a measurement of
support.** `bench/bootstrap/scaffold.sh` tests whether `internal/extract/$LANG/` exists
with production files. A language can already be served at `TierStandard` by
`internal/extract/langspec/$LANG.go` with no such directory, and a directory can exist
while none of the framework's dispatch idioms resolve. Establish which case this is before
you write a single line of the worklist.

## RUN

`$KEY`, `$LANG`, `$FRAMEWORK`, `$TITLE`, `$WDIR`, `$SENSE_ROOT`, `$CLONES` are exported.
Work from `improvement-loop/`.

1. The request that opened the window, verbatim:

       cat "$WDIR/request.json"

2. What exists today for this language, all four namespaces:

       ls "$SENSE_ROOT/internal/extract/$LANG" 2>&1
       ls "$SENSE_ROOT/internal/extract/langspec/$LANG.go" 2>&1
       ls "$SENSE_ROOT/internal/model/$FRAMEWORK.go" "$SENSE_ROOT/internal/resolve/$FRAMEWORK.go" 2>&1
       ls "$SENSE_ROOT/internal/conventions/detectors_$LANG.go" 2>&1

   A `langspec` entry is partial support and its contents bound it: read the file and say
   which node kinds it declares. Quote them.

3. What the stack profile says this vertical is made of - the framework-role repos are
   your corpus candidates:

       cat "stacks/$KEY.conf"

4. A reference lane, for what "ready" has meant before. Read one language that has a
   directory, and list what it is made of:

       ls "$SENSE_ROOT/internal/extract/ruby" "$SENSE_ROOT/internal/extract/php"

5. The measurement, and it is the only one that decides anything. Pick ONE real repository
   of this stack from the conf's framework-role list, clone it shallow under `$CLONES`,
   index it with the INSTALLED binary, and ask Sense three questions over MCP:

       git clone --depth 1 <url> "$CLONES/<name>"
       (cd "$CLONES/<name>" && sense scan -dir .)
       python3 bench/lib/mcp_probe.py "$CLONES/<name>" \
         '[{"name":"sense_status","arguments":{}}]'

   `sense_status` prints the per-language file, symbol and tier counts. Then pick two
   symbols you have READ in that clone - a framework-dispatched one (a controller action,
   an injected service, a handler registered rather than called) and a plainly-called one -
   and ask for each:

       python3 bench/lib/mcp_probe.py "$CLONES/<name>" \
         '[{"name":"sense_graph","arguments":{"symbol":"<Sym>"}},
           {"name":"sense_blast","arguments":{"symbol":"<Sym>"}}]'

   For every symbol, cite the source at `path:line` in the clone and quote what Sense
   returned. An empty return next to a relationship you read in the file IS the gap; a
   populated return means that idiom is already served and does not belong on the worklist.

## DECIDE

One verdict.

- `WORKLIST` - at least one relationship that exists in the clone at `path:line` does not
  come back over MCP, and closing it is inside the three lanes. Write the worklist: one
  row per idiom, each with the source cite that proves it exists and the quoted empty
  return that proves Sense misses it.
- `ALREADY-READY` - every probed idiom resolves. The window closes without code. The
  missing `internal/extract/$LANG/` directory is then a bootstrap-check gap, not a product
  gap: say so in one line, because that is a real finding and the next window should not
  re-derive it.
- `OUT-OF-SCOPE` - the only fix violates the identity: a new command, a new output format,
  a config knob, a fifth tool, a performance rewrite, or a heuristic that would have to
  live in a generic file. Name the rule it breaks. This is a legitimate ending, not a
  failure.

**Scope the worklist to what the corpus can prove.** An idiom you cannot point at in a
cloned repository cannot be proven in `04-prove`, so it does not go on the worklist however
plausible it is. Two to five rows is a lane; twelve rows is a rewrite and it will not land.

**The corpus is two repositories, not one.** One framework-role repo and one application
repo of this stack, both real, both cloned. One repo proves you fit one codebase's habits.

## ARTIFACT

Write `$WDIR/corpus.txt`, one `name|url` per line, two lines, the clone you already made
first.

Write `$WDIR/worklist.md`, five headings:

    # Verdict            WORKLIST, ALREADY-READY or OUT-OF-SCOPE, and the sentence that decides it
    # What exists today  the four namespaces, quoted, and what langspec covers if it is there
    # The measurement    per probed symbol: the source cite, and what MCP returned, quoted
    # The worklist       one row per idiom: the idiom, its source cite, the namespace that
                         will carry it, and what a passing test would assert
    # Not on the list    idioms that already resolve, and idioms ruled out of scope with the
                         rule each one breaks

Then `$WDIR/intake.verdict.json`:

    {
      "phase":    "intake",
      "repo":     "<key>",
      "verdict":  "WORKLIST" | "ALREADY-READY" | "OUT-OF-SCOPE",
      "artifact": "product-window/<key>/worklist.md",
      "notes":    "one line naming the idiom count and the corpus repos"
    }

## DONE WHEN

- The four namespaces were listed from disk, and `langspec/$LANG.go` was read if present.
- At least one repo of this stack is cloned and indexed under `$CLONES`.
- Every worklist row carries a `path:line` in a clone AND the quoted MCP return that misses it.
- `corpus.txt` has two lines and both URLs were cloned successfully.
- The verdict JSON exists and parses.

## DO NOT

- Do not write product code, tests or a branch. This phase measures and scopes.
- Do not put an idiom on the worklist that you cannot point at in a cloned repository.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
