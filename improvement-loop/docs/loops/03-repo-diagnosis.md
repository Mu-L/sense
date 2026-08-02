# Loop 3 - Repo diagnosis

> Third of the three per-repo loops ([authoring](01-repo-authoring.md) →
> [run](02-repo-run.md) → diagnosis). The laws all three share live in
> [`campaign-laws.md`](campaign-laws.md) and are not repeated here.

**RUN:** the credit table, the fingerprint/cycle counter, `transcript_miss.py`,
`rescore_diff.py`. **DECIDE:** which branch the cell belongs to, and whether a probe's kill is
real - a probe graded at mention level is not a kill. Claims leaving a DECIDE step are quoted
output or labelled assumptions ([`campaign-laws.md`](campaign-laws.md), RUN vs DECIDE).

## Goal

Read every run this repo produces and turn it into the next move: the material for a better scenario,
or exactly one named cause with a detector behind it. Exit state: the cell goes back to
[Authoring](01-repo-authoring.md) with a lever, forward to Loop 4 as a confirmed win, parked as a
product-gap hypothesis for Loop 7, or escalated as a swap with the numbers attached.
**This loop cannot write a loss.**

## Two reads, and they run on different inputs

This loop runs on **every** run, not only on a sub-floor verdict, because the validation run exists to
be read here.

| | The struggle read | The taxonomy |
|---|---|---|
| Runs on | every run, including a win | a scored sub-floor verdict |
| Asks | where did the baseline have a hard time, and what did Sense reach that it did not | which of six causes produced this number |
| Vertex | the `bench-struggle-read` agent | the `bench-evaluator` agent |
| Anchored on | `credit_table.py` (per gold item, both arms) | the branch detectors, each producing output |
| Produces | gold rows and a shape for the next draft | one named branch and one lever |

**The struggle read is the adversary probe's disclaimer, measured instead of self-reported.** Authoring's
probe says what it could not establish; this read shows it. The items the baseline missed, and what it
spent its move budget on before missing them, are the next scenario's material. Keep it an analysis, but
keep it anchored: the credit table is the mechanical input, and a claim about the run that is not
traceable to it is prose.

**Do not route the struggle read through `bench-evaluator`.** That agent is defined to emit a one-line
verdict block, to stand only on script output, and to refuse anything at or above the win bar. Those
properties are why it is trustworthy on a loss, and they make it the wrong instrument for an open-ended
read whose output is scenario material. `bench-struggle-read` is the separate vertex: it splits the
gold into diluters, sense-only reach and shared misses, quotes the move the baseline spent instead of
each miss, checks whether it exhausted its budget, and reports what it did NOT examine. It names no
branch and issues no verdict.

## Six crafting cycles per repo, then swap

A repo may go round authoring → run → diagnosis **six times**. The counter advances on every cycle and
**resets only on measured movement in the credit table**, never on an edit alone: six cosmetic re-shapes
would otherwise burn the budget and fire the swap on a repo nobody actually pushed on. `vertical-loop.sh`
keeps the count in `.loop-state.json` under `<repo>#cycle`, and detects movement with
`credit_table.py --fingerprint`, which hashes the sense-only and neither rows only. Gold churning in and
out of the diluter bucket is deliberately NOT movement - counting it would reset the counter forever and
the swap would never fire.

At six with no movement the swap fires with the dossier, and the slot takes **its own declared
backup** from `slate.json`, never the next repo on the slate. The replacement enters
[Authoring](01-repo-authoring.md) with the counter reset to zero.

## Product duties (per Sense surface)

- **blast - the budget-trim audit, before blaming the scenario.** On every sub-floor verdict, check
  `sense-io.jsonl` against the gold to see whether the blast budget trimmed a dependency the gold needed.
  "Send the right info" is a hypothesis this loop is uniquely positioned to falsify, and today it is the
  check most often skipped.
- **Tool contracts and response shape - double-file every misuse finding.** The harness may compensate
  mid-campaign (arm prompt, run script), but the product-side cause - contract text, a default, a hint,
  empty-result guidance - is ledgered for Loop 7. **Compensation without the ledger entry loses the
  product signal**, which is the whole reason this loop exists.
- **Genuine product gaps are parked, never fixed mid-vertical.** File the hypothesis with its evidence and
  move on; the spike belongs to Loop 7.

## Identity

- **Character:** judgment, adversarial on the taxonomy half. That half is one keystroke from
  game-until-green if its separate evaluator is removed.
- **Unit of work:** one run on one cell - the struggle read on any of them, the taxonomy on a
  scored sub-floor verdict.
- **Position:** consumes every run's transcripts, credit table and capture from
  [Run](02-repo-run.md); produces scenario material and a routed lever back to
  [Authoring](01-repo-authoring.md), a confirmed win forward to Loop 4, a parked gap for Loop 7, or a
  swap dossier that hands the slot back to the backup admitted at bootstrap.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | the session agent that ran the cell | it may propose, it may not conclude |
| Struggle read | the `bench-struggle-read` agent - runs on EVERY run, including a win | returns scenario material, never a verdict or a branch |
| Evaluator | the `bench-evaluator` agent - adversarial, separate, this taxonomy as its rubric | stands only on mechanical verifier output; anything else it says is prose |
| Mechanical verifier | `pergroup.py`, `scorer.py`, `transcript_miss.py`, `tool_use_audit.py`, `resolve_oracle.py`, `relationship_audit.py`, `rescore_diff.py` | each branch below names its own |
| Rubric judge | pinned, distinct from the evaluator | |
| Human | none - the loop routes, swaps and closes on its own | every branch must still cite its detector's output; that is what replaces the reviewer |

## The taxonomy

Dispatch through the branches **in this order** - cheapest lever first - and conclude nothing but WIN
until every branch is exhausted with evidence. Default stance: *there is an unfound win axis here; prove
otherwise.*

| # | Cause | Detector | Lever | Cost |
|---|---|---|---|---|
| 1 | Gold mis-curation | per-dependency tally from `scored.json` | re-target gold, then re-score the existing transcripts with `scorer.py` | $0 |
| 2 | Scenario shape wrong: assembly cost too low. NOT "fan vs chain" - both fail their own way (a fan whose members are each locally decidable is batchable; a chain that is short and all-nameable is two-hop-greppable, the tied mastodon draft). Also satisficing-friendly prompts. | tally pattern + transcript read | re-author so the answer needs `path:line` the baseline cannot afford to pin → re-bench | paid |
| 3a | Sense returned it, the agent dropped it | `transcript_miss.py` (cited-not-returned, fallback reads, empties), `mcp_count` | fix the harness or the output shape upstream - **never the scorer** | $0 |
| 3b | The agent misused the tool: wrong tool for the question shape, wrong params, abandoned after one empty result, ignored hints | `tool_use_audit.py` over `sense-io.jsonl` | product meta-surface fix (contract, hint, setup) ledgered for Loop 7; harness compensation allowed meanwhile | cheap |
| 4 | Judge or scorer error | hand-audit the per-dependency credits (basename false-credit guard), `relationship_audit.py` | fix the scorer **with a guard test** | $0 |
| 5 | Genuine product gap | `resolve_oracle.py` fact-check on known-true edges | append to that vertical's gap list (`verticals/<stack>/results/loopA-gaps.md`), which Loop 7 reads as a set via `verticals/*/results/loopA-gaps.md`, **parked** for Loop 7's window | $0 now |
| 6 | Seam measurably nonexistent | existence measurement on the index. AXIS-DEAD IS NOT REPO-DEAD, and an adversary probe graded at MENTION level is not a kill - re-grade it at `path:line` before believing it | swap, with the numbers attached and the axes tried named | $0 |

Branch 6 is the one branch that ends the repo, and it is a measurement, never an impression: nothing
screens seams before a scenario exists any more, so the existence check is made here or nowhere.
Branch 5 is the only branch touching product truth, and it is asynchronous by law.

Branch 3b has a required negative side: a run where the same parameter was a deliberate,
prompt-justified narrowing must **not** be flagged. A detector that cannot tell misled from intentional
is over-tuned.

## Stop conditions

- **Success (struggle read):** the credit table read, the baseline's misses named with what it spent
  its budget on instead, and that material handed to Authoring as the next draft's gold candidates.
- **Success (taxonomy):** one branch named, its detector output recorded, and the lever handed to the
  owning loop. A branch chosen without detector output is not a diagnosis.
- **Budget:** the struggle read and every branch except 2 are $0; branch 2 re-enters the paid path
  through Authoring and Run, never directly. Park with the analysis on disk.
- **Failure:** all branches exhausted with evidence → swap, with the dossier: what was tried, the
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
| none | - | - | - |

The swap gate and the tie-diagnosis review were REMOVED 2026-07-31. A swap now fires on the
mechanical condition alone - six crafting cycles with no movement in the credit table, or branch 6
measuring the seam absent - and the dossier is written for the record rather than for approval.
**A branch ruled out without its detector's output was always invalid; now it is also uncaught**,
so the evidence requirement is the whole of what keeps this loop honest.

## State / memory

- `verticals/<vertical>/results/` - read-only here, plus the branch analysis written alongside the cell.
- The per-vertical gap list `verticals/<stack>/results/loopA-gaps.md`, append-only, for branch 5; Loop 7 globs `verticals/*/results/loopA-gaps.md`, which still finds an archived vertical.
  It is deliberately NOT per-vertical: a product gap is product state, Loop 7 reads it across
  stacks, and cross-stack recurrence is the strongest signal in it. A vertical that gets reset
  leaves its gaps behind rather than taking them.
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
  the swap dossier.

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

- **Built:** the `bench-evaluator` agent with this taxonomy as its rubric, the `bench-struggle-read`
  agent (the every-run read), `credit_table.py` + `test_credit_table.py` (its mechanical input and the
  movement detector), the cycle counter and swap trigger in `vertical-loop.sh`, `tool_use_audit.py` (with
  `test_tool_use_audit.py` covering the contract-bug replay and the deliberate-narrowing negative side),
  `transcript_miss.py`, `resolve_oracle.py`, `relationship_audit.py`, `rescore_diff.py`,
  `loss-anatomy.md`.
- **BUILT 2026-08-01 (was deferred 2026-07-30):** the budget-trim audit is now
  `bench/lib/context_cost_audit.py`, pinned by `test_context_cost_audit.py`. It reads
  `sense-io.jsonl` + both arms' `scored.json` and reports injected context per tool,
  the cache-read delta, the **re-read multiplier** (a response is injected once and
  re-read on every later turn, so its cost is size x turns-remaining), and ranked trim
  candidates. It runs from the `report` phase whenever `cost_parity.py` prints a MISS.
  The deferral's own stated cost came true: on 2026-08-01 the rails cell won at a 26%
  premium with no instrument to ask why, and the finding went to the stopper lane
  instead of harvest. The superseded reasoning is kept below.
- ~~**Missing, deferred by decision 2026-07-30:**~~ the budget-trim audit stays a **hand check**. It needs
  per-run gold cross-referenced against the full captured responses - real work - and it affects
  diagnosis *routing*, which no longer has a human reviewing it at all. The shown-over-MCP
  gold check in [Authoring](01-repo-authoring.md) was scripted first because it gates whether a WIN is real.
  **The cost of this deferral, stated plainly:** the check most able to falsify "send the RIGHT info"
  remains the easiest one to skip, so every sub-floor verdict this vertical rests on someone actually
  opening `sense-io.jsonl` before blaming the scenario.
- **First live use:** the first php-laravel cell that lands sub-floor.
