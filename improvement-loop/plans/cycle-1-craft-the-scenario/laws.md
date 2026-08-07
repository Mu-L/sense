# LAWS

The standing laws of this bench. One line each, statement only. They bind every phase and
they are not re-argued at runtime.

## What may be concluded

- **RUN vs DECIDE.** A RUN step (index, gates, stamping, rendering, `ledger_check`, the phase
  order) is followed exactly - no substitute, no hand-rolling what a script does; if it looks
  wrong, say so and stop. A DECIDE step (anchor, gold, the pay call, routing) is judgment, and
  every factual claim leaving it is quoted command output or is labelled an assumption.
- **QUOTE IT OR YOU HAVE NOT VERIFIED IT.** Before stating a finding, quote the output that
  shows it. Negative claims - cannot, no, never, dead, nonexistent, not reached - need a
  second, DIFFERENT probe before they are stated at all.
- **NO PREDICTORS BEFORE A SCENARIO EXISTS.** A win is crafted, never detected by a script
  beforehand. A killer runs against a scenario that exists and may kill it; a predictor
  guesses before one does and is banned.
- **A METRIC WITH A FREE PARAMETER IS SWEPT, NEVER EVALUATED.** Report the whole curve, not the
  chosen point. A fixed per-miss cost was measured at 1/3, reported and recommended before the
  sweep was run; the sweep showed `delta = cost * 1.8` on that cell - monotone, no optimum,
  0.0625 giving +0.112 and 1.0 giving +0.800 - and that 1/3 sat at the edge of a window derived
  FROM the cell it was meant to judge. If turning the knob one way always makes us look better,
  it is a dial and not a measurement. A blend whose delta is a convex combination of its terms
  is safe by construction: it lands BETWEEN them and cannot manufacture a win neither supports.
- **WE SCORE THE HEADLINE MODEL FORWARD (ruling, 2026-08-04).** A number banked on a RETIRED
  model is historical and does NOT constrain a scoring change. The killer "it must hold on the
  banked cell" was treating a record on a retired model as a live constraint. Killers still
  bind on every cell benched on the headline model.
- **THE FIRST MEASUREMENT IS A REAL BASELINE ARM.** Nothing kills a draft before a two-arm run
  has answered its question at a real wall. Every cheaper substitute this bench has built - the
  token-cover battery, the import screen, the scored adversary probe, the hand-run read-cost
  census - was measured wrong, and each one killed a repo that a benched scenario then won.
  A hand estimate of a baseline is not evidence about a baseline.
- **PRECISION RANKS, IT NEVER KILLS.** Dependents divided by grep hits is the one profile
  figure that has separated wins from ties across three verticals: 0.078 preceded a +1.00 cell,
  0.78 was a deliberate control tie, and a cell near 0.94 handed its baseline 17 of 18 rows in
  one grep. Use it to order candidates. It orders; the run decides.
- **THE AUTHORING CYCLE IS UNATTENDED AND BOUNDED.** A routed lever re-enters authoring
  immediately, up to six cycles on one repo, then parks. Your artifact is not a report filed
  for later: the next authoring agent reads it within the minute and it is the only thing that
  agent has to improve on. A verdict with no named rows and no route is a wasted cycle.
- **AN OCCURRENCE-LIST QUESTION CAN WIN, AND THE SESSION IS PART OF WHY.** "Find every place
  that depends on X" is the shape of the banked mastodon cell: baseline 7 of 23, sense 19 of 23,
  +0.53. So the kind is not disqualifying, and a law saying otherwise was written on 2026-08-03
  from five failures and retracted the same day against that cell. What differs is the SESSION:
  the banked ask sits at step 4 of 7, so both arms arrive with wall and context already spent,
  while the five failures asked it cold at step 2 of 2 and the plain arm scored 0.81, 0.16,
  0.09, 1.00, 1.00. Budget pressure is a candidate cause, under test, not yet established.
- **"WHAT HOLDS THIS" NEEDS A RING, AND RUBY HAS NONE.** The strongest measured kind is +0.58 to
  +1.00 on four Go cells, but it needs a non-empty retention ring: `ring_sweep.py` measured ZERO
  rings on both Ruby repos swept (2026-08-03, top twelve anchors each, mastodon and the banked
  rails winner). A kind that cannot fire in the stack is not an option, it is an unanswerable
  question.
- **AIM AT THE WINDOW, NOT AT THE OPPOSITE OF THE LAST FAILURE.** Both conditions at once: the
  plain arm at or below 0.50 AND ours at least 0.50 above. Correcting only the last cycle's
  complaint lands on the far side and burns a cycle proving it. Read every previous cycle, not
  the latest one.
- **A FREE ROW IS A GIFT TO THE BASELINE, AND THE RANKING NAMES THEM.** Citation rate per
  gold row across every run (`inverse_frequency.py`) sorts gold into three classes: rows the
  baseline never cites and sense always does (the discriminator), rows every run cites (free,
  and they put a floor under the baseline no wording can lift), and rows nobody ever cites
  (hard, or unreachable if the blast payload does not carry them). Measured on mastodon at
  n=5: ten of twenty-three rows free, two unreachable across two models, one row
  discriminating. Scope the ranking to the HEADLINE model - mixing generations reorders the
  middle - and do not re-gold below ~5 runs per arm: a row that looked like a perfect
  discriminator at n=2 was 4/5 at n=5.
- **ITERATE ON THE QUESTION, KEEP THE ANCHOR.** A sub-floor cell is a question that the
  baseline's route answered, not a dead contract. Measured both ways: a tied `serialize_payload`
  axis became a winning teardown audit on the same repo, and a dead dispatch axis became a
  winning retention audit on the SAME TYPE. Re-scouting a new contract discards the only
  measured thing the loop owns.
- **TRY HARDER.** The default stance is "there is an unfound win axis here; prove otherwise",
  starting at proposal time. No phase offers a tie, a boundary framing or ballast; it deepens
  the contract hunt and widens the pool first.
- **THE LOOP CANNOT RECORD A LOSS.** It wins, parks, or hands up a swap with the numbers
  attached. A sub-floor verdict is a routed lever.
- **AXIS-DEAD IS NEVER REPO-DEAD.** A screen bounds the axis it measured and nothing else, and
  an adversary probe graded at MENTION level is not a kill - re-grade it at `path:line` first.
- **RUN FIRST, EXPLAIN AFTER.** A kill or a hold reasoned without a run is not a verdict. A
  session ends with a run artifact or it parks; protocol writing is overhead, never the
  deliverable.

## Scenario and gold

- **THE DISCRIMINATOR IS CITATION COST, NOT DISCOVERY.** The metric is cited recall at
  `path:line`. A baseline greps a filename cheaply; what it cannot afford is opening sixteen
  files to pin sixteen lines. Curate gold from the scattered periphery, keep the contract
  centre as non-scoring anchors, demand `path:line`, and never judge a shape by whether grep
  finds the file.
- **A CORRECTNESS ORACLE MUST NOT BE SATISFIABLE BY THE ROUTE.** Scoring a citation as
  on-target when the audited token appears on the pinned line is a restatement of "did you
  find this by grepping for that token", and it scores the grepper 100% where the structural
  arm scores 0% on rows where BOTH are right: measured 67.4% baseline against 48.4% sense over
  10 transcripts, with the difference being `def attached_to_preview_card` (sense) against
  `Status.joins(...)` (baseline) one line below it. If citation correctness is ever scored the
  oracle is a RANGE - does the pin fall inside the symbol carrying the dependency - never a
  text match, and its circularity must be answered before it runs.
- **A GOLD ROW MUST BE CITEABLE IN THE FORM THE ASK DEMANDS.** The headline scores `path:line`,
  so a row for an artifact the ask says to ADD has no line to cite and can never score. Measured
  on the banked Status control: its step 7 asks for "the specs/fixtures that must be updated or
  added", and `spec:status` / `spec:post-svc` scored **mentioned 2/2 in 8 of 10 runs, cited 0/2
  in 10 of 10**, both arms, both models, since June. The arms did the work and named the files
  in the only natural form (`spec/models/status_spec.rb`, no line). Before golding a row, say
  out loud what the ask asks for it and what a correct answer looks like: if a line number would
  be meaningless in that sentence, the row is mention-only or it does not belong in gold.
  A never-cited row is NOT automatically hard or unreachable - check the ask's wording before
  checking the blast payload.
- **A GOLD ROW SHOULD COST A READ - AND THE RUN IS WHAT CHARGES IT.** `grep -rn` prints
  `path:line`, so a row whose dependency IS the token occurrence can be free: a gold set whose
  eighteen rows were each a single `Setting.<key>` read scored 17 of 18 to a baseline in nine
  tool calls and 157 seconds. Prefer rows that must be opened. But this is a hand estimate of a
  baseline, it is not a measurement of one, and run as a per-row census it killed four shapes
  on a repo that banks +0.53. It is an authoring preference. It may not kill a draft, and it
  never belongs in a phase that runs before the two-arm bench.
- **THE ASK NAMES THE MECHANISM, NEVER THE INVENTORY.** Describing HOW a dependent hides is
  the task; listing WHERE dependents live is the answer. "A shared concern, an association, a
  service that derives something from the model rather than naming the class plainly" is a
  mechanism. "Validation rules, background workers and schedulers, response headers, the
  data-retention policy, the onboarding follow list, service-discovery documents, static
  assets served from a stored value, and the historical data migrations" is an inventory: nine
  categories that between them name almost every gold row's job, and the baseline transcript
  shows it working down them. The neutrality gate reads TOKENS - paths, symbols, counts, tool
  names - so a semantic inventory passes it clean. This one is caught by hand or not at all.
- **GOLD RAILS, all of them, every scenario.** One item per FILE; every blast-sourced row
  appears in the output the agent is SHOWN over MCP; every chain is hand-audited at its
  construction site; per-dependency credits are hand-audited, never taken from a script tally;
  gold is the scattered periphery, selected for CITATION COST and never for token-darkness.
- **TOKEN-DARKNESS IS NOT THE MECHANISM.** A dependent whose line writes the anchor token is
  still gold. Measured: a banked cell scored `dependents` +0.72 (baseline 2.5 of 16, n=2) on
  a gold set where 16 of 16 files carry the token and one `grep -rl` returns all of them,
  while a cell curated FOR darkness put the baseline at 10 of 16 and died on its ceiling.
  Darkness was one mechanism on one repo; it was read as the law and it inverts the result.
- **NO GREP SCREEN IS A GATE, IN ANY FORM.** Not a token cover, not an importer dump, not a
  per-row coverage census, not a darkness scan. Every one of them answers "a covering grep
  exists", and the metric is "the agent runs it to closure at `path:line` inside its wall" -
  a different question that only a run answers. The import screen in its old home rejected four
  of four banked wins. Two verticals disagree on the mechanism outright: a Rails cell banks
  +0.53 with 16 of 16 gold files carrying the anchor token, and a Go cell banks +1.00 on
  dependents that never name the type at all. Record what a grep prints in the yaml header;
  never let it decide.
- **MEMORIZATION IS A GOLD-TIME CONSTRAINT, NEVER AN ADMISSION VERDICT.** A famous repo is not
  disqualified; a memorized gold row is. Every target is churn-dated after the model snapshot
  or is a line-level structural fact the model cannot recite.
- **THE CONTROL BOUND IS RETIRED. DO NOT REBUILD IT.** `delta = sense - control` with recall
  capped at 1.00 is true arithmetic that never once prevented a spend.

## Spending

- **THE MINI-BENCH COMES FIRST, BOTH ARMS, ONCE.** The two-step probe scenario is run before
  any seven-step session is written, at a real wall, unscored. It passes only when the baseline
  holds at or below 0.50 of `dependents` AND sense beats it by at least +0.50. The precedent it
  reproduces: a banked +1.00 cell was authored in full only after its mini run measured
  baseline 0 of 5 against sense 5 of 5.
- **THE VALIDATION RUN IS UNSCORED, BOTH ARMS, ONCE**, at the cell's real wall, before
  anything paid. A hand-grep is not a run. A baseline-only validation tells you the scenario
  is hard, not that Sense reaches.
- **IF THE BASELINE ASSEMBLES THE SET, DO NOT PAY.** That holds regardless of how good the
  scenario looks.
- **THE BAR IS +0.50 ON A GOLD GROUP, AND RECALL CAPS AT 1.00.** So a group whose baseline
  already sits at B can never deliver more than `1.00 - B`: a baseline holding 0.625 makes
  +0.50 unreachable before the pair runs. Discriminating is not the same as clearing the
  bar - check the arithmetic against the floor, not against zero.
- **EVERY ARM GETS 2 RUNS PER CELL**, a third when the two disagree too much. A cell benched
  x1 is a sample: it may not close a question, settle a win or a loss, or enter an article.
- **CANNOT-FINISH-AT-BUDGET IS A RESULT.** Never raise the watchdog to rescue a stalled arm. A
  failed exam is not an invalid exam - stop and read the transcripts.
- **ONE SENSE RETRY, NEVER A LOOP - AND THE REPLACED RUN IS PARKED, NEVER SCORED.** The sense
  arm runs first and its wall IS the baseline's budget (`paired sense wall x 1.2`), so a
  watchdogged sense run is a *censored* wall - it says "at least the ceiling", not what the
  work took - and nothing honest can be derived from it. That, and only that, is why the
  retry exists: to recover an uncensored wall, never to buy Sense a second draw at scoring.
  Two consequences bind.
  **The retry is sense-only.** A baseline that runs out of `sense_wall x 1.2` is the
  measurement, not a malfunction - it is the win condition itself, and 12 such runs are
  where cycle 2's result actually lives. Never "fix" a timed-out baseline by re-running it.
  **The replaced run is parked** (`park_superseded`, lib/bench-paths.sh: `run-N` ->
  `failed-run-N`). Scoring both the run you declared unfit and its replacement is a double
  count - it put 3 sense runs against a 2-run baseline in two cycle-2 cells, with the
  superseded 0.0 still in the mean.
  **The cap is load-bearing.** Because the replaced run is parked, raising the retry cap
  above one would let Sense re-roll until it got a clean run and quietly delete every
  failure on the way - the exact bias this loop exists to avoid. One retry: a second
  failure STANDS as the result and is scored. Do not raise it. If a cell needs three
  attempts, that is the finding.
- **COSTING MORE IS A PRODUCT FINDING, NOT A STOPPER.** A cell that wins its discriminator at
  a cost premium says the arm is not reaching at parity, which is a lever, not a halt. The
  cost axis is priced tokens - every billed token in input-token equivalents - never the
  uncached remainder.

## The instrument

- **MCP IS THE ONLY SURFACE.** Every check that queries Sense goes through the MCP server,
  never the CLI. The CLI diverges by design - different defaults, caps and budget - so a CLI
  call measures a surface no arm touches.
- **STOPPER.** A bug in anything whose output becomes a number that decides something halts
  the line: no new cells, no verdicts, no swaps, no kills, until the human rules. Continue
  without a ruling only on a re-score diff proving zero impact. A mitigation is not a
  resolution.
- **THE SIMULATED ADVERSARY PROBE IS RETIRED.** It never estimated the benched arm in either
  direction: on the parked cell it reached 4 of 16 where the arm reached 10; on the banked cell
  it reached at most 7 of 16 where the arm reached 2.5, and a leaked run reached 15 of 16 over
  that same arm. No correction factor fits points that disagree by 2.5x one way and 0.36x the
  other. The mini-bench measures the same thing with a real baseline at a real wall and costs
  about the same. Do not rebuild it.
- **ONE REPO AT A TIME, TO A VERDICT.** No second repo's scenario is authored while the first
  is mid-diagnosis. A parked repo resumes from state, it never restarts.
- **A PHASE IS SELF-CONTAINED.** Its input is a file on disk and its output is a file plus a
  verdict, never a conversation carried forward. If a phase needs something the previous one
  did not write down, that is a defect in the previous phase.
- **WHAT A PER-REPO PHASE DOES NOT OWN.** The slate belongs to bootstrap. Product fixes are
  parked, never made mid-vertical. The agent survey is read in aggregate, never per run.
  Cross-model confirmation is downstream.
- **NO HUMAN GATE IN THE PER-REPO PHASES.** They author, spend, diagnose and swap on their
  own; what replaced each removed gate is mechanical and named in the plan that runs it.
- **INSTRUMENTS CARRY, DOCS DO NOT.** A check that must survive the vertical is a script in
  `bench/lib/`, never a paragraph in a page.
