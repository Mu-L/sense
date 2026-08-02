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

0. If archived probes exist (`$VDIR/results/dryrun/$REPO/adversary-probe*.md`), read them
   FIRST - as evidence of what assembly COSTS in this repo, never as a ban list. An axis is
   closed only when `probe_score.py` measured the probe covering the pool; probe prose about
   what it "reached" is a claim, and on the record here it has been wrong by six rows in both
   directions. Re-run the score yourself if you want the number:

       python3 bench/lib/probe_score.py <archived probe> <the shape it ran against>

1. Candidate anchors, as a LISTING - never a gate, never a ranking you defer to:

       python3 bench/lib/anchor_rank.py "$CLONE" --top 20

2. For each of two or three candidates, the scatter and grep-precision read:

       python3 bench/lib/seam_hunt.py "$CLONE" <Symbol> --propose [--file <path>]

   LOWER precision is better: it means grepping the token returns far more files than the
   real structural callers, so a baseline cannot cleanly enumerate the set by text search.
   Read SCATTER as the signal and `VERDICT: grep-clean` as a note, never a kill - the banked
   +0.72 cell in this vertical is grep-clean at the file level. A precision ABOVE 1.00 means
   false caller edges, not darkness; check a sample by hand before believing it.

3. What the arm is actually SHOWN for the winning candidate, at MCP defaults:

       python3 bench/lib/mcp_probe.py "$CLONE" \
         '[{"name":"sense_blast","arguments":{"symbol":"<Symbol>"}}]'

   Pass no `min_confidence`. The shown set is cap-limited, so raising it does not add rows,
   it changes which rows win the same slots.

4. The import battery, against the candidate axis - RECORDED, never a gate here:

       grep -rln '<the module path a dependent must IMPORT to use the contract>' "$CLONE" | wc -l

   IMPORT-LAW kills an axis whose dependents must each write an import of the contract, because
   that import line hands the baseline a `path:line` for free. It is an IMPORT dump, not a token
   grep. In a stack with no import statement - Ruby, and anything autoloaded - the battery
   degenerates into "does the token appear in the file", which is a DISCOVERY bar on a CITATION
   metric and is exactly what the DECIDE section below forbids. Record the number in the shape
   and let the probe and the validation run decide.

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

Do NOT screen an anchor by whether grep finds its files, and do NOT screen a dependent by
whether its line writes the anchor token. That reads a discovery bar on a citation metric and
it kills repos that bank wins. Measured, in this vertical: a banked cell scored `dependents`
+0.72 (baseline 2.5 of 16, sense 14 of 16, n=2 per arm) on a gold set where **16 of 16 files
carry the anchor token** and one `grep -rl` returns all of them. Token-brightness predicted
nothing. The rows the baseline missed were bright, scattered, and too expensive to open.

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
candidate anchors. It is not a tie and it is not a boundary; it is a routed lever. Every
anchor rejected on the way there carries the measurement that rejected it, and **"its
dependents write the anchor token" is not a measurement** - a scored probe or a real import
cover is. `NO-AXIS` reached by a battery of token greps is a report about grep, not about
the repo.

## DONE WHEN

- `shape.md` exists and carries all six headings.
- The periphery pool holds at least twelve FILES, each with a `path:line`.
- No pool file appears in the anchors list.
- The import battery hit count is recorded.
- The verdict JSON exists and parses.

## DO NOT

- Do not write gold, a yaml, a rubric, or run the adversary probe - the next phases own those.
- Do not state a negative claim (dead, unreachable, nothing there) from one command; a second,
  different probe or it does not go in the file.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
