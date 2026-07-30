# Loop 3c - Run

> Stage 3 of 4. Shared laws, the depth-first rule and the ledger namespace live in the parent
> ([`03-per-repo-convergence.md`](03-per-repo-convergence.md)) and are not repeated here.

## Goal

Spend once on one repo - both arms, identical prompt, ×2, at the cell's real wall, with full Sense I/O
captured - and let the mechanical arbiter say WIN or sub-floor. Exit state: a confirmed win with its
definition-of-done checks numbered, or a sub-floor verdict handed to
[Diagnosis](03-d-diagnosis.md) with its transcripts intact.

## Product duties (per Sense surface)

- **All five tools - full I/O capture, standing requirement.** Every paid run tees each MCP request and
  its complete response to `results/<run>/sense-io.jsonl` via `bench/lib/mcp_tee.py`, wired in all three
  runners (`SENSE_IO_CAPTURE=0` is the kill switch). This is the raw material for the budget-trim audit,
  the misuse audit and the loss anatomy. **A run without capture wastes its product-telemetry half** -
  the bench half still scores, but the product learns nothing, and that is not an acceptable paid run.
- **The agent survey** is generated automatically on clean sense-arm runs (`bench-sense-local.sh` →
  `survey.json` plus a transcript-verified line in `surveys.jsonl`). It is **never read per-run and never
  feeds a verdict here**; Loop 5 reads it in aggregate.
- Reading the capture is not this stage's job. This stage guarantees it exists.

## Identity

- **Character:** scheduler. The judgment happened upstream; here the work is spending correctly once and
  refusing to spend twice on the same question.
- **Unit of work:** one cell - one repo, one frozen scenario, both arms, ×2.
- **Position:** consumes the frozen scenario from [Authoring](03-b-authoring.md); produces scored
  transcripts and a verdict for [Diagnosis](03-d-diagnosis.md), Loops 4/5/6.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | `bench/drivers/session-run.sh` and the arm runners (`bench-sense-local.sh`, `codex-run.sh`, `opencode-run.sh`) via `vertical-loop.sh` | |
| Evaluator | the `bench-win-confirm` agent - the five definition-of-done checks on a WIN | confirms or bounces; **never diagnoses a sub-floor verdict, never fault-finds a clean win** |
| Mechanical verifier | `pergroup.py` on the real transcripts | the WIN arbiter, never a proxy, never the judge's mood |
| Rubric judge | pinned prompt version, distinct from every evaluator | merging judge and evaluator re-creates self-grading |
| Human | authorises the spend | **permanent anchor, never demotable** |

## The dry-run gate (spend-time go/no-go)

Before the spend gate, the **sense arm** runs the real pipeline on the real runner. This is a measurement of
whether Sense can reach `base + 0.50` at this wall, and it is separate from Authoring's design-time
adversary probe. Two rules that cost mistakes:

- **A hand-grep is not a dry-run.** The gate needs real arm asymmetry from real runs; hand-simulating the
  baseline produces false ties.
- **If the baseline assembles the set, do not pay.** That verdict stands regardless of how good the
  scenario looks.

## The wall, and what a wall failure means

- **×2 is the settled standard.** A ×1 result is a sample, not a result, and carries an OPEN flag.
- **Cannot-finish-at-budget IS a result.** Never raise the watchdog to rescue a stalled arm: a failed exam
  is not an invalid exam. If the sense arm never reaches synthesis, that is a real failure - stop and read
  the transcripts rather than buying more wall.
- A throttled subscription produces a false loss. Check arm health before believing a floored score
  (`audit_watchdog.py`, `runs-variance.sh`).

## Stop conditions

- **Success:** discriminator ≥ +0.50 (favored +0.80) on the bench arm ×2, plus the mechanical
  definition-of-done - leak-free prompt, `mcp_count > 0`, no hallucinated citations, legitimate baseline -
  each numbered by `bench-win-confirm`. The win path fires **no event**, only a notice; Loop 5's harvest
  then runs.
- **Budget:** spend ceiling or turn cap → park with state intact; the repo resumes from
  `.loop-state.json`. A cap that fires is a measurement, not an obstacle: discharge the knob-first
  protocol ([`../decision-errors.md`](../decision-errors.md)) before any ceiling moves.
- **Failure:** sub-floor after ×2 → hand the cell to [Diagnosis](03-d-diagnosis.md) with the transcripts,
  the scored output and the capture. **This stage never writes a loss and never diagnoses one** - it
  reports the number and stops.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Spend | preflight and the sense-arm dry-run both passed, before the paid ×2 | yes | **never** - spend anchor |

## State / memory

- `verticals/<vertical>/results/<cell>/` - `transcript.json`, `scored.json`, `run_meta.json`,
  `sense-io.jsonl`, `survey.json`.
- `.loop-state.json` - the phase cursor for this repo.
- Ledger: `loop3/<repo>/event-c` (spend approved, with the wall and arm plan) and `loop3/<repo>/run-<n>`
  (the verdict). Run entries **require** the provenance line - Sense version, pinned repo commit,
  scenario version - enforced by `ledger_check.py`.
- Never delete clean runs. A cleanup that removes both arms destroys the only evidence a verdict rests on.

## Un-fakeable check

- `pergroup.py` on the on-disk transcripts, backed by the leak check, `mcp_count`, the hallucinated-cite
  check and `transcript_miss.py`. Any claim about this cell not traceable to those outputs is prose.
  **Standing caveat:** `transcript_miss.py` cannot parse opencode transcripts - on opencode arms, stand on
  `channels.json` plus the raw transcript instead, never on the miner's silence.

## Inputs / outputs

- **Consumes:** the frozen scenario + audited gold + the scenario-integrity gate record; the cell's real wall and control
  means from Eligibility.
- **Produces:** scored transcripts, `sense-io.jsonl` per run, the confirmed win notice or the sub-floor
  verdict, and the surveys line Loop 5 aggregates.

## Fixture test (standalone, $0)

- **Win confirmation:** the pre-verified chatwoot cell (+0.91 on dependencies, +0.68 overall). The
  confirm vertex must return WIN CONFIRMED with all five checks numbered - recompute match, direction
  agreement, `mcp_count` 5/5, five citation spot-checks against the pinned commit, legitimate baseline -
  **and stop.** Any fault-finding on a clean win means the prompt is over-tuned.
- **False-positive control:** the netbox run whose 32,097-character inline answer tripped a
  detector-level false positive. The stage must confirm the cell stands, reproduce `channels.json` from
  the raw transcript, and not be fooled by the opencode parsing gap.
- **Provenance control:** a report whose headline delta does not derive from the on-disk runs must be
  caught here, not downstream. This has happened once and was found only by recomputing from disk.

## Built vs missing

- **Built:** `vertical-loop.sh` phase machine and gates, the three arm runners, `mcp_tee.py` capture,
  `pergroup.py` / `scorer.py` / `judge.py`, `runs-variance.sh`, `audit_watchdog.py`, the
  `bench-win-confirm` agent.
- **Missing:** the budget-trim audit needs per-run gold cross-referenced against the *full* captured
  responses - the capture exists, the cross-reference does not. Stop-hook wiring stays deferred; it is
  friction reduction, not a prerequisite.
- **First live use:** the first php-laravel cell to clear the scenario-integrity gate.
