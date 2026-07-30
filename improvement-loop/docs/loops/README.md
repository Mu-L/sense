# Loops - the registry and the one-pagers

> **What this is.** The loop system for running a vertical bench with progressively less manual work.
> One file per loop, all following [`00-template.md`](00-template.md). Rules stay in the manifesto; this folder is operating structure, not authority.
> The goal every loop answers to lives in [`goal.md`](../goal.md): make Sense the LLM's best companion
> on the benched stack and globally; the WIN is evidence of it, not the point. The readability contract
> (per-vertical `LEDGER.md` + `STATUS.md`, write-only for the loops) is [`00-ledger.md`](00-ledger.md).

## The registry

| # | Loop | One iteration is | Stops when | Human gate | Status |
|---|---|---|---|---|---|
| 0 | Program (campaign cadence) | one complete vertical | anchor saturation (discovered, not set) | stack sequence; permanent | doc + human rhythm; never software |
| 1 | Vertical bootstrap | one scaffold element brought to ready | scaffold valid + carry-forward empty | stack + extractor confirm | ~80% scripted (`new-vertical.sh`) |
| 2 | Repo admission | one candidate measured against the seam-existence gate | 4 admitted per §7.0 + backups | none - autonomous since 2026-07-29 | `admission_gate.py` built + backtested (K7 import law, four-anchor fixture) |
| 3 | Per-repo convergence - **four stages**, parent map in [`03-per-repo-convergence.md`](03-per-repo-convergence.md) | one repo taken to a verdict, depth-first | WIN ≥ +0.50 or swap escalated | the scenario-integrity, spend and swap gates | see the four rows below |
| 3a | [Eligibility](03-a-eligibility.md) | one control probe on one repo, slate-wide and $0 | slate ranked, at least one bound-legal cell | none - a bound kill is arithmetic | `control_bound.py` wired; `probe` ledger key + ranked-slate artifact missing |
| 3b | [Authoring](03-b-authoring.md) | one scenario + gold for the top-ranked repo | the scenario-integrity gate signed on audited gold | B (permanent) | scripted except the 0.3/0.7 gold check |
| 3c | [Run](03-c-run.md) | one cell, both arms, ×2, at the real wall | WIN confirmed or sub-floor reported | C (permanent) | runners + capture + `bench-win-confirm` live |
| 3d | [Diagnosis](03-d-diagnosis.md) | one sub-floor verdict | one branch named with detector output | D (permanent) + async tie review | `bench-evaluator` live; budget-trim audit missing |
| 4 | Matrix fill | one confirmation-arm ×1 cell | all arm×repo cells done, anomalies re-run | budget policy set once | scripts built; scheduling manual |
| 5 | Harvest | one repo's transcripts mined | ledgers appended for all 4 | none (advisory by design) | scripted (`loopA-scan.sh`) |
| 6 | Publish | one article pack authored + validated | the publish sign-off sign-off | the publish sign-off; permanent | packs + prompt 08 exist |
| 7 | Product-fix window | one gap → spike → no-regress bench → ship or revert | gap list empty/parked, pre-next-vertical | fix authorization + council (proposal AND code) + PR review | detection built; propose→re-bench missing |

Two meta-loops sit above the table: the **trust ledger** (a gate demotes from blocking to advisory after a
vertical of clean reviews; the three permanent anchors never demote) and the **endgame loop** (Loop 7 becoming
self-proposing against the frozen anchor). Documented forward-horizon; no one-pager until they are buildable.

Session pickup is the vertical's `STATUS.md` (Readability Phase 2; re-render via
[`render-status.sh`](../../bench/lib/render-status.sh) `<key> <doc-dir>`), with `LEDGER.md` opened on demand. The go
campaign's `next-steps.md` is frozen history and lives outside this folder; it is never updated again,
and nothing here reads it. The operator's manual (how to start, stop, resume; what a
human reviews and when; where spend happens) is [`how-to-run.md`](../how-to-run.md).
Empirical laws distilled from past campaigns live in [`campaign-laws.md`](campaign-laws.md); Loops 2/3 read it at slate composition and scenario authoring.
record as history, re-runnable by ruling).

## Operating rules

- **Define forward, test on fixtures, go live forward.** One-pagers are written in registry order. Every loop
  is tested standalone against the frozen ruby-rails + python-django artifacts with pass criteria taken from
  known history (e.g. the Loop-2 gate must fail haystack and pass sentry). Loops go live in forward order as
  the next vertical reaches each stage, gates fully blocking on first use.
- **Loops nest and overlap; the registry is an ownership map, not a timeline.** Loop 3 runs four times inside
  one turn of Loop 0 - **depth-first, one repo to a verdict before the next opens**, except its slate-wide
  $0 eligibility stage. Loop 4 overlaps Loop 3; Loop 7 runs in the seam between verticals.
- **The one-level-down test.** Any responsibility you are tempted to add to a loop must first fail the
  question "could this live one level down?" Loop 0 stays two decisions thin by design.
- **No un-fakeable check, no automation.** A one-pager that cannot name the mechanical check that keeps its
  loop honest is describing a loop that is not ready to automate. Leave its gates blocking.
- **The instrument serves the goal.** Every one-pager carries a Product duties section (`goal.md`); a
  loop that owes the product nothing must say so explicitly. Automation that banks wins but teaches the
  product nothing is optimizing the wrong thing.

## Conventions-axis ownership

The conventions bench is deferred by decision (2026-07-06, `02-bench-harvest.md` (private tree)),
but its machinery has named loop owners so the deferred axis cannot silently evaporate:

| Piece | Owner |
|---|---|
| Per-vertical sweep (`sense conventions` on the 4 pinned repos) | Loop 2 - it already clones and indexes every candidate |
| A-D ledger recording + DoD re-check | Loop 5, per-vertical tier |
| §8 local-law build-gate bench (write-task, Rails corpus) | Loop 7: named item of the pre-Laravel window |
| End-of-program cross-stack conventions pass (category-D corpus) | Loop 0 milestone |
