# ruby-rails - STATUS (auto-rendered)

> AUTO-RENDERED 2026-08-06 05:53 GMT by `bench/lib/render-status.sh ruby-rails`.
> Do not edit by hand; do not use for loop decisions (write-only law, `ledger.md`).
> Position is authoritative ON DISK: the results tree, `repos.md`, `LEDGER.md`.

## Loop position (`.loop-state.json`)

| repo | phase | verdict on disk |
|---|---|---|
| chatwoot | done | - (no agent at this phase) |
| discourse | done | - (no agent at this phase) |
| mastodon | done | - (no agent at this phase) |
| rails | done | - (no agent at this phase) |

Resume: `VERTICAL=ruby-rails bash bench/drivers/vertical-loop.sh <repo>`

## Cycle 2: cross-model boards (`.cycle2-state.json`, `reports/`)

| repo | phase | re-runs |
|---|---|---|
| chatwoot | gate | 0 |
| discourse | gate | 0 |
| mastodon | gate | 0 |
| rails | gate | 0 |

_no board published yet._

Resume: `VERTICAL=ruby-rails bash bench/drivers/cycle2-board.sh <repo>`

## Banked cells (`verticals/ruby-rails/banked.jsonl`)

| repo | verdict | overall Δ | best group | runs | scenario |
|---|---|---|---|---|---|
| chatwoot | WIN | +0.26 | dependents +0.58 | 2/2 | `24f720898c0385b9` |
| discourse | WIN | +0.54 | dependents +0.71 | 2/2 | `1def723310067e48` |
| mastodon | WIN | +0.47 | dependents +0.78 | 2/2 | `27dfc6000a5e98f3` |
| rails | WIN | +0.27 | dependents +0.56 | 3/3 | `3f210bcde96c18e1` |

_Rows are re-derivable from the results tree: `banked.py rebuild verticals/<key>`._

## Results cells on disk (`verticals/ruby-rails/results`)

- `claude-opus-5/1def723310067e48/baseline/discourse` - 2 run(s)
- `claude-opus-5/1def723310067e48/sense/discourse` - 2 run(s)
- `claude-opus-5/24f720898c0385b9/baseline/chatwoot` - 2 run(s)
- `claude-opus-5/24f720898c0385b9/sense/chatwoot` - 2 run(s)
- `claude-opus-5/27dfc6000a5e98f3/baseline/mastodon` - 2 run(s)
- `claude-opus-5/27dfc6000a5e98f3/sense/mastodon` - 2 run(s)
- `claude-opus-5/3f210bcde96c18e1/baseline/rails` - 3 run(s)
- `claude-opus-5/3f210bcde96c18e1/sense/rails` - 3 run(s)
- `claude-opus-5/minibench/1b6e6d97c70982fa/minibench/1b6e6d97c70982fa/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/1b6e6d97c70982fa/minibench/1b6e6d97c70982fa/sense/discourse` - 1 run(s)
- `claude-opus-5/minibench/249ee785aca231f4/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/249ee785aca231f4/sense/chatwoot` - 2 run(s)
- `claude-opus-5/minibench/55662cc22444c233/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/55662cc22444c233/sense/discourse` - 1 run(s)
- `claude-opus-5/minibench/563eb01ace77a74f/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/563eb01ace77a74f/sense/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/73e80a260d06f08b/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/73e80a260d06f08b/sense/discourse` - 1 run(s)
- `claude-opus-5/minibench/7c9558546ad9e2db/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/7c9558546ad9e2db/sense/discourse` - 1 run(s)
- `claude-opus-5/minibench/7f9e423233e62113/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/7f9e423233e62113/sense/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/82b4eae75397a62f/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/82b4eae75397a62f/sense/discourse` - 1 run(s)
- `claude-opus-5/minibench/95e070ed48cf0022/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/95e070ed48cf0022/sense/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/97ed494629a484c5/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/97ed494629a484c5/sense/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/a54d1d3cface69e6/minibench/a54d1d3cface69e6/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/a54d1d3cface69e6/minibench/a54d1d3cface69e6/sense/discourse` - 1 run(s)
- `claude-opus-5/minibench/c67d1d8ff8373b6e/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/c67d1d8ff8373b6e/sense/chatwoot` - 1 run(s)
- `claude-opus-5/minibench/cfba4f24dedacca1/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/cfba4f24dedacca1/sense/discourse` - 2 run(s)
- `claude-opus-5/minibench/f4283b9e2d348040/baseline/discourse` - 1 run(s)
- `claude-opus-5/minibench/f4283b9e2d348040/sense/discourse` - 1 run(s)
- `claude-opus-5/minibench/f5335174fcda4ffa/baseline/mastodon` - 1 run(s)
- `claude-opus-5/minibench/f5335174fcda4ffa/sense/mastodon` - 2 run(s)
- `claude-opus-5/minibench/unversioned/baseline/mastodon` - 1 run(s)
- `claude-opus-5/validation/077b1225bb1b78e0/baseline/discourse` - 1 run(s)
- `claude-opus-5/validation/077b1225bb1b78e0/sense/discourse` - 2 run(s)
- `claude-opus-5/validation/1def723310067e48/baseline/discourse` - 1 run(s)
- `claude-opus-5/validation/1def723310067e48/sense/discourse` - 1 run(s)
- `claude-opus-5/validation/24f720898c0385b9/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/validation/24f720898c0385b9/sense/chatwoot` - 1 run(s)
- `claude-opus-5/validation/27dfc6000a5e98f3/baseline/mastodon` - 1 run(s)
- `claude-opus-5/validation/27dfc6000a5e98f3/sense/mastodon` - 1 run(s)
- `claude-opus-5/validation/441f437eecaff61a/baseline/discourse` - 7 run(s)
- `claude-opus-5/validation/441f437eecaff61a/sense/discourse` - 12 run(s)
- `claude-opus-5/validation/f0b650548df212b8/baseline/chatwoot` - 1 run(s)
- `claude-opus-5/validation/f0b650548df212b8/sense/chatwoot` - 1 run(s)
- `claude-opus-5/validation/f576efebe028db23/baseline/mastodon` - 5 run(s)
- `claude-opus-5/validation/f576efebe028db23/sense/mastodon` - 5 run(s)
- `claude-opus-5/validation/f6a017bb2fc1451e/validation/f6a017bb2fc1451e/baseline/discourse` - 1 run(s)
- `claude-opus-5/validation/f6a017bb2fc1451e/validation/f6a017bb2fc1451e/sense/discourse` - 1 run(s)
- `claude-opus-5/validation/validation/baseline/rails` - 1 run(s)
- `claude-opus-5/validation/validation/sense/rails` - 1 run(s)

## Ledger

- latest entry: 2026-08-04 | bench/discourse-paid-WIN | the loop recovered from its own failure mode, unattended
- full narrative: `LEDGER.md` (this folder)
- decision-errors intake: 0 open incident(s) (`../../docs/decision-errors.md`)

## Slate

- `repos.md` (this folder) - admitted repos, pins, and per-candidate verdicts
