# LAWS - product-independent laws carried out of past campaigns

> **What this is.** The laws that survive a campaign because they are facts about scorers, language
> shape, and gold construction - NOT positions, verdicts, or product state. They carry **no re-run
> prohibition**. One law per bullet: statement, then its provenance. Bootstrap reads this file at
> slate composition and Authoring reads it at scenario time; it is a checklist, not a narrative.

## Admission and axis screening

- **IMPORT-LAW.** An axis is dead if an importer dump covers its gold: `grep -rln '<module
  path>'` is a one-command adversary, and any gold whose files import the subject's package
  scores ~1.00 for free (gold.py credits any-line pins). Screen every candidate axis with the
  import battery BEFORE any drafting.
  _Provenance: `repos.md` (private tree) §IMPORT-LAW PROBE (2026-07-17): gitea
  `context.Base` killed at cited_recall 1.00 by a 366-file import dump; same screen killed
  gitea Session + client.Client and photoprism entity.Session. LEDGER
  `ruling/import-law-v10-queue`._
- **K7 (IMPORT-LAW) - MECHANISM RETIRED 2026-07-31, law UNIMPLEMENTED.** It lived in
  `admission_gate.py` bar 3, which went with the seam gate (it ran before a scenario existed,
  and the gate backtested at 4 of 4 banked wins rejected). The law's recorded kills stand; its
  automation does not exist, and re-siting it inside Loop 3 - where a scenario is there to run
  it against - is an OPEN decision, not a settled one. The retired bar 3 carried the pkg-import
  row + K7 = import cover ≥ 0.8 AND retained prod files < floor. The retention exemption is
  load-bearing: a blanket K7 back-kills genuine retention golds. The four-anchor backtest
  fixture (gitea BALLAST/K7, pebble GRAY, doltdb BALLAST, attributes GRAY) is the regression
  guard - run it after any gate edit.
  _Provenance: LEDGER `ruling/bar3-k7-import-law` (2026-07-17); the blanket-K7 smoke
  back-killed pebble.Batch and was caught by the fixture before landing._
- **CONTROL BOUND - RETIRED 2026-07-31 with the eligibility stage; do not rebuild it.** The
  bound was `delta = sense - control` and `recall <= 1.0`, therefore `+0.50` requires
  `control <= 0.50`. The arithmetic is true and it bought nothing: it never once prevented a
  spend. Its ten-of-twenty figure was computed retrospectively over 384 runs that had ALREADY
  been paid for, and it was then cited for months as a saving. `control_bound.py` and its
  preflight gate are deleted.
  _Provenance: php-laravel `STATUS.md` correction (2026-07-30): "the arithmetic bound has NEVER
  prevented a spend... We paid for all of them."_
- **NO PREDICTORS BEFORE A SCENARIO EXISTS.** A win is CRAFTED for a repo and stack; it is not
  detected by a script beforehand. Anything that scores a repo before there is a task to score
  it against is a predictor, and predictors here have produced false kills, never a win. The
  boundary is sharp: a **killer** runs against a scenario that exists and may kill it; a
  **predictor** guesses before one does and is banned.
  _Provenance: owner ruling 2026-07-30. The php-laravel battery killed EVERY framework shape
  pre-authoring and reported "zero win candidates"; the falsifier it never ran, a baseline
  dry-run at giant scale, then measured the baseline dropping 9 of 15 gold. Four days, ~40
  ledger entries and 0 paid cells against one 4-minute run. Filed as decision-error Class 8,
  procedure as shield._
- **ONE-TOKEN-COVER KILLS ARE SIZE-CONDITIONED.** "A covering grep exists" is not "the agent
  runs it to closure". Valid as a kill at enumerable scale (≤15k symbols); above that it is a
  hypothesis and the dry-run decides.
  _Provenance: the same giant-scale run that overturned the Container kill._
- **MEMORIZATION DISCRIMINATOR.** Famous repos are memorized; the discriminator is obscure
  internals, and on a famous repo the module path gets rewritten (the `gitea.dev` pattern) so
  recall cannot come from training data.
  _Provenance: memory `feedback_memorization_confound_famous_repos`; the gitea module-path
  rewrite; grpc-go carried an open bar-5 memorization ruling for exactly this._

## Scenario and gold construction

- **RETENTION-SHAPE.** The grep-dark axis is retention: what retains X transitively, including
  behind interface-typed fields (`sense_blast` retained_via_interfaces). And axis-dead is
  NEVER repo-dead - a kill is the agent's failure to find the axis, not the repo's failure to
  have one.
  _Provenance: owner ruling 2026-07-15 (memory `project_retention_shape_law_gitea_dead`); the
  pebble retention win and PR #212 both came from digging past an axis kill._
- **GO-NAMING / MEMBER-NAME LAW.** Gold anchored on a member whose name greps cleanly via
  qualified tokens (`pebble.Batch`) dies at the $0 gate; pick subjects whose references are
  receiver-typed or implicit, not name-searchable. The qualified-token illusion is the same
  law from the caller side.
  _Provenance: pebble/Batch $0 gate kill 2026-07-14 (memory
  `project_3109_pebble_batch_gate_kill`); qualified-token-illusion, PR #210 window._
- **GOLD RAILS (all of them, every scenario).** Gold is one item per FILE; every SHOWN blast
  row survives BOTH min_confidence 0.3 AND 0.7; every chain is construction-site hand-audited;
  per-dep credits are hand-audited (the gold.py basename false-credit burned 28% of the
  corpus); gold is the grep-DARK residue - dark to the union of ALL grep patterns AND the
  import dump.
  _Provenance: memories `feedback_batching_law_scenario_design`,
  `feedback_gold_basename_false_credit`; LEDGER `stopper/gold-basename-false-credit`
  (106 of 384 runs re-scored); grpc-go groundwork's 7-file grep-dark residue is the worked
  example (`grpcgo-attributes-groundwork.md` (private tree))._

## Run count (maintainer ruling 2026-07-21, program-wide, supersedes the ×1 practice)

- **EVERY LLM ARM GETS 2 RUNS PER CELL. A THIRD WHEN THE TWO DISAGREE TOO MUCH.** Not just the
  headline arm - ollama, codex, kimi, and any arm added later. A cell benched ×1 is a SAMPLE and may
  never be published as a result; describe it if you must, but it cannot close a question, settle a
  win/tie/loss, or enter an article.
  _Provenance: the go step-7 retraction, `LEDGER.md` (private tree) `stopper/cheap-arm-x1-unsound`.
  Four ×1 cheap-arm cells were closed at $0 as "real model results" on a clean, cross-tabbed
  mechanism story. The maintainer refused the reading and ordered re-runs; two of the four then moved
  by half the scale (nomad +0.00 → +0.23, pebble +0.00 → +0.43) with an in-cell spread up to 0.86,
  and the published value had been the LOW sample in both. The mechanism story was not wrong so much
  as irrelevant: it explained one sample. Memory `feedback_runs2_settled_standard` gave ×2 for the
  stable headline arm; this ruling extends the floor to every arm and adds the ×3 escape hatch._

## The per-repo loops (folded in 2026-07-31 from the retired Loop 3 parent map)

Authoring, Run and Diagnosis were four sub-stages under one parent. Eligibility was deleted and
the other three became loops in their own right, so the laws they all shared live here.

- **ONE REPO AT A TIME, TO A VERDICT.** No second repo's scenario is authored while the first is
  mid-diagnosis. Each repo is checkpointed in `.loop-state.json`; a parked repo resumes, never
  restarts. Loops 4/5/6 stay aggregate and downstream, on frozen won scenarios only.
  _Provenance: Rails won its cells one repo at a time with no loop system at all, while later
  verticals produced protocol across four repos and no runs. The known cost, stated so nobody
  rediscovers it: the first repo pays the whole instrument-debugging tax._
- **THE LOOP CANNOT RECORD A LOSS.** It wins, parks, or hands up a swap with the numbers
  attached. A sub-floor verdict is a routed lever, never a loss.
- **RUN FIRST, EXPLAIN AFTER** (ruling 2026-07-19, supersedes any ordering rule that conflicts).
  A kill or hold reasoned without a run is not a verdict. A session ends with a run artifact or
  the lane parks; ledger and protocol writing is overhead, never the deliverable. Rulings go up
  for money and repo-slot life-or-death only, no option menus. Delegation aims at the hunt, not
  the paperwork.
- **TRY HARDER** (ruling 2026-07-13). The default stance is "there is an unfound win axis here;
  prove otherwise", and it starts at proposal time, not at tie-diagnosis time. When a proposal
  reaches for "honest boundary", "tie" or "ballast", the required response is to deepen the
  contract hunt and widen the pool. No loop offers a boundary framing; it exhausts axes first.
- **A STAGE IS SELF-CONTAINED.** Its input is a file on disk, its output is a file plus a ledger
  entry, never a conversation carried forward. If a loop needs something the previous one did
  not write down, that is a defect in the previous loop, not a reason to widen this one.
- **WHAT THE PER-REPO LOOPS DO NOT OWN.** The slate and its conventions sweep belong to
  bootstrap. Product fixes are parked for Loop 7 and never made mid-vertical. The agent survey
  is read in aggregate by Loop 5, never per run. Cross-model confirmation is Loop 4.

## Process laws already wired elsewhere (pointers, not duplicates)

- **STOPPER law** (measurement-instrument bugs stop the line): `how-to-run.md` gate table +
  `ledger_check.py` rule 10 + `bench/lib/rescore_diff.py`. It halts all three per-repo loops
  immediately: no new cells, no verdicts, no swap dossiers, no kills, until the owner rules.
- **Gate-fidelity law** (the adversary probe must be stronger than the production arm) and the
  **agent dry-run gate before any paid bench**: `00-bootstrap.md` / `02-repo-run.md` + memory
  `feedback_agent_dryrun_gate_before_bench`.
- **Instruments that carry** (bench/lib, not vertical docs): `screen.py`, `mcp_probe.py`,
  `rescore_diff.py`, `select_final.py`, `scaffold_check.py`.
