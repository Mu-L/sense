# LAWS - product-independent laws carried out of the go-legacy campaign

> **What this is.** The distillation ordered by the go-vertical reset (LEDGER
> `ruling/go-vertical-reset`, 2026-07-17): the laws that survive the campaign because they are
> facts about scorers, language shape, and gold construction - NOT positions, verdicts, or
> product state. Everything else from that campaign is history kept outside this folder, and per the
> owner's explicit amendment carries **no re-run prohibition**. One law per bullet: statement, then legacy
> provenance. The fresh vertical's Loops 2/3 read this file at slate composition and
> scenario authoring; it is a checklist, not a narrative.

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
- **CONTROL BOUND.** The admission gate is control ≤ 0.25 AND sense ≥ 0.75, and the control is
  MEASURED (`bench/lib/control_bound.py` probes at the wall), never predicted - three
  predictions died in one day. The baseline is a control, not an adversary.
  _Provenance: corpus mining 2026-07-16, all 384 recorded runs, $0
  (memory `project_grep_hostility_catalog_research`)._
- **ARITHMETIC KILL FIRST.** Sense is a floor-raiser (sense ≈ 0.68 + 0.25·base across the
  corpus); delta ≥ +0.50 REQUIRES base ≤ 0.50, so a cell whose control probes above 0.50 is
  dead before it runs. Check the arithmetic before any spend.
  _Provenance: same corpus mining; 10 of 20 paid cells in the recorded corpus were
  arithmetically unwinnable._
- **MEMORIZATION DISCRIMINATOR.** Famous repos are memorized; the discriminator is obscure
  internals, and on a famous repo the module path gets rewritten (the `gitea.dev` pattern) so
  recall cannot come from training data.
  _Provenance: memory `feedback_memorization_confound_famous_repos`; gitea module-path rewrite
  in the legacy campaign; grpc-go carried an open bar-5 memorization ruling for exactly this._

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

## Process laws already wired elsewhere (pointers, not duplicates)

- **STOPPER law** (measurement-instrument bugs stop the line): `how-to-run.md` gate table +
  `ledger_check.py` rule 10 + `bench/lib/rescore_diff.py`.
- **Gate-fidelity law** (the adversary probe must be stronger than the production arm) and the
  **agent dry-run gate before any paid bench**: `02-repo-admission.md` /
  `03-per-repo-convergence.md` + memory `feedback_agent_dryrun_gate_before_bench`.
- **Instruments that carry** (bench/lib, not vertical docs): `repo_screen.py`, `control_bound.py`, `mcp_probe.py`, `rescore_diff.py`,
  `select_final.py`, `bootstrap_check.py`.
