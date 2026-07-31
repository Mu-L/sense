# Loop 4 - Matrix fill

> **Status: defined 2026-07-11; scripts built, scheduling manual.** Fills the confirmation half of the
> anchor matrix (repos × tools × LLMs): every non-headline arm, ×1, on all 4 repos. The headline Opus ×2
> belongs to the per-repo loops; this loop never produces a verdict, it produces the cross-model / cross-harness
> confirmation rows. Its binding constraint is provider weekly caps, not work or intelligence: it is a
> scheduler fighting quotas.

## Goal

Fill every confirmation-arm × repo cell honestly under provider quota constraints, so the vertical's
verdict generalizes across the tool/model matrix instead of resting on one cell. Exit state: every cell
filled or explicitly marked degraded/OPEN, and the cross-model / harness rows fillable from disk.

## Product duties (per Sense surface)

- **setup / tool contracts / response shape:** the confirmation arms are the adoption stress test. Each
  arm×harness pair reveals which setup path, contract text, or hint failed a weaker model (Codex
  AGENTS.md; opencode MCP registration + cold start). The misuse audit runs per arm and is compared
  ACROSS arms: a misuse only weak models make is a hint/setup gap; one every model makes is a contract
  defect. This cross-arm comparison is the one product analysis only this loop can produce.
- **All five tools:** `sense-io.jsonl` capture runs on every arm's cells, same as the headline arm; a
  confirmation cell without capture starves Loop 5's mining.
- **Others:** none beyond the above; this loop schedules and captures, it does not analyze (its captures
  feed Loop 5).

## Identity

- **Character:** scheduler. No judgment inside the loop body; the design is entirely policy (ordering,
  caps, anomaly handling) plus honest bookkeeping.
- **Unit of work:** one cell = one confirmation arm × one repo, both toolsets, ×1, on the frozen scenario.
- **Position:** consumes the per-repo loops' banked wins (a cell runs only on a FROZEN scenario, post-WIN; running on
  a draft wastes quota because a re-author invalidates the cell); produces the filled matrix consumed by
  Loop 5's harvest and Loop 6's cross-model / harness fact-pack rows.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | **The default confirmation driver is `bench/drivers/sweep-breadth.sh`** - one per confirmation LLM, launched in parallel. It walks breadth-first (PASS 1 = run-1 for every repo, PASS 2 = run-2, additive via `KEEP_RUNS=1`) and holds a per-provider lock so same-provider arms serialize while different providers run concurrently. Model-id routed (`claude-*` → bench-sense-local, `gpt-*` → codex-run, opencode providers → opencode-run). `sweep.sh` (depth-first ×1) and `sweep-resume.sh` are NOT the default - reach for them only for a one-off single cell or an explicit resume. This is the CONFIRMATION path only: the headline Opus ×2 belongs to Loop 2 (`runs-variance.sh RUNS=2`), and no breadth cell runs until that win freezes. | |
| Evaluator | the anomaly rule (below) + adoption/contamination checks per cell | mechanical, no adversarial prompt needed |
| Mechanical verifier | `mcp_count` per cell, contamination guard + offload gate in `opencode-run.sh`, `--strict-mcp-config`, `report-matrix.sh` rendering from disk | |
| Human | cap/budget policy set once per vertical; arm-degraded verdicts | no per-cell gate |

The post-run agent survey (`08-agent-survey.md`) fires on **all three runners** - `bench-sense-local.sh`
(incl. ollama provider), `codex-run.sh` (`codex exec resume`), `opencode-run.sh` (`opencode run -s`) -
each normalizing through its own parser into the canonical shape `survey_verify.py` reads. Two per-arm
guards: the survey turn always runs AFTER the answer is banked and can only fail itself, never the run
(matters on quota-fragile opencode windows); opencode surveys fire only on `hclass=ok` cells, so
cap/truncation/offload runs are never surveyed.

## Scheduling policy (the design piece)

1. **Every arm covers all 4 repos** (no asymmetric coverage); ×1 each, always OPEN-flagged (the RUNS=2 law
   belongs to the headline arm only).
2. **Cap-aware ordering:** small/medium cells first, big repos last and ≤2 per arm-week (an Ollama big-repo
   run burns ~34% of the weekly quota; same-provider ids - Qwen + Devstral, or GLM + Mistral - share one
   Ollama limit and count against it as one pool). Concurrency across providers is now mechanically safe:
   the default `sweep-breadth.sh` (and `sweep.sh`) holds a per-provider lock (`provider_of` →
   `pace_provider_lock_acquire`) for a model's whole sweep, so two same-provider sweeps serialize while
   different providers run in parallel - this stops same-provider runs from truncating each other on the
   rolling window, but it does NOT create weekly budget, so the ≤2-big-repos-per-week cap still applies to
   the pool, not per-id.
3. **Adoption is earned before the first cell, not debugged after:** `sense setup --tools <harness>` per
   arm (Codex requires `AGENTS.md`; opencode requires MCP registration + a warm cold start). An
   adoption-zero cell is a wiring failure, not a data point: fix the wiring, re-run the cell.
4. **Cap hit → durable handoff:** a `provider_cap_error` / `*_session_failed` / watchdog-stall flag (written
   to the cell's `run_meta.json`, and never scored as a 0.0) makes `sweep-breadth.sh` **stop the pass
   cleanly** - completed runs are kept, nothing is wiped, and re-running the same command skips valid cells
   and retries the capped one. Park, resume next window; write the exact resume command into that arm's
   matching prompt file (`sweep-resume.sh` for single-cell resume). Session vs weekly is not auto-classified
   (both read as "re-run after reset"), and Kimi raises no clean cap flag - lean on the pacing cooldown + a
   budget read there. Never idle-poll a quota.
5. **Anomaly rule:** a single weird cell (a sudden loss or collapse out of family with the arm's other
   cells) is presumed degraded-window variance → **single-cell re-run** in a clean window. Never
   `rm -rf` a run directory to "redo" a cell (the KEEP_RUNS=0 incident deleted both arms of clean data);
   re-runs add, deletion is manual and human-approved only.
6. **Throttle honesty:** a weak subscription produces false LOSSES, not noise. If an arm's window is
   degraded and a re-run confirms throttling rather than signal, the cell is marked degraded/OPEN and the
   arm's row says so; publishing a throttled loss as a product result is forbidden (adapt the bench to weak
   subscriptions, never conclude from them).
7. **Status reporting:** the per-arm table format, always: Done | In flight | Left per arm, plus the repo
   queue. No prose status.

## Stop conditions

- **Success:** all arm×repo cells filled or explicitly marked degraded/OPEN, anomalies re-run once,
  `report-matrix.sh` regenerated from disk, cross-model and harness fact-pack rows fillable.
- **Budget:** weekly caps. The loop parks per arm (rule 4) and resumes; a vertical's matrix is expected to
  take multiple cap-weeks and that is schedule, not failure. Relaxing any cap or ceiling first discharges
  the Class-2 protocol (knob-first, [`decision-errors.md`](../decision-errors.md)); the knob is never the
  first lever.
- **Failure:** an arm cannot produce an honest cell (persistent throttling, harness cannot adopt) →
  escalate: drop or defer the arm for this vertical with the reason recorded in its row. The loop never
  fills a cell with a number it does not believe.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Cap/budget policy | once, at vertical start (which arms, which subs, weekly ceilings) | yes | after clean history |
| Arm-degraded verdict | rule 6 confirms throttling over signal | yes, async | after clean history |

No per-cell gates: cells are ×1 confirmations on already-approved scenarios with already-approved spend
policy. This is the most demotable loop in the registry.

## State / memory

- The results tree itself (per arm/repo run dirs) is the state; `report-matrix.sh` renders the matrix from
  disk, so "where the loop is" is always recomputable and never trusted from a status file.
- Cap handoffs live in the arm's matching prompt file (state + resume command), the durable-handoff rule.
- **Readability duty:** append `loop4/<arm>/{done,parked}` entries (per arm, never per cell) to
  `verticals/<vertical>/LEDGER.md`; `STATUS.md` stays a render (`render-status.sh`), never a source,
  which preserves the rule above (contract in [`ledger.md`](ledger.md)).

## Un-fakeable check

- Per cell: `mcp_count` (adoption), contamination guard (no MCP leak into baseline, `--strict-mcp-config`
  verified), billed tokens recorded. Per loop: the matrix is regenerated from disk artifacts, never
  hand-edited. A cell without its transcript does not exist.

## Inputs / outputs

- **Consumes:** the per-repo loops' frozen, won scenarios + the pinned indexes; the arm plan from bootstrap; provider
  quota windows.
- **Produces:** the filled confirmation matrix (rows for `03-cross-model.md` / `04-harness.md`), per-arm
  boards, billed-token records for the efficiency-at-parity reporting.

## Fixture test (standalone, $0)

- **Rendering:** `report-matrix.sh` over the frozen python-django results must reproduce the published
  matrix exactly.
- **Contamination:** feed the guard a transcript with a leaked global MCP server → must flag (the
  bench-sense-local leak incident is the reference case).
- **Adoption-zero:** the Codex-without-`AGENTS.md` runs must be classified wiring-failure, not loss.
- **Anomaly rule:** replay the Qwen session-2 board → the rule must select single-cell re-runs for the
  out-of-family cells and leave the clean cells untouched.

## Built vs missing

- **Built:** `sweep-breadth.sh` (the default), `sweep.sh` / `sweep-resume.sh` (single-cell / resume) - idempotent, model-id dispatch,
  `codex-run.sh`, `opencode-run.sh` (offload gate + contamination guard), `report-matrix.sh`, the per-arm
  status format, the durable-handoff pattern. The full Django matrix (5 arms × 6 repos) ran on these.
- **Missing → BUILT 2026-07-11:** the `matrix-plan.yaml` format exists
  with the python-django plan back-filled as the worked example (arms, runners, the ollama-weekly
  shared pool with the ~34%/big-run note, adoption preflights, prompt files, repo tiers). Go's plan needs
  the owner's arm/ceiling decisions (the standing open question). `sweep-resume.sh` reading it is optional
  later wiring; the file already makes rule 2 checkable from disk. The two opencode-run.sh hardening
  fixes are COMMITTED (475d3ff, 2026-07-11).
- **First live use:** the Go vertical's confirmation arms, after its first per-repo wins bank.
