## Scenario Evaluation

Results: 3 tools × 4 scenarios

Each scenario declares a **must-find set** of code locations a good answer should surface. The headline metric is **cited recall** - the share of that set the answer pinned to an exact location (`path:line`), so an agent can navigate straight there. Each repo below leads with a table of the axes that make up the comparison:

- **Cited recall (the headline)** - share of the must-find set pinned to an exact location (`path:line`, `path (line N)`, a `"line": N` field, or an unambiguous name + line).
- **Mention recall** - share the answer named at all, location optional (how complete the map is).
- **Billed context** - billed tokens (uncached input + output) used to produce the answer, with uncached input shown alongside. Lower is better; never traded against recall.

The aggregate adds the **B-score** = `0.55·cited recall + 0.25·correct-relationship rate + 0.20·truthfulness` - one blended number for the whole answer. Efficiency is reported separately and only credited when recall holds.

**Citations** are `file:line` / `file:Symbol` references the answer printed. Each is checked against the repo at the benchmarked commit; the ones that did not resolve are listed in [`citation-hallucinations.md`](citation-hallucinations.md).

### Reading the scores

| Metric | Best | Meaning |
|--------|------|---------|
| cited_recall | Higher | The headline. Of the must-find items the scenario declares, the share the answer pinned to an exact location (`path:line`) so an agent can jump straight there. |
| b_score | Higher | One blended score: 55% cited recall + 25% correct-relationship rate + 20% truthfulness. A single number for the whole answer's quality. |
| relationship_audit | Higher | Coverage: the share of the must-find set the answer named at all, graded against the authored relationships. |
| related_recall | Higher | Coverage with the CORRECT relationship stated, not just the name. Naming an endpoint is easy; stating how it connects is the harder test. |
| grounded_precision | Higher | Truthfulness: of the items the answer described, the share described correctly (1 minus false-claims over described). |
| contradictions | Lower | Count of confidently false relationship claims. The fabrication signal. |
| process_efficiency | Lower | Reads, tool calls, and billed tokens spent - credited as a saving only when recall is at least as high as the baseline, so a cheaper-but-thinner answer never wins. |
| efficiency | Higher | Combined token and time efficiency, calibrated per repo. |
| tokens | Lower | Billed (uncached) tokens - lower is cheaper. |
| wall_time | Lower | Wall-clock time. |
| cost_usd | Lower | API cost in USD. |
| cites | Higher | Citations that resolved against the repo checkout: `grounded/total`. A trailing **!N** flags line numbers past end-of-file (made-up). Reported, not folded into the headline. |

### baseline-dolt

| Tool | Mention recall | Cited recall (fixed) | Billed ctx | Uncached in | Cached read | Time |
|------|---------------:|---------------------:|-----------:|------------:|------------:|-----:|
| _invalid-300s-fable | 0% (0/12) | 0% (0/12) | 18,403 | 52 | 1,316,170 | 300s |

### dolt

| Tool | Mention recall | Cited recall (fixed) | Billed ctx | Uncached in | Cached read | Time |
|------|---------------:|---------------------:|-----------:|------------:|------------:|-----:|
| baseline | 0% (0/12) | 0% (0/12) | 21,422 | 45 | 1,185,420 | 480s |
| sense | 100% (12/12) | 100% (12/12) | 31,483 | 35 | 1,258,837 | 346s |

_Billed-context Δ (sense vs baseline): **+47%** - Sense loads more._

### pebble

| Tool | Mention recall | Cited recall (fixed) | Billed ctx | Uncached in | Cached read | Time |
|------|---------------:|---------------------:|-----------:|------------:|------------:|-----:|
| baseline | 57% (4/7) | 57% (4/7) | 32,581 | 52 | 1,390,424 | 478s |
| sense | 100% (7/7) | 100% (7/7) | 27,634 | 38 | 1,226,988 | 380s |

_Billed-context Δ (sense vs baseline): **-15%** - Sense loads less._

#### Spend per correctly-cited row

_Each axis divided by the rows that arm actually pinned (baseline 4/7, sense 7/7). Negative Δ = Sense buys a correct row for less. Only meaningful because recall held: a cheaper arm that found less is not more efficient._

| Axis (per correct row) | baseline | sense | Δ |
|------|---------:|------:|----:|
| Billed tokens | 8,145 | 3,948 | **-51.5%** |
| Total tokens (incl. cache) | 371,540 | 189,851 | **-48.9%** |
| Tool calls | 10 | 4 | **-65.2%** |

### sense-dolt

| Tool | Mention recall | Cited recall (fixed) | Billed ctx | Uncached in | Cached read | Time |
|------|---------------:|---------------------:|-----------:|------------:|------------:|-----:|
| _invalid-300s-fable | 0% (0/12) | 0% (0/12) | 10,545 | 38 | 1,385,011 | 299s |

### Aggregate

Ranked by **cited recall** (the headline). **B-score** = `0.55·cited + 0.25·correct-relationship rate + 0.20·truthfulness`. The `Failures` column shows scenarios the tool could not complete. Costs marked `*` are estimated from partial token usage.

| Rank | Tool | Scenarios | Failures | **Cited Recall** | **B-score** | Rel Audit (cov) | Related | Grounded Prec. | Contradict. | Avg Efficiency | Avg Tokens | Avg Time | Total Cost | Avg Grounding |
|-----:|------|----------:|--------:|---------------:|-----------:|--------------:|--------:|---------------:|------------:|--------------:|-----------:|--------:|-----------:|--------------:|
| 1 | sense :1st_place_medal: | 2 | 0 | 1.0000 | **1.0000** | 1.0000 | 1.0000 | 1.0000 | 0 | 0.0197 | 29,558 | 362.6s | $8.91 | 100.0% (321/321) |
| 2 | baseline :2nd_place_medal: | 2 | 0 | 0.2857 | **0.4211** | 0.2560 | 0.2560 | 1.0000 | 0 | 0.0715 | 27,002 | 478.9s | $7.93 | 100.0% (78/78) |
| 3 | _invalid-300s-fable :3rd_place_medal: | 2 | 0 | 0.0000 | - | 0.0000 | 0.0000 | - | 0 | 0.2597 | 14,474 | 299.4s | $7.55 | 100.0% (1/1) |


### Process efficiency (at held recall)

_Sense recall is HIGHER (1.00 vs 0.29) - any process saving is a bonus on top of a completeness win._

| Process axis | baseline | sense | Δ |
|------|---------:|------:|----:|
| Reads | 1 | 8 | **+650%** |
| Tool calls | 38 | 26 | **-30%** |
| Billed tokens | 27,002 | 29,558 | **+9%** |
