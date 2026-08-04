# Pickup - 2026-08-04

**The rebuilt loop crafted a winning scenario, unattended. The axis is answered: yes, it can.**

Two banked cells on `claude-opus-5`, both confirmed by `pergroup.py`:

    mastodon  dependents  baseline 4/20, 5/20   sense 20/20, 15/20   +0.65   WIN
              write-path  baseline 2/4,  2/4    sense 4/4,   4/4     +0.50   WIN
    rails     dependents  baseline 5/9, 3/9, 3/9  sense 9/9, 9/9, 8/9  +0.56  WIN

Mastodon ran `minibench -> expand -> preflight -> validate -> bench -> harvest` with no human
in the chain, on the Account anchor that had failed eight cycles the day before.

## FIRST: close mastodon. It is banked but its harvest is unfinished.

Four things, roughly one paid pair plus reading. Do them before taking a new repo - an
unfinished harvest is exactly how the old +0.72 cell ended up off this tree and unusable.

1. **Write the cost-parity pitch. This one is DoD-blocking.** The loop printed
   `COST_PARITY: MISS ratio=1.17` and its own checklist says a MISS is acceptable only if it
   carries a named trim candidate AND a Loop 7 pitch: *"a premium with no lever is an
   unfinished harvest."* The lever is already measured, so this is writing, not measuring:
   re-read multiplier **12.9x**, largest single responses `sense_graph` 6,153 tokens and
   `sense_blast` 4,220. For each, is every field load-bearing for the answer the agent gave?
   Fields returned and never cited cost their size times the re-reads.
2. **Take the paid cell to n=3.** The arms disagree: sense scored 20/20 then 15/20. The
   standing law is two runs per arm and a third when they disagree too much. Rails got its
   third today for a much smaller spread.
3. **Read the sense 15/20 transcript.** Five rows Sense found in one run and missed in the
   next, on the question we just banked. That is our own tool failing with the evidence on
   disk, and it is the best product lead currently available.
4. **Then refresh this page.**

## What changed today, and why it matters to the next session

- **Transcripts survive.** `FORCE_WIPE=1` did not merely destroy working space, it disabled
  the guard that refuses to overwrite SCORED runs; the next validation cycle would have taken
  all five opus-5 pairs. Cycles now append.
- **A one-armed run no longer satisfies a two-arm gate.** The gates asked whether ANY run
  existed at the current version; a cycle killed between its arms leaves exactly one, and
  resuming would have skipped the measurement and advanced on half a pair.
- **One results root per question.** Runs land in `<root>/<version>/<arm>/<repo>/run-N`.
  Fifteen readers walk `<root>/<arm>/<repo>/run-*` and one filtered on `scenario_version`, so
  the isolation is the DIRECTORY, not fifteen instrument changes - the same ruling already
  taken for validation runs. A reader aimed at the wrong level now says "no scored runs"
  instead of averaging two questions: it fails loudly.
- **Every question's bytes are archived.** `scenario_archive.py` content-addresses the exact
  bytes `scenario_version.py` hashes, so a number on disk can always reach its gold.
- **Difficulty survives a re-gold.** `inverse_frequency.py --by-path <store>` keys rows on the
  gold file path and merges questions, scoped to one model.

## Settled - do not re-litigate

- **opus-4.8 is history.** Its numbers are labelled historical and do NOT constrain a scoring
  change. We score opus-5 forward. (Ruling, Luc, 2026-08-04.)
- **The headline stays `cited_recall`.** All-or-nothing gating and a fixed per-miss cost were
  both built, measured on every cell, and rejected - gating erased the strongest banked win
  and invents wins at n=2; per-miss is a dial whose delta rises monotonically with its own
  parameter. `0.4 * completion + 0.6 * omission` is REPORTED beside the headline
  (`omission_lens.py`), decides nothing, and cannot manufacture a win because its delta is a
  convex combination of its terms.
- **Do not build the `score.sh` version guard.** The loop cannot hit that bug any more; it
  would only protect hand-typed commands against two retired directories (`bench/results` and
  `verticals/ruby-rails/archive/...`), which are still in the old flat layout. Caution, not work.

## Open, and known

- **`score.sh` applies the CURRENT scenario to every run under `RESULTS_DIR`.** It fired
  today, re-scoring the ten banked Status runs against the Account question before anyone
  noticed. Recovered exactly, because the transcripts survived and the question was archived.
  Path isolation makes it structurally impossible through the normal chain. **Never hand-set
  `RESULTS_DIR` at an old-layout tree and run `score.sh`.**
- **`specs` is unciteable by construction** in the banked Status control: its ask says specs
  must be "updated or added", and a spec you are about to write has no line to cite. Measured
  mentioned 2/2 in 8 of 10 runs, cited 0/2 in 10 of 10, both arms, both models, since June.
  Contained - only that one scenario carries the group, and `laws.md` now has the rule.
- **Citation correctness is unmeasured, and the obvious fix is biased.** A token-presence
  oracle scores the grepper 100% and the structural arm 0% on rows where both are right
  (measured 67.4% baseline vs 48.4% sense). If it is ever scored the oracle must be a RANGE,
  not a text match, and its circularity answered first.
- **Rails has no article and mastodon has no article.** Both cells are banked and neither is
  written up.

## After mastodon

Two repo slots remain in the bench set (see `verticals/ruby-rails/repos.txt`). And Luc's own
plan below is now unblocked at step 1: we know how to craft a winning scenario, so benching
the other models becomes the next real question.

____
Do not take the below into consideration, they are unstructured random notes for me (luc):

We're proving the rebuilt bench loop can craft a winning scenario.
once we kown how to create a winning scenario:

1. we bench all LLMs
2. we then analyze each transcripts to see where Sense can improve. store the info with proof and create a card for it (with previous scores to compare)
3. We pike the card, build and re-bench to check if brings better results, otherwise we drop the pike.
4. If better results, we clean the pike and pr with a qlty pass, coverage pass, council review.


how do we score omissions like the below where baseline missed 2? Maybe we are not harsh enough or score in a way that does not punish omissions. And do we track and score hallucinations?

  ┌──────────┬──────┬────────────────┬───────────────┐
  │   arm    │ wall │     budget     │  dependents   │
  ├──────────┼──────┼────────────────┼───────────────┤
  │ sense    │ 343s │ 480s           │ 16/16 = 1.000 │
  ├──────────┼──────┼────────────────┼───────────────┤
  │ baseline │ 405s │ 412s (matched) │ 14/16 = 0.875 │
  └──────────┴──────┴────────────────┴───────────────┘


First pick an axis Status in our case
Second, iterate 6 times over it
Third, if still no win, change axis.
repeat 3 times
Report to human
