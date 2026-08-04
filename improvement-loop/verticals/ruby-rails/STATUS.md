# ruby-rails - STATUS (auto-rendered)

> AUTO-RENDERED 2026-08-04 13:14 GMT by `bench/lib/render-status.sh ruby-rails`.
> Do not edit by hand; do not use for loop decisions (write-only law, `ledger.md`).
> Position is authoritative ON DISK: the results tree, `repos.md`, `LEDGER.md`.

## Loop position (`.loop-state.json`)

| repo | phase | verdict on disk |
|---|---|---|
| mastodon | done | - (no agent at this phase) |
| rails | done | - (no agent at this phase) |

Resume: `VERTICAL=ruby-rails bash bench/drivers/vertical-loop.sh <repo>`

## Matrix (report-matrix.sh, VERTICAL=ruby-rails)

## ruby-rails - Sense vertical benchmark

This is the benchmark, the methodology, and the raw data behind the ruby-rails write-ups: how much a structural code index (**Sense**) helps an AI coding agent answer questions about real-world codebases in this stack, measured across several models.

Every scenario is run twice with the same model: a **baseline** arm (the agent's normal tools) and a **sense** arm (the same tools plus the Sense index). Each scenario declares a must-find set of code locations, and the score is **cited recall** - the share of that set the answer pinned to an exact `path:line`. The deltas below are sense minus baseline, so **positive means Sense helped**.

Jump to: [Methodology](#methodology) · [Results](#results) · [Per-model reports](#per-model-reports) · [Per-repo variance](#per-repo-variance)

_No model results yet._

_Full report with methodology: `verticals/ruby-rails/results/report.md`_

## Results cells on disk (`verticals/ruby-rails/results`)

_no run cells on disk yet_

## Ledger

- latest entry: 2026-08-04 | stopper/gold-matcher-path-only | RULED: a class name plus a line is a citation
- full narrative: `LEDGER.md` (this folder)
- decision-errors intake: 0 open incident(s) (`../../docs/decision-errors.md`)

## Slate

- `repos.md` (this folder) - admitted repos, pins, and per-candidate verdicts
