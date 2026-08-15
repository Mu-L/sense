Diagnose two arms of the bitwarden-server board that did not convert: GLM-5.2 and Kimi K3.
This is a READ over runs that already exist. $0, no model is called, nothing is re-run.

Start by re-rendering and reading STATUS.md
(`bash improvement-loop/bench/lib/render-status.sh csharp-aspnet`), then read the three
2026-08-14 entries in `verticals/csharp-aspnet/LEDGER.md`. Cycle 1 and cycle 2 are both
`done`; there is no loop to advance and no repo to take. Do not start a driver.

## What is already established - do not re-derive it

`finding/dependents-reached-by-shape-of-the-ask` (LEDGER, 2026-08-14) measured, across 17
sense runs and 5 models, whether Sense actually RETURNED the 16 gold `dependents` rows, by
intersecting the gold `match:` paths against the raw MCP traffic in each run's
`sense-io.jsonl`. Result: the payload was complete in 13 of 17 runs. Where it was complete
AND the run finished, 7 of 8 cited all 16. So the board's losses split two ways:

- RETRIEVAL miss - GLM-5.2 runs 1, 2, 3: the dependents were never in the payload.
- DELIVERY miss - Kimi K3 runs 1-5, 7, 8, and mistral run-6: the payload was complete and
  the answer still did not carry them.

The within-model control is the load-bearing evidence: GLM scored 0.400, 0.300, 0.400 on
its three incomplete-payload runs and 0.967 on run-8, the one run that called
`sense_blast {"symbol": "IPricingClient"}`. Same model, same prompt, same index.

## The two questions this session owes

**1. GLM: why does the ask come out shallow three times in four?**
Runs at `results/glm-5.2_cloud/a4012313daeeb8e9/sense/bitwarden-server/run-{1,2,3,8}/`.
The three losing runs reached for `depth: 1` callers, a method-level symbol
(`IPricingClient.GetPlanOrThrow`), the CONCRETE class `PricingClient`, and file paths.
Run-8 reached for the interface type. `sense-io.jsonl` holds every call and response;
`transcript.json` holds the reasoning around them. What is in run-8's context, or its
preceding turns, that is absent from the other three? Is there anything in what Sense
RETURNS on the shallow calls that would have pointed at the type-level question - and if
not, is that the gap worth a product pitch?

**2. Kimi: where do 9k-17k output tokens go when the answer is 174 characters?**
Runs at `results/kimi-for-coding_k3/a4012313daeeb8e9/sense/bitwarden-server/run-{1,2,3,4,5,7,8}/`.
Seven runs, all with 16/16 in the payload, all stalled mid-run (`stalled_midrun`, 400-1219s)
with 100-500 char answers. Read the transcripts: is it looping, re-reading, composing and
being cut, or never starting to compose? The 2026-08-14 quota reset was ruled out as the
cause - runs 7 and 8 ran on a full quota and failed identically.

## Two hazards, both live

- **Four baselines were capped.** Every baseline in the 2026-08-14 afternoon batch
  (GLM run-3, run-8; mistral run-7, run-8) was killed at `hard_cap_timeout`, because the
  matched budget gives the control `sense wall x 1.2` and those sense runs were fast.
  Pooling capped with uncapped pairs manufactures a Sense win: GLM reads negative on its two
  cleanly-paired runs and positive if all four are averaged. Do not average them, and do not
  re-publish the board without deciding this explicitly - the published page is currently
  correct-but-stale, which is the safer of the two errors.
- **Citation grounding is noisy on the ollama arms** and gold recall is not. Mistral cited
  paths as `/src/...`; the checker logged "file not found" for all of them while the same run
  scored 16/16 on gold. Do not read `citation-hallucinations.md` as a recall signal.

## Scope

Read-only. Do not re-run arms, do not edit `bench/` scripts, do not re-publish the board,
do not touch a scenario. If the dig produces something worth acting on, write it as a
`finding/<slug>` LEDGER entry and run `python3 bench/lib/ledger_check.py csharp-aspnet`
before calling it recorded - written and recorded are different claims.

The five killers in `improvement-loop/CLAUDE.md` apply to every claim that leaves this
session, and `plans/cycle-1-craft-the-scenario/laws.md` binds the operator session too.
