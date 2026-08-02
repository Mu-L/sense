# PLAN 01-curate

## TASK

Turn the shape and the adversary probe's disclaimer into one stamped scenario, its rubric and
its hand-audited gold.

## SCOPE

You curate gold for the shape you were handed, and nothing else. **Out of scope:** picking a
new contract, re-running the probe, running the bench, editing a script, any other repo.

An empty disclaimer is NOT a reason to return `NO-AXIS`. The probe is a lead, not a gate, and
a probe that pinned the whole pool has been measured sitting over an arm that pinned a sixth
of it. `NO-AXIS` here means the POOL cannot yield twelve heterogeneous rows one file each -
a property of the shape, not of what the probe managed to find.

## RUN

`$CLONE`, `$REPO`, `$VERTICAL`, `$VDIR`, `$YAML`, `$RUBRIC` are exported. Work from
`improvement-loop/`.

1. Read both inputs before writing anything:

       $VDIR/results/dryrun/$REPO/shape.md
       $VDIR/results/dryrun/$REPO/adversary-probe.md

   The probe's **honesty disclaimer is the strongest lead on the discriminator axis**. What
   it says it could not establish is the first place to look for gold. What it reached is a
   warning, not a verdict - read `$VDIR/results/loop/$REPO/probe.score.txt` for how much of
   the pool it actually pinned, then curate from the rest FIRST. A row the probe pinned is
   still gold if the axis holds: on the banked cell the probe pinned 15 of 16 gold rows and
   the benched arm pinned 2.5, so "the probe found it" has never once meant "the arm will".

2. Pull the dependent set the arm is SHOWN, and read every candidate row's real line:

       python3 bench/lib/mcp_probe.py "$CLONE" \
         '[{"name":"sense_blast","arguments":{"symbol":"<Symbol>"}}]'

   The blast payload gives the enclosing `def`, NOT the line that touches the contract. Open
   the file and pin the line that actually uses it.

3. Memorization, per candidate row, on a famous repo:

       python3 bench/lib/memorization_probe.py "$CLONE" <Symbol> --json <out>.json

   Cut what comes back recited. A famous repo is not disqualified; a memorized row is.

4. After writing the yaml and rubric, in this order:

       python3 bench/lib/scenario.py "$YAML" --prompt
       python3 bench/lib/rubric_check.py "$YAML"
       python3 bench/lib/gold_confidence_check.py "$YAML" <Symbol> --repo "$CLONE" --group dependents
       python3 bench/lib/gold_audit.py stamp "$YAML"
       python3 bench/lib/gold_audit.py verify "$YAML"

   `--group dependents` scopes the shown-over-MCP check to the blast-sourced group. Handed the
   whole gold it fails every hand-sourced row and narrows the bench to one tool.
   `stamp` writes one TODO row per gold item; you replace each TODO by opening the file and
   reading the credit. `verify` fails while any TODO remains.

## DECIDE

Every gold row. The rails, all of them:

- **One item per FILE.** A group listing several symbols from one file rewards a single read.
- **Three groups.** `contract` and `write-path` are the anchors both arms reach and they do
  NOT score. `dependents` is the scattered residue that decides the cell.
- **The probe disclaimer is a LEAD, not a filter.** Prefer the territory it disclaimed. Do
  not exclude a row because the probe pinned it. The probe is one unscored simulation of the
  baseline and this vertical has it wrong by six rows in BOTH directions on the record: it
  disclaimed rows the benched baseline then found, and pinned rows the benched baseline then
  missed. A `dependents` group built as the complement of one probe run is a group selected
  for what that run happened to miss - which is how a cell reached validation with the
  baseline holding 10 of 16 and a dead ceiling. Select for citation cost: scatter across
  areas, one file each, a line that must be opened to be pinned. `pay_ceiling.py` at
  validation is the only measurement of what the baseline actually holds.
- **Hand-audit every credit.** Basename matching has awarded credit for the wrong file. Run
  the tally, then read the credits - the tally alone has passed wrong gold.
- **The prompt is neutral.** No paths, no class names, no counts, no tool names, no answer
  shape. Both arms get the identical prompt and gold is never rendered into it.
- **Every step prompt demands `file:line`**, and the audit step says a filename alone does not
  count.

## ARTIFACT

Write `$YAML` and `$RUBRIC`. The yaml carries `name`, `repo`, `contract_symbol`,
`contract_file`, `description`, `steps` (each with `name`, `prompt`, `checks`), `scoring` and
`gold`. The rubric carries exactly two top-level keys, `audience` and `steps`, with one rubric
step per scenario step, **names matching verbatim and in order**, and every step declaring
`map_quality`, `specificity`, `justification` and `uncertainty`. It carries no `repo:` key.
A gold row is:

    - {id: d:<slug>, group: dependents, match: [<path fragment>], relation: "<path>:<line> <the exact expression> - one phrase on HOW it uses the contract"}

`gold_audit.py stamp` writes `$VDIR/scenarios/$REPO.gold-audit.json` alongside them.

Then write `$VDIR/results/loop/$REPO/curate.verdict.json`:

    {
      "phase":    "curate",
      "repo":     "<repo>",
      "verdict":  "GOLD" | "NO-AXIS",
      "artifact": "verticals/<vertical>/scenarios/<repo>.yaml",
      "notes":    "one line"
    }

## DONE WHEN

- `scenario.py --prompt` shows no path, symbol, count or tool name in the rendered prompt.
- `rubric_check.py` exits 0: the rubric satisfies the judge's contract. An unjudgeable
  rubric is not caught until after both arms are spent, so it is caught here.
- `gold_confidence_check.py --group dependents` exits 0: every blast-sourced row appears in
  the output the agent is SHOWN over MCP, at the arm's defaults, truncation included.
- `gold_audit.py verify` exits 0: zero TODO rows remain and the gold has not changed under a
  finished sheet.
- `dependents` holds at least twelve rows across at least six areas, one file each.
- The verdict JSON exists and parses.

## DO NOT

- Do not build `dependents` as the complement of the probe run. The probe is not the baseline.
- Do not take a credit from a script tally; open the file and read the line, every row.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
