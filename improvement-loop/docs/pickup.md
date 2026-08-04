# Pickup - 2026-08-03

**The axis: can the rebuilt bench loop craft a winning scenario?**

Not yet. The loop itself is rebuilt, committed and working; what it has not produced is a
scenario that clears +0.50 on the headline model. Today established, with numbers, why that
is harder than it was in June, and it is not because the loop cannot author.

## The one thing to know

The banked mastodon `Status` scenario, unchanged from git, run on two models the same day
against the same clone, scorer and judge:

    claude-opus-4-8   baseline 2/16   sense 12/16   +0.625
    claude-opus-5     baseline 14/16  sense 16/16   +0.125

**The frontier baseline got dramatically stronger at exactly what Sense sells here.** It no
longer just finds scattered dependents, it writes shell loops that manufacture `path:line`
for a whole candidate list at once (`awk '!seen[$1]++'`, then `for f in ...; grep -n | head -1`).
Citation cost - the mechanism every banked Rails win rests on - is closed on opus-5.

Four cheaper explanations were tested and killed in this order, each with a run:

- **budget pressure** - the 7-step session does not hold opus-5 down (0.94 with 423s of a 480s wall)
- **the scorer** - re-scoring today's run with June's `bench/lib` gives byte-identical numbers
- **scenario crafting** - nine authoring cycles, eight of them measured
- **citation precision** - line-level matching separates the arms 0/7 vs 6/7 on opus-4.8 and
  10/14 vs 11/14 on opus-5, i.e. not at all where it matters

## What is closed, and how firmly

| anchor | model | runs | result |
|---|---|---|---|
| `Status` | opus-5 | n=5 per arm | baseline 0.863, sense 0.975, delta +0.112, **ceiling +0.137** |
| `Account` | opus-5 | 8 measured cycles | 7 of 8 put the baseline at or near ceiling |

`Status` is dead by arithmetic: a perfect sense arm cannot clear +0.50 against a baseline at
0.863. `Account` has been asked eight ways and never landed in the window.

## The one open lead

**Cycle 2** (`scenarios/mastodon.yaml.line-not-file.bak`) is the only ask ever to hold the
opus-5 baseline inside the window: **0.158**. It failed on OUR side - the sense arm answered
step 1 literally, walked `sense_graph(callers)` on five finder methods, and never called
`sense_blast` on the class.

Cycle 7 fixed that routing (confirmed in the transcript: `sense_blast Account` was called) and
still landed FAR, because the arm then ran eight anchored greps and hand-verified everything
it had already been given. **The sense arm treats blast output as a hint to check, not an
answer to cite.** That is a product observation, not a bench one, and it is the most
interesting thing left on the table.

Whether a question can hold the baseline down AND let Sense reach is still unproven in both
directions.

## THE LEAD TO START WITH (found after the rest of this page was written)

**Omissions are scored linearly, and that is what is hiding the signal.** `cited_recall` is
`cited/total`, so missing 2 of 16 costs 12.5% - while the ask itself says "an unlisted
dependent counts as missed. A missed dependent is a regression shipped." The metric does not
believe the task.

Completeness-gated on the SAME five opus-5 pairs already on disk, no re-benching:

    baseline dependents  0.875 0.875 0.812 0.938 0.812   complete: [0,0,0,0,0]
    sense    dependents  1.000 1.000 1.000 0.875 1.000   complete: [1,1,1,0,1]

    LINEAR recall      baseline 0.863  sense 0.975  delta +0.112   dead
    COMPLETENESS-GATED baseline 0.000  sense 0.800  delta +0.800   clears the floor

The baseline never once produced a complete audit; Sense did four times in five. Defensible
rather than goalpost-moving: a 14-of-16 audit does not let a maintainer safely rework the
contract, and completeness IS the product claim.

Before acting on it, the same discipline that killed the line-level proposal:

- **must hold on other cells** - free re-score across every run on disk; if it turns the
  banked +0.72 into noise or flips a tie into a false win, it fails. (That cell is
  **mastodon on opus-4.8**, not rails - `LEDGER.md:1019`. Corrected 2026-08-04, and the
  constraint itself was then retired by ruling: opus-4.8 is history.)
- **high variance** - one miss is a zero, so it needs n=5+ per arm; only `Status` has that
- **STOPPER-class** - headline metric change, needs a re-score diff and re-measurement of
  every banked number
- **grounded-but-wrong citations matter MORE under gating**, and nothing catches them today:
  `citation_grounding` only asks whether a `path:line` resolves, not whether it is the right
  line. Both arms produced resolving citations ~180 lines from the dependency today.

### Tomorrow, in order

1. **Build the completeness re-score as a REPORTING tool** (like `parity_rescore.py`: reads
   `scored.json`, writes nothing, changes no artifact). Per run, per group: was the group
   complete, yes or no.

       new: bench/lib/completeness.py  <repo> <results-root>...   + its test

2. **Run it over EVERY cell on disk**, not just mastodon - `verticals/*/results/*/validation`,
   `.../minibench`, and the paid roots. Report linear delta beside gated delta per cell.

3. **Read the three killers before believing it.**
   - does it hold on the banked mastodon opus-4.8 cell, or turn +0.72 into noise?
   - does it flip any TIE into a win? (that would be the metric inventing a result)
   - at n<5, how often does one miss swing a cell? quote the number.

4. **Only then** decide whether it becomes the headline. If yes it is a STOPPER change:
   re-score diff first, then re-measure every banked number, then update `laws.md`.

5. **Fix the grounded-but-wrong gap before gating ships.** Under gating, a citation that
   resolves to the wrong line counts toward "complete". Both arms produced citations ~180
   lines off today. `citation_grounding` cannot see it.

## Three fixes queued, none started

1. **Stop the transcript wipe.** `FORCE_WIPE=1` on the unscored roots (added today so cycles
   could re-run at the same path) destroys each cycle's transcripts. We archive the agent's
   READ (`minibench.N.md`, seven of them) but never the runs it read. Every real finding today
   came from a transcript; none of them would be reproducible after the fact. `KEEP_RUNS=1`
   fixes it and the pairing table already keys on run index.
2. **Make difficulty accumulate across re-questions.** `inverse_frequency.py` keys on
   `scenario_version`, which correctly refuses to blend two different questions - but a
   re-question that re-golds the same files gets a fresh version and loses all history. Keying
   row identity on the gold's `match` path would let difficulty persist per FILE across cycles.
   As built, the ranking is silent exactly when the author is choosing rows.
3. **Then re-question with memory that survives a re-gold.** Not before 1 and 2; another blind
   cycle tests nothing new.

## What was built today (10 commits, `af3eb18..5447a24`)

- **plans/** - `01-author` -> `02-minibench` -> `03-expand` -> `04-validate`, plus `05-handoff`.
  Scout and curate are gone. The first spend is a REAL two-arm run on a two-step probe
  scenario; nothing kills a draft before it.
- **driver** - every routed lever re-enters authoring immediately with the credit table, six
  cycles, then a plain-language handoff for the human. Nothing is deleted on a rejection.
- **matched budget** - the baseline's wall is its PAIRED sense run x1.2, per repo/model/scenario,
  recomputed on scenario change. Voids rather than scores when the sense partner is invalid.
- **interleaved arms** - sense run-1, baseline run-1, ... Arm was perfectly confounded with
  time-of-run before.
- **`inverse_frequency.py`** - ranks gold rows by citation rate across runs. On `Status` at n=5:
  1 discriminating row, 10 free rows, 2 unreachable.
- **`parity_rescore.py`** - truncate an arm at the other's cost. Built, tested, and its method
  does not apply: every arm writes its answer atomically in the last 2-4% of the session.

## Honest caveats

- **Three claims were made from n=1 today and retracted within the hour**: a cost win
  (gone at n=2 - sense uses slightly MORE tool calls), the line-level matching proposal, and a
  decapitation prediction. A fourth, an out-of-clone `Read` by a benched arm, is now
  unverifiable because the transcript was wiped. Treat any single-pair observation here as a
  hypothesis.
- The `laws.md` entry saying an occurrence-list question cannot win was written and **retracted
  the same day** against the banked cell that contradicts it. The retraction is in place.
- `ring_sweep.py` measures ZERO retention rings on both Ruby repos, so the strongest Go
  question kind (+0.58..+1.00 on four cells) cannot be asked in this vertical at all.

## State on disk

- Loop STOPPED by hand at `minibench`, cycle 3 of 6, on `Account`. Cycle 10's baseline was
  mid-run when it was stopped, so that pair is incomplete: resuming re-runs it, `--reset`
  starts clean. Nine cycles measured, eight on the FAR side, one negative.
- Nine authoring attempts kept as `.bak`, named for what each tried (six from the morning,
  three more from the evening: assembly-map, inherited-helpers, line-pin-no-shape).
- `scenarios/mastodon.yaml.banked-status-control.bak` - the June winner, byte-identical.
- Cycles 9 and 10 snapshotted to the session scratchpad (`cycle-snapshots/`); cycles 1-8
  are gone (see fix 1). Copy them somewhere durable before that scratchpad is cleared.
- Results tree is gitignored by policy; everything else is committed.


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
