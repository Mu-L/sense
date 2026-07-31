# Loop 5 - Harvest

> **Status: defined 2026-07-11; scripted, discipline documented.** The loop that makes the campaign
> cumulative: it mines what the paid runs already bought and files it where the other loops read it.
> Advisory by design, on both sides: it never gates a bench (Loop A ≠ Loop B), and nothing it finds is
> fixed mid-vertical (manifesto §12). Its entire value is captured for $0; its entire discipline is
> refusing to spend more than $0.

## Goal

Bank everything the paid runs already bought, for $0: verified product gaps for Loop 7, conventions
material for the law ledger, cross-cutting rows for the program cadence, and staged lessons for the next vertical. This
loop is what makes the campaign cumulative; without it every vertical starts from scratch. It is the
program's widest product-telemetry pipe: three of `goal.md`'s sensory systems land their entries here.

## Product duties (per Sense surface)

- **All five tools - misuse mining:** run `tool_use_audit.py` (to build) over each repo's
  `sense-io.jsonl`: wrong tool for the question shape, wrong params, abandoned-on-empty, ignored hints,
  never-tried-search. Verified entries land in a **misuse ledger category** alongside the resolution gaps
  in `verticals/<stack>/results/loopA-gaps.md`, each naming its meta-surface (contract / hint / setup / response shape).
- **blast - budget-trim audit:** did `ApplyBlastBudget` drop items the gold needed and the agent lacked?
  A confirmed trim-miss is a product finding on the "send the RIGHT info" thesis itself.
- **search / status / conventions - feature-coverage check** (to build): per vertical, which surfaces the
  paid transcripts exercised and at what quality; unexercised surfaces recorded as such. The blind-spot
  detector: an unexercised surface is an unimproved surface.
- **conventions:** the A-D ledger DoD re-check (`02-bench-harvest.md`) is this loop's per-vertical tier,
  unchanged.
- **Loss anatomy:** every tie/loss this vertical produced gets its distilled row (the transcript-level
  reason grep won) appended to the loss-anatomy ledger.

## Identity

- **Character:** checklist-convergence. Detectors and prompts exist; the loop is running them at the two
  fixed moments and verifying entries before they land.
- **Unit of work:** one harvest target: a repo's paid transcripts (per-repo tier) or one ledger/fact-pack
  destination (per-vertical tier).
- **Position:** consumes the per-repo loops' transcripts + verdicts and Loop 4's matrix; produces Loop 7's worklist
  (`loopA-gaps`), the program's cross-cutting rows, the conventions ledger, and the next vertical's
  `carry-forward.md`. Two tiers:
  - **Per repo** (fires inside Loop 2's win path, automatically): `loopA-scan.sh harvest` →
    `transcript_miss.py` mines the ×N transcripts (cited-not-returned / fallback-reads / empty returns),
    adoption-gated, appends to `verticals/<stack>/results/loopA-gaps.md`.
  - **Per repo, survey channel** (fires inside `session-run.sh`'s sense arm, automatically): the
    post-run agent survey turn → `survey_verify.py` transcript-verifies and appends to the model root's
    `surveys.jsonl`; read only in aggregate at loop close via `--report`, next to the
    `transcript_miss.py` mine (process + thresholds: `08-agent-survey.md`, ledger
    `../FRICTION.md`).
  - **Per vertical** (fires at DoD, before the publish sign-off): `../cross-cutting/prompts/harvest-after-vertical.md`
    appends one row per fact pack (methodology / providers / cross-model / harness / product); the
    conventions ledger (`02-bench-harvest.md` (private tree), categories A-D) gets its DoD
    re-check; unplaced lessons stage into `carry-forward.md` with a named destination each.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | `loopA-scan.sh harvest` (per repo, auto via `vertical-loop.sh`); session agent running the harvest prompt (per vertical) | |
| Evaluator | the verification discipline: MISS-3 (reproduce on the pinned index, cross-check a second repo to split general defect from repo quirk); category-C ideas must pass the irreplaceability test | an unverified entry is discarded, not recorded |
| Mechanical verifier | `transcript_miss.py` + `resolve_oracle.py` on pinned indexes; append-only ledgers; adoption gate on mining | |
| Human | async review of the gap list at the publish sign-off; none blocking | already advisory by design |

## Stop conditions

- **Success:** all 4 repos mined; every ledger entry verified per the discipline; fact-pack rows appended;
  conventions ledger re-checked; `carry-forward.md` staged with each item naming its graduation
  destination.
- **Budget:** **$0, as law.** Query-time probes on pinned indexes only; never a paid run to feed a ledger.
  If verifying an entry would cost money, it stays a candidate, marked unverified.
- **Failure:** an entry does not reproduce → discard it. There is no escalation path because there is
  nothing to escalate: this loop only records facts it can demonstrate.

## Human events

None blocking. The gap list and ledger deltas are presented at the publish sign-off as part of the vertical's closing
dossier; the fix decisions they seed belong to Loop 7's authorization gate, not here.

## State / memory

- `verticals/<stack>/results/loopA-gaps.md` - append-only product-gap ledger (Loop 7's inbox).
- `02-bench-harvest.md` (private tree) - the cumulative conventions ledger (A-D categories,
  recording discipline at the top of the file).
- `../cross-cutting/0*.md` - one row per vertical, via the harvest prompt.
- the retired carry-forward staging file - staging with the graduation rule: nothing stays once it has a durable home;
  empty carry-forward = next vertical bootstrappable.
- **Readability duty:** append `loop5/<repo-or-tier>` entries to `verticals/<vertical>/LEDGER.md` as
  each harvest tier completes (write-only for the loop; contract in [`ledger.md`](ledger.md)).

## Un-fakeable check

- Every entry carries its evidence: a transcript line (miss), a live-CLI repro on the pinned index
  (oracle), or a second-repo cross-check (MISS-3). Append-only files make silent revision visible in
  `git diff`. The adoption gate keeps "the agent ignored the tool" from being recorded as "the tool
  missed."

## Inputs / outputs

- **Consumes:** paid transcripts (Loops 3/4), pinned indexes, verdicts, the vertical's lessons.
- **Produces:** Loop 7's verified worklist (resolution gaps + the misuse category); the program's cross-cutting
  rows; conventions-law raw material; the loss-anatomy rows; the feature-coverage record;
  the staged carry-forward that seeds the next vertical (including the misuse lessons bootstrap graduates into
  arm prompts and setup).

## Fixture test (standalone, $0)

- Re-run `transcript_miss.py` over the frozen rails transcripts → must reproduce the banked first catches
  (graph empty on ambiguous symbols; blast top-60 cap evicting true deps).
- Re-run the oracle against a pinned index → must reproduce a known ledger entry's repro.
- The gin conventions sweep (A1-A5, C3 → pitches 30-01..30-07) is the reference for what a
  correctly-verified ledger-to-pitch flow looks like.

## Built vs missing

- **Built:** `loopA-scan.sh` (harvest wired into `vertical-loop.sh`'s win path), `transcript_miss.py`,
  `resolve_oracle.py`, the harvest prompt, the conventions ledger + recording discipline, the graduation
  rule.
- **Missing:** the misuse ledger category file convention inside `verticals/<stack>/results/loopA-gaps.md` (first entries
  land with the Go vertical; the frozen-board catches are staged in the loss-anatomy/goal docs meanwhile).
  BUILT 2026-07-11: `tool_use_audit.py` (detectors + `--coverage` feature-check + degraded
  `--from-transcript` mode, fixtures in `test_tool_use_audit.py`; validated against all 38 frozen
  headline sense-arm transcripts). The loss-anatomy ledger backfill is in [`loss-anatomy.md`](loss-anatomy.md).
  The resolution-gap side is complete. Wiring the per-vertical tier as a `vertical-loop.sh`
  end-of-vertical phase is optional polish, not a gap.
- **First live use:** already live (it ran through Django); first Go use fires with the first Go win.
