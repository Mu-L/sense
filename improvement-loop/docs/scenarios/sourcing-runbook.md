# Scenario-sourcing runbook - make Sense beat baseline ≥50pts per repo, honestly

**Purpose.** The real leverage in a vertical benchmark is the *scenarios*,
repo by repo. This runbook drives, for ONE repo at a time, the authoring of a
scenario + gold where an agent using Sense beats a grep/read-only baseline by
**≥50 points** (chatwoot precedent: 0.41 → 0.96, +55pt), variance-proven, with the
win coming entirely from a real maintainer flow the baseline cannot reach without
Sense - never from tuning the bench.

The only honest way to find such a flow is to **read both arms' transcripts side by
side after a bench** and isolate what Sense reached that grep/read could not.

---

## ⚠️ Hard-won lesson - read this BEFORE you trust any score (cost many sessions)

**The LLM judge is BLIND TO OMISSION. A judge "tie" is not evidence of a tie.**

The judge scores each answer in isolation with no reference for what a *complete*
answer must contain. So it rates an incomplete-but-confident answer as
"exhaustive" and routinely fails to separate a thorough arm from a partial one.
On chatwoot the reference-blind judge scored **baseline 0.86 ≈ sense 0.82** - and
called a **35%-recall** baseline audit *"map_quality 0.9, Exhaustive"* - while the
transcripts and objective recall showed Sense winning **0.94 vs 0.35**. Multiple
sessions were burned concluding "no win / falsified" from blind-judge ties that
were the judge failing, not the tool tying.

**The method that actually finds the truth (do this every time, in this order):**
1. **Objective recall first.** `gold_recall` (esp. `cited_recall`) against the
   fixed must-find denominator is the ground truth - it is the ONLY scorer that
   knows what a complete answer must contain, so it is the only one that catches
   what the baseline silently dropped.
2. **Read both transcripts side by side** (runbook loop step 2). Record per gold
   target {hit_by_baseline?, hit_by_sense?, how}. This is the real evidence.
3. **Only then look at the judge** - and treat it as suspect. If the judge
   contradicts (1) and (2), THE JUDGE IS WRONG. Do not conclude "no win" from it.

**When the judge is wrong, FIX THE JUDGE - don't trust its blind verdict and don't
hand-tune scores.** The fix is to make it *reference-aware*: feed it the must-find
set + each target's true relation and grade `covered`/`related` per item - the
`relationship_audit` pattern (`bench/lib/relationship_audit.py`, gold targets carry
a `relation:` field authored from source, never shown to the agent). On chatwoot
this flipped the verdict to match reality: **baseline covered 0.47 / related 0.45;
sense covered 0.96 / related 0.92, ×3, no overlap.** Fixing the judge's accuracy is
legitimate *because Sense genuinely wins on the truth* - that is the opposite of
tuning a result. Full account: `29-03-forem-reaction-antilitmus-2026-06-15.md`.

**Two metric dead-ends from the same arc, so you don't repeat them:** (a) objective
F1/precision (`gold_f1`) favours Sense but DILUTES the headline (+25 vs recall's
+59) because curated gold ⊊ the files a thorough answer legitimately cites; keep it
secondary. (b) a reference-BLIND per-step judge criterion (`relationship`) does NOT
separate - only the *reference-aware* audit does.

---

## Read these FIRST (required context)

**The win exemplar - the shape to emulate:**
- `verticals/<key>/scenarios/<repo>.yaml` + `chatwoot.rubric.yaml` - the only decisive win
  (Inbox teardown, association/concern/polymorphic-reached indirect dependents).
- `chatwoot-hardened-variance-2026-06-15.md` (private tree) - why it
  won and held ×3 (no sense run overlaps any baseline run).

**The cautionary examples - what NOT to ship:**
- `verticals/<key>/scenarios/<repo>.yaml` + `solidus.rubric.yaml` - read-easy in structural
  clothing: leaked domain nouns, colocated/enumerable gold (`ls conditions/`),
  baseline reaches the gold by reading. **DEMOTED** (unscored). The anti-pattern.
- `29-02-solidus-config-string-findings-2026-06-15.md` (private tree)
  - the no-win post-mortem + enumerable-gold variance.

**The discriminator (why most centers tie):**
- `redesign-findings-2026-06-14.md` (private tree) - grep *over*-covers
  literal-token seams (→ tie) and *under*-recalls association/concern/polymorphic
  ones (→ win).
- `29-01-mastodon-closure-findings-2026-06-15.md` (private tree) -
  a real resolver fix that produced only a *tie*; the ~25-center recon that found
  NO chatwoot-grade seam in mastodon. Big in-degree ≠ grep-hard.
- `first-run-findings.md` (private tree) §E - the `_cited` scorer bug,
  the deferred-tool adoption gate, enumerable gold.
- `transcript-analysis-2026-06-14.md` (private tree) - worked example
  of the side-by-side transcript read this runbook depends on.

**The metric being evolved (so you know what you're changing and why):**
- `bench/lib/gold.py` - how gold mention/cited is scored (file-path substring in the
  answer text). **This is grep-shaped** (see "The metric" below).
- `bench/lib/judge.py` + `bench/lib/judge_prompt.v1.md` - the rubric/judge layer
  (`map_quality` etc.) where precision and relationship-correctness must live.
- `bench/lib/scorer.py` (the scoring contract) - where any criterion/metric change is pre-registered.
- `bench/drivers/runs-variance.sh` - the variance harness (>=3 runs).

**This session's resolver fix (changes what Sense can now resolve):**
- `internal/resolve/resolver.go` (`resolveLexicalInherits`) - relatively-named
  superclasses in namespaces now resolve, so inheritance fan-outs in deeply-
  namespaced repos are visible to `graph`/`blast`. Rebuild before relying on it.

**Standing memories (auto-loaded; the load-bearing ones):**
`feedback_deep_transcript_analysis` (never author from metadata - read transcripts),
`feedback_rebuild_embed_before_sweep`, `project_blast_output_order_nondeterministic`
(blast text is display-capped + nondeterministic - use the DB), `feedback_dig_beyond_symptoms`,
`project_2902_classname_attribute_pivot` (the inheritance-fix + solidus story).

---

## Preconditions (every repo, every time)

- Build the current binary: `rm ~/.local/bin/sense && go build -o ~/.local/bin/sense ./cmd/sense`.
- Rebuild the index with the fresh binary BEFORE any Sense query:
  `cd /Users/luc/Developer/luuuc/oss/sense-benchmark/sense/<REPO> && sense scan -rebuild -embed`.
  (`-rebuild` re-resolves scan-layer edges; `-embed` enables the NL search the agent uses.)
- Clones: `…/sense-benchmark/sense/<REPO>` and `…/baseline/<REPO>`.
- **GROUND TRUTH IS THE DB, NOT `blast`'s stdout.** `blast` caps display at 100 with
  nondeterministic order - never parse it for a dependent set. Query `.sense/index.db`
  (`sense_edges`/`sense_symbols`) and spot-check every claimed dependent against source
  with grep before trusting it. (This caught 3 false conclusions in the sourcing pass.)

---

## The loop (iterate until acceptance)

1. **Bench it.** `RUNS=3 bash bench/drivers/runs-variance.sh <REPO>`
   (>=3 runs - these scenarios are bimodal; never conclude from >3).
2. **Deep transcript analysis - the core work.** Open baseline and sense transcripts
   side by side, per step. Do NOT infer from scores. For each gold target, record
   `{hit_by_baseline?, hit_by_sense?, how each got there}`, then classify:
   - **Sense-only flows** (sense hit, baseline missed): name the exact mechanism -
     "sense fired `blast/graph <X>`, surfaced indirect dependent `<Y>`; baseline
     grepped `<token>` / read `<files>` and never reached `<Y>`." These flows ARE the
     scenario's reason to exist.
   - **Non-discriminating** (both hit): token-present / read-easy → cut (it pads the
     denominator and measures read-ease, not the tool).
   - **Adoption gaps** (sense COULD but the agent didn't fire the right tool, queried
     a wrong/legacy symbol, or fell back to read): record separately - that's a
     hook/search-ranking issue, NOT weak-seam evidence.
3. **Re-author around the Sense-only flows.** Rewrite the maintainer question so it
   genuinely requires those flows (a real "what breaks if I change/remove X"
   teardown); re-curate gold to the TRUE impact set, weighted to grep-hard indirect
   deps; cut non-discriminating targets.
4. **Re-bench. Repeat** until acceptance.

---

## What a REAL Sense win looks like (learned the hard way)

- NOT direct dependents - almost always token-present, grep finds them (even chatwoot
  Inbox had 0 grep-blind *direct* deps).
- IS a curated set of **indirect dependents reached via association / concern /
  polymorphism** that a thorough grep+read audit under-recalls.
- **Constrained**: avoid hub-explosion (`X → User → everything`). The gold is a
  coherent impact set a maintainer must not miss, not a transitive blob.
- Reject **mastodon-shape** centers (dependents name the token → grep over-covers →
  tie) and **enumerable/colocated** gold (one dir `ls` enumerates → read-easy → tie).

---

## The metric - gold is grep-shaped; lead with precision instead

**The core doubt, validated:** gold = file-path substring recall, and file-path recall
is grep's home turf. grep/`ls`/read emit *paths* natively; Sense emits *relationships*.
So gold reduces Sense's structural output to its file endpoints and rewards the
baseline's mode of knowing. Even when gold is grep-hard it's a weak, partially-fakeable
proxy. **Keep gold recall as the objective floor, not the headline.**

Measure what grep CANNOT produce:

1. **Precision-AND-recall (F1) of the claimed dependent set vs gold.** Gold today only
   scores recall and never penalizes naming **non-dependents**. The baseline's
   `grep token` returns a noisy list (false positives) or under-includes; Sense's
   `blast` set is clean and complete. Scoring F1 makes grep noise cost the baseline on
   the same answer - grep-can't-fake, and it captures the chatwoot mechanism (baseline
   was both noisy and missed the indirect deps). A false-positive dependency is a real
   maintainer defect (wrong refactor), so this reflects real value.
2. **Relationship / path correctness.** Shift the question from "which files" (grep
   turf) to "the impact **chain** - what breaks, and *via what path*" (`A affected via
   A→B→X`). grep can't assemble a correct path; Sense traverses it. Score chains, not
   endpoints - this lives in the judge/`map_quality` layer.

Caveats: precision/path scoring leans on the **LLM judge** (noisier than substring
recall - which is why the bench originally leaned on objective gold). And it is a
scorer/methodology change: **pre-register it in `bench/lib/scorer.py` (the scoring contract), apply it identically
to both arms, never reverse-engineer it to a result.**

---

## Honesty guardrails (non-negotiable)

- Fix the SCENARIO to reflect real maintainer work; never tune gold/rubric/scorer to
  manufacture a win. The win must generalize to any agent on any repo.
- Per-target anti-litmus: `grep -ri <token>` must demonstrably UNDER-recall each
  Sense-only gold target. Run it before AND after; record both.
- Don't leak domain nouns in the rendered prompt - a leaked noun is a grep handle that
  asymmetrically arms the baseline (the solidus failure). Keep prompts neutral.
- Report adoption gaps honestly; if the win depends on a tool the agent doesn't reliably
  reach for, that's a separate fix, not a scenario win.

---

## Acceptance (the metric that rewards Sense's mode, not grep's)

- HEADLINE: on the impact question, Sense beats baseline by **≥50pts on F1
  (precision-AND-recall) of the claimed dependent set vs gold** - baseline's grep noise
  and missed indirect deps both cost it.
- Plus **relationship correctness**: the answer states HOW each dependent is reached;
  judge-scored, grep can't assemble it.
- Gold file-path recall retained as the OBJECTIVE FLOOR/denominator, not the headline.
- Variance-proven >=3 runs (no sense run overlapping any baseline run, à la chatwoot).
- Every Sense-only target passes the grep anti-litmus; prompt leaks nothing; gold = the
  real impact set; transcript shows Sense firing the structural tool and returning the
  resolved set that produced the win.

---

## Deliverables (per repo)

- Updated `bench/scenarios/<REPO>.yaml` + `<REPO>.rubric.yaml`.
- A findings doc under the rails vertical findings tree (private): the Sense-only flows (with
  transcript evidence), per-target anti-litmus before/after, the variance table, the
  F1/precision deltas, and any adoption gaps.
- Pre-registered metric change (if any) recorded in `bench/lib/scorer.py` (the scoring contract).

---

## Repo ordering & cost

- Validate the harness/metric on **chatwoot** (proven win) first, then push the others.
- **forem** is freshly indexed (fixed binary + embed) and richest in polymorphism →
  strongest fresh candidate. **discourse** centers are mastodon-shape (probed: 0
  grep-blind); deprioritize. Skip small/collocated gems and mastodon (no chatwoot-grade
  seam, already swept). **solidus** stays demoted.
- ~$17/iteration (sessions + judge + audit), so front-load the transcript analysis to
  minimize re-benches.
