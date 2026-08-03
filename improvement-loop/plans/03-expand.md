# PLAN 03-expand

## TASK

Expand the measured two-step question into the full seven-step work session, without moving the
discriminator that the mini-bench just proved.

## SCOPE

You rewrite one scenario in place and extend its gold and rubric. **Out of scope:** changing
the anchor, changing the discriminator step's question, re-running either arm, the paid bench,
editing a script, any other repo. If the mini-bench verdict on disk is not `PROCEED`, write one
line in `notes` and stop.

## RUN

`$CLONE`, `$REPO`, `$VERTICAL`, `$VDIR`, `$YAML`, `$RUBRIC` are exported. Work from
`improvement-loop/`.

1. Read the measured inputs before writing anything:

       $VDIR/results/loop/$REPO/minibench.md
       $YAML

2. Rewrite `$YAML` to the seven steps below, then extend `$RUBRIC` to match.

3. Pull the shown set again for the anchors and specs you are adding:

       python3 bench/lib/mcp_probe.py "$CLONE" \
         '[{"name":"sense_blast","arguments":{"symbol":"<Symbol>"}}]'

4. Open every added row and pin the line that actually touches the contract.

5. In this order:

       python3 bench/lib/scenario.py "$YAML" --prompt
       python3 bench/lib/rubric_check.py "$YAML"
       python3 bench/lib/gold_confidence_check.py "$YAML" <Symbol> --repo "$CLONE" --group dependents
       python3 bench/lib/gold_audit.py stamp "$YAML"
       python3 bench/lib/gold_audit.py verify "$YAML"

## DECIDE

### The seven steps

Every banked win in this bench is this session. Only the contract noun and the axis sentence
change between repos; the skeleton does not.

    1. Orient in the codebase          where the contract sits, how the code is organized.
                                       Ends with "No Explore agents."
    2. Map the <contract> contract     what it exposes and what it pulls in - concerns,
                                       embeds, associations. file:line for each piece.
    3. Trace the write path            how the thing is created, or torn down, and what runs
                                       around it. Which parts run inline vs asynchronously.
    4. Audit every dependent           THE DISCRIMINATOR. Carried over VERBATIM.
    5. Trace the guards                how code confirms it is acting on a real, current
                                       instance before acting. file:line throughout.
    6. Assess the blast radius         which dependents are at risk, ranked, and what must be
                                       re-verified. "A missed high-risk dependent is the one
                                       that pages someone."
    7. The change + verification map   what you will edit, every dependent grouped by area,
                                       and the specs that must be updated.

The steps accumulate: each is worked in order in one session, so by step 4 both arms have
already spent context and wall on steps 1 to 3. That is part of the measurement, not decoration.

### The rules that do not bend

- **Step 4 is byte-identical to the probe scenario's step 2**, and its `dependents` gold is
  byte-identical too. Move either and the mini-bench number stops describing this scenario.
  If you believe step 4 needs a rewrite, that is a `REQUESTION`, not an expansion.
- **New gold goes in NON-SCORING groups.** `contract`, `write-path` (or `teardown`) and
  `specs` are what both arms reach. Adding to `dependents` here is untested gold: it never
  faced a baseline.
- **The prompt stays neutral** - no paths, class names, counts, tool names, or answer shape -
  and **every step demands `file:line`.**
- **The ask names the MECHANISM, never the INVENTORY.** The neutrality gate reads tokens, so a
  list of functional categories passes it clean and still hands over the answer. Read the
  finished prompt back and quote any phrase that identifies a single gold row's file.
- **Every step carries its checks**: a `response_richness` floor, and `mcp_tool_used` /
  `no_grep` at `layer: adoption` so tool reach is recorded without scoring the answer.
- **One item per FILE**, hand-audited at its real line, in every group.

## ARTIFACT

`$YAML` rewritten to seven steps with `name`, `repo`, `contract_symbol`, `contract_file`,
`description`, `steps`, `scoring`, `gold`. The header comment carries the seam profile, the
axis sentence, the mini-bench figures, and any superseded question with the number that killed
it. `$RUBRIC` extended to seven matching steps, names verbatim and in order, each declaring
`map_quality`, `specificity`, `justification` and `uncertainty`.

`gold_audit.py stamp` writes `$VDIR/scenarios/$REPO.gold-audit.json` alongside them.

Then write `$VDIR/results/loop/$REPO/expand.verdict.json`:

    {
      "phase":    "expand",
      "repo":     "<repo>",
      "verdict":  "SCENARIO" | "REQUESTION",
      "artifact": "verticals/<vertical>/scenarios/<repo>.yaml",
      "notes":    "one line"
    }

`REQUESTION` only if expanding revealed that step 4 cannot survive verbatim - a gold row that
does not hold at its line, or a step that cannot be written without naming a gold file.

## DONE WHEN

- `$YAML` has seven steps in the order above; `$RUBRIC` has seven matching names in order.
- Step 4's prompt and the `dependents` gold are unchanged from the probe scenario, verified by
  diffing against the `.bak` or the previous commit.
- Every step's prompt contains `file:line`.
- `scenario.py --prompt` shows no path, symbol, count or tool name.
- `rubric_check.py` exits 0.
- `gold_confidence_check.py --group dependents` exits 0.
- `gold_audit.py verify` exits 0.
- The finished ask has been read back as an inventory check, with any offending phrase quoted
  and rewritten as a mechanism.
- The verdict JSON exists and parses.

## DO NOT

- Do not add rows to `dependents`, and do not reword step 4. Both invalidate the measurement.
- Do not take a credit from a script tally; open the file and read the line, every added row.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
