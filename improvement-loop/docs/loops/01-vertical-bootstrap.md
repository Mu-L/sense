# Loop 1 - Vertical bootstrap

> Stand up the scaffold a new stack needs between "Loop 0 picked the stack" and "Loop 2 starts
> admitting repos." This page IS the spec.

## Goal

**Done when `bash bench/drivers/loop1-bootstrap.sh <key> --lang <lang>` returns
`status: BOOTSTRAPPED`** - one goal, one mechanical stop-condition, nothing a session can talk its
way past. The driver runs the whole loop: preflight, stamp, evaluate, record, return.

**Calling contract, so this runs anywhere** (an orchestrator, CI, a headless agent): progress goes
to STDERR and the result JSON is the only thing on STDOUT, so `result=$(loop1-bootstrap.sh ...)` is
the whole integration. Every exit path returns a JSON, including the failures - a caller never has
to parse prose to learn what happened. Loop 1 needs no clone root, no GitHub token and no `sense`
binary; its only environment is this repo and python3, and the preflight says so rather than
failing three steps later.

## Product duties (per Sense surface)

- **graph/blast (scan layer):** the extractor-readiness check IS a product duty, not paperwork:
  confirm `internal/extract/<lang>/` covers the stack's dispatch idioms. A gap found here becomes a
  Loop 7 work item or a resequencing decision, never a "bench around it."
- **conventions:** confirm `detectors_<lang>.go` + the framework model exist for the stack; record
  absence in the conventions ledger as a stack-maturity item, not a bootstrap blocker.
- **setup / tool contracts / response shape:** carry the previous vertical's *misuse lessons* (the
  misuse ledger, [`../goal.md`](../goal.md) sensory system 2) into the arm wiring
  (`bench/drivers/*-run.sh`) and the `sense setup` expectations. Adoption wiring is staged here, not
  debugged in Loop 4.
- **search / status:** none; no agent runs at bootstrap time, so no lever.

## Identity

- **Character:** checklist-convergence. No judgment inside the loop body; the two judgment calls
  (stack slot, extractor readiness) are entry/exit gates, not iterations.
- **Unit of work:** one scaffold element brought to ready. The elements: (1) directory stamp,
  (2) extractor-readiness confirmation, (3) confirmation-arm choice + harness wiring.
- **Position:** consumes Loop 0's stack decision; produces the scaffold Loops 2 and 3 build on.
  **Boundary: Loop 1 never touches repos.** Candidate pools, pinning, cloning and indexing are Loop 2.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | `bash bench/drivers/loop1-bootstrap.sh <key> --lang <lang>` (wraps `new-vertical.sh`) | idempotent; skips every existing file; no model in the path |
| Evaluator | `python3 bench/lib/bootstrap_check.py <key> --lang <lang> --strict` | a separate pass, never the stamping step's self-report |
| Mechanical verifier | the same check, plus `new-vertical.sh` idempotence (a second run creates nothing) | |
| Human | stack confirm (Loop 0 handoff); extractor-readiness verdict when ambiguous | stack confirm is a Loop 0 permanent anchor; the extractor verdict is trust-ledger demotable |

## Stop conditions

- **Success:** the driver returns `BOOTSTRAPPED` because `bootstrap_check.py --strict` exits 0 - `verticals/<key>/` carries
  every stamped element (README tracker, `repos.md` slate, empty `findings/`, `scenarios/`,
  `repos.txt`, `PINNED_COMMITS.json`, `arms.txt`), every path its stamped docs cite resolves, those
  docs hold zero stale previous-stack references, and `internal/extract/<lang>/` exists with
  production files.
- **Budget:** one session. This loop is cheap by construction; multiple sessions means something
  upstream is wrong, usually an extractor gap discovered late.
- **Failure:** the extractor is not ready for the stack. Escalates as a sequencing decision, not a
  bootstrap task: the lane parks until language support lands, or the gap becomes a Loop 7 work item
  first. Verticals are parallel lanes (ruling 2026-07-17): only paid cells block on extractor
  readiness, $0 work in the lane continues. **Loop 1 never builds product code.**

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Stack confirm | entry (Loop 0 handoff) | yes | never (Loop 0 anchor) |
| Extractor verdict | readiness check is ambiguous | yes | after clean history |

## State / memory

- The scaffold itself is the state: `new-vertical.sh` is idempotent and non-destructive, so "where the
  loop is" is readable from what exists on disk.
- No `.loop-state.json` needed; do not add one.
- **Readability duty:** the driver appends `loop1/scaffold` to `verticals/<key>/LEDGER.md` from the
  facts it just measured - stamped elements, extractor files, arms - and skips if the entry is
  already there (write-only for the loop; contract in [`00-ledger.md`](00-ledger.md)). Pass
  `--reason` to state why this vertical was opened; it is the one field a script cannot measure.
  The LESSON field stays empty on a clean walk on purpose: a loop that reports a lesson every time
  it runs is manufacturing them.
- **A lesson from the previous vertical graduates into its durable home** - a prompt, a one-pager, a
  gate, or a memory - at the moment it is learned. There is no staging file for them and none is to be
  added; `STATUS.md` is the one pickup render.

## Un-fakeable check

- File-level facts only, all four inside `bootstrap_check.py`: the stamped tree carries every template
  element, the stale-ref scan returns zero hits under `--strict`, `internal/extract/<lang>/` exists
  with production files, and a re-run of `new-vertical.sh` creates nothing.

## Inputs / outputs

- **Consumes:** Loop 0's stack decision ([`../vertical-program.md`](../vertical-program.md) §Priority).
- **Produces:** `verticals/<key>/` - the whole vertical in one folder: `repos.txt`, `scenarios/`,
  `PINNED_COMMITS.json`, `arms.txt` (membership, pins, and the tools/LLMs - the one place a model id is
  named), a README tracker, a `repos.md` slate, `findings/` with the campaign scorecard, a seeded
  `LEDGER.md`, plus the arm plan (headline Opus 4.8 ×2 + confirmation arms per
  [`../cross-cutting/02-providers.md`](../cross-cutting/02-providers.md)). Stack-agnostic method docs
  are NOT copied here; they live once in [`../scenarios/crafting.md`](../scenarios/crafting.md) and
  [`../findings/`](../findings).

## Fixture test (standalone, $0)

- **Known-answer control, both directions:** `bootstrap_check.py <stamped-key> --lang <lang>` exits 0;
  `bootstrap_check.py not-a-vertical --lang <lang>` exits 1 with
  `structure: no verticals/not-a-vertical/ directory`. A check that only ever passes is not a check.
- **Idempotence:** `new-vertical.sh <existing-key>` must report every element `[skip]` and create
  nothing.
- **Throwaway stamp:** `new-vertical.sh zz-fixture --no-doc`, confirm `scenarios/` + `repos.txt` are
  stamped, re-run for idempotence, then delete `verticals/zz-fixture/`.
- **The dangling-ref check has teeth, both directions:** citing a path that does not exist FAILS;
  citing a real one (e.g. `internal/extract/php/laravel.go`) PASSES.
- **The stale-ref scan has teeth:** plant a previous stack's key in a stamped `repos.md` and
  `--stale <that-key> --strict` FAILS with `stale-refs: repos.md: 1× '<that-key>'`; remove it and the
  check goes green.
- **Every exit path returns JSON:** run the driver with no `--lang`, with a `--lang` that has no
  extractor, and from outside the sense repo. All three print a parseable verdict on stdout and a
  distinct exit code. A loop that fails silently cannot be orchestrated.
- **The record is idempotent:** a second run leaves exactly one `loop1/scaffold` entry.

## Built vs missing

- **Built:** `bench/drivers/loop1-bootstrap.sh` (the loop, one command, JSON return);
  `bench/drivers/new-vertical.sh` (stamp, idempotent, composes every file it writes);
  `bench/lib/bootstrap_check.py` (structure, stale-ref scan - WARN normally, FAIL under `--strict`,
  over the vertical's own stamped docs - and extractor existence); `bench/lib/loop1_ledger.py` (the
  entry, from measured facts).
- **Return values:** `BOOTSTRAPPED` (0), `EXTRACTOR-NOT-READY` (65), `USAGE` (64),
  `PREFLIGHT-FAILED` (70), `STAMP-FAILED` (71), `NOT-BOOTSTRAPPED` (the evaluator's own code),
  `RECORD-FAILED` (72). On success the JSON carries the next loop and its command.
- **Missing:** the arm plan has no mechanical check; it is prose in `verticals/<key>/README.md` that a
  human reads. Small wiring, not design work - if it recurs as a miss, it becomes an enforcement item.
