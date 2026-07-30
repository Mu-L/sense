# Loop 1 - Vertical bootstrap

> **Status: built and exercised.** The scaffold loop: everything between "Loop 0 picked the stack" and
> "Loop 2 starts admitting repos." This page IS the spec: the bootstrap runbook it used to summarise was
> deleted once the generator and this one-pager covered the same ground.

## Goal

Stand up the scaffold a new stack needs before any repo enters the picture. **Done when `python3 bench/lib/bootstrap_check.py <key> --lang <lang> --strict` exits 0** - one goal, one mechanical stop-condition, nothing a session can talk its way past.

## Product duties (per Sense surface)

- **graph/blast (scan layer):** the extractor-readiness check IS a product duty, not paperwork: confirm `internal/extract/<lang>/` covers the stack's dispatch idioms (the seams Loop 2's oracle will measure). A gap found here becomes a Loop 7 work item or a resequencing decision, never a "bench around it."
- **conventions:** confirm `detectors_<lang>.go` + the framework model exist for the stack; record absence in the conventions ledger as a stack-maturity item, not a bootstrap blocker.
- **setup / tool contracts / response shape:** carry the previous vertical's *misuse lessons* (the misuse ledger, [`../goal.md`](../goal.md) sensory system 2) into the arm wiring (`bench/drivers/*-run.sh`) and the `sense setup` expectations. The Codex AGENTS.md lesson is the reference: adoption wiring is staged here, not debugged in Loop 4.
- **search / status:** none; no agent runs at bootstrap time, so no lever.

## Identity

- **Character:** checklist-convergence. No judgment inside the loop body; the two judgment calls (stack slot, extractor readiness) are entry/exit gates, not iterations.
- **Unit of work:** one scaffold element brought to ready. The elements: (1) directory stamp, (2) extractor-readiness confirmation, (3) confirmation-arm choice + harness wiring.
- **Position:** consumes Loop 0's stack decision; produces the scaffold Loop 2 (repo admission) and Loop 3 (scenario authoring) build on. **Boundary: Loop 1 never touches repos.** Candidate pools, pinning, cloning, and indexing are Loop 2, even though the runbook lists them adjacently.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | session agent running `bash bench/drivers/new-vertical.sh <key> --title "<Stack>"` | |
| Evaluator | `python3 bench/lib/bootstrap_check.py <key> --lang <lang> --strict` | a separate pass, never the stamping agent's self-report |
| Mechanical verifier | the same check, plus `new-vertical.sh` idempotence (a second run creates nothing) | |
| Human | stack confirm (Loop 0 handoff); extractor-readiness verdict when ambiguous | stack confirm is Loop 0's permanent gate; the extractor verdict is trust-ledger demotable |

## Stop conditions (all three, explicit)

- **Success:** `bootstrap_check.py <key> --lang <lang> --strict` exits 0 - `verticals/<key>/` carries every
  stamped element (README tracker, repos.md slate, empty `findings/`, `scenarios/`, `repos.txt`,
  `PINNED_COMMITS.json`, `arms.txt`), every path its stamped docs cite resolves, those docs hold zero stale
  previous-stack references, and `internal/extract/<lang>/` exists with production files.
- **Budget:** one session. This loop is cheap by construction; if it is consuming multiple sessions, something upstream is wrong (usually an extractor gap being discovered late).
- **Failure:** the extractor is not ready for the stack. Escalates to the human as a sequencing decision, not a bootstrap task: either the stack's lane parks until the language support lands, or the gap becomes a Loop 7 work item first (since the 2026-07-17 re-ruling, verticals are parallel lanes: only paid cells block on extractor readiness; $0 work in the lane continues). **Loop 1 never builds product code.**

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Stack confirm | entry (Loop 0 handoff; runbook §0) | yes | never (Loop 0 anchor) |
| Extractor verdict | readiness check is ambiguous | yes | after clean history |

## State / memory

- The scaffold itself is the state: `new-vertical.sh` is idempotent and non-destructive (it skips every existing file), so "where the loop is" is readable from what exists on disk.
- No `.loop-state.json` needed; do not add one.
- **Readability duty:** append `loop1/scaffold` to `verticals/<key>/LEDGER.md` at scaffold stamp (write-only for the loop; contract in [`00-ledger.md`](00-ledger.md)).

## Un-fakeable check

- File-level facts only, all four inside `bootstrap_check.py`: the stamped tree carries every template element, the stale-ref scan returns zero hits under `--strict`, `internal/extract/<lang>/` exists with production files, and a re-run of `new-vertical.sh` creates nothing. Nothing here can be gamed by prose.

## Inputs / outputs

- **Consumes:** Loop 0's stack decision ([`../vertical-program.md`](../vertical-program.md) §Priority).
- **Produces:** `verticals/<key>/` - the whole vertical in one folder: `repos.txt`, `scenarios/`, `PINNED_COMMITS.json` and `arms.txt` (membership, pins, and the tools/LLMs - the one place a model id is named), a README tracker, a repos.md slate, and `findings/` with the campaign scorecard. The stack-agnostic method docs are NOT copied here; they live once in [`../scenarios/crafting.md`](../scenarios/crafting.md), [`../findings/workflow.md`](../findings/workflow.md) and [`../findings/`](../findings). Plus the arm plan (headline Opus 4.8 ×2 + confirmation arms per [`../cross-cutting/02-providers.md`](../cross-cutting/02-providers.md)); a seeded `LEDGER.md`.

## Fixture test (standalone, $0)

- **Known-answer control, both directions** (run 2026-07-29): `bootstrap_check.py php-laravel --lang php` exits 0 on the stamped vertical; `bootstrap_check.py not-a-vertical --lang php` exits 1 with `structure: no verticals/not-a-vertical/ directory`. A check that only ever passes is not a check.
- **Idempotence:** `new-vertical.sh <existing-key>` must report every element `[skip]` and create nothing.
- **Throwaway stamp:** `new-vertical.sh zz-fixture --no-doc`, confirm `scenarios/` + `repos.txt` are stamped, re-run for idempotence, then delete `verticals/zz-fixture/`.
- **The dangling-ref check has teeth, both directions** (run 2026-07-29): citing
  `bench/drivers/provision-repos.sh` or `verticals/<key>/PINNED_COMMITS.json` FAILS; citing
  `internal/extract/php/laravel.go` or `bench/drivers/provision-repos.sh` PASSES.
- **The stale-ref scan has teeth** (run 2026-07-29): plant `ruby-rails` in a stamped `repos.md`, and `--stale ruby-rails --strict` FAILS with `stale-refs: repos.md: 1× 'ruby-rails'`; remove it
  and the check goes green again.

## Built vs missing

- **Built:** `bench/drivers/new-vertical.sh` (stamp, idempotent, composes every file it writes); `bench/lib/bootstrap_check.py` (the evaluator: structure, stale-ref scan - WARN normally, FAIL under `--strict`, over the vertical's own stamped docs - and extractor existence).
- **Missing:** the arm plan has no mechanical check; it is prose in `verticals/<key>/README.md` and a human reads it. Small wiring, not design work - if it recurs as a miss, it becomes an enforcement item.
- **First live use:** the go vertical (slot 3), then php-laravel, re-stamped clean in `improvement-loop/` on 2026-07-29 and green on the check.

---

**Two things this loop used to carry and no longer does.** It had a second exit condition - an empty `carry-forward.md` - and a fifth work element, graduating that file's staged items. `carry-forward.md` was retired as a live surface (`STATUS.md` is the one pickup render), so both are gone and the `bootstrap_check` audit for it was removed with them. A lesson from the previous vertical now graduates into the durable home it belongs to (a prompt, a one-pager, a gate, or a memory) at the moment it is learned, not into a staging file that the next vertical has to drain.
