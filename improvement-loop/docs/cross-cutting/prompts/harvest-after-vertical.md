# Prompt - harvest a finished vertical into the cross-cutting articles

**When to run:** a stack vertical just reached Definition of Done (its per-repo packs are benched and frozen).
This prompt pulls that vertical's cross-stack data points into the four cumulative articles. It is the
**gather/maintain** half of the cross-cutting mechanic (`../README.md`).

**Boundary:** this updates fact packs only. It never writes published prose (that is `social-writing`), and it
never invents a number that is not in a snapshot.

---

## Task

For the vertical `<STACK>` (e.g. `python-django`), whose results live in
`verticals/<STACK>/results/`:

1. **Read the source of truth.** `verticals/<STACK>/results/report.md` (the cross-model matrix) and the
   per-agent boards under `<STACK>/<model>/`. Regenerate them first if stale
   (`bash bench/drivers/report-matrix.sh <STACK>` or the equivalent), so the rows you append are current.

2. **Update BOTH layers** (see `_skeleton.md` - the table is the index, the detail section is the body) in each
   of the five packs:
   - `01-methodology.md` - any new trap / fairness correction / judge issue this stack surfaced.
   - `02-providers.md` - the cheap-arm efficiency signs and any throttle artifact on this stack's cheap arm.
   - `03-cross-model.md` - the per-model overall Δ and deps Δ, with the rigor tier of each arm.
   - `04-harness.md` - any new per-harness adoption fact (wiring, cold start, mcp_count).
   - `05-product.md` - every product byproduct this vertical shipped (resolver/extractor/output fix), with its
     commit and the bench repo that flagged it; note which generalize vs which are stack-specific.

   For each: **append one terse row** to the "data points by vertical" table, AND **add one
   "Per-vertical detail" subsection** holding the scars, specific commits/files, anecdotes, and caveats that
   would be lost if you only kept a row. Never lose a fact (detail section); never bloat the index (table).

   **Plus one destination outside `cross-cutting/`:** `02-bench-harvest.md` (private tree) -
   append whatever this vertical surfaced about `sense_conventions` into its A–D categories (accuracy
   defects, enhancement candidates, new-detector ideas, law-divergent corpus candidates). It lives apart
   because it feeds a build gate and the end-of-program conventions pass, not an article. If the vertical's
   scout-time conventions sweep was skipped, run it now ($0, pinned indexes) before harvesting.

3. **Tag each row** `agrees` / `refines` / `reverses` (see `_skeleton.md`). A **`reverses`** row is the most
   important outcome: foreground it in the detail section AND the prose, never bury it. A finding that survives
   a real counterexample honestly is the credible kind.

4. **Reconcile the prose** above each table if the new row changes the headline (e.g. a reversal, or a
   non-monotonic point that needs a caveat). Keep the across-agents caveat and the rigor tiers intact.

5. **Resolve or carry forward** each article's "Open questions" that this stack answered; add any the stack
   raised.

6. **Report** a short diff summary: which rows were added, any reversal, any open question closed/opened.

## Guardrails (from `_skeleton.md` and the manifesto)

- Provenance, never duplication: link back to `verticals/<STACK>/results/report.md`; no orphan numbers.
- Recall comparable across agents; cost only within one agent.
- Adoption before delta (mcp_count > 0) for any cross-harness claim.
- Tag breadth arms as breadth; only the hardened headline arm carries a headline claim.
- No em-dashes; byline "Luc B. Perussault-Diallo".
