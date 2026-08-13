# improvement-loop / docs

**The authority on DESIGN, read by a human.** The authority at RUNTIME is
[`../plans/cycle-1-craft-the-scenario/`](../plans/cycle-1-craft-the-scenario), handed to an agent by the driver. If a session opens a page here
mid-run, that is a bug report: what it needed was missing from its plan, and it goes in the
plan. Scripts live in [`../bench/`](../bench); nothing here is executable.

## The authorities (a rule lives in exactly one of these)

| Doc | What it settles |
|---|---|
| [`goal.md`](goal.md) | the fixed star: make Sense the LLM's best companion on the benched stack and globally |
| [`manifesto.md`](manifesto.md) | bench law: what a vertical is, what a win is, what may never be automated |
| [`judging-contract.md`](judging-contract.md) | how an answer is scored and judged |
| [`help-the-ai.md`](help-the-ai.md) | what "helps the AI" means, measurably |
| [`vertical-program.md`](vertical-program.md) | the sequence of verticals and when the program stops |
| [`how-to-run.md`](how-to-run.md) | session mechanics: pickup, duties, where to write |
| [`bootstrap.md`](bootstrap.md) | the one command that stands a vertical up: hunt, screen, compose, pin, index |
| [`campaign-laws.md`](campaign-laws.md) | the standing empirical laws, each with its provenance |
| [`loss-anatomy.md`](loss-anatomy.md) | how cells lose, and the vertex fixture record |
| [`ledger.md`](ledger.md) | the readability contract: per-vertical `LEDGER.md` + `STATUS.md`, write-only for the loops |
| [`arms.default.txt`](arms.default.txt) | **the only place a model id is named.** Copied to `verticals/<key>/arms.txt` at stamp; every driver and analysis tool resolves through it |

## The loop, and who runs what

Standing a vertical up is one command and is not a loop: it converges on the first pass or
stops with a named status ([`bootstrap.md`](bootstrap.md)). What follows loops, per repo,
depth-first - one repo to a verdict before the next opens, up to six crafting cycles before
the slot swaps to its own declared backup.

| Actor | Owns | Never does |
|---|---|---|
| bash (`bench/drivers/vertical-loop.sh`) | order, state, gates, spawning every agent, refusing to advance | judgment |
| headless plan agent | one phase's judgment; writes one artifact + one verdict JSON | choose the next phase, spawn its own judge |
| vertex agent (`bench-struggle-read`, `bench-evaluator`, `bench-win-confirm`, the adversary probe) | independent adversary and evaluation, spawned by bash | author anything it later grades |
| benched arms | being measured | anything loop-side |
| session agent | writing the plans; product-fix spikes | running the per-repo loop |
| human | stack queue empty, STOPPER ruling, fix authorization, publish sign-off | the per-repo phases, by law |

The phases are `index → scout → probe → curate → preflight → validate → bench → report →
harvest`. Three of them carry judgment and get a plan; the rest are bash, or bash spawning a
vertex whose prompt is fixed. What is downstream of a won cell - matrix fill, harvest, publish,
the product-fix window, the agent survey - is [`parked/`](parked) and not live.

**Known missing:** stop-hook wiring for the run phase stays deferred. It is friction reduction,
not a prerequisite.

## Per vertical

A vertical lives in ONE folder outside this one: [`../verticals/<key>/`](../verticals), stamped by
[`../bench/bootstrap/stamp.sh`](../bench/bootstrap/stamp.sh). The stamp carries structure and
never values - no results, no scores, no filled pack. It holds both halves together - `repos.txt`, `scenarios/`, `results/` and the tracker, repo slate,
and the findings packs. `LEDGER.md` (append-only narrative) and `results/` stay
private; `STATUS.md` is a render, never a source.

## Cumulative, across verticals

[`cross-cutting/`](cross-cutting) carries the fact-packs that outlive one campaign (methodology,
providers, cross-model, harness, product), and [`FRICTION.md`](FRICTION.md) is the forward-only
ledger of agent-reported friction: hypothesis → finding → filed → shipped or killed.
