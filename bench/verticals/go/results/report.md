# go - Sense vertical benchmark

This is the benchmark, the methodology, and the raw data behind the go write-ups: how much a structural code index (**Sense**) helps an AI coding agent answer questions about real-world codebases in this stack, measured across several models.

Every scenario is run twice with the same model: a **baseline** arm (the agent's normal tools) and a **sense** arm (the same tools plus the Sense index). Each scenario declares a must-find set of code locations, and the score is **cited recall** - the share of that set the answer pinned to an exact `path:line`. The deltas below are sense minus baseline, so **positive means Sense helped**.

Jump to: [Methodology](#methodology) · [Results](#results) · [Per-model reports](#per-model-reports) · [Per-repo variance](#per-repo-variance)

## Methodology

**The question.** Does giving an AI coding agent a structural index of a codebase make it answer questions about that code more completely and more precisely? Sense is that index: it maps a repo's symbols, call relationships, and dependents so the agent can look them up instead of reading files one at a time.

**The two arms.** Every scenario runs twice with the *same* model and the *same* underlying toolkit. The **baseline** arm uses the agent's normal tools (file reads, grep, and so on). The **sense** arm adds the Sense index on top. Nothing else changes, so any gap between the two is attributable to the index.

**The repositories.** The scenarios run against 4 real-world codebases from this stack, each pinned to a fixed commit so a run is reproducible. They span small libraries to large applications, including ones far too big to fit in a single context window.

**The scenarios.** Each scenario is a realistic, multi-step comprehension task (for example: trace a request from its controller through to persistence and locate the tests that cover it). Each one declares a **must-find set** - the exact code locations a complete, correct answer should surface. Scenarios are written so that a naive text search does not trivially answer them: the relevant code is scattered across non-obvious places.

**The metrics.** The headline is **cited recall**: of the must-find set, the share the answer pinned to an exact `path:line` an agent could jump straight to. Reported alongside it are **mention recall** (named at all, location optional), **relationship correctness** (states the right connection, not just the name), **truthfulness** (no confidently false claims), and **billed tokens** (the context the answer cost to produce). Recall is the goal; tokens are reported but never traded against it.

**Grading.** A separate judge model (claude-opus-4-7) grades each answer's coverage against the authored must-find set, so a confident-sounding but incomplete answer is penalised for what it leaves out. Every `path:line` an answer prints is then checked against the repo at the benchmarked commit; any citation that does not resolve is listed per model in the [citation check](#per-model-reports).

**Repeatability.** Run counts vary by arm (1x to 2x): the headline arm carries the RUNS=2 law, while cross-model confirmation arms run 1x by design and their numbers are directional, carrying an OPEN flag. The run-to-run spread is published under [Per-repo variance](#per-repo-variance), so a headline number is trusted only when it is stable rather than a lucky draw.

## Results

The raw numbers, 6 models across 4 repos. Each model's full per-repo tables are linked under [Per-model reports](#per-model-reports).

### Per-model summary

One row per model. **repos** is how many of the vertical's scenarios it was benched on; the two Δ columns are the mean cited-recall lift (sense − baseline) across them - **overall** for the whole scenario, **deps** for the harder `dependents` group (what depends on a given symbol). Positive means Sense helped that model on average.

| model | repos | mean overall Δ | mean deps Δ |
|---|---|---|---|
| claude-fable-5 | 4 | +0.72 | +0.50 |
| claude-opus-4-8 | 4 | +0.70 | +1.00 |
| glm-5.2_cloud | 4 | +0.51 | +0.50 |
| gpt-5.5 | 4 | +0.43 | +0.42 |
| kimi-for-coding_k3 | 4 | +0.57 | +0.50 |
| ollama-cloud_mistral-large-3_675b | 4 | +0.35 | +1.00 |

### Overall cited-recall Δ (sense − baseline), by model × repo

Every cell is the cited-recall lift for one model on one repo. For example, `+0.40` means the sense arm pinned 40 percentage points more of that repo's must-find set to an exact location than the baseline did. A near-zero value is a tie; a `-` means that repo was not benched for that model.

| model | consul | dolt | nomad | pebble |
|---|---|---|---|---|
| claude-fable-5 | +1.00 | +1.00 | +0.47 | +0.43 |
| claude-opus-4-8 | +0.54 | +0.71 | +0.57 | +1.00 |
| glm-5.2_cloud | +0.46 | +0.67 | +0.47 | +0.43 |
| gpt-5.5 | +0.46 | +0.42 | +0.47 | +0.36 |
| kimi-for-coding_k3 | +0.38 | +0.75 | +0.73 | +0.43 |
| ollama-cloud_mistral-large-3_675b | -0.08 | +0.00 | +0.60 | +0.86 |

### Efficiency by model (baseline → sense)

What each arm spent to produce its answers, averaged across the model's repos and shown as baseline → sense. These are consumption figures, independent of any provider's price (no dollar cost). **billed** is the tokens you actually pay for (uncached input + output); **cached** is cache-read context; **wall s** is session wall-clock seconds. Lower is cheaper - but recall is never traded for a smaller token bill, so read this alongside the lift above, not instead of it.

| model | wall s | billed tok | cached tok | output tok | billed Δ% |
|---|---|---|---|---|---|
| claude-fable-5 | 518 → 369 | 31,148 → 29,678 | 2,136,328 → 1,148,518 | 31,077 → 29,645 | -5% |
| claude-opus-4-8 | 311 → 274 | 20,311 → 19,909 | 1,303,162 → 514,964 | 20,256 → 19,890 | -2% |
| glm-5.2_cloud | 409 → 478 | 5,730,120 → 1,847,768 | 0 → 0 | 20,380 → 19,016 | -68% |
| gpt-5.5 | 274 → 301 | 185,793 → 127,997 | 1,327,120 → 1,169,872 | 13,358 → 13,937 | -31% |
| kimi-for-coding_k3 | 1198 → 803 | 97,243 → 77,978 | 1,583,872 → 819,840 | 21,404 → 17,276 | -20% |
| ollama-cloud_mistral-large-3_675b | 512 → 353 | 1,269,857 → 458,281 | 0 → 0 | 7,920 → 5,960 | -64% |

## Per-model reports

Full per-repo tables and the citation check for each model:

| model | report | citation check |
|---|---|---|
| claude-fable-5 | [report.md](claude-fable-5/report.md) | [citation-hallucinations.md](claude-fable-5/citation-hallucinations.md) |
| claude-opus-4-8 | [report.md](claude-opus-4-8/report.md) | [citation-hallucinations.md](claude-opus-4-8/citation-hallucinations.md) |
| glm-5.2_cloud | [report.md](glm-5.2_cloud/report.md) | [citation-hallucinations.md](glm-5.2_cloud/citation-hallucinations.md) |
| gpt-5.5 | [report.md](gpt-5.5/report.md) | [citation-hallucinations.md](gpt-5.5/citation-hallucinations.md) |
| kimi-for-coding_k3 | [report.md](kimi-for-coding_k3/report.md) | [citation-hallucinations.md](kimi-for-coding_k3/citation-hallucinations.md) |
| ollama-cloud_mistral-large-3_675b | [report.md](ollama-cloud_mistral-large-3_675b/report.md) | [citation-hallucinations.md](ollama-cloud_mistral-large-3_675b/citation-hallucinations.md) |

## Per-repo variance

Run-to-run spread per repo (is the headline stable or noise?):

[consul](variance/consul.md) · [dolt](variance/dolt.md) · [nomad](variance/nomad.md) · [pebble](variance/pebble.md)
