# PLAN 01-scout

## TASK

Pick this repo's contract symbol, the discriminator axis and the headline ask, and write the
shape file the adversary probe will be run against.

## SCOPE

You produce a shape and nothing else. **Out of scope:** gold rows, the scenario yaml, the
rubric, the adversary probe, any run, any edit to a script, any other repo. You do not grade
your own shape - a separate probe does that next, and it is spawned by the driver, not by you.
If a script here is broken, write one line naming it in `notes` and stop; do not fix it.

## RUN

`$CLONE`, `$REPO`, `$VERTICAL` and `$VDIR` are exported. Work from `improvement-loop/`.

0. If `$VDIR/results/dryrun/$REPO/adversary-probe.md` exists, read it FIRST. A previous
   shape was assembled without Sense, and every axis its Method section reached is dead.
   Do not propose one of them again.

1. Candidate anchors, as a LISTING - never a gate, never a ranking you defer to:

       python3 bench/lib/anchor_rank.py "$CLONE" --top 20

2. For each of two or three candidates, the scatter and grep-precision read:

       python3 bench/lib/seam_hunt.py "$CLONE" <Symbol> --propose [--file <path>]

   LOWER precision is better: it means grepping the token returns far more files than the
   real structural callers, so a baseline cannot cleanly enumerate the set by text search.

3. What the arm is actually SHOWN for the winning candidate, at MCP defaults:

       python3 bench/lib/mcp_probe.py "$CLONE" \
         '[{"name":"sense_blast","arguments":{"symbol":"<Symbol>"}}]'

   Pass no `min_confidence`. The shown set is cap-limited, so raising it does not add rows,
   it changes which rows win the same slots.

4. The import battery, against the candidate axis:

       grep -rln '<the contract token or module path>' "$CLONE" | wc -l

   If an importer dump covers the dependent set you are aiming at, the axis is dead: those
   files score for free. Pick another axis.

## DECIDE

The anchor, the axis and the ask. The recipe, in order:

1. **Pick the repo's central contract** - the model or type everything hangs off, not a clever
   corner. It must be the thing a maintainer would actually rework.
2. **Frame it as a teardown audit**: "what depends on this before I rework it". Real,
   recurring, and it forces enumeration rather than a single lookup.
3. **Take the dependents from the blast output and keep the BORING ones** - the backup
   exporter, the annual-report presenter, the permalink redirector, the CLI recount. One row
   per FILE. The centre is not the discriminator; the scattered periphery is.
4. **Split the pool**: the contract itself and its obvious write path are ANCHORS, which both
   arms reach and which do not score. The scattered residue is what decides the cell.
5. **Write the headline ask to demand `file:line`.** Without it the task grades at mention
   level and both arms tie.

Watch the context radius, not the hit count: a judgment repeated across many hits is batchable
by a baseline when each hit is decidable locally. A narrow per-hit context radius means an
unwinnable ask however many hits it has.

Do NOT screen an anchor by whether grep finds its files. That reads a discovery bar on a
citation metric and it kills repos that bank wins.

## ARTIFACT

Write `$VDIR/results/dryrun/$REPO/shape.md` with exactly these six headings:

    # Contract          symbol + file:line
    # Axis              one sentence: what the scattered residue is, and why it scatters
    # Headline ask      the paragraph the probe will attempt, verbatim, demanding file:line
    # Periphery pool    candidate dependent FILES with path:line, one per line, from blast
    # Anchors           the contract + write-path files that will NOT score
    # Import battery    the command you ran and its hit count

Then write `$VDIR/results/loop/$REPO/scout.verdict.json`:

    {
      "phase":    "scout",
      "repo":     "<repo>",
      "verdict":  "SHAPE" | "NO-AXIS",
      "artifact": "verticals/<vertical>/results/dryrun/<repo>/shape.md",
      "notes":    "one line"
    }

`NO-AXIS` only after the contract hunt was deepened and the pool widened on at least three
candidate anchors. It is not a tie and it is not a boundary; it is a routed lever.

## DONE WHEN

- `shape.md` exists and carries all six headings.
- The periphery pool holds at least twelve FILES, each with a `path:line`.
- No pool file appears in the anchors list.
- The import battery hit count is recorded and the pool is not covered by it.
- The verdict JSON exists and parses.

## DO NOT

- Do not write gold, a yaml, a rubric, or run the adversary probe - the next phases own those.
- Do not state a negative claim (dead, unreachable, nothing there) from one command; a second,
  different probe or it does not go in the file.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
