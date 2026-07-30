# Survey friction ledger

> Fed by `bench/lib/survey_verify.py --report` at loop close (process:
> `loops/00-agent-survey.md`). One row per friction hypothesis. Status is FORWARD-ONLY:
> `hypothesis → finding → filed → shipped | killed`. Promotion to finding needs verified instances
> from ≥2 independent model×repo runs AND a clean confound cross-tab. Killed rows stay - they are
> the record that stops a dead idea from resurfacing as "new".
>
> **First harvest 2026-07-21** (go vertical, Loop 5): 35 surveyed runs across 4 models, via
> `bench/lib/survey_verify.py --report`. Rows 1-4 clear the promotion bar (>=2 independent
> model x repo runs + a clean confound cross-tab: the ask spans 8 repos and 4 models, so it
> tracks neither a repo nor a model).

## Rows

| # | Friction (agent's words, compressed) | Surface | Evidence (model×repo runs, verified n) | transcript_miss agrees? | Status | Exit / issue |
|---|---|---|---|---|---|---|
| 1 | "Give each `retained_via_interfaces`/`composed_by` row the RETAINING FIELD's own name and line, not the holder struct's declaration line" | `sense_blast`, `sense_graph` | **4 independent models, 8 repos**: claude-opus-4-8 (consul ×3, nomad ×3, dolt ×3, pebble ×2, teleport ×2, grpc-go ×2, miniflux), kimi-k3 (consul, nomad, dolt, pebble), glm-5.2 (consul, nomad, dolt), mistral-large-3 (consul) | yes - every citing run also shows the grep/read fallback in-transcript | **finding** | - |
| 2 | "Emit the ASSIGNMENT / injection site for each retained row (where a concrete value is actually stored into the interface-typed field)" | `sense_blast` | **3 independent models**: claude-opus-4-8 (pebble ×3, consul ×3, teleport), kimi-k3 (pebble, nomad), glm-5.2 (pebble), mistral-large-3 (pebble) | yes | **finding** | - |
| 3 | "`carrier` is a guess and is wrong more often than right on hub symbols; mark inferred vs type-checked or drop it" | `sense_blast` | claude-opus-4-8 (consul ×3, dolt, teleport), kimi-k3 (consul, nomad) - 2 models, 3 repos | yes (3 carriers hand-checked wrong in one consul run) | **finding** | - |
| 4 | "Retention through BOUND METHOD VALUES and closures is invisible, and on nomad it was the highest-risk holder in the audit" | `sense_blast` | claude-opus-4-8 (nomad ×3, consul ×2, miniflux), kimi-k3 (consul) - 2 models, 3 repos | yes - `index_caveat` already names it, so the gap is known and load-bearing | **finding** | - |
| 5 | "Never silently trim `retained_via_interfaces`; give a cursor or name the omitted symbols (`hidden: 135`, `retained_trimmed: true`)" | `sense_blast` | claude-opus-4-8 (dolt, teleport) - 1 model, 2 repos | yes (re-run at `max_hops=1` returned a SUBSET, 40 of 48) | hypothesis | pagination pitch already carded |
| 6 | "Flag or filter production vs test rows; `include_tests:false` does not filter retained rows" | `sense_blast` | claude-opus-4-8 (nomad ×2, teleport, laravel-framework) - 1 model, 3 repos | yes (a test-only RPCHandler route presented at production confidence) | hypothesis | needs a 2nd model |

## Confab rates (per model, cumulative)

| Model | instances | verified | confab rate | note |
|---|---|---|---|---|
| claude-opus-4-8 | 359 | 223 | 38% | n=23 runs, avg self-score 8.0 |
| kimi-for-coding/k3 | 53 | 37 | 30% | n=4 runs, avg self-score 8.2 |
| ollama-cloud/glm-5.2 | 53 | 35 | 34% | n=4 runs, avg self-score 7.8 |
| ollama-cloud/mistral-large-3:675b | 28 | 0 | **100%** | n=4 runs, avg self-score 8.5 - EVERY cited instance failed transcript verification. Its surveys carry no evidentiary weight; note it also self-scores HIGHEST of the four. Self-report is a hypothesis, the transcript is the killer. |

