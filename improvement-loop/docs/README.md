# improvement-loop / docs

The rule-book and the working surfaces for the loop. Scripts live in [`../bench/`](../bench);
nothing here is executable.

## The authorities (a rule lives in exactly one of these)

| Doc | What it settles |
|---|---|
| [`goal.md`](goal.md) | the fixed star: make Sense the LLM's best companion on the benched stack and globally |
| [`manifesto.md`](manifesto.md) | bench law: what a vertical is, what a win is, what may never be automated |
| [`judging-contract.md`](judging-contract.md) | how an answer is scored and judged |
| [`help-the-ai.md`](help-the-ai.md) | what "helps the AI" means, measurably |
| [`vertical-program.md`](vertical-program.md) | the sequence of verticals and when the program stops |
| [`how-to-run.md`](how-to-run.md) | session mechanics: pickup, duties, where to write |
| [`arms.default.txt`](arms.default.txt) | **the only place a model id is named.** Copied to `verticals/<key>/arms.txt` at stamp; every driver, analysis tool and one-pager resolves through it |

## The loops

[`loops/`](loops) holds one page per loop (bootstrap → admission → convergence → matrix fill →
harvest → publish → product fix), plus the ledger discipline (`00-ledger.md`) and the standing
empirical laws distilled from past campaigns (`campaign-laws.md`).

## Per vertical

A vertical lives in ONE folder outside this one: [`../verticals/<key>/`](../verticals), stamped by
[`../bench/drivers/new-vertical.sh`](../bench/drivers/new-vertical.sh). The stamp carries structure and
never values - no results, no scores, no filled pack. It holds both halves together - `repos.txt`, `scenarios/`, `results/` and the tracker, repo slate,
and the findings packs. `LEDGER.md` (append-only narrative) and `results/` stay
private; `STATUS.md` is a render, never a source.

## Cumulative, across verticals

[`cross-cutting/`](cross-cutting) carries the fact-packs that outlive one campaign (methodology,
providers, cross-model, harness, product), and [`FRICTION.md`](FRICTION.md) is the forward-only
ledger of agent-reported friction: hypothesis → finding → filed → shipped or killed.
