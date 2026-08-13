# PLAN 04-prove

## TASK

Read the real-code probe results the driver just produced and rule on one thing: does this
lane resolve this stack as it is actually written, without moving any other language.

## SCOPE

You read output that already exists and issue one verdict. **Out of scope:** editing
product code, editing tests, re-running the probes, re-indexing, changing `probes.json`,
opening a pull request, deleting a branch, `verticals/`. You do not fix what you find - a
`REVERT` sends the whole lane back, and that is the design.

**The probes are guaranteed to have RUN.** The driver built the branch binary, re-indexed
every corpus clone with it, executed every row of `probes.json` over MCP and wrote the raw
returns to `$WDIR/probes/`. If a row's output file is missing or empty, that is a defect in
the driver: **write it down and STOP - do not re-run it by hand and do not rule on the rows
that happen to be there.**

## RUN

`$KEY`, `$LANG`, `$WDIR`, `$SENSE_ROOT`, `$CLONES`, `$BRANCH` are exported. Work from
`improvement-loop/`.

1. What was supposed to happen, and what was built:

       cat "$WDIR/probes.json"
       cat "$WDIR/build.md"

2. The probe results, per row - this is your one mechanical input:

       cat "$WDIR/probes/summary.txt"

   Each row prints `PASS` or `MISS`, its expected substring and its `source` cite. For every
   `MISS`, open the raw return AND the source line before you say anything about it:

       cat "$WDIR/probes/<row>.json"
       sed -n '<line>p' "$CLONES/<repo>/<path>"

   A `MISS` on a row whose source cite turns out to be wrong is a bad probe, not a bad
   lane. Say which it is; you are the only reader who can tell them apart.

3. What Sense now sees in each corpus clone, over MCP:

       python3 bench/lib/mcp_probe.py "$CLONES/<repo>" '[{"name":"sense_status","arguments":{}}]'

   Quote the language row: files, symbols, tier. A lane that resolves its probes while the
   language sits at a handful of symbols is reading a fraction of the corpus, and that
   belongs in the artifact.

4. The control, which is the no-regress half:

       cat "$WDIR/probes/control.txt"

   The driver scanned three repositories in other languages with the INSTALLED binary
   before the build and with the BRANCH binary after it, and recorded each one's symbol
   and edge counts. Both sides were really re-scanned; reading an index the binary never
   rebuilt would compare a number with itself. **Identical counts is the passing state.**
   Any movement means the change is not per-language, whatever the diff looks like. An
   `UNRUN` row is not a pass: say so and STOP.

## DECIDE

One verdict.

- `PROVEN` - every probe row PASSes, or the only MISSes are rows whose `source` cite you
  have shown to be wrong; and every control repo's counts are identical before and after.
- `REVERT` - a probe MISSes against a source cite that holds, or a control repo moved. The
  lane goes back. Name, per row, what was expected and what came back, and what the source
  says at that line: the next window rewrites against this file and it is the only thing it
  will have.

**A control repo that moved is a REVERT on its own**, even with every probe green. A
per-language change that alters another language's index is a different change from the one
that was reviewed, and the identity of this cycle rests on that boundary.

**Fixture-green is not proof and may not be cited as any part of this verdict.** `03-build`
already established the tests pass; this phase exists because that is a statement about
fixtures. Rule on `$WDIR/probes/` and on nothing else.

Every claim in `notes` is quoted output. A number or a substring you did not read out of
`$WDIR/probes/` does not go in the file.

## ARTIFACT

Write `$WDIR/prove.md`, five headings:

    # Verdict          PROVEN or REVERT, and the sentence that decides it
    # Probe table      one row per probe: PASS or MISS, expected, returned, source cite
    # Corpus reach     the sense_status language row per corpus clone, quoted
    # Control          per control repo, symbols and edges before and after, and whether
                       they are identical
    # If REVERT        what a corrected lane must resolve, per row, in the form the probe
                       asks for it

Then `$WDIR/prove.verdict.json`:

    {
      "phase":    "prove",
      "repo":     "<key>",
      "verdict":  "PROVEN" | "REVERT",
      "artifact": "product-window/<key>/prove.md",
      "notes":    "one line: probes passed of total, and whether controls held"
    }

## DONE WHEN

- Every probe row is accounted for in the probe table, quoted from `$WDIR/probes/`.
- Every MISS has had its source line opened and read, and is classified as a lane miss or
  a bad probe.
- The control counts are quoted per repo, before and after, and compared in writing.
- The `sense_status` language row is quoted per corpus clone.
- The verdict JSON exists and parses.

## DO NOT

- Do not edit code, tests or probes to turn a MISS into a PASS. Name it and route it.
- Do not rule on fixture tests, and do not re-run the probes by hand.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
