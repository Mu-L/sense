# PLAN 01-bench

## TASK

Read the confirmation arms the driver just ran against a question that already won, and
rule on one thing: is this run set sound enough to build a public page from.

## SCOPE

You read runs that already exist and issue one verdict. **Out of scope:** re-authoring the
question, editing the scenario or its gold, re-scoring, running the headline arm, raising a
watchdog, writing the page, any other repo. You do not judge whether Sense did well - that
is what the page will say, from the numbers, without you.

## RUN

`$VERTICAL`, `$VDIR`, `$REPO`, `$HEADLINE`, `$ARMS`, `$YAML`, `$LOOPDIR` are exported.
Work from `improvement-loop/`.

1. The build gate, before believing any column:

       python3 bench/lib/board.py gate "$VDIR" --repo "$REPO" --headline "$HEADLINE"

   A non-zero exit means the installed Sense is not the build the headline column was
   banked at. Stop and say so; there is no board to build today.

2. The numbers, which are your whole mechanical input:

       python3 bench/lib/board.py assemble "$VDIR" --repo "$REPO" \
         --headline "$HEADLINE" --arms "$ARMS" --scenario "$YAML" > "$LOOPDIR/numbers.json"

       python3 -c "import json,sys
d = json.load(sys.stdin)
print('version', d['scenario_version'])
for c in d['columns']:
    print(c['model'], 'measured=', c.get('measured'), 'routing=', c.get('routing'),
          'runs=', c.get('runs'), 'split=', (c.get('mechanism') or {}).get('verdict_split'))
" < "$LOOPDIR/numbers.json"

3. Per arm, the mechanism table and its routing states, run by run:

       VERSION="$(python3 -c "import json,sys; \
print(json.load(sys.stdin)['scenario_version'].split(':')[-1])" < "$LOOPDIR/numbers.json")"

       for m in $ARMS; do
         root="$VDIR/results/$(printf '%s' "$m" | tr '/:' '__')/$VERSION"
         echo "== $m"
         python3 bench/lib/mechanism_table.py "$root" "$REPO" --scenario "$YAML"
       done

## DECIDE

One verdict for the run set. The recipe, applied per arm:

| what you see | what it is | what it earns |
|---|---|---|
| `harness-failure` in any run | the MCP server never came up: nothing was measured | RE-RUN that arm |
| fewer than 2 measured sense runs | an incomplete pair | RE-RUN that arm |
| `verdict_split` true | the two runs land in different dominant cells | RE-RUN that arm, once, for a third run |
| `never-routed` or `search-only` | the server was up and the model did not use it | KEEP. A finding, not a fault |
| `routed`, 2 measured runs | a measurement | KEEP |

A `never-routed` arm is never re-run to get a better number. It is the result.

Re-runs are bounded: an arm that has already been re-run twice for the same reason is
reported as it stands, with the reason named. Never raise a watchdog or a budget to
rescue one.

## ARTIFACT

Write `$LOOPDIR/bench.md`: one short section per arm, each carrying the arm's routing
states, its measured run count, and the one line of quoted output you based the call on.
Then the verdict.

Write `$LOOPDIR/validity.verdict.json` - named for the PHASE, which is how the driver
looks it up:

    {
      "phase":    "validity",
      "repo":     "<repo>",
      "verdict":  "BOARD" | "RERUN",
      "artifact": "verticals/<vertical>/results/cycle2/<repo>/bench.md",
      "rerun":    ["<model id>", ...],
      "keep":     ["<model id>", ...],
      "notes":    "<one line, or the blocked reason>"
    }

`phase`, `repo` and `artifact` are required by the driver's guard and `artifact` is
checked to EXIST, relative to `improvement-loop/`.

`BOARD` means every arm is either a measurement or a reported finding, and the page can
be built. `RERUN` names the arms the driver must run again and nothing else.

## DONE WHEN

- `bench.md` exists and names every arm in `$ARMS`.
- `validity.verdict.json` parses, carries all five required keys, and its verdict is one
  of the two.
- Every arm appears in exactly one of `rerun` or `keep`.
- Every claim in `bench.md` is quoted command output.

## DO NOT

- Do not edit the scenario, its gold, its rubric, or any script.
- Do not re-run the headline arm, or run anything yourself: the driver runs arms.
- Do not call an arm a loss. You rule on whether it was MEASURED, never on how it did.
