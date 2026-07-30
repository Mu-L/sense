# Loop 3d - Diagnosis

> Stage 4 of 4. Shared laws, the depth-first rule and the ledger namespace live in the parent
> ([`03-per-repo-convergence.md`](03-per-repo-convergence.md)) and are not repeated here.

## Goal

Turn a sub-floor verdict into exactly one named cause, each candidate ruled in or out by a detector that
produced output. Exit state: the cell is routed back to an earlier stage with a lever, parked as a product
gap for Loop 7, or escalated as a swap with the numbers attached. **This stage cannot write a loss.**

## Product duties (per Sense surface)

- **blast - the budget-trim audit, before blaming the scenario.** On every sub-floor verdict, check
  `sense-io.jsonl` against the gold to see whether the blast budget trimmed a dependency the gold needed.
  "Send the right info" is a hypothesis this stage is uniquely positioned to falsify, and today it is the
  check most often skipped.
- **Tool contracts and response shape - double-file every misuse finding.** The harness may compensate
  mid-campaign (arm prompt, run script), but the product-side cause - contract text, a default, a hint,
  empty-result guidance - is ledgered for Loop 7. **Compensation without the ledger entry loses the
  product signal**, which is the whole reason this stage exists.
- **Genuine product gaps are parked, never fixed mid-vertical.** File the hypothesis with its evidence and
  move on; the spike belongs to Loop 7.

## Identity

- **Character:** judgment, adversarial. This is the stage that is one keystroke from game-until-green if
  its separate evaluator is removed.
- **Unit of work:** one sub-floor verdict on one cell.
- **Position:** consumes the verdict, transcripts and capture from [Run](03-c-run.md); produces a routed
  lever back to [Eligibility](03-a-eligibility.md) / [Authoring](03-b-authoring.md) / Run, a parked gap
  for Loop 7, or a swap dossier that hands the slot back to Loop 2.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | the session agent that ran the cell | it may propose, it may not conclude |
| Evaluator | the `bench-evaluator` agent - adversarial, separate, this taxonomy as its rubric | stands only on mechanical verifier output; anything else it says is prose |
| Mechanical verifier | `pergroup.py`, `scorer.py`, `transcript_miss.py`, `tool_use_audit.py`, `resolve_oracle.py`, `relationship_audit.py`, `rescore_diff.py` | each branch below names its own |
| Rubric judge | pinned, distinct from the evaluator | |
| Human | the swap gate (unwinnable repo) and the tie-diagnosis review | the swap gate is a **permanent anchor**; the review is demotable |

## The taxonomy

Dispatch through the branches **in this order** - cheapest lever first - and conclude nothing but WIN
until every branch is exhausted with evidence. Default stance: *there is an unfound win axis here; prove
otherwise.*

| # | Cause | Detector | Lever | Cost |
|---|---|---|---|---|
| 1 | Gold mis-curation | per-dependency tally from `scored.json` | re-target gold, then re-score the existing transcripts with `scorer.py` | $0 |
| 2 | Scenario shape wrong (fan instead of chain, satisficing-friendly prompt) | tally pattern + transcript read | re-author → the scenario-integrity gate again → re-bench | paid |
| 3a | Sense returned it, the agent dropped it | `transcript_miss.py` (cited-not-returned, fallback reads, empties), `mcp_count` | fix the harness or the output shape upstream - **never the scorer** | $0 |
| 3b | The agent misused the tool: wrong tool for the question shape, wrong params, abandoned after one empty result, ignored hints | `tool_use_audit.py` over `sense-io.jsonl` | product meta-surface fix (contract, hint, setup) ledgered for Loop 7; harness compensation allowed meanwhile | cheap |
| 4 | Judge or scorer error | hand-audit the per-dependency credits (basename false-credit guard), `relationship_audit.py` | fix the scorer **with a guard test** | $0 |
| 5 | Genuine product gap | `resolve_oracle.py` fact-check on known-true edges | append to the vertical's gap list, **parked** for Loop 7's window | $0 now |
| 6 | Seam measurably nonexistent | existence measurement on the index | the swap gate swap, with the numbers attached | $0 |

Branch 6 should be rare, because Loop 2 gates admission on the same measurement. When it fires anyway
that is a **Loop 2 calibration finding** and it goes in the gate's ledger, not only in this one.
Branch 5 is the only branch touching product truth, and it is asynchronous by law.

Branch 3b has a required negative side: a run where the same parameter was a deliberate,
prompt-justified narrowing must **not** be flagged. A detector that cannot tell misled from intentional
is over-tuned.

## Stop conditions

- **Success:** one branch named, its detector output recorded, and the lever handed to the owning stage.
  A branch chosen without detector output is not a diagnosis.
- **Budget:** every branch except 2 is $0; branch 2 re-enters the paid path through the scenario-integrity gate and C, never
  directly. Park with the branch analysis on disk.
- **Failure:** all branches exhausted with evidence → the swap gate, with the dossier: what was tried, the
  per-dependency tally, and the product-gap hypothesis. **The loop wins, parks, or escalates a swap.**

Three protocols bind every write-up here ([`../decision-errors.md`](../decision-errors.md)):

- **Stale verdicts.** Any kill, tie or loss cited in a park or swap dossier is re-verified at $0 on the
  current binary and index first. A verdict from an older binary is a hypothesis.
- **Reach, in the same sentence as the kill.** State which axis, edge kind, tool or anchor set was
  measured - and therefore what is **not** concluded. An axis screen bounds an axis, never a repo. One
  axis dying on one repo licenses exactly one sentence and is never grounds to shrink the program order.
  Precedent: a repo written off as dead was hiding a real product gap in an edge kind the screen never
  touched, found only by digging further.
- **The next move after a $0 kill is the next angle, not the exit.** Bank a screen's saving, never its
  verdict.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Unwinnable repo (swap) | taxonomy exhausted, still < +0.50 | yes | **never** - repo-selection anchor |
| Tie-diagnosis review | a branch is picked, before acting on ANY branch | yes, async | yes - narrows to branch-2-only after one clean vertical under this split (trust ledger) |

## State / memory

- `verticals/<vertical>/results/` - read-only here, plus the branch analysis written alongside the cell.
- The vertical's gap list, append-only, for branch 5.
- Ledger: `loop3/<repo>/swap` (the dossier) and `loop3/<repo>/close` (the cell's final state). Both
  **require** the provenance line and carry the diagnosis branch, enforced by `ledger_check.py`.

## Un-fakeable check

- Every branch ruled out must cite its detector's output. The evaluator stands on
  `pergroup.py`, `transcript_miss.py`, `tool_use_audit.py`, `resolve_oracle.py` and the hand audit;
  a ruled-out branch with no output attached counts as not yet examined.

## Inputs / outputs

- **Consumes:** the sub-floor verdict, scored transcripts, `sense-io.jsonl`, the gold and the scenario;
  the loss taxonomy in [`loss-anatomy.md`](loss-anatomy.md).
- **Produces:** the named branch with evidence, the routed lever, parked gap entries for Loop 7, or the
  the swap gate swap dossier.

## Fixture test (standalone, $0)

The evaluator must rediscover what the humans found, and must not invent problems. Five fixtures, all run:

- **Branch 1:** the pre-retarget `scored.json` plus transcripts of the relation cell → must propose the
  re-target that took +0.19 to +0.56. **Passed** - found 14 of 17 diluters plus three canceling swing
  items, named the sense-only cited files as the retarget pool, ruled out 3a/3b/4/5/6 with detector
  output, and refused the paid branch.
- **Branch 3a:** the positive replay is corpus-impossible (all on-disk and git-history transcript blobs
  scanned with the gate's own detector: zero surviving offload transcripts). The **negative** control ran
  instead and **passed** - confirmed the cell stands and was not fooled by the opencode parsing gap.
- **Branch 3b:** replay the blast `min_confidence` contract bug as a `sense-io.jsonl` - the agent passes
  0.7 off the schema's claimed default and loses gold that survives only at 0.3. `tool_use_audit.py` must
  flag a **contract defect on the product surface**, not agent error, and must not flag the deliberate
  narrowing.
- **Branch 6:** the haystack record → **passed**: swap with fresh mechanical evidence (the admission gate
  re-run), branch 5 correctly parked to Loop 7, no grinding on a dead seam.
- **Negative control:** a winning cell → must confirm and stop. **Passed** on the chatwoot cell. A
  separate negative-control attempt **redirected** instead: it caught a real provenance break, which was
  resolved the same day. Not over-tuning - every claim was verified.

## Built vs missing

- **Built:** the `bench-evaluator` agent with this taxonomy as its rubric, `tool_use_audit.py` (with
  `test_tool_use_audit.py` covering the contract-bug replay and the deliberate-narrowing negative side),
  `transcript_miss.py`, `resolve_oracle.py`, `relationship_audit.py`, `rescore_diff.py`,
  `loss-anatomy.md`.
- **Missing, deferred by decision 2026-07-30:** the budget-trim audit stays a **hand check**. It needs
  per-run gold cross-referenced against the full captured responses - real work - and it affects
  diagnosis *routing*, where the async tie-diagnosis review still puts a human in the loop. The 0.3/0.7
  gold check in [Authoring](03-b-authoring.md) was scripted first because it gates whether a WIN is real.
  **The cost of this deferral, stated plainly:** the check most able to falsify "send the RIGHT info"
  remains the easiest one to skip, so every sub-floor verdict this vertical rests on someone actually
  opening `sense-io.jsonl` before blaming the scenario.
- **First live use:** the first php-laravel cell that lands sub-floor.
