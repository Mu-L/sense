# Loops - the registry and the one-pagers

> **What this is.** One file per loop, all following [`template.md`](template.md). Rules stay in the
> manifesto; this folder is operating structure, not authority. The goal every loop answers to lives
> in [`goal.md`](../goal.md): make Sense the LLM's best companion on the benched stack and globally;
> the WIN is evidence of it, not the point.

## Start here

    bash bench/bootstrap/run.sh          # -> READY-FOR-LOOP3

One command takes the next queued stack from nothing to a verified slate of 4 indexed repos:
[`00-bootstrap.md`](00-bootstrap.md). No human and no model in it. It needs the latest `sense`
installed, a next key in `verticals.txt`, and that key's `stacks/<key>.conf` - and it stops with a
named status if any of the three is missing, so an orchestrator never parses prose.

Bootstrap is not a loop. It converges on the first pass or stops with a reason. What follows does
loop.

## The loops

| # | Loop | One iteration is | Stops when | Human gate | Status |
|---|---|---|---|---|---|
| 1 | [Repo authoring](01-repo-authoring.md) | one scenario + gold for one repo | the scenario-integrity gate signed on audited gold | B (permanent) | scripted except the per-dep hand audit |
| 2 | [Repo run](02-repo-run.md) | one validation run (unscored, both arms), then one paid cell ×2 at the real wall | WIN confirmed or a verdict reported | C (permanent) | runners + capture + `bench-win-confirm` live; `BENCH_VALIDATION=1` routes + stamps the unscored run |
| 3 | [Repo diagnosis](03-repo-diagnosis.md) | one run read: the struggle read always, the taxonomy on a sub-floor verdict | material handed back to authoring, or one branch named with detector output | D (permanent) + async tie review | `bench-evaluator` + `bench-struggle-read` live; budget-trim audit still a hand check |
| 4 | [Matrix fill](04-matrix-fill.md) | one confirmation-arm ×1 cell | all arm×repo cells done, anomalies re-run | budget policy set once | scripts built; scheduling manual |
| 5 | [Harvest](05-harvest.md) | one repo's transcripts mined | ledgers appended for all 4 | none (advisory by design) | scripted (`loopA-scan.sh`) |
| 6 | [Publish](06-publish.md) | one article pack authored + validated | the publish sign-off | the publish sign-off; permanent | packs + prompt 08 exist |
| 7 | [Product-fix window](07-product-fix-window.md) | one gap → spike → no-regress bench → ship or revert | gap list empty/parked, pre-next-vertical | fix authorization + council (proposal AND code) + PR review | detection built; propose→re-bench missing |

Above them sits the **program cadence**: one turn is one complete vertical, and it stops at anchor
saturation. It is a human rhythm plus `verticals.txt`, and it is never software.

Two meta-loops sit above the table: the **trust ledger** (a gate demotes from blocking to advisory
after a vertical of clean reviews; the three permanent anchors never demote) and the **endgame loop**
(Loop 7 becoming self-proposing against the frozen anchor). Documented forward-horizon; no one-pager
until they are buildable.

## The other files here

| File | What it is |
|---|---|
| [`ledger.md`](ledger.md) | the readability contract: per-vertical `LEDGER.md` + `STATUS.md`, write-only for the loops |
| [`08-agent-survey.md`](08-agent-survey.md) | the post-run agent self-report channel; Loops 4, 5 and 7 read it |
| [`campaign-laws.md`](campaign-laws.md) | empirical laws distilled from past campaigns, plus the laws the three per-repo loops share; Loop 1 reads it at scenario authoring |
| [`loss-anatomy.md`](loss-anatomy.md) | the catalogue of how cells lose |
| [`template.md`](template.md) | the one-pager skeleton (stale; reviewed with the per-repo loops) |

Session pickup is the vertical's `STATUS.md` (re-render via
[`render-status.sh`](../../bench/lib/render-status.sh) `<key> <doc-dir>`), with `LEDGER.md` opened on
demand. The operator's manual (how to start, stop, resume; what a human reviews and when; where spend
happens) is [`how-to-run.md`](../how-to-run.md).

## Operating rules

- **Define forward, test on fixtures, go live forward.** Every loop is tested standalone against the
  frozen ruby-rails + python-django artifacts with pass criteria taken from known history. Loops go
  live in forward order as the next vertical reaches each stage, gates fully blocking on first use.
- **Loops nest and overlap; the registry is an ownership map, not a timeline.** Loops 1-3 are the
  per-repo cycle and run once per repo in the slate - **depth-first, one repo to a verdict before
  the next opens** - and a repo goes round them up to six times before it swaps. Loop 4 overlaps
  them; Loop 7 runs in the seam between verticals.
- **The one-level-down test.** Any responsibility you are tempted to add to a loop must first fail
  the question "could this live one level down?"
- **No un-fakeable check, no automation.** A one-pager that cannot name the mechanical check that
  keeps its loop honest is describing a loop that is not ready to automate. Leave its gates blocking.
- **The instrument serves the goal.** Every one-pager carries a Product duties section (`goal.md`); a
  loop that owes the product nothing must say so explicitly.

## Conventions-axis ownership

The conventions bench is deferred by decision (2026-07-06), but its machinery has named owners so the
deferred axis cannot silently evaporate:

| Piece | Owner |
|---|---|
| Per-vertical sweep (`sense conventions` on the 4 pinned repos) | bootstrap - it already clones and indexes every candidate |
| A-D ledger recording + DoD re-check | Loop 5, per-vertical tier |
| §8 local-law build-gate bench (write-task, Rails corpus) | Loop 7: named item of the pre-Laravel window |
| End-of-program cross-stack conventions pass (category-D corpus) | the program cadence |
