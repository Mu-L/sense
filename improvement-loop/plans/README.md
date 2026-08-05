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

One folder per cycle, each with its own `laws.md`. The laws are NOT shared: cycle 1
cannot record a loss, cycle 2 publishes one.

| Cycle | What it does |
|---|---|
| `cycle-1-craft-the-scenario/` | takes a repo, hands back a scenario that wins on the headline model |
| `cycle-2-bench-across-models/` | takes that banked win, puts it to the other models, publishes the page |

### `cycle-1-craft-the-scenario/`

| File | Phase it runs | Verdicts |
|---|---|---|
| `01-author.md` | anchor, question, two-step probe scenario, gold | `DRAFT`, `NO-ANCHOR` |
| `02-minibench.md` | read the two-arm run, rule on expanding | `PROCEED`, `REQUESTION`, `NO-ANCHOR` |
| `03-expand.md` | the seven-step session on the measured question | `SCENARIO`, `REQUESTION` |
| `04-validate.md` | read the unscored full run, rule on paying | `PAY`, `DO-NOT-PAY` |
| `05-handoff.md` | after six cycles, the human's plain-language one page | `HANDOFF` |
| `laws.md` | not a plan - prepended to all of them | - |

### `cycle-2-bench-across-models/`

| File | Phase it runs | Verdicts |
|---|---|---|
| `01-bench.md` | read the confirmation arms, rule on whether the run set is sound | `BOARD`, `RERUN` |
| `02-report.md` | write the Reading section at the top of the rendered page | `READING` |
| `laws.md` | not a plan - prepended to both | - |

Cycle 2 has two plans and not five because most of it is not judgment. Eligibility, the
build gate, the mechanism table, the numbers and the page itself are all deterministic:
same inputs, same bytes. The two judgments left are whether the runs measured anything,
and what the result means to a reader.

Plan files are numbered in the order the driver runs them. `laws.md` and this file take no
prefix.

Authoring is a cycle, not a line, and the driver runs it unattended: `NO-ANCHOR`,
`REQUESTION` and `DO-NOT-PAY` all re-enter `01-author.md` immediately, with the credit table
that rejected the question, and the anchor stays. The scenario is rewritten in place; it is
never deleted and re-scouted. The loop parks for a human only after six cycles on one repo, and when it does it writes
`05-handoff.md` first: a plain-language read of all six attempts, so what reaches the human is
a decision to make, not a dead end to debug.

So a verdict here is not a report filed for later. The artifact you write is read by the next
authoring agent within the minute, and it is the only thing that agent has to improve on.

Phases with no plan have no judgment in them: `index`, `preflight`, `bench` and `harvest` are
bash, and `report` is bash spawning a vertex agent whose prompt is fixed. A phase gets a plan
when it stalls, never in advance.

## The two rules that keep this from rotting

1. **A plan is added when a phase stalls, never in advance.** If a session opens a `docs/` page
   mid-run, that is the bug report: what it needed was missing from the plan, and it goes in
   the plan.
2. **The plan is authority at runtime; the doc is authority on design.** They will disagree
   eventually. At runtime the plan wins and the disagreement is filed; at design time the doc
   wins and the plan is rewritten.
