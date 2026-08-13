# Loop 6 - Publish

> **Status: defined 2026-07-11; packs + validator + entry prompt exist.** The loop that turns a finished
> vertical into its public artifacts: 7 article packs (one per repo + the campaign scorecard), validated
> against the numbers on disk, signed off at the publish sign-off, then handed to the `social-writing` project. The
> boundary is absolute: **a bench session authors fact packs, never publication prose**, and prose work
> never happens in this repo.

## Goal

Turn a finished vertical into its validated public artifacts: 7 fact packs whose every number traces to a
run record on disk, reconciled with the verdicts, signed off at the publish sign-off, and handed to `social-writing`.
Exit state: the vertical is closed in the program ledger and nothing published can drift from the data.

## Product duties (per Sense surface)

- **The product-improvement story:** the campaign scorecard carries a "what this vertical taught Sense"
  row: gaps found and fixed, misuse surfaces corrected, extraction matured. Sourced from the ledgers like
  every other number, never asserted. The bench is the instrument; the article also says what the product
  learned, because that is the program's first-order output (`goal.md`).
- **All surfaces, indirectly:** the reconciliation pass covers the ledger-sourced claims the same way it
  covers the win numbers; a product claim that cannot be traced to a ledger entry fails validation.
- **Direct duties:** none; no agent runs and no product measurement happens here.

## Identity

- **Character:** checklist-convergence. Pack authoring is templated (`_skeleton`), number-sourcing is
  scripted, validation is mechanical; the judgment lives in one place, the publish sign-off.
- **Unit of work:** one article pack authored from results and validated to 0 FAIL.
- **Position:** consumes Loop 3's verdicts + transcripts, Loop 4's matrix + billed-token records, Loop 5's
  harvest (the gap list is part of the publish sign-off dossier); produces the pack set for `social-writing` and
  the vertical's closing scorecard. 4 repos → 5 articles → one week (manifesto cadence).

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | session agent via **prompt 08** (the entry point), passing the articles dir + `--results` explicitly | |
| Evaluator | the pack validator (set must validate 0 FAIL) + a reconciliation pass: scorecard verdicts vs `pergroup.py` outputs | |
| Mechanical verifier | the validator; numbers sourced from run records on disk, never retyped | |
| Human | the publish sign-off: sign off scorecard + 6 verdicts + matrix + gap list; trigger the handoff | permanent anchor (publish) |

## Editorial law (imported by pointer, enforced here)

- **The article matches the numbers** (DoD §14); every claim traces to a run record.
- **Headline metric = reach at billed-token parity + sense-only reach**; completeness is a gate, not a
  headline; efficiency reported only at recall parity. Cheap-arm results publish **billed** tokens.
- **Snapshot rule:** once published, an article's numbers freeze. Later re-pins or re-scores do not chase
  published prose; drift is recorded in the repo, not edited into history.
- **Ties publish honestly:** the small-slot "where baseline ties Sense" row is part of the story, not a
  blemish to bury (§7.0 ballast exists for exactly this row).
- **Artifact hygiene:** full-name attribution (Luc B. Perussault-Diallo), no tool branding, no council
  names, no em-dashes.
- ×1 confirmation cells carry their OPEN flag into any table that cites them.

## Stop conditions

- **Success:** 7 packs validate 0 FAIL; scorecard reconciles with the verdicts and the matrix; the publish sign-off
  signed; handoff delivered to `social-writing`.
- **Budget:** sessions, not dollars ($0 loop). If pack production drags, the packs are too far from the
  skeleton, which is a template fix, not more effort.
- **Failure:** a number will not reconcile. If the pack is wrong, fix the pack. If the *verdict* is wrong,
  that is not a publish problem: stop, reopen upstream (Loop 3's record), and treat it as an incident;
  publishing never papers over a reconciliation failure.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| E: DoD + publish sign-off | all packs validate, dossier assembled | yes | never (permanent anchor) |
| Pack review | packs drafted, before the publish sign-off | yes, async | after clean history |

## State / memory

- `improvement-loop/verticals/<key>/findings/` - packs 01..NN, `00-scorecard.md`, `_skeleton`, media.
- The the publish sign-off dossier is assembled from live sources (verdicts, matrix render, gap list), not stored
  separately; the packs + scorecard ARE the durable output.
- **Readability duty:** append `loop6/event-e` to `verticals/<vertical>/LEDGER.md` at vertical close;
  `ledger_check.py` runs here, advisory, alongside the closing review (contract in
  [`ledger.md`](../ledger.md)).

## Un-fakeable check

- The validator run (0 FAIL) against the results on disk, plus scorecard-vs-`pergroup.py` reconciliation.
  A pack number that cannot be traced to a run record fails; prose cannot fix it.

## Inputs / outputs

- **Consumes:** verdicts + scored transcripts (Loop 3), matrix + billed tokens (Loop 4), harvest dossier
  (Loop 5), the `_skeleton` template.
- **Produces:** the validated pack set (the `social-writing` handoff), the campaign scorecard, the signed
  the publish sign-off that closes the vertical in the program ledger.

## Fixture test (standalone, $0) - RUN 2026-07-11

- Validator on the frozen python-django packs → 6 checked, 0 need attention (reference state) ✓.
- **Tamper test, two probes:** (a) `headline:` block - deps_delta 0.50→0.30 in a saleor pack copy →
  **caught**, "OUTDATED deps_delta +0.30→+0.50" ✓ (the validator recomputes from live scored.json).
  (b) `axes:` block - cited_delta 0.15→0.35 → **NOT caught**: the judge-axis numbers are validated by
  nothing (check_findings_stats reads only `headline:`; findings_audit checks structure/links/board).
  GAP recorded in Built vs missing.
- Tamper-test bonus catch: the sentry pack's validated headline already read `deps_delta: 0.10` /
  TIE ◆ - proof the pack pipeline stayed honest through the sentry provenance incident (only
  report.json's row was stale; zero published drift).
- Known benign diff: the rails skeleton drift traces to the cycle-30 re-pins, not the pack flow; expected,
  documented, not "fixed."

## Built vs missing

- **Built:** prompt 08 as the entry point, the pack skeleton, the validator (Django set validates 0 FAIL),
  the scorecard format, the `social-writing` handoff convention.
- **Missing:** the `axes:` frontmatter block (judge-score numbers, "regen via scoreboard.py") has NO
  mechanical validator - the 2026-07-11 tamper test falsified cited_delta undetected. **DECIDED
  2026-07-12 (the owner): extend `check_findings_stats.py`** to recompute axes from judged.json/scoreboard,
  with the falsified-cited_delta tamper case as the new fixture; queued as $0 bench-side work before
  the Go vertical's Loop 6 use. Also: prompt 08 still
  takes the articles dir + `--results` by hand (small polish); prompt-07 creation owed from the Django
  campaign. The loop is operable today.
- **First live use (next):** Go vertical DoD, after its matrix fills.
