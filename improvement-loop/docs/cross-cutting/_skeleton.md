# Cross-cutting skeleton - conventions unique to cumulative cross-stack packs

> Inherits the full claims/tone discipline from `../findings/pack-contract.md`. This file only adds
> what is specific to articles that span verticals.

## The two layers (this is how we avoid both loss AND pollution)

A single row per vertical is too thin to hold a vertical's worth of insight, but a long prose dump per vertical
would pollute the cross-vertical comparison. So every cross-cutting article carries **two layers**, and the
balance between them is deliberate:

1. **The "data points by vertical" table - the scannable quantitative spine.** One row per finished vertical,
   terse, the numbers only. This is the *index*: it lets a reader compare across stacks at a glance and see at
   once whether a finding holds. Keep it short; it must stay readable as 7+ rows accumulate.
2. **The "Per-vertical detail" section - the unbounded body.** A subsection per vertical *below* the table,
   with **no length limit**: the scars, the specific commits/files, the anecdotes, the caveats, the things that
   would be lost if we only kept a row. This is where insight lives; the table only points at it.

**The rule:** never lose a fact (it goes in the detail section), never pollute the index (the table stays
terse). When a vertical finishes, the harvest prompt adds one table row **and** one detail subsection.

Canonical table shape (adapt per article):

| vertical | when | the stack's data point | agrees / refines / reverses |
|---|---|---|---|
| Ruby/Rails | 2026-06 | <the number(s) from `report.md`> | seed |
| Python/Django | TBD | … | … |

- **`agrees`** - the new stack reproduces the finding. **`refines`** - same direction, new nuance/bound.
  **`reverses`** - the finding does not hold here; this is the most valuable row and must be foregrounded, not
  buried (a finding that survives a reversal honestly is more credible than one that never met a counterexample).

## Provenance line (mandatory, top of each pack)

Each pack states its source snapshots explicitly: `verticals/<stack>/results/report.md` (the cross-model
matrix) and `<stack>/<model>/` (the per-agent boards). No number appears in a pack that is not in one of those.

## The across-agents caveat (carry in every pack that compares models or harnesses)

Two facts must travel with any cross-model or cross-harness claim, every time:
1. **Recall is comparable across agents; cost is NOT.** Report cost only within one agent (harness · model).
2. **The cross-agent win is conditional on adoption.** If a harness does not call the Sense tools, its sense arm
   ≈ its baseline arm. State adoption (mcp_count > 0) before stating a delta.

## Rigor tiers (so a breadth arm is never dressed as the headline)

- **Hardened** = the headline arm at the manifesto Definition of Done (×2, transcript-verified). Only
  this tier carries a headline claim.
- **Breadth** = supplementary non-Opus confirmation arms (open models, the cross-model matrix, ×1). They widen
  the cross-agent evidence and are reported with their honest losses, never as a headline. Tag every breadth
  number as such.
