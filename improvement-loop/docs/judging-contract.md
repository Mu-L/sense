# The Judging Contract - hard rules so judging cannot diverge

> **Why this exists.** The manifesto §9 already said "objective cited-recall is THE HEADLINE; the
> reference-blind judge LIES; never headline it." The harness diverged anyway: `reporter.py` ranked the
> report by the blind `fairness` composite (0.55·llm_quality), and never computed average recall. Result:
> the default report rated the **Chatwoot +0.44 recall win as a −0.018 "tie"** and showed Sense tying on
> 12/13 repos and *losing* raix. **Prose principles don't bind code.** Each HARD RULE below is therefore
> backed by code + a guard test; a violation must red CI, not drift silently.

## Hard rules (enforced)

1. **The headline is objective `cited_recall`; the blind composite is RETIRED.**
   `cited_recall` (location-pinned) ranks the report - it is where Sense's structural advantage
   concentrates (mean margin +0.28 vs baseline; the blind composite showed only +0.017, a ~16×
   understatement that silently favored the baseline). The reference-BLIND `scenario_quality`/`fairness`/
   `llm_quality` composite is NO LONGER RENDERED in the report. A single fair blended number, **B-score =
   `0.55·cited_recall + 0.25·related_recall + 0.20·grounded_precision`**, accompanies it - every term an
   objective/reference-aware axis Sense wins on merit; NO efficiency term (it dilutes and is not a
   correctness axis - report it separately, gated at held recall). `gold_f1` is DROPPED (its precision
   punished Sense for real beyond-gold finds). `citation_grounding` is excluded+renormalized in
   `fairness.compute` when the repo checkout was missing (a harness gap must not penalize the arm that
   cites more). These are FAIRNESS corrections: the prior scoring was biased toward the baseline.
   *Enforced:* `reporter.build_aggregate` sorts by `(avg_cited_recall, avg_relationship_audit)`, emits
   `cited_recall` + `b_score` as the leading columns, and renders neither `fairness` nor `llm_quality`.
   *Guards:* `test_reporter.py::{RankByRecallHeadline, BScoreTest, FormatMarkdownSmokeTest}`.

2. **Recall is measured against the AUTHORED must-find set (gold), never the judge's opinion of
   completeness.** A reference-blind quality score may not stand in for completeness.
   *Why:* the blind judge is omission-blind - it rates a 60%-recall answer ~0.84.

3. **The completeness judge must be reference-aware.** `relationship_audit` grades the whole answer vs the
   gold `relation` fields. Curate gold with `relation:` on every discriminator target so this runs.

4. **A claim is credited only if grounded** (cite resolves AND the stated relation matches the authored
   one); confident-FALSE assertions are penalized, not rewarded.
   *Enforced:* `relationship_audit` grades each covered gold item tri-state - `covered` / `related` /
   `contradicted`. A `contradicted` item (a confident relation claim that conflicts with the authored
   `relation`, e.g. the forem baseline calling the non-rendering `LiquidEmbedExtractor` a "renderer") is
   never also `related`, and drops `grounded_precision = 1 − contradicted/covered`. Silence/vagueness is
   omission (hurts recall), NOT fabrication. The reporter leads with `grounded_precision` + a raw
   contradiction count. The mechanical `citation_grounding` (file/line resolution) stays as the precision
   floor; the authored `relation` is the ground truth for the semantic layer - no code snippet or extra
   judge call needed (it folds into the one reference-aware call).
   *Guard:* `bench/lib/test_relationship_audit.py::ContradictionTest`,
   `bench/lib/test_reporter.py::FabricationAggregateTest`.

   *Fact channel (prove-then-encode).* When digging proves a hard fact about the code, encode it in gold so
   the judge never re-litigates it: correct the `relation:`, and for confusable items add an OPTIONAL
   `disambiguator:` note (a fact about the CODE, often what it is NOT - e.g. discourse `update_post_image`
   "NOT a URL rewriter", forem `UnpublishAllsQuery` "a READ query, NOT the bulk-unpublisher"). The audit
   renders it to the grader as `verified_fact:`. HARD rule: a fact enters gold ONLY after verification
   against source at the pinned commit (never from a model's guess), it describes the CODE not either
   arm's output (so it stays symmetric), and it is never rendered to the agent. Calibration: a contradiction
   requires the answer to ASSERT a wrong role - bare enumeration / vagueness is `related=false`, not
   contradicted (the inter-judge contradiction signal is noisy; 2-judge agreement is the high-precision gate).

5. **Efficiency is reported only at HELD correctness.** Never rank a cheaper-but-less-complete answer over
   a complete one; surface tokens/reads/tool-calls as a secondary axis gated on recall parity.
   *Enforced:* `reporter._process_efficiency_md` emits the reads / tool-calls / billed-tokens delta as a
   headline ONLY when sense recall is at parity (within ±0.02) or higher than baseline; below parity it
   states the win is NOT claimed and withholds the savings table.
   *Guard:* `bench/lib/test_reporter.py::ProcessEfficiencyHeldRecallTest`.

6. **Process discipline:** single runs are noise (Opus ×2, non-Opus ×1); arms differ ONLY by toolset (+ its setup);
   gold is never shown to the agent; upstream anti-AI protest banners are stripped from BOTH arms
   (`bench-sense-local.sh` per-repo prep) so refusal noise can't bias a repo.

## The evidence this contract corrects (2026-06-19, canonical Opus-4.8 results, re-reported $0)

| headline | Sense win/tie/lose across 13 repos |
|---|---|
| OLD (blind `llm_quality`) | 1 win (rails), 11 ties, 1 LOSS (raix) - erases chatwoot/mastodon/gitlab/discourse/solidus |
| NEW (`cited_recall` + `relationship_audit`) | **10 wins**, 3 ties (redmine, lobsters-canonical, raix) |

The NEW headline recovers six documented, robust wins the blind composite had erased, surfaces four more,
and turns raix's unfair "loss" into a fair tie - all on objective ground truth, applied symmetrically.

## Status of the four fixes
- **Fix 1 (headline = recall/relationship_audit; demote blind composite):** DONE + guarded.
- **Fix 2 (reference-aware per-step criteria):** largely SUBSUMED by `relationship_audit` now being the
  headline; optional refinement.
- **Fix 3 (semantic grounding / anti-fabrication):** DONE + guarded. `relationship_audit` now flags
  `contradicted` gold items and reports `grounded_precision` + a contradiction count; the reporter leads
  with both. Populating the field on real data requires a RE-JUDGE (new judge schema); old judged.json
  renders `-` gracefully.
- **Fix 4 (process-efficiency at held correctness as a headline axis):** DONE + guarded. The aggregate
  carries avg reads / grep / mcp / tool-calls; `_process_efficiency_md` surfaces the delta only at held
  recall.

See `feedback_judge_scores_low_value_for_sense` (memory) for the full diagnosis.
