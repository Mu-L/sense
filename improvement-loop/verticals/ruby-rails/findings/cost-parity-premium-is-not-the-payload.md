# Finding: the cost premium is not the payload, and the trim cannot close it

**Source:** Loop 5 harvest on the `mastodon` cell (WIN, dependents +0.65, write-path +0.50,
scenario `sha256:27dfc6000a5e98f3`, `claude-opus-5`, n=2, 2026-08-04).
**Status:** answers the DoD line *"a MISS carrying a named trim candidate and a Loop 7 pitch"*.
The trim candidate is real and re-measured below. The pitch is **not** the trim.
**Supersedes in part:** [`blast-payload-redundancy.md`](blast-payload-redundancy.md), whose
priced projection is off by ~10x (see "The correction").

## The miss

    COST_PARITY: MISS ratio=1.17 baseline=356156 sense=416490

## Where the 60,334 priced tokens actually are

`priced = input + 5x output + 0.1x cache_read + 1.25x cache_write` (`scorer.priced_tokens`).
Decomposed over the two paired runs:

| component | baseline mean | sense mean | delta | priced delta | share |
|---|---:|---:|---:|---:|---:|
| output | 26,943 | 38,674 | +11,730 | **+58,652** | **97%** |
| cache_read | 1,199,636 | 1,367,786 | +168,150 | +16,815 | 28% |
| cache_write | 81,152 | 67,540 | −13,611 | **−17,014** | −28% |
| input (uncached) | 38 | 1,918 | +1,880 | +1,880 | 3% |
| | | | | **+60,334** | |

The payload story is the two middle rows, and they **cancel**: Sense injects more context to
re-read (+16,815) and writes less of it to cache (−17,014), for a net −199. Every priced
token of the premium is output.

That output is not the answer text. Measured off the transcripts, the sense arm writes
*less* prose than the baseline (16,515 / 19,454 tokens of assistant text vs 19,526 / 21,418)
and fewer tool-call tokens (1,830 / 2,411 vs 2,829 / 2,763). The gap is in redacted thinking
blocks, which bill at 5x and are invisible in the transcript body.

## The trim candidate, re-measured on this cell

Still real, and the same two shapes the rails cell found. On mastodon's captured MCP bytes,
across both sense runs:

| shape | evidence | bytes |
|---|---|---:|
| `relation` is one constant string | `Counter({'calls Account': 60})` on the 60-row `direct_callers` | 1,620 / response |
| `ref == file + ":" + line_start` | true on 60 of 60 rows | `file` + `line_start` + `line_end` = 5,061 / response |

Aggregated over every MCP response in both runs: **2,928 of 13,068 injected tokens per run
(22.4%) are constant-valued or exactly reconstructable from a field already present.**

## The arithmetic that kills the trim as a cost lever

Injected tokens are paid at the **cache-read** rate. At this cell's measured 12.9x re-read
multiplier:

    2,928 injected x 12.9 re-reads x 0.1 priced-per-cache-read = ~3,777 priced tokens

which is **6.3%** of the 60,334 premium. And the ceiling is not far above it: deleting
*every byte Sense injects* removes the whole 168,150-token cache-read delta = 16,815 priced =
**27.9%** of the premium. A payload trim cannot reach parity on this cell, because 97% of the
premium is on a line the payload does not touch.

Same check on the rails cell (n=3, `3f210bcde96c18e1`, `COST_PARITY: MISS ratio=1.49`):
cache-read delta 261,392 = 26,139 priced = **31%** of its 83,607 premium. Same ceiling, same
conclusion.

## The correction

`blast-payload-redundancy.md` projected the trim as `1.26x -> 1.07x`, "PASS". It subtracted
the re-read-multiplied token count (25,654 / 33,351) **1:1** from a priced total that prices
cache reads at **0.1x**. Corrected, that lever is worth ~3,335 priced tokens on a ~170,000
baseline: about **2 points of ratio, not 19**. The finding's byte measurements stand; its
price does not, and it should not be pitched to Loop 7 as a route to parity.

## What Loop 7 gets instead

1. **The trim ships or not on its own merits, not on cost parity.** 22% fewer bytes for zero
   information loss is a better payload for the consumer (the `enhance` lane). It is worth
   ~6% of one cell's premium and should never again be sold as the fix for a MISS.
2. **The premium's real question is output.** Why does the sense arm emit ~12k more billed
   output tokens? The transcript says it is thinking, not prose. Nothing in the bench can see
   inside a redacted thinking block, so this is currently **unmeasurable** with what we
   capture. Naming it is the honest end of this harvest.
3. **At n=2 the premium is not resolvable anyway.** Per-run priced totals: baseline 296,353 /
   415,959, sense 375,014 / 457,966. The within-arm spread (119,606) is twice the
   between-arm difference of means (60,334). The *sign* is consistent (sense > baseline in
   5 of 5 paired runs across both cells); the *magnitude* is not measured yet. Any cost
   verdict quoted to two significant figures is over-reading the data.
