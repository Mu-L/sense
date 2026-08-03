# ruby-rails - STATUS (auto-rendered)

> AUTO-RENDERED 2026-08-03 07:44 GMT by `bench/lib/render-status.sh ruby-rails`.
> Do not edit by hand; do not use for loop decisions (write-only law, `ledger.md`).
> Position is authoritative ON DISK: the results tree, `repos.md`, `LEDGER.md`.

## Loop position (`.loop-state.json`)

| repo | phase | verdict on disk |
|---|---|---|
| mastodon | author | none - the next run RE-SPAWNS the author agent |
| rails | done | - (no agent at this phase) |

Resume: `VERTICAL=ruby-rails bash bench/drivers/vertical-loop.sh <repo>`

## Matrix (report-matrix.sh, VERTICAL=ruby-rails)

### Results

The raw numbers, 1 models across 1 repos. Each model's full per-repo tables are linked under [Per-model reports](#per-model-reports).

#### Per-model summary

One row per model. **repos** is how many of the vertical's scenarios it was benched on; the two Δ columns are the mean cited-recall lift (sense − baseline) across them - **overall** for the whole scenario, **deps** for the harder `dependents` group (what depends on a given symbol). Positive means Sense helped that model on average.

| model | repos | mean overall Δ | mean deps Δ |
|---|---|---|---|
| claude-opus-5 | 1 | +0.30 | +0.56 |

#### Overall cited-recall Δ (sense − baseline), by model × repo

Every cell is the cited-recall lift for one model on one repo. For example, `+0.40` means the sense arm pinned 40 percentage points more of that repo's must-find set to an exact location than the baseline did. A near-zero value is a tie; a `-` means that repo was not benched for that model.

| model | rails |
|---|---|
| claude-opus-5 | +0.30 |

#### Efficiency by model (baseline → sense)

What each arm spent to produce its answers, averaged across the model's repos and shown as baseline → sense. These are consumption figures, independent of any provider's price (no dollar cost). **billed** is the tokens you actually pay for (uncached input + output); **cached** is cache-read context; **wall s** is session wall-clock seconds. Lower is cheaper - but recall is never traded for a smaller token bill, so read this alongside the lift above, not instead of it.

| model | wall s | billed tok | cached tok | output tok | billed Δ% |
|---|---|---|---|---|---|
| claude-opus-5 | 203 → 208 | 16,715 → 17,904 | 588,600 → 746,666 | 16,682 → 17,876 | +7% |

### Per-model reports

Full per-repo tables and the citation check for each model:

| model | report | citation check |
|---|---|---|
| claude-opus-5 | [report.md](claude-opus-5/report.md) | [citation-hallucinations.md](claude-opus-5/citation-hallucinations.md) |

### Per-repo variance

Run-to-run spread per repo (is the headline stable or noise?):

[mastodon](variance/mastodon.md) · [rails](variance/rails.md)

_Full report with methodology: `verticals/ruby-rails/results/report.md`_

## Results cells on disk (`verticals/ruby-rails/results`)

- `claude-opus-5/baseline/rails` - 2 run(s)
- `claude-opus-5/sense/rails` - 2 run(s)

## Ledger

- latest entry: 2026-08-02 | ruling/the-ask-was-the-answer | the arm is exonerated; the scenario hands over the gold
- full narrative: `LEDGER.md` (this folder)
- decision-errors intake: 0 open incident(s) (`../../docs/decision-errors.md`)

## Slate

- `repos.md` (this folder) - admitted repos, pins, and per-candidate verdicts
