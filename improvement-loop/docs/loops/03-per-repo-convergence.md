# Loop 3 - Per-repo convergence (parent map)

> **What this file is.** Loop 3 is four stages, not one loop. Each stage has its own one-pager in
> the four `03-*` stage files in this folder, its own goal, its own mechanical check, and its own ledger keys.
> This file holds only what all four share: the order they run in, the laws that bind every stage, and
> the state they hand each other. Nothing here is repeated in the stage files, and no stage repeats
> anything here.

## Goal

Take one admitted repo from Loop 2 to an honest WIN (discriminator ≥ +0.50 on the bench arm ×2, DoD
clean) or to an evidence-backed swap escalation. **The loop is structurally unable to record a loss:
it wins, parks, or hands up a decision.**

## The four stages

| Stage | Goal | Ledger keys | Un-fakeable check | Human event |
|---|---|---|---|---|
| [Eligibility](03-a-eligibility.md) | Prove the cell can arithmetically clear +0.50 before anyone authors anything | `loop3/<repo>/probe` | `control_bound.py` exit code | none - a bound kill is arithmetic |
| [Authoring](03-b-authoring.md) | Produce a leak-free scenario with hand-audited gold that survives the adversary probe | `loop3/<repo>/{scenario,event-b}` | `scenario.py --prompt` leak check + per-dep hand audit | B (scenario integrity) |
| [Run](03-c-run.md) | Spend once, at the cell's real wall, ×2, and confirm or fail the DoD | `loop3/<repo>/{event-c,run-<n>}` | `pergroup.py` on the real transcripts | C (spend) |
| [Diagnosis](03-d-diagnosis.md) | Turn a sub-floor verdict into one named branch with detector output, never into a loss | `loop3/<repo>/{swap,close}` | the branch detectors, each producing output | D (unwinnable repo) |

A stage is **self-contained**: its input is a file on disk, its output is a file plus a ledger entry.
Never a conversation carried forward. If a stage needs something a previous stage did not write down,
that is a defect in the previous stage, not a reason to widen this one.

## Depth-first: one repo reaches a verdict before the next repo opens

**Batch what is $0 and cross-repo. Serialize everything paid.**

- **Eligibility runs slate-wide, up front.** It is free, and it is how the slate gets *ranked*: the repo
  with the **weakest control** goes first, because a weak control leaves the most headroom to the bar.
  Serializing this stage would mean picking the first repo blind.
- **Authoring, Run, and Diagnosis run one repo at a time, to a verdict.** No second repo's scenario is
  authored while the first is mid-diagnosis. Each repo is checkpointed in `.loop-state.json`; a parked
  repo resumes, never restarts.
- **Loops 4/5/6 stay aggregate and downstream.** A cross-model matrix and a cross-repo article are
  batched by definition, and they run only on frozen, won scenarios.

The known cost, stated so nobody rediscovers it: the first repo pays the whole instrument-debugging
tax, and if it turns out unwinnable the vertical has learned less than a shallow pass over four repos
would have taught it. The slate-wide eligibility sweep is the mitigation - going deep on the
highest-probability cell first is what makes the tax buy something.

This is not a new law. It is the law the practice broke: Rails won its cells one repo at a time with no
loop system at all, while later verticals produced protocol across four repos and no runs.

## The run-first inversion (ruling, 2026-07-19 - supersedes any ordering rule that conflicts with it)

1. **Run first, explain after.** Every candidate cell gets its memorization probe and its baseline mini
   dry-run in its FIRST session. Batteries, gates, and budget arguments are tools for EXPLAINING a
   dry-run result and curating gold. They never block or replace the run. **A kill or hold reasoned
   without a run is not a verdict.**
2. **A session ends with a run artifact - a dry-run or a bench cell - or the lane parks.** Ledger,
   protocol and status writing is overhead, never the session's deliverable.
3. **Rulings go up for money and repo-slot life-or-death only.** Everything else the agent decides,
   records, and moves past. No option menus.
4. **Delegation aims at the hunt, not the paperwork.** The mini-dry-run pipeline is mechanical; batch it
   across the slate's candidate anchors with cheap agents instead of ruling on shapes for a week.

Scale caveat from the same ruling: a one-token-cover kill is valid only at enumerable scale (≤15k
symbols). On giant repos that argument is advisory and the dry-run decides - "a covering grep exists"
is not "the agent runs it to closure".

## The try-harder law (ruling, 2026-07-13)

The default stance is **"there is an unfound win axis here; prove otherwise"**, and it starts at
proposal time, not at tie-diagnosis time. When a proposal reaches for "honest boundary", "tie",
"ballast", or any half-a-win framing, the required response is to deepen the contract hunt and widen the
pool. Gate verdicts are contract-specific, not repo-specific: one shallow probe per repo is not a hunt.
Only the small slot in the [§7.0 composition](../manifesto.md) is ballast by design; every other slot and
every scenario targets a WIN. **No stage of this loop offers a boundary framing - it exhausts axes first.**

## Laws every stage inherits

Imported by pointer, never restated in a stage file:

- Identical prompt both arms; gold never rendered into a prompt; **×2 is the settled standard** and a ×1
  result carries an OPEN flag; never delete clean runs; scenario authoring is serial, never forked onto
  shared files; no mid-campaign commits of bench files ([`../manifesto.md`](../manifesto.md),
  [`campaign-laws.md`](campaign-laws.md)).
- The agent dry-run gate before any paid bench - real arm asymmetry, never a hand-grep.
- **STOPPER:** a bug found in ANY measurement instrument (`gold.py`, `scorer.py`, `judge.py`,
  `admission_gate.py`, `grounding.py`, `pergroup.py`, `efficiency.py`) halts **all four stages
  immediately** - no new cells, no verdicts, no swap dossiers, no kills. Log it to the vertical's
  `LEDGER.md` the moment it is found, recheck at $0 with `rescore_diff.py`, and the human rules.
  Continuing without a ruling requires proving zero impact with a zero-diff re-score; a mitigation is
  not a resolution. Enumerate what the bad number retro-invalidates before anything else.
- The recorded-decision protocols in [`../decision-errors.md`](../decision-errors.md): stale verdicts
  re-verified at $0 before being cited, knob-first before any ceiling moves, a kill's reach stated in
  the same sentence as the kill, and no courtesy pauses - only the scenario-integrity, spend and swap gates, a STOPPER, or an honesty
  pause stop this loop.

## What this loop does not own

- **The conventions slate sweep** belongs to Loop 2, which already clones and indexes every candidate.
- **Product fixes.** A genuine product gap is parked for Loop 7 and never fixed mid-vertical. This loop
  files the hypothesis with its evidence; it does not touch Sense's code.
- **The agent survey.** Clean sense-arm runs generate it automatically; it is never read per-run and
  never feeds a verdict here. Loop 5 reads it in aggregate ([`00-agent-survey.md`](00-agent-survey.md)).
- **Cross-model confirmation.** One arm ×2 decides the verdict; every other arm is Loop 4.

## Shared state

- `.loop-state.json` - per-repo phase cursor, gitignored, resumable.
- `verticals/<vertical>/results/` - transcripts, scored, judged, and `sense-io.jsonl` per run.
- `verticals/<vertical>/scenarios/<repo>.yaml` + `.rubric.yaml`.
- `verticals/<vertical>/LEDGER.md` - the `loop3/` key namespace above. Write-only for every stage: the
  ledger is never read to decide anything ([`00-ledger.md`](00-ledger.md)).

## Position

- **Consumes:** one admitted slot from Loop 2 (pinned repo, index, contract, gate measurements) and the
  scenario-crafting law ([`../scenarios/crafting.md`](../scenarios/crafting.md)).
- **Produces:** the verdict plus scored transcripts (Loops 4/5/6), product-gap hypotheses (Loop 7),
  frozen scenario artifacts, and either a win notice or a swap dossier. A swap hands the slot back to
  Loop 2 for backup admission.

## Wiring this split needed (built 2026-07-30)

1. **`loop3/<repo>/probe` in `ledger_check.py`**, with build + wall required on its Provenance line.
2. **`control_bound.py --slate`** (extended, not a new script, so the ranking cannot drift from the bound
   verdict): emits the ranked paid queue and refuses to rank on an expired or unstamped probe.
3. **`gold_confidence_check.py`** for the 0.3-and-0.7 check in Authoring. The blast budget-trim audit
   stays a hand check by decision, with its cost recorded in [Diagnosis](03-d-diagnosis.md).
4. **`sense_build.py`** - the build identity that makes expiry mechanical, plus the Makefile dev label
   so a local build names the fix it carries.

Bound verdicts expire when Sense ships. **No probe kill is permanent** - it is a kill *on one Sense
version*, and Loop 7 owns the trigger that marks the old ones stale
([`03-a-eligibility.md`](03-a-eligibility.md), [`07-product-fix-window.md`](07-product-fix-window.md)).
