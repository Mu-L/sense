# plans/

## What a plan is

An executable order handed to a headless agent by `bench/drivers/vertical-loop.sh`. One plan
is one phase's judgment: the agent runs the listed commands, makes the one decision the driver
cannot make, writes one artifact and one verdict JSON, and dies.

A plan is **not** a doc. It carries no history, no provenance, no rationale for its own
design, and **no outbound links** - a plan that ends in "see the design doc" reproduces the bug
this folder exists to remove. Everything the agent needs is in the file, or in `laws.md`, which
the driver prepends to every plan.

## The fixed section order

    TASK       one sentence
    SCOPE      what is out - the scope-lock paragraph
    RUN        exact commands, in order
    DECIDE     the one judgment, plus the recipe
    ARTIFACT   the file it must write + the verdict JSON schema
    DONE WHEN  mechanical stop conditions
    DO NOT     three lines maximum

## The files

| File | Phase it runs | Verdicts |
|---|---|---|
| `01-scout.md` | pick the contract, the axis and the headline ask | `SHAPE`, `NO-AXIS` |
| `01-curate.md` | gold on the axis the adversary probe disclaimed | `GOLD`, `NO-AXIS` |
| `02-validate.md` | read the unscored run, rule on paying | `PAY`, `DO-NOT-PAY` |
| `laws.md` | not a plan - prepended to all of them | - |

Plan files carry the loop number they replace, so the start point stays visible. `laws.md` and
this file take no prefix.

Phases with no plan have no judgment in them: `index`, `preflight`, `bench` and `harvest` are
bash, and `probe` and `report` are bash spawning a vertex agent whose prompt is fixed. A phase
gets a plan when it stalls, never in advance.

## The two rules that keep this from rotting

1. **A plan is added when a phase stalls, never in advance.** If a session opens a `docs/` page
   mid-run, that is the bug report: what it needed was missing from the plan, and it goes in
   the plan.
2. **The plan is authority at runtime; the doc is authority on design.** They will disagree
   eventually. At runtime the plan wins and the disagreement is filed; at design time the doc
   wins and the plan is rewritten.
