# csharp-aspnet - STATUS (auto-rendered)

> AUTO-RENDERED 2026-08-14 19:14 GMT by `bench/lib/render-status.sh csharp-aspnet`.
> Do not edit by hand; do not use for loop decisions (write-only law, `ledger.md`).
> Position is authoritative ON DISK: the results tree, `repos.md`, `LEDGER.md`.

## Loop position (`.loop-state.json`)

| repo | phase | verdict on disk |
|---|---|---|
| bitwarden-server | done | - (no agent at this phase) |

Resume - run the line for the repo you are taking:
- `VERTICAL=csharp-aspnet bash bench/drivers/vertical-loop.sh bitwarden-server`

## Cycle 2: cross-model boards (`.cycle2-state.json`, `reports/`)

| repo | phase | re-runs |
|---|---|---|
| bitwarden-server | done | 2 |

Published boards:

- `reports/bitwarden-server-a4012313daeeb8e9.md`

Start:  `VERTICAL=csharp-aspnet bash bench/drivers/cycle2-board.sh --eligible`, then
`VERTICAL=csharp-aspnet bash bench/drivers/cycle2-board.sh <repo>` for one of them.

## Banked cells (`verticals/csharp-aspnet/banked.jsonl`)

| repo | verdict | overall Δ | best group | runs | scenario |
|---|---|---|---|---|---|
| bitwarden-server | WIN | +0.45 | dependents +0.75 | 2/2 | `a4012313daeeb8e9` |

_Rows are re-derivable from the results tree: `banked.py rebuild verticals/<key>`._

## Results cells on disk (`verticals/csharp-aspnet/results`)

- `claude-opus-5/a4012313daeeb8e9/baseline/bitwarden-server` - 2 run(s)
- `claude-opus-5/a4012313daeeb8e9/sense/bitwarden-server` - 2 run(s)
- `claude-opus-5/minibench/0e55d143a9867e2b/baseline/bitwarden-server` - 1 run(s)
- `claude-opus-5/minibench/0e55d143a9867e2b/sense/bitwarden-server` - 1 run(s)
- `claude-opus-5/minibench/47cd7630c0ec9f65/baseline/bitwarden-server` - 1 run(s)
- `claude-opus-5/minibench/47cd7630c0ec9f65/sense/bitwarden-server` - 1 run(s)
- `claude-opus-5/minibench/57c3bcf795401ab9/baseline/bitwarden-server` - 1 run(s)
- `claude-opus-5/minibench/57c3bcf795401ab9/sense/bitwarden-server` - 1 run(s)
- `claude-opus-5/minibench/c555aeb98e6791f3/baseline/bitwarden-server` - 1 run(s)
- `claude-opus-5/minibench/c555aeb98e6791f3/sense/bitwarden-server` - 1 run(s)
- `claude-opus-5/validation/a4012313daeeb8e9/baseline/bitwarden-server` - 1 run(s)
- `claude-opus-5/validation/a4012313daeeb8e9/sense/bitwarden-server` - 1 run(s)
- `glm-5.2_cloud/a4012313daeeb8e9/baseline/bitwarden-server` - 5 run(s)
- `glm-5.2_cloud/a4012313daeeb8e9/sense/bitwarden-server` - 5 run(s)
- `gpt-5.6-sol/a4012313daeeb8e9/baseline/bitwarden-server` - 2 run(s)
- `gpt-5.6-sol/a4012313daeeb8e9/sense/bitwarden-server` - 2 run(s)
- `kimi-for-coding_k3/a4012313daeeb8e9/sense/bitwarden-server` - 8 run(s)
- `mistral-large-3_cloud/a4012313daeeb8e9/sense/bitwarden-server` - 4 run(s)
- `ollama-cloud_mistral-large-3_675b/a4012313daeeb8e9/baseline/bitwarden-server` - 4 run(s)
- `ollama-cloud_mistral-large-3_675b/a4012313daeeb8e9/sense/bitwarden-server` - 4 run(s)

## Ledger

- latest entry: 2026-08-14 | finding/k3-never-starts-composing | not a truncated answer: across 7 runs k3 wrote 2,153 characters of prose in total
- full narrative: `LEDGER.md` (this folder)
- decision-errors intake: 0 open incident(s) (`../../docs/decision-errors.md`)

## Slate

- `repos.md` (this folder) - admitted repos, pins, and per-candidate verdicts
