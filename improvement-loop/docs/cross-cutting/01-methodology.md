---
slug: benchmarking-dev-tools-for-ai-agents-honestly
role: "cross-cutting fact pack - how we benchmark a code-intelligence tool for AI agents without fooling ourselves"
status: seeded (Rails)
data: verticals/*/results/report.md + docs/manifesto.md
byline: "Luc B. Perussault-Diallo"
rigor: methodology (no model-comparison claims; this pack is about the method)
---
# Benchmarking a dev tool for AI agents, honestly

**The finding.** Most "our tool helps AI" benchmarks measure the tool against a strawman or against an LLM
judge that cannot see what the answer left out. The hard part is not running the bench; it is keeping it honest
when you are the one who benefits from the number. This pack is the method we built to do that, and the
mistakes that cost us the most to learn. It is the trust layer under every other number we publish.

## The five traps (each cost real time; each is now a rule)

1. **The judge is blind to omission.** A reference-blind LLM judge rates a 35%-recall answer "exhaustive,"
   because it grades what is present, not what is missing. Fix: make the judge **reference-aware**. Feed it the
   must-find set and each target's true relation, and grade covered/related per item. Truth is the transcripts
   plus objective recall, never the judge's prose verdict.
2. **You are the baseline - the tool is yours, not a rival.** When you run the bench on the frontier model the
   two arms are you *without* the tool and you *with* it, so the question is "where does the tool make me better
   than I am without it?" - not "where does it beat me?" You read, recall, and trace your way to most answers even
   without it, which breeds false confidence and hides exactly the seam the tool fills. Your own competence is the
   blind spot; never thumb the scale for the tool-less arm.
3. **A tie is a missing axis, not a verdict.** The Rails framework case looked like a no-win until the
   *memorization* axis flipped it: the baseline recites a famous public API from training and ties on it, so the
   discriminator had to be the obscure internals that are not in any model's weights. A tie means you have not
   yet found the axis, not that the repo is unwinnable.
4. **Never conclude from a proxy.** grep-reachability is not "the baseline finds it"; one run is noise; metadata
   is not evidence. Read both transcripts side by side, per gold target, before any conclusion.
5. **A new metric is a hypothesis; backtest it on the wins you already banked.** Every gate and every ranking
   we proposed was plausible, and they went 0 for 5 in a single day. The one that looked strongest - rank
   candidates by how many things hold them - finds three of four banked wins at rank 2 to 4 and buries the
   fourth, the biggest win in the corpus, at rank 50 of 901. Tuning on the cases that agree with you is how a
   gate learns to reject exactly the results you are trying to reproduce. Fix: `gate_backtest.py`, which runs
   any proposed gate against the banked wins and refuses to attest if it rejects one.

## The fairness corrections (every one removed an anti- OR pro-tool bias)

- Dropped `gold_f1` and retired the reference-blind composite (it rated a 44%-complete audit "exhaustive").
- Excluded efficiency from the recall blend (cost is a separate axis, never folded into reach).
- Demoted step-averaged `completeness` to a gate, because it compresses a 6× reach gap into a +0.27 score (it
  saturates near 1.0 and the one discriminating step is 1/N of the average). Headline the **unbounded** signal:
  dependents-cited count, tool-only reach, coverage per billed token.

## The honesty guardrails (publishable)

Arms differ only by toolset; identical prompt; the prompt leaks no paths/symbols/counts/tool-names; the gold is
the must-find set, not a stacked deck; **report where the baseline wins or ties**; pin commits and publish the
harness. A bench that only flatters the tool measures nothing.

## What "the bench is trustworthy" means concretely (the convergence bar)

From `goal.md`: scores reproducible; judge self-agreement <5% across iterations;
per-scenario ranks stable; discriminates (≥0.10 fairness gap on most scenarios); tracks human judgment
(held-out correlation ≥0.85). These are the conditions an automated loop would later optimize against, which is
why the trustworthy bench is a *prerequisite* for any auto-improvement (see
`../goal.md`).

## Data points by vertical

| vertical | when | the method's data point | agrees / refines / reverses |
|---|---|---|---|
| Ruby/Rails | 2026-06 | 4 traps surfaced and codified; 3 fairness corrections shipped; reference-aware judge built (`relationship_audit.py`); held-out anchor frozen (`freeze-heldout.sh`). 12W/1T/0L headline survived all corrections. | seed |
| Python/Django | TBD | does a typed-ish framework surface a new trap (e.g. memorization stronger / weaker)? | … |
| Go | 2026-07 | **A trap about the instrument itself, not the content: your own validity flag can invert your results.** `run_meta.valid` was stamped `rc == 0`, so the watchdog's exit 124 voided every run the wall clock cut short. Reclassifying 16 void runs from evidence: 15 were real measurements (answers of 5.6k-45.8k chars) and the 1 genuine artifact (a 203-char crash stub) was the one being AVERAGED IN. Exactly inverted. Two corollaries, both found the same day: (a) three instruments - `matrix.py`, `scoreboard.py`, `check_findings_stats.py` - each carried their own copy of the run-aggregation glob, so when one learned to drop artifacts the campaign published two different headlines for the same cell (dolt +0.50 vs +0.38); (b) `check-findings.sh` passed args to one checker and invoked the other with NONE, so it audited a DIFFERENT VERTICAL and printed green - an unpointed gate reports success for work it never inspected. Rule: the rule that decides which runs count lives in ONE place, and every gate must print what it was pointed at. | **refines** (adds the instrument-integrity trap to the content traps) |

## Per-vertical detail

### Ruby/Rails (seed) - the scars, each a hard-won rule

- **The false-tie catalog.** Rails taught us four distinct causes of a fake tie, each needing a different
  transform: *colocated* (fan-out in one dir → baseline `ls`-es it), *resolver-gap* (the edge is real but Sense
  does not resolve it), *2-hop-greppable* (every node is a named class), and *memorization* (baseline recites a
  famous public API from weights). A tie is never a verdict; it is "which of these four, and how do I transform
  past it."
- **The $0 gold-retarget lever.** Changing *which* files are in the gold leaves the ×2 transcripts valid, so you
  can re-score with `scorer.py` and preview a gold tweak without paying for a re-bench. This is how Rails went
  from +0.19 to +0.56 on the framework case with no extra spend. The per-dep tally (which gold id the baseline
  cited vs missed, across runs) is the single most important diagnostic.
- **The judge fights took many sessions.** "The judge is blind to omission" was learned the expensive way: a
  reference-blind judge repeatedly rated 35–44%-recall answers "exhaustive." The fix (`relationship_audit.py`,
  reference-aware, graded against each gold target's `relation:` field) is now load-bearing. The standing rule:
  when the judge contradicts the transcripts, fix the judge, never conclude "no win."
- **"You are the baseline" was concrete, not abstract.** The Rails framework case was exactly this trap: reading
  and recalling our way to the public API bred false confidence that "there's no win here," until the
  memorization axis isolated the non-memorized engine-room internals (+0.56 where it looked like a tie).
- **Metric dilution, twice.** First F1/overall recall hid a +0.56 discriminator behind a +0.19 overall; then
  step-averaged completeness compressed a 6× reach gap into +0.27. Both fixed by headlining the unbounded
  discriminator signal. Same lesson, two metrics.
- **Anti-fabrication is a measurable axis.** On discourse/solidus/forem the baseline confidently *mis-describes*
  a dependent's role; the reference-aware judge catches it against the gold relation. ⚑ wins are a category, not
  a footnote.

## Open questions (carry forward)

- Does the memorization trap get worse on more-famous frameworks (Django) and weaker on niche ones?
- Can the per-dep tally diagnosis (which gold dep separates) be mechanized without a human reading transcripts?
  (This is the seam between "trustworthy bench" and "auto-improvement" - Loop A in
  `../goal.md`.)
