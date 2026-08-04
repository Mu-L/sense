# Finding: `sense_blast` pays ~10x for bytes it duplicates

> **PRICING SUPERSEDED 2026-08-04.** The byte measurements below hold and were replicated on
> mastodon. The priced projection ("1.26x -> 1.07x, PASS") does not: it subtracts
> re-read-multiplied tokens 1:1 from a total that prices cache reads at 0.1x, over-crediting
> the lever ~10x. See [`cost-parity-premium-is-not-the-payload.md`](cost-parity-premium-is-not-the-payload.md).

**Source:** Loop 5 harvest on the `rails` cell (WIN, discriminator +0.56, 2026-08-01).
**Status:** IN SCOPE for Loop 7 (owner ruling, 2026-08-01). Optimizing an existing
payload for its consumer is the `enhance` lane — *"better output of an existing
surface"* — not new surface area. Not built yet because it needs a bench, not because
it needs permission.

## The finding

The rails cell won at a **1.26x priced-token premium** (`cost_parity.py`). The premium
is not verbosity — output tokens are at parity (1.07x). It is **context carried**:
cache-read diverges 1.27x and cache-write 1.68x. `context_cost_audit.py` locates it:

| tool | calls | ~tokens injected | share |
|---|---:|---:|---:|
| `sense_blast` | 2 | 13,067 | **41%** |
| `sense_graph` | 7 | 12,940 | 40% |
| search / status / conventions | 5 | 5,984 | 19% |

~16,000 tokens injected per run against a **158,066-token** cache-read delta:

> **RE-READ MULTIPLIER = 9.9x.** A tool response is injected once and re-read as
> cached input on every later turn, so its cost is size x turns-remaining. A byte
> saved early is paid back about ten times.

## What is actually in the bytes

Largest single response: 25,944 chars (~6,486 tokens), `sense_blast` on
`ActiveRecord::Relation`. Top-level split: `direct_callers` 65%, `indirect_callers`
20%, everything else 15%.

Inside `direct_callers` (60 entries), measured on the captured payload:

| field | chars | % of response | note |
|---|---:|---:|---|
| `ref` | 4,313 | 17% | `file:line_start` |
| `file` | 4,160 | 16% | **derivable from `ref`** |
| `symbol` | 4,024 | 16% | load-bearing |
| `relation` | 1,680 | 6% | **constant**: `"calls Relation"` x60 |
| `line_start` | 993 | 4% | **derivable from `ref`** |
| `line_end` | 874 | 3% | rarely cited |

**`ref == file + ":" + line_start` on 60 of 60 entries, exactly.** Same duplication in
`indirect_callers` (20 of 20 carry both). So ~20% of the payload is byte-for-byte
derivable from a field already present, and another 6% is one constant string repeated
sixty times.

## The lever, priced

Applied to the blast/graph share of injection (81%), at the measured 9.9x multiplier:

| change | injected | paid | resulting ratio |
|---|---:|---:|---:|
| today | — | — | 1.26x |
| drop `file` + `line_start` | −2,591 | −25,654 | **1.11x** |
| + hoist the constant `relation` | −3,369 | −33,351 | **1.07x** — PASS |

Zero information loss: every dropped byte is reconstructible from `ref`. This is the
difference between a cell that wins at a 26% premium and one that wins **at parity**.

## Scope: in, and why

`NON-GOALS.md` closes "new query or output formats" — but its own framing is *"a refusal
to grow surface area"*, and *"not a moratorium on improvement"*. Removing bytes that
carry no information does not grow the surface; it makes the existing four tools better
for the consumer they exist to serve, which is the `enhance` lane (patch bump, changelog
"Enhancements"). **Owner ruling 2026-08-01: this is in scope.**

## The two real costs

- **It breaks consumers that read `.file`.** Our own `gold_confidence_check._paths()`
  walks both `file` and `ref` and would survive, but any agent or script reading `file`
  directly would not. That is a migration to plan, not a blocker.
- **It has not been benched.** The table above is projection from measured bytes and a
  measured multiplier, not a result. Per `BENCHMARKING.md` nothing behavioral ships on a
  projection — this earns its way in with its own cell showing reach HELD and the ratio
  moved, or it does not ship.

## What Loop 7 does with it

1. Decide the shape: drop `file`/`line_start` outright, or omit them only when exactly
   reconstructible from `ref` (keeping them when they are not) — the second is
   non-breaking for consumers that check before reading.
2. Bench it: reach must hold. A trim that costs a single cited dependent is a loss, not
   a saving, however good the ratio looks.
3. Re-measure the multiplier on the next vertical before generalising it — it is a
   general mechanism, but its SIZE here is n=1 cell, 2 runs.
