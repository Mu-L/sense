# How to run the loops (operator's manual)

> **What this is.** The short manual for the owner and future contributors: how to start, stop, resume, what
> to review and when, and where the money goes. It restates the registry and one-pagers for the person
> at the keyboard; on any conflict, the one-pagers and the manifesto win. Added 2026-07-12 from the owner's
> round-2 review.

> **RULING 2026-07-19 (the owner: "go") - RUN-FIRST INVERSION.** Full text in
> `loops/campaign-laws.md`. Operator's summary: a candidate cell gets its scenario and an
> unscored validation run early (run first, explain after); a lane session ends
> with a run artifact or the lane parks; only money and repo-slot life-or-death reach the owner;
> the dry-run pipeline is batched to cheap agents across a slate. Where any one-pager below
> conflicts with this, this wins.

## The one-sentence model

A **vertical** (one stack: rails, django, go, ...) is one turn of the program. Inside it, bootstrap
stands the vertical up (scaffold + admit repos), then Loops 3→6 run in registry order
(converge each repo → fill the matrix → harvest → publish), and Loop 7 (the only loop that touches
product code) runs in the seam AFTER the vertical closes and BEFORE the next one starts. Nothing
advances to the next vertical on its own: the program cadence is a human rhythm, never software, so a new vertical
starts only when you say so.

Standing the vertical up is not a loop: it converges on the first pass or stops with a named status,
and it is one command, [`loops/00-bootstrap.md`](loops/00-bootstrap.md).

## Starting

1. **A fresh session picks up with one line:** "Re-render and read `verticals/<vertical>/STATUS.md`
   (`bash improvement-loop/bench/lib/render-status.sh <key>`), then continue from
   its pickup pointer." STATUS.md is the session-pickup state (Readability Phase 2, ratified
   severable 2026-07-16). `LEDGER.md` (the why) is opened ON DEMAND, at the step being executed,
   never wholesale at pickup.
2. **Starting a NEW vertical**: you confirm the stack by putting it in
   `verticals.txt` and writing its `stacks/<key>.conf` (the two hand-written prerequisites), then:

       bash bench/bootstrap/run.sh          # -> READY-FOR-LOOP

   That is bootstrap, repo hunt, screening, composition, pinning and indexing in one command, with a
   JSON verdict on stdout. It stops with a named status if a precondition fails (a stale `sense`
   binary, a missing stack profile, an extractor that is not ready) and creates nothing in those
   cases. Full status table: [`loops/00-bootstrap.md`](loops/00-bootstrap.md).
3. **Everything downstream reads its position from disk** (scaffold files, `repos.md`, `slate.json`,
   the results tree). There is no hidden state: if you wonder "where are we?", the answer is on disk, and
   `report-matrix.sh` re-renders the matrix from it.

## Basecamp pilot duties (2026-07-17, one week, then evaluate)

Basecamp is the human surface for the pilot week; the repo stays source of record for
everything a script reads (write-only law intact). IDs, the agent profile, and CLI quirks:
the private basecamp config. All writes go through `--profile sense-agent`. Session duties:

1. **At pickup, after STATUS.md:** poll the Rulings list
   (`basecamp todos list --list 10106566942 --in 48115802 --profile sense-agent`) and read
   new comments on open items - an owner comment IS the ruling; execute it. Check off a
   ruling once consumed.
2. **New ruling needed → a todo, not a memory string.** File it to the Rulings list with a
   self-contained evidence summary and options, assigned to the owner. "AWAITING the owner" no longer
   lives only in memory/STATUS.
3. **Stoppers and event completions ping Campfire** via the chatbot lines_url (write-only,
   posts as sense_agent) - this is the paging channel, in addition to the LEDGER entry,
   never instead of it.
4. **Consequential LEDGER entries** (verdict/stopper/ruling keys) get a message-board
   projection; the LEDGER stays the record.
5. **At session close:** answer the "What moved today?" check-in (question 10106804753)
   in plain sentences.
6. **Pitch cards** move on the Pitches board with pitch state
   (Inbox > Shaped > Crafting > Benching > Shipped | Killed).

**Content rules (the owner, 2026-07-17, standing):** (1) NEVER council member names in Basecamp
content - use the role ("the Go clarity reviewer"), this is important; (2) rich formatting
is mandatory: titles, lists, bold, code, quotes, links, line returns - no plain-text
blobs; (3) simple words, fast to grasp, always with context and the why, pros and cons
when applicable, the agent's take when applicable. Ruling todos follow Context / Why /
Options / The ask / My take.

Verify-after-write: the CLI can silently no-op (comment quirk in the conf; `todos update
--description` drops the description on v0.7.2, use the raw API with `content` +
`assignee_ids` included); a write is posted when a follow-up list/show says so, same
"it's written vs it's recorded" rule as the LEDGER.

## Stopping, parking, resuming

- **Any loop can stop mid-flight without losing work.** The per-repo loops checkpoint per repo in
  `.loop-state.json` and resumes from it, never restarts. Loop 4 parks per arm on a cap hit by writing
  the state + exact resume command into that arm's prompt file (`sweep-resume.sh` resumes). Bootstrap and Loops
  5, 6 are checklist runs: re-running them skips what is done (idempotent scripts).
- **To stop deliberately:** just stop; nothing needs a shutdown step. To resume: the pickup line above.
- **Never** delete run directories to redo a cell (re-runs add; deletion is human-approved only), and
  never commit bench files mid-campaign.

## Your gates (what you review, and when)

You are pinged at these events; everything else runs without you. **The per-repo loops 1-3 have NO
human gate at all as of 2026-07-31**: they author, spend, diagnose and swap on their own. Runs are on
a SUBSCRIPTION by default (API optional), so the binding constraint on a cycle is the weekly reset
rather than a dollar figure - and nothing checkpoints it. What stands in place of the removed gates
is mechanical and listed in each one-pager. The repo slate stopped being a gate on 2026-07-29 when
bootstrap began composing it.

| When | Event (loop) | What you actually review | Permanent? |
|---|---|---|---|
| Vertical start | Stack confirm (before bootstrap) | the stack choice + sequencing | yes |
| Scaffold stamped | Prompt-refresh review (bootstrap) | stamped prompts, no stale stack refs | demotable |
| ~~Slate composed~~ | ~~the admission sign-off~~ | retired 2026-07-29 - bootstrap admits its own slate; read it after the fact in `repos.md` + the `bootstrap/slate` ledger entry | no |
| ~~Scenario drafted~~ | ~~the scenario-integrity gate~~ | retired 2026-07-31 - Loop 1 checks its own gold (leak check, shown-over-MCP, per-dep audit) | no |
| ~~Before paid runs~~ | ~~the spend gate~~ | retired 2026-07-31 - Loop 2 spends on its validation run's verdict; nothing checkpoints the weekly quota | no |
| ~~Repo unwinnable~~ | ~~the swap gate~~ | retired 2026-07-31 - Loop 3 swaps on six cycles without movement, or a measured-absent seam | no |
| ~~Tie diagnosed~~ | ~~Tie-diagnosis review~~ | retired 2026-07-31 - the branch stands on its detector's output alone | no |
| Vertical start (once) | Cap/budget policy (4) | which arms, which subscriptions, weekly ceilings | demotable |
| Packs drafted | Pack review (6) | fact packs vs the numbers | demotable |
| Vertical close | **the publish sign-off** (6) | scorecard, 6 verdicts, matrix, gap list; publish sign-off | yes (anchor) |
| Window opens | Worklist selection (7) | which ledger gaps get fixed this window | yes |
| Fix proposed | **Council: proposal** (7) | the intended fix BEFORE code: approach, layer, blast radius, identity check | yes |
| Fix finished | **Council + PR review** (7) | the diff itself: new bugs, side effects, gates green | yes |
| Bench gate | Re-bench spend (7) | paid no-regress runs for a shipped fix | yes |
| **Measurement bug found** | **STOPPER** (any loop) | **the blast radius + what it retro-invalidates; YOU decide fix/continue/re-score/re-publish** | **yes (anchor)** |

**This table is exhaustive - if a question does not map to a row, there is no gate.**

**The STOPPER row is the one gate the AGENT must pull on its own** (the owner, 2026-07-15). Find a bug in
a measurement instrument - anything whose output becomes a number that decides something
(`gold.py`, `scorer.py`, `judge.py`, `screen.py`, `grounding.py`, `pergroup.py`,
`efficiency.py`) - and everything downstream STOPS: no new cells, no verdicts, no seat/kill
rulings, no publishing. (Doc work, unrelated product windows, and the investigation itself
continue - an absolute "stop everything" is unenforceable and gets rationalized away.) Then:
recheck with hard facts, run `bench/lib/rescore_diff.py` for the blast radius, log a
`stopper/<slug>` ledger entry, and **the human rules**. You may only continue WITHOUT a ruling if
you can PROVE zero impact - a re-score showing a zero diff. If you cannot produce that diff, it is
a stopper. **A mitigation is not a resolution:** "I'll hand-audit it every run" is a stopgap that
depends on the agent being perfect forever; it carries an expiry and a human owner, or it is a
silent liability. Born 2026-07-15 - see `decision-errors.md` Class 7.

Discharge the
**Class-4 protocol** (courtesy pause, [`decision-errors.md`](decision-errors.md)) before writing any
question to the owner: name the row it belongs to. If it is not one of {paid spend, an honesty pause, a slot
DISPOSITION}, do not ask - **continue and report**. A message that lists $0 work and then asks whether
to do it is the signature of the error; seat/anchor rulings are delegated, and asking to continue is a
stall dressed as deference. (Ratified 2026-07-15.)

## Where the tokens go (and where they cannot)

- **$0 by law or construction:** bootstrap, Loops 5 (harvest is "$0 as law": if verifying costs money, the
  entry stays unverified), 6, and Loop 7's detection half. All fixture tests are $0.
- **Paid, and UNGATED since 2026-07-31:** Loop 2's headline runs (Opus ×2 per repo) and Loop 4's
  confirmation cells (×1 each, behind the once-per-vertical cap policy, cap-aware ordering, big repos
  last). Loop 7's re-bench is behind its own spend gate.
- **The "loops forever" fear is scoped away twice.** Across verticals: the program never self-advances; a
  vertical's total spend is bounded by its 4 repos × the arm plan you approved, and the program pauses
  indefinitely between verticals at zero cost. Within a vertical, since 2026-07-31: nothing gates a
  paid unit, so the bound is the arm plan times 4 repos times the crafting cycles a repo is allowed -
  six - and the subscription's own weekly reset. Bootstrap in particular is NOT "loop through the verticals": it is
  one command for a single vertical. There is no unattended path from "session running"
  to "subscription drained"; the worst unattended case is Loop 4 finishing the cells already covered by
  the cap policy you set, then parking on the weekly cap.
- Budget stop conditions are per loop and explicit: hit one and the loop PARKS with state intact, it
  never grinds.

## Safety rails on the product (the scare, answered)

- **Only Loop 7 touches Sense's code.** Bootstrap "never builds product code"; per-repo findings park to the
  ledger, never fixed mid-vertical (manifesto §12); Loop 5 is advisory by design; query-layer fixes are
  explicitly flagged as MORE dangerous mid-vertical, not less (`goal.md`, discipline boundary).
- **Inside Loop 7, five stacked guards:** (1) council review of the PROPOSAL before any code (pipeline
  step 2); (2) failing repro test first, red-then-green; (3) the bench gate: no-regress on every vertical
  a change touches, run, not reasoned about; (4) the identity check: feature-complete v1, three lanes,
  no knobs, no fifth tool, out-of-scope gaps recorded, not forced; (5) council + PR review of the
  finished diff before merge. Both council passes are permanent, never demotable, and the spike method's
  exit is a full revert (byte-identical tree) when value is not proven.
- Loop 7's gates never demote while it ships product code; the trust ledger applies to bench-side loops
  only.

## Where to read more

- Registry + operating rules: [`README.md`](README.md). The goal above all loops: [`goal.md`](goal.md).
- One page per loop: `01`..`07` (identity, actors, stop conditions, fixtures, built-vs-missing).
- Deep design of the per-repo loops: `loops/01-repo-authoring.md`, `loops/02-repo-run.md`,
  `loops/03-repo-diagnosis.md`; their shared laws: `loops/campaign-laws.md`. Bench law: the manifesto
  (`manifesto.md`), which wins every conflict.
