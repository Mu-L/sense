# Loop 7 - Product-fix window

> **Status: defined 2026-07-11; detection half built, propose half manual.** The only loop that touches
> Sense's code. It runs in the seam between verticals: after the publish sign-off closes vertical N, before bootstrap
> builds vertical N+1's indexes. That placement is law, not convenience: a mid-vertical product fix
> invalidates frozen numbers (manifesto §12), and the next vertical's index build freezes the binary again.
> This is Loop A in the endgame's terms, and its safety property is that its truth is a **fact**, never a
> judgment: "this edge exists in the code; does Sense resolve it?"

## Goal

Make Sense factually better between verticals: take verified gaps from the ledgers through repro → fix →
bench gate, and ship what proves value or revert cleanly, without ever destabilizing a running campaign's
frozen numbers or denaturing the product. Exit state: the window's worklist is shipped-or-reverted and the
re-index flag is set for the next bootstrap if owed.

## Product duties (per Sense surface)

This loop IS the product duty; its addition from `goal.md` is that the worklist is **bucketed per
surface and channel**, and the window triages ALL buckets, not just resolution:

- **Resolution facts** (graph/blast/conventions accuracy): oracle-repro'd edges and detector defects; the
  classic lane, unchanged.
- **Misuse-inducing surfaces** (tool contracts, defaults, hints, empty-result guidance, setup): sourced
  from the misuse ledger. The repro for these is the transcript pattern + a before/after probe on the
  pinned index, still a fact, never taste. The blast `min_confidence` contract bug is the reference: a
  schema that misleads the LLM is a product defect of the same rank as a missed edge.
- **Per-stack extraction maturity:** thin-seam patterns flagged by the admission screens.
- **Query-layer caution, doubled for misuse fixes:** contract/hint changes take effect immediately and
  steer agent behavior directly; they are the cheapest fixes and the most regression-prone, so the
  cross-vertical no-regress gate (the global-change law) applies with no small-fix exemption.
- **Scheduled window items:** the conventions §8 local-law build-gate bench is a named item of the
  pre-Laravel window (ownership table in `README.md`).

## Identity

- **Character:** product-touching. Everything here inherits the full production discipline (CLAUDE.md,
  coverage floor, complexity ledger, per-language namespacing, NON-GOALS) on top of the bench law.
- **Unit of work:** one verified gap taken through: re-assess factual in current `main` → spike or fix →
  bench gate → ship or revert.
- **Position:** consumes Loop 5's ledgers (`verticals/<stack>/results/loopA-gaps.md`, the conventions ledger's graduated
  pitches, carry-forward D items, and the survey friction ledger's **filed** rows -
  `../FRICTION.md`, process `08-agent-survey.md`; the window records their
  shipped/killed exit on the row); produces a better binary for the next vertical and the re-index flag
  for bootstrap. **Hard deadline:** the window closes when bootstrap starts indexing; anything unfinished parks
  to the next window, it does not slip mid-vertical.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | session agent: repro test, fix, bench runs | |
| Evaluator | the bench gate (the "help the AI" definition) + council review, TWO passes: proposal (pre-code) and finished diff (pre-merge) | value proven, never asserted |
| Mechanical verifier | the failing test that reproduces the gap; `pergroup.py` deltas on real runs; `make ci`; the oracle re-run on the fixed binary | |
| Human | window worklist selection; council + PR merge; re-bench spend | all permanent while this loop ships code |

## The per-gap pipeline (the design piece)

0. **Window-open intake (once per window, not per gap):** re-read
   [`decision-errors.md`](../decision-errors.md); cluster new incidents into classes, tighten the protocols,
   and record the pass even when empty (the auto-improvement intake law).
1. **Re-assess factual in current `main`** (§12(a)). The ledger entry was true when banked; verify it
   still is (the oracle re-runs for $0) before spending anything.
2. **Council review of the PROPOSED fix, before any code (added 2026-07-12, the owner's round-2 review).**
   The proposal goes through `/council` before work starts: the gap, the intended approach, the layer it
   touches, its blast radius (verticals affected, per the global-change law), and the identity-check
   verdict. This is the cheap point to catch a fix that would denature the product or trade one gap for
   two side effects; the merge-time pass (step 8) cannot redirect a wrong approach cheaply, this one can.
   No council-reviewed proposal, no branch.
3. **Write the failing test first.** The gap is a fact, so it has a repro: an edge that exists in code
   that Sense does not resolve. No repro test, no fix.
4. **Spike method for uncertain fixes:** prove value or revert, ≤3 iterations, never skip council. The
   record shows both exits are real (L2/L2b killed; the pretix blast-cap commit cut via byte-diff; versus
   L1 dedup shipped at −19% billed at parity).
5. **Bench gate, scoped by blast radius:** a stack-specific fix is gated by that vertical's bench; a
   cross-cutting / query-layer change takes a **no-regress bench on EVERY vertical it touches** (the
   global-change law), plus side-effect checks across repo sizes (§12(b)). RUN the bench, don't reason
   about it.
6. **Layer rule on ship:** a scan/resolve/extract-layer fix sets the re-index flag bootstrap owns (the
   affected slate indexes rebuild before the next authoritative sweep); an `mcpio`/query-layer fix takes
   effect on existing indexes immediately.

   **Scope the rebuild to what the fix touched.** A per-language extractor change does not invalidate
   the other languages' indexes, and proving that is cheap: the php holder-composition fix was verified
   PHP-only by rescanning a Go, a Ruby and a Python repo and finding byte-identical edge counts. So the
   flag means `VERTICALS="php-laravel" bash bench/drivers/rescan-all.sh`, or
   `bash bench/lib/ensure-index.sh <repo>` for a single index - not the bare driver, which rebuilds and
   re-embeds every repo of every vertical and costs hours to refresh indexes the change cannot reach.
   Use `--check` first when unsure what is actually stale.
7. **Identity check throughout:** the fix must not denature the product. Feature-complete v1, three lanes,
   no config knobs, read-only, no fifth tool; conventions ideas additionally pass the irreplaceability
   test. A gap whose only fix violates these is recorded as out-of-scope, not forced.
7b. **Post-implement mechanical gate (the owner, 2026-07-12 - runs after EVERY implementation, BEFORE the
   council's diff review):** (i) `make ci` full green (build + cover + lint + ledger); (ii) coverage
   **above 94%** for every file AND function created or modified by the change (stricter than the
   repo-wide 92% floor, applied to the touched set); (iii) **qlty code coverage green**. The council
   reviews only diffs that have already cleared these bars - machine checks are not the council's job.
7c. **Side-effect RUN, mandatory before ANY PR (the owner, 2026-07-13):** one temp single sense-arm run
   per benched vertical (the smallest repo with a frozen cell), on the branch binary, into a scratch
   `RESULTS_DIR` (frozen runs untouched), compared against the frozen cell's adoption + cited-recall.
   Rebuild those repos' indexes with the branch binary first when the change could touch them
   (`sense scan -rebuild -embed`; clone-hygiene check before the rescan). A $0 structural rescan-diff
   does NOT discharge this - RUN, don't reason (the global-change law). Probe-sized spend is
   standing-authorized by this step.
7d. **Class-6 gate - the records half of 7b** (an available check left unrun, ratified 2026-07-15;
   [`decision-errors.md`](../decision-errors.md)). 7b machine-checks the CODE; nothing was machine-checking
   the RECORD, and the record is what the next session inherits. Before this window is claimed recorded:
   (i) **any contracted surface gets its contract read and its checker RUN** - `ledger_check.py` for the
   LEDGER, the loader/oracle for scenario+rubric, `scaffold_check.py` for scaffold; "it's written" is
   not "it's recorded"; (ii) **any instrument whose output becomes evidence** (probe scripts, classifiers,
   gate loops) is first run against a **known-answer control** - one row known true, one known false - a
   tool that cannot fail visibly cannot be trusted quietly; (iii) **recorded harness lessons are
   pre-flight** (macOS `timeout`, subshell PATH). Costs seconds; the 2026-07-15 precedent is a
   PATH-stripped audit loop that printed the exact reverse of its finding and a ledger that failed its
   own checker on eight counts.

7e. **The PR body states where the gap came from.** Every PR out of this window opens with a line
   naming its origin:

   > Found through the Sense Improvement Loop while benching `<vertical stack>` with `<repo>`.

   Not decoration. A product fix from this loop is evidence that the bench found something a human
   reading the code did not, and that provenance is the loop's whole claim to exist. Six months on, the
   difference between "someone noticed a missing php case" and "benching laravel surfaced a channel the
   contract promised and never filled" is the difference between a tidy-up and a working instrument.
   It also hands a reviewer who was not in the session the one piece of context a diff cannot carry.

8. **Council review of the FINISHED code, at merge.** The second `/council` pass reviews the diff itself
   for new-bug introduction and side effects before the PR merges (the existing Council + PR review
   event). Two passes, two questions: step 2 asks "is this the right change to make?", this one asks
   "did the change introduce what it should and nothing else?". Neither substitutes for the other, and
   neither ever demotes.

## Stop conditions

- **Success:** the window's authorized worklist is shipped-or-reverted, each shipped fix bench-proven and
  merged; the ledger entries close with their PR links; the re-index flag is set if owed.
- **Budget:** the window's calendar (bounded by the next vertical's start) + per-fix spike caps (≤3
  iterations) + re-bench spend authorization.
- **Failure (per gap):** the fix cannot prove value → **revert fully** and record the attempt in the
  ledger entry; the gap stays open, the codebase stays clean. A parked gap is normal; a half-shipped fix
  is not.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Window worklist | window opens; ledger triaged into authorized fixes | yes | never (spend + scope) |
| Council: fix proposal | a gap's fix approach is drafted, BEFORE any code (pipeline step 2) | yes | never (shipping code) |
| Council + PR review | a fix is ready to merge (pipeline step 8) | yes | never (shipping code) |
| Re-bench spend | bench gate needs paid runs | yes | never (spend anchor) |

This loop's gates never demote while it ships product code; the trust ledger applies to bench-side loops,
not to Sense's `main`.

## State / memory

- `verticals/<stack>/results/loopA-gaps.md` entries (open → closed-with-PR, or attempt-recorded).
- Branches per fix (`fix/<slug>`) or enhancement (`enhance/<slug>`); conventional commits (a real defect
  is `fix`, better output of an existing surface is `enhance`, both patch; new capability is `feat`;
  bench-only work is `bench(<scope>)`), `git-cliff --bumped-version` before PR.
- The current live window (pre-Go, worklist authorized 2026-07-12) is the concrete example: the
  fold-gate fix turned out ALREADY MERGED (PR #192, v1.11.17; the "awaiting PR" state was stale, caught
  by the bar-4 investigation), the blast `min_confidence` contract fix is the window's one authorized
  product item, and the NetBox-flagged fixes + loopA-gaps sweep are explicitly PARKED to the next
  window.
- **Readability duty:** append `loop7/<window>/{open,<fix>-shipped,<fix>-reverted,close}` entries to
  `verticals/<vertical>/LEDGER.md`, shipped fixes with before/after incl. side-effect benches
  (write-only for the loop; contract in [`ledger.md`](ledger.md)).

## Un-fakeable check

- Three, stacked: the repro test (red on old binary, green on new), the bench delta (`pergroup.py` on real
  transcripts, no-regress across touched verticals), and `make ci` (coverage floor, ledger, hermetic
  boundaries). A fix that cannot go red-then-green on a repro is not fixing a Loop-A fact.
- For misuse-surface fixes the repro is the before/after probe: the old contract/hint produces the misuse
  pattern in the ledgered transcripts, the new one demonstrably cannot (probe on the pinned index, plus the
  no-regress bench). Still red-then-green, just at the MCP boundary instead of a unit test.

## Inputs / outputs

- **Consumes:** verified gap ledgers (Loop 5, resolution + misuse categories), pitches graduated from the
  conventions ledger, carry-forward D items, spend authorization.
- **Produces:** merged fixes + version bumps, closed ledger entries, the re-index flag for bootstrap, revert
  records (as valuable as ships: they close dead ends permanently), and - **decided 2026-07-30** - the
  **probe-expiry trigger**: every shipped fix marks every `loop3/<repo>/probe` verdict measured on the
  previous Sense version as STALE, so [Diagnosis](03-repo-diagnosis.md) re-verifies instead of standing on
  an expired kill. A fix that makes a dead cell live is a *success* of this loop, and the only way the
  program notices is if this loop invalidates the old measurement. A revert fires the trigger too: it also
  moves the version.

## Fixture test (standalone, $0)

- **Replay reverse-FK** (the reference window): the oracle fails on the pre-fix binary against the pinned
  saleor index and passes on the fixed one; the no-regress bench then read parity (the cycle-30
  precedent). Any new window's tooling must be able to reproduce that arc on the archived artifacts.
- The revert lane's fixture is the byte-diff: a cut commit leaves the tree byte-identical to before the
  spike.

## Built vs missing

- **Built:** the detection half (`transcript_miss.py`, `resolve_oracle.py`, ledgers), the discipline
  (spike method, §12, bench gate, layer rule), and the precedents proving both exits.
- **Missing:** the propose→re-bench-vs-frozen-anchor→accept-on-lift half (endgame step 3). Today the
  proposal is agent-drafted and the acceptance is human + bench; the frozen cross-stack anchor it would
  need does not exist yet ("enough verticals" is the program's discovery target). This gap is the endgame, not
  a wiring item; do not attempt it piecemeal.
- **First live use (next):** the pre-Go window, whose worklist triage is the first act after the publish sign-off of
  the current campaign.
