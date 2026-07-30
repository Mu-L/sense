# Cross-Cutting Journey - the articles that span verticals

> **What this is.** The home for the insights that are NOT tied to one stack vertical and that **accumulate
> across every vertical we run**: how we benchmark honestly (methodology), how providers/subscriptions behave
> (the cheap-arm law), how the value scales across LLMs, and how each agent harness adopts Sense. The Rails
> vertical seeded these; Django, Laravel, and every vertical after add a data point to the same four articles.
>
> Per-vertical evidence (the per-repo teardowns, the campaign scorecard) lives under that vertical's folder
> (e.g. `verticals/<key>/findings/`). This folder holds the evidence that is **true across stacks**.

## Why this folder exists (the one principle that shapes everything here)

A per-repo teardown **freezes** once the repo is benched. These four articles are the opposite: **living and
cumulative.** Each carries a "data points by vertical" table that grows by one row every time a vertical
finishes. That is precisely why they cannot live inside any single vertical's folder, and why they sit at the
`improvement-loop/docs/` level next to the manifesto and the program.

## The five articles

| # | Article | The finding | Seeded by |
|---|---|---|---|
| 01 | [`01-methodology.md`](01-methodology.md) | How to benchmark a dev tool for AI agents honestly (the judge is blind to omission; you are the baseline; a tie is a missing axis). | Rails |
| 02 | [`02-providers.md`](02-providers.md) | The cheap-arm law: a weak result is a channel/subscription artifact, not a model verdict. | Rails (Qwen↔Kimi) |
| 03 | [`03-cross-model.md`](03-cross-model.md) | The map's value scales with the model: the frontier model extracts the most from the same local index. | Rails (Opus/Qwen/Kimi) |
| 04 | [`04-harness.md`](04-harness.md) | Adoption is earned per harness: Sense rides MCP into any agent, but each harness has to be wired to call it. | Rails (Claude Code/Codex/OpenCode) |
| 05 | [`05-product.md`](05-product.md) | What the bench bought Sense: the product fixes/features each vertical surfaces (the byproduct that pays for itself). | Rails (PRs #136–#161) |

Each is a **fact pack** (insights + numbers + caveats), not finished prose. The prose is drafted downstream.

**Each pack has two layers** (see [`_skeleton.md`](_skeleton.md)): a terse **table** (one row per vertical, the
scannable index) and an unbounded **per-vertical detail** section (the body, where no insight is lost). The
table prevents pollution; the detail section prevents loss.

## The mechanic - create, gather, maintain

1. **Create / seed** (once): the five packs above + `_skeleton.md`. For a deep one-shot population from the
   existing back-catalogue (memories, commit history, the manifesto, the bench results), run
2. **Gather** (per vertical, when it finishes): run [`prompts/harvest-after-vertical.md`](prompts/harvest-after-vertical.md).
   It appends one row to each pack's table AND a per-vertical detail subsection, from that vertical's
   `verticals/<stack>/results/report.md`, noting any new caveat or reversal the new stack surfaced.
3. **Maintain**: when the numbers move (re-bench, re-judge, re-scan), the same harvest prompt reconciles the
   rows. Numbers only ever come from the snapshots; this folder never holds a number that isn't traceable.

## Two hard rules (so it does not rot)

- **Provenance, never duplication.** Per-vertical numbers live in `verticals/<stack>/results/report.md`.
  These articles **aggregate across stacks and link back**. No number lives here that isn't in a snapshot.
- **Handoff boundary (same as manifesto §13).** This folder holds **facts and insights**. The published prose
  is drafted in `/Users/luc/Documents/Writing/social-writing` by a separate session. A bench/vertical session
  **never writes to `social-writing`**; it keeps these packs organized and hands them off.

## Claims discipline

The full claims/tone discipline is shared with the per-vertical articles and lives in
`../findings/pack-contract.md` (lead with the finding; measure the thesis, never assert it; no
em-dashes; byline "Luc B. Perussault-Diallo"). This folder's `_skeleton.md` only adds the conventions unique to
cumulative cross-stack packs.
