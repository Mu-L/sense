# FROZEN

**This tree stopped moving on 2026-08-18**, at `dce2796`, the tip of `main` when
this record was written. Apart from this record itself, the last commit to change
anything under it is `465c97f` (2026-08-15).

`improvement-loop/` was the first bench instrument for Sense. It has been
replaced by `lab/`, which has now reproduced a recorded result, authored a fresh
scenario to a verdict, and produced one shipped product change: `e9497e8`,
`fix(resolve): bind a declared type through its enclosing scope`, merged as #291.
That third proof is the one this freeze waited for.

## What frozen means

- **no new campaigns** here; all authoring and measurement happens in `lab/`
- **no edits, including fixes.** A frozen tree with a known defect is readable.
  A tree someone patched on the last day is a tree whose final state nobody can
  reason about
- **it stays in the repository**, as history and as the source of the artifact
  corpus: 238 recorded transcripts (234 of the run directories carry a
  `run_meta.json`), which are the fixtures for the pure-layer tests in `lab/`
- **nothing reads it at runtime**, which has been true since the cycle 02
  migration. Re-verified at the freeze rather than assumed: the only remaining
  mentions anywhere are three inert strings (a `root` path recorded inside two
  frozen gate fixtures, and a sentence in `lab/internal/transcript/testdata/README.md`
  describing the one-time corpus sweep) plus one comment in the root `Makefile`.
  The stronger check was also run: with this whole tree moved out of the
  repository, `go test -count=1 ./...` in `lab/` passes in every package
- `improvement-loop/CLAUDE.md`, `docs/` and `plans/` still read as live
  instructions. They are **historical**: they describe how a decision used to be
  made, and nothing should be run from them

**Deletion is a separate decision, later, and there is no hurry.** A frozen tree
costs disk and nothing else. `bench/` at the repository root is *not* frozen by
this: it is replaced by `compare` in cycle 08, and its retirement is that
cycle's business.

## Campaigns, and where they ended

| vertical | repositories | where it ended |
|---|---|---|
| `ruby-rails` | chatwoot, discourse, mastodon, rails | all four `done` through the loop, all four `done` through the cycle-2 cross-model boards (2 re-runs each). Four published boards in `reports/` |
| `csharp-aspnet` | bitwarden-server | `done` through the loop and `done` through the cycle-2 board (2 re-runs). One published board in `reports/` |

**`php-laravel` was screened and never opened.** A stack config
(`stacks/php-laravel.conf`) exists, and five repositories were indexed for it
(coolify, bagisto, filament, laravel-framework, snipe-it); there is no vertical
tree, no results and no cells. It ends as screening, not as a campaign.

The queue in `verticals.txt` lists fifteen verticals that were never started.
They are a plan, not a backlog with work in it.

## Banked cells, and the models they were banked on

Five cells, all `WIN`, all banked on **`claude-opus-5`** as the headline arm.

| vertical | repo | overall Δ | best group | runs | scenario | Sense version |
|---|---|---|---|---|---|---|
| ruby-rails | discourse | +0.5435 | dependents +0.71 | 2/2 | `1def723310067e48` | 1.13.5 |
| ruby-rails | mastodon | +0.4737 | dependents +0.78 | 2/2 | `27dfc6000a5e98f3` | 1.13.5 |
| ruby-rails | rails | +0.2667 | dependents +0.56 | 3/3 | `3f210bcde96c18e1` | 1.13.5 |
| ruby-rails | chatwoot | +0.2632 | dependents +0.58 | 2/2 | `24f720898c0385b9` | 1.13.5 |
| csharp-aspnet | bitwarden-server | +0.45 | dependents +0.75 | 2/2 | `a4012313daeeb8e9` | 1.14.0 |

All five rows crossed over verbatim into `lab/internal/gate/testdata/`
(`banked-ruby-rails.jsonl`, `banked-csharp-aspnet.jsonl`), which is where the
regression corpus is read from now.

Confirmation arms were run beyond the headline model: `glm-5.2_cloud`,
`gpt-5.6-sol`, `kimi-for-coding_k3`, `mistral-large-3_cloud` and
`ollama-cloud_mistral-large-3_675b`. Their runs are on disk under each
vertical's `results/`; they are not what any cell was banked on.

## Scenarios that crossed over

All five, into `lab/scenarios/<name>/`, with their gold and rubric:

| scenario | gold rows | scoreable |
|---|---|---|
| chatwoot | 38 | yes |
| mastodon | 38 | yes |
| bitwarden-server | 30 | yes |
| discourse | 23 | yes |
| rails | 25 | **no, quarantined** |

154 rows in total, which is the count that arrived.

## Gold rows quarantined, and why

**25 rows, all of them the rails scenario, one reason:**

> no `path:line`, so nothing an arm writes could ever match it

Every one of rails' 25 rows fails it, so the whole scenario is unscoreable and
`Gold.Group` refuses the group rather than returning an empty one — a silent
0 of 25 would look exactly like an arm that found nothing. The state is pinned
by `TestWhichShippedScenariosCanBeScored` in `lab/internal/scenario`, so fixing
rails or formally dropping it has to come past a test.

The other four scenarios quarantine nothing: 0 of 30, 0 of 38, 0 of 23, 0 of 38.

Reproduce any of it with:

```
sense-lab validate -scenario lab/scenarios/<name>/<name>.yaml
```

Two of the validator's checks report `NOT CHECKED` without a repository checkout
(resolves-at-the-pinned-commit, and the covering-grep free-row flag). Pass
`-checkout` and `-commit` to run them.
