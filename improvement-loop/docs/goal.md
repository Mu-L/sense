# The goal above the loops

> **What this is.** The program goal every loop answers to. The loops automate the bench; this file states
> what the bench is FOR, so the automation never optimizes the instrument and forgets the purpose.
> Authorities it restates, not replaces: the product thesis (the private product definition: Sense's consumer is the AI;
> north star = send the RIGHT info) and the manifesto (bench law). Added 2026-07-11 from the owner's review
> feedback on the first one-pager set.

## The goal

**Every vertical must leave Sense measurably better as the LLM's companion, on the benched stack and
globally.** The WIN (discriminator ≥ +0.50 on Opus ×2) is the *evidence* that Sense helps; it is not the
point. A vertical that banks six wins and teaches the product nothing is a failed vertical wearing a green
scoreboard.

A vertical's products, in value order:

1. **A better Sense**: resolution gaps fixed, misuse-inducing surfaces corrected, per-stack extraction and
   detectors matured (the Loop 5 → Loop 7 pipe, widened below).
2. **Proof it helps**: the wins, honestly measured (Loops 3/4).
3. **The story**: the packs (Loop 6).

## The surfaces ("better" means one of these, named)

Every duty, ledger entry, and fix names its surface. The five tools: **graph** (edge coverage on the
stack's dispatch idioms, confidence honesty, fold/dedup correctness), **blast** (anchor resolution, budget
trimming: does `ApplyBlastBudget` drop what the agent needed?, determinism), **search** (NL retrieval and
ranking), **conventions** (detector accuracy/noise, per-stack detectors, the future law contract),
**status** (freshness/coverage legibility to the agent). Plus the meta-surfaces, where misuse is born:
**setup/onboarding** (`sense setup` per client, AGENTS.md), **tool contracts** (schemas, descriptions,
defaults: the blast `min_confidence` 0.7-vs-0.3 contract bug is the canonical misuse-inducing defect), and
**response shape** (hints, empty-result guidance, error text, per-tool budgets).

## The three sensory systems (how the loops see product truth)

1. **Resolution gaps** (BUILT): `resolve_oracle.py` + `transcript_miss.py` → `loopA-gaps/<stack>.md`.
   Truth is a fact: this edge exists in code, Sense misses it.
2. **Misuse** (BUILT 2026-07-11): `bench/lib/tool_use_audit.py` over full request/response captures
   (degraded `--from-transcript` mode for pre-capture runs) → the misuse ledger. Truth is a transcript
   fact: wrong tool for the question shape, wrong params, abandoned after one empty result, ignored hints.
   The lever is usually a product meta-surface (contract, hint, setup), not the agent. Detectors:
   contract-misled (with the misled-vs-deliberate discriminator), ignored-hint, abandoned-on-empty,
   wrong-tool-shape; fixtures in `test_tool_use_audit.py`. First real catches on the frozen boards:
   a cold `min_confidence=0.6` blast on chatwoot (schema-steered), and litellm's
   `low_confidence_hidden` hint never acted on - both response-shape/contract levers.
3. **Loss anatomy** (TO BUILD): one distilled row per recorded tie/loss stating the transcript-level reason
   grep won (window-batching, covering declaration, memorized API, satisficing shape). Feeds Loop 2's bar 3
   so admission recalibrates from every vertical's losses, not from a one-time backtest.

Plus one coverage check (BUILT: `tool_use_audit.py --coverage`): per vertical, which surfaces the paid
transcripts actually exercised, at what quality (calls, empties, errors, bytes, params seen per tool).
Graph and blast dominate the win axes today; search, conventions, and status ride along unpressured. An
unexercised surface is an unimproved surface; the check makes that visible instead of silent.

**The enabler for 2 and 3: full Sense I/O capture - BUILT 2026-07-11.** Every runner tees each MCP
request + complete response to `results/<run>/sense-io.jsonl` via `bench/lib/mcp_tee.py`, a
byte-transparent, fail-open stdio shim that exists only where a runner names it (zero product change;
`SENSE_IO_CAPTURE=0` reverts). Acceptance test: `bench/lib/test_mcp_tee.py`.

## Discipline boundary (where a finding may act)

- **Product surfaces** (everything above): findings park to the Loop 7 window, always. Note the trap:
  query-layer fixes (contracts, hints, response shape) take effect IMMEDIATELY on existing indexes, which
  makes them MORE dangerous mid-vertical, not less; they change agent behavior under frozen numbers.
- **Bench harness** (arm prompts, run scripts, capture, scoring): may move mid-campaign with the usual
  re-run discipline. It is instrumentation, not product. When a harness compensation masks a product cause
  (the opencode offload gates), the product cause is still ledgered; compensation without the ledger entry
  loses the signal.

## Per-loop enforcement

Every one-pager carries a **Product duties (per Sense surface)** section: what that loop owes the product.
A loop with no duty says "none" explicitly, with why. Duties are review items at the loop's human events;
a skipped duty is a finding, not a style choice.

## The bench-gate principle (the organizing rule)

**Nothing ships in Sense without a bench proving it helps the AI.** A **stack-specific** change (a Ruby
refiner, a Django resolver) is gated by that vertical's bench; a **cross-cutting** change (output budgets,
ranking, a generic resolver) is gated by a mix of cross-stack repos across the tool/model matrix (the
held-out anchor). "Proving it helps" means improving the value metric on the targeted slice **without
regressing** the cross-stack anchor or side-effect repos of all sizes (manifesto §12). Same shape as a test
suite, except the test is a differential bench (sense vs baseline) and the assertion is a value lift, not an
exact output.

**The anchor is a MATRIX, not a repo set.** The unit is repos × tools × LLMs. A Sense change does not help
"the AI" in the abstract; it helps some agents on some models, and the cheap-arm law
([`cross-cutting/02-providers.md`](cross-cutting/02-providers.md)) plus per-harness adoption
([`cross-cutting/04-harness.md`](cross-cutting/04-harness.md)) mean a change can lift the frontier arm and do
nothing on a throttled open arm. A fix proven only on one harness × one model is proven on one cell.

## Auto-improvement: which half is live, which is fenced

Two loops hide inside "auto-improve", and conflating them is the trap:

- **Loop A - auto-improve the PRODUCT** (resolver, extractors, output budgets). Safe to automate, because
  its truth does not depend on a judge: *"this edge exists in the code; does Sense resolve it?"* is a fact,
  checkable mechanically. This is the near-term investment, and it is what the three sensory systems above
  feed.
- **Loop B - auto-generate winning SCENARIOS / auto-diagnose ties.** The dangerous loop. If an LLM authors
  the scenario, an LLM judge grades it, and an LLM diagnoses the tie, the result is a closed loop optimizing
  for a flattering number - exactly what manifesto Prime Directives 1/3/4 exist to prevent. **Stays
  human-anchored, by design.** "The vertical is not fully automatable" is a fact, not a gap.

The **sensory half** is live or buildable now for $0 (the oracle, `transcript_miss.py`, the ledgers, the
misuse audit, loss anatomy, feature coverage). The **motor half** (propose → re-bench against a frozen
cross-stack anchor → accept-on-lift) stays fenced: do not attempt it piecemeal (Loop 7's rule). Building the
sensory systems IS the preparation - the anchor corpus and the gap language it will need are exactly these
ledgers. The target is **few human interactions over time, never zero**; "zero human" is the wrong goal and a
regression in trust.
