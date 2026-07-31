# Loop 2 - Repo run

> Second of the three per-repo loops ([authoring](01-repo-authoring.md) → run →
> [diagnosis](03-repo-diagnosis.md)). The laws all three share live in
> [`campaign-laws.md`](campaign-laws.md) and are not repeated here.

## Goal

Run the scenario twice over: once unscored to find out whether it is the right scenario, then, if it
is, once for real - both arms, identical prompt, ×2, at the cell's real wall, with full Sense I/O
captured - and let the mechanical arbiter say WIN or sub-floor. Exit state: a confirmed win with its
definition-of-done checks numbered, or a verdict handed to [Diagnosis](03-repo-diagnosis.md) with its
transcripts intact.

**Every run leaves here for Diagnosis, including the validation run and including a win.** The
validation run's whole purpose is the read Diagnosis does on it.

## Product duties (per Sense surface)

- **All five tools - full I/O capture, standing requirement.** Every paid run tees each MCP request and
  its complete response to `results/<run>/sense-io.jsonl` via `bench/lib/mcp_tee.py`, wired in all three
  runners (`SENSE_IO_CAPTURE=0` is the kill switch). This is the raw material for the budget-trim audit,
  the misuse audit and the loss anatomy. **A run without capture wastes its product-telemetry half** -
  the bench half still scores, but the product learns nothing, and that is not an acceptable paid run.
- **The agent survey** is generated automatically on clean sense-arm runs (`bench-sense-local.sh` →
  `survey.json` plus a transcript-verified line in `surveys.jsonl`). It is **never read per-run and never
  feeds a verdict here**; Loop 5 reads it in aggregate.
- Reading the capture is Diagnosis's job. This loop guarantees it exists, on the validation run too.

## Identity

- **Character:** scheduler. The judgment happened upstream; here the work is spending correctly once and
  refusing to spend twice on the same question.
- **Unit of work:** one cell - one repo, one frozen scenario, both arms, ×2.
- **Position:** consumes the frozen scenario from [Authoring](01-repo-authoring.md); produces scored
  transcripts and a verdict for [Diagnosis](03-repo-diagnosis.md), Loops 4/5/6.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | `bench/drivers/session-run.sh` and the arm runners (`bench-sense-local.sh`, `codex-run.sh`, `opencode-run.sh`) via `vertical-loop.sh` | |
| Evaluator | the `bench-win-confirm` agent - the five definition-of-done checks on a WIN | confirms or bounces; **never diagnoses a sub-floor verdict, never fault-finds a clean win** |
| Mechanical verifier | `pergroup.py` on the real transcripts | the WIN arbiter, never a proxy, never the judge's mood |
| Rubric judge | pinned prompt version, distinct from every evaluator | merging judge and evaluator re-creates self-grading |
| Human | none - the loop spends on its own judgement | the validation run is what stands between a scenario and the money |

## The validation run (unscored, both arms, once)

Before the paid pair, the scenario runs for real on the real runner: **both arms, one run each, at the
cell's wall.** It is a measurement of whether the scenario is the right scenario, and it produces the
material Diagnosis reads. It is separate from Authoring's design-time adversary probe, which is a
measurement of what a grep-and-read baseline could reach on the shape before anyone benched it.

- **`vertical-loop.sh` runs it as the `validate` phase**, between preflight and the paid bench, and
  skips it when one already exists on disk (delete `results/validation/` to force a fresh one).
- **It is never scored and never cited.** Run it with `BENCH_VALIDATION=1`, which routes every runner
  to `results/<model>/validation/` and stamps `"scoring": false` into `run_meta.json`. A number from a
  ×1 unscored run is a sample; it may not close a question, settle a win or tie, or enter an article.
  **The isolation is the results root, not a flag the scorer honours:** `pergroup.py` and `scorer.py`
  walk `RESULTS_DIR`, so a validation cell is invisible to them by construction and no measurement
  instrument had to change to make it so (which would have been a STOPPER).
- **Both arms, not just the baseline.** A shape can pass the baseline dry-run and die on the sense arm
  the same day: measured, baseline 6/15 (a win candidate) then sense-arm cited 3/15 with the agent
  never calling `sense_blast`. A baseline-only validation tells you the scenario is hard, not that
  Sense reaches.
- **A hand-grep is not a run.** Hand-simulating the baseline produces false ties.
- **If the baseline assembles the set, do not pay.** That stands regardless of how good the scenario
  looks. Route to Diagnosis, which reads the run and hands Authoring the next draft.

The cost is stated, not hidden: this is one extra pair of sessions per crafting cycle. It buys the
only thing that has ever moved this program, which is a run instead of an argument.

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
- **Failure:** sub-floor after ×2 → hand the cell to [Diagnosis](03-repo-diagnosis.md) with the transcripts,
  the scored output and the capture. **This stage never writes a loss and never diagnoses one** - it
  reports the number and stops.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| none | - | - | - |

The spend gate was REMOVED 2026-07-31. Runs go through a SUBSCRIPTION by default (the API path is
optional), so what a cycle consumes is quota against the weekly reset, not dollars - but nothing
checkpoints that consumption any more. The validation run is the only thing left standing between a
scenario and a paid pair: it is unscored, both arms, and if the baseline assembles the set the pair
does not run. Treat a validation run that says "do not pay" as binding, because nothing reviews it
afterwards.

## State / memory

- `verticals/<vertical>/results/<cell>/` - `transcript.json`, `scored.json`, `run_meta.json`,
  `sense-io.jsonl`, `survey.json`.
- `.loop-state.json` - the phase cursor for this repo.
- Ledger: `loop2/<repo>/event-c` (spend approved, with the wall and arm plan) and `loop2/<repo>/run-<n>`
  (the verdict). Run entries **require** the provenance line - Sense version, pinned repo commit,
  scenario version - enforced by `ledger_check.py`.
- Never delete clean runs. A cleanup that removes both arms destroys the only evidence a verdict rests on.

## Un-fakeable check

- `pergroup.py` on the on-disk transcripts, backed by the leak check, `mcp_count`, the hallucinated-cite
  check and `transcript_miss.py`. Any claim about this cell not traceable to those outputs is prose.
  **Standing caveat:** `transcript_miss.py` cannot parse opencode transcripts - on opencode arms, stand on
  `channels.json` plus the raw transcript instead, never on the miner's silence.

## Inputs / outputs

- **Consumes:** the frozen scenario + audited gold, and the
  cell's real wall.
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

- **Built:** `vertical-loop.sh` phase machine (`index → scout → preflight → validate → bench →
  report → harvest`), the three arm runners, `BENCH_VALIDATION=1`
  (results-root routing in `bench-paths.sh` + the `scoring` stamp in all four runners, pinned by
  `test_validation_isolation.py`), `mcp_tee.py` capture,
  `pergroup.py` / `scorer.py` / `judge.py`, `runs-variance.sh`, `audit_watchdog.py`, the
  `bench-win-confirm` agent.
- **Missing:** the budget-trim audit needs per-run gold cross-referenced against the *full* captured
  responses - the capture exists, the cross-reference does not. Stop-hook wiring stays deferred; it is
  friction reduction, not a prerequisite.
- **First live use:** the first cell whose validation run says the scenario discriminates.
