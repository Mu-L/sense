# Pickup - 2026-08-04 (evening)

**The mastodon harvest is CLOSED, and the cell got bigger, not smaller: +0.78 on dependents.**

Both open questions were answered, and both came out the opposite way to what the morning
page predicted. Nothing is blocked; nothing was bought.

## The 15/20 was the scorer's handwriting, and it is fixed

Sense returned all five rows and the agent named all five - as
`Admin::ActionLogsController#index` `:7`. `gold.py` matched file paths only, so it scored
five correct answers as misses. **Ruled (Luc, 2026-08-04): a class name plus a line IS a
citation.** Shipped as the symbol oracle in `gold.py`, derived from the path alone so it
stays arm-blind, with tests.

Re-scored all 27 runs in the vertical: 4 moved, and one of them is a **baseline** run
(opus-4.8 validation, 6 -> 8) - the arm-blindness check passing on real data. Rails does not
move. Only the banked cell was re-scored on disk; the others sit at older scenario versions
where `score.sh` would apply the current question.

**The n=3 run was not bought and is no longer owed.** It was called for by the
two-runs-disagree rule, and the disagreement WAS the matcher: the sense arm scored 20/20
twice. Full story: `verticals/ruby-rails/findings/gold-matcher-is-path-only.md`.

## Cost parity: answered, and the old pitch was wrong

`COST_PARITY: MISS ratio=1.17`. The premium is **97% output tokens**, not context:

    output      +11,730  x5    = +58,652
    cache_read +168,150  x0.1  = +16,815
    cache_write -13,611  x1.25 = -17,014      <- the two payload lines cancel

The trim candidate is real and re-measured (22.4% of injected bytes are a constant `relation`
or reconstructable from `ref`) but worth ~6% of the premium; deleting every injected byte
caps at 28%. The rails finding's `1.26 -> 1.07 PASS` projection multiplied by the re-read
count and forgot that a re-read prices at 0.1x - a ~10x over-credit, now corrected in place.
The output premium is thinking tokens, which are redacted in the transcript and therefore
currently unmeasurable. Written up:
`verticals/ruby-rails/findings/cost-parity-premium-is-not-the-payload.md`.

Also true, and it should temper any cost verdict: at n=2 the within-arm priced spread
(119,606) is twice the between-arm difference (60,334). The sign is consistent across 5 of 5
paired runs; the magnitude is not measured.

## Where the two banked cells stand

    mastodon  dependents  baseline 4/20, 5/20   sense 20/20, 20/20   +0.78   WIN  (n=2)
              write-path  baseline 2/4,  2/4    sense 4/4,   4/4     +0.50   WIN
              overall     0.53                  1.00                 +0.47
    rails     dependents  baseline 5/9, 3/9, 3/9  sense 9/9, 9/9, 8/9  +0.56  WIN  (n=3)

Harvest checklist: cost parity **done**, transcript read **done**, n=3 **dissolved**, this
page **refreshed**. Mastodon is closed.

## Settled - do not re-litigate

- **opus-4.8 is history.** Its numbers are labelled historical and do NOT constrain a scoring
  change. We score opus-5 forward. (Ruling, Luc, 2026-08-04.)
- **The headline stays `cited_recall`.** All-or-nothing gating and a fixed per-miss cost were
  both built, measured on every cell, and rejected. `0.4 * completion + 0.6 * omission` is
  REPORTED beside the headline (`omission_lens.py`) and decides nothing.
- **Do not build the `score.sh` version guard.** The loop cannot hit that bug any more.
- **A payload trim is not a cost lever.** Measured twice now. It may still ship as an
  `enhance` for the consumer's sake; it must never again be pitched as the route to parity.

## Open, and known

- **`score.sh` applies the CURRENT scenario to every run under `RESULTS_DIR`.** Path isolation
  makes this structurally impossible through the normal chain. **Never hand-set `RESULTS_DIR`
  at an old-layout tree and run `score.sh`.**
- **`specs` is unciteable by construction** in the banked Status control. Contained to that
  one scenario; `laws.md` carries the rule.
- **Citation correctness is unmeasured, and the obvious fix is biased.** If it is ever scored
  the oracle must be a RANGE, not a text match.
- **Thinking tokens are invisible to the bench.** They carry the entire cost premium on this
  cell and the transcript redacts them. No instrument sees inside.
- **Rails has no article and mastodon has no article.** Both cells banked, neither written up.

## Next

Two repo slots remain in the bench set (`verticals/ruby-rails/repos.txt`), and both banked
cells still have no article. Luc's plan is unblocked at step 1 - we know how to craft a
winning scenario, so benching the other models is the next real question.
