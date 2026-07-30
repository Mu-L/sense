# Loop 2 - Repo admission

> **Status: defined 2026-07-11; `admission_gate.py` built and runnable, no driver calls it - run it by hand
> per candidate.** Everything repo-shaped, from candidate pool to pinned, provisioned, indexed slate: the
> choice (composed per manifesto §7.0), the pin + provision (`bench/drivers/provision-repos.sh`), the
> indexes (`bench/lib/ensure-index.sh`). The design addition: the **admission gate** runs
> the seam-existence measurement BEFORE a repo joins the slate, not weeks later inside Loop 3. Haystack
> (0/1256 grep-invisible edges, discovered after crafting attempts) and pretix (gold citable via one declared
> covering pattern) are the two slots this gate exists to save.

## Goal

Deliver a slate of 4 repos that can win before a dollar is spent: pinned, provisioned, indexed, measured
against the seam-existence gate, composed per §7.0 with a backup per slot. Unwinnable repos die at
admission for $0 (the haystack and pretix slots were lost weeks later inside Loop 3; this gate exists so
that never happens again).

## Product duties (per Sense surface)

- **blast:** bar 4 doubles as a product regression net: an anchor that fails default-blast retrieval on a
  freshly built index (the saleor break) is a *product finding first*, an admission verdict second; it is
  filed to the gap ledger, not just marked "reject."
- **graph:** the gate's grep-invisible edge counts are per-stack extraction telemetry. A stack whose
  candidates ALL measure thin is an extractor-maturity signal for Loop 7, not merely a pool problem;
  record the pattern, don't just widen the pool.
- **conventions:** the slate sweep is **this loop's** - it already clones and indexes every candidate, so
  running `sense conventions` here costs nothing extra. The ledger recording is Loop 5's.
- **search / status / setup / contracts / response shape:** none; no agent runs in this loop.

## Identity

- **Character:** mechanical. The gate produces numbers and the numbers compose the slate; since 2026-07-29
  no human confirms it (see Human events).
- **Unit of work:** one candidate repo taken through: clone → index → gate measurement → slot verdict
  (admit / reject), recorded in `repos.md`.
- **Position:** consumes Loop 1's scaffold (`repos.md` skeleton, `repos.txt`) and the stack's candidate
  pool; produces the pinned slate Loop 3 converges on. **Re-entered mid-vertical** when Loop 3's the swap gate
  swaps a repo: the backup is admitted through the same gate, never waved in.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | session agent: pool research, `provision-repos.sh`, `ensure-index.sh`, gate runs, `repos.md` drafting | |
| Evaluator | the admission-gate verdict per candidate - every bar measured, incl. memorization (`memorization_probe.py`), channel/banner, and the unconditional adversary probe | numbers PLUS one agent verdict: the probe is the one bar that is not a number, and it is the one that has never been wrong |
| Mechanical verifier | the gate script on the real index (to build; wraps `seam_hunt.py` + `structural_surplus.py` + `resolve_oracle.py`) | |
| Human | none - reads `repos.md` + the `loop2/slate` ledger entry after the fact | the admission sign-off retired 2026-07-29 |

## Bar 0 - IS THE RELATIONSHIP UNDECLARED? Run this before anything else

**The law: depth does not protect a relationship, IMPLICITNESS does.** Sense's edge is over
relationships the source does not spell out. If the source declares the edge at the definition site,
a text parser recovers 100% of it AT ANY DEPTH, and the candidate is dead no matter how scattered,
how deep, or how large the fan-out.

**This law was already written down - as sub-law (b) of bar 3, learned from litellm - and on
2026-07-30 it did not fire.** A full day went into a php candidate with a 7-hop chain, 1 direct user
and 215 transitive dependents, on the reasoning that seven grep rounds defeat a baseline. The
adversary wrote a php class parser in 17 tool calls and returned **216 against 215, recall 0.991**,
ceiling +0.009. The law was correct and buried in a sub-clause, where nothing executed it. Hence a
bar 0 with a command, not a paragraph.

**The screen, one blast per candidate:**

    sense blast <Anchor> --file <path> --json    # undeclared share = ring / total_affected
    #   ring = retained_via_interfaces_count, the dependents reached ONLY through an
    #          interface-typed field, which is the case no parser can recover
    # FLOOR: share >= 0.05. Below it, REJECT before spending anything else on the candidate.

**Backtest (the licence to gate - it must admit what we banked):**

| | undeclared share |
|---|---|
| pebble `Batch`, nomad `Server`, dolt `DoltDB`, consul `Server` (all 4 banked wins) | 0.094 / 0.106 / 0.187 / 0.272 |
| filament `EvaluatesClosures`, akaunting `Relationships`, invoiceninja `Client` (all bound-killed) | 0.003 / 0.000 / 0.004 |

**NECESSARY, NOT SUFFICIENT, and do not turn it into a ranking.** Within go the wins rank 7 of 12,
5 of 10 and 3 of 7 by this number; pebble `File` scores 1.449 and `Task` 0.851 and neither ever won.
It is a floor that removes the arithmetically dead, and it is the SIXTH metric proposed in one day -
the other five all failed as rankings. Rank with it and it will fail too.

**The confound, stated because it is not fully broken:** every banked win is go and every rejected
anchor is php, so "undeclared share" is partly a proxy for "is go". What weakens the confound: two of
38 go anchors measured 0.000 (nomad `Context`, consul `HTTPFlags`), so the number does vary within
the language. It is a HYPOTHESIS with n=4 wins until a vertical carries both shapes in one language.
The prediction it makes, and the way to kill it: the go shape should reappear in any language with
IMPLICIT satisfaction (rust traits, java/kotlin interfaces via DI) and should NOT appear in php, ruby
or python explicit hierarchies, whatever their depth.

## The admission gate (the design piece)

Per candidate, on its built index, slot-aware. **Bar 0 above runs first and is $0; a candidate that
fails it never reaches bar 1.**

1. **Contract exists** - `seam_hunt.py --propose` finds a central abstraction with a heterogeneous,
   scattered, non-obvious fan-out (manifesto §7.0's repo-choice language).
2. **Seam existence - two components, calibrated 2026-07-11** (the admission backtest, internal tree: `bench/results/admission-backtest-2026-07-11.md`):
   (a) grep-invisible dependents AND (b) grep-NOISE (token precision = dependents ÷ repo-wide files
   containing the token). The backtest overturned the naive form of this bar: **invisibility alone would
   have rejected sentry** (0 invisible deps - every dep declares a `group: Group` field; the win lived in
   noise: precision 0.034, 1988 hits for 68 deps). The calibrated win signature: no usable covering
   pattern + token precision ≤ 0.3 + total_affected ≥ 500.
3. **Not grep-reconstructable** - no single declared textual pattern covers the candidate gold (the pretix
   `related_name` lesson: a covering pattern makes the seam baseline-reachable and the scenario baseline-hell
   by design). This bar is informed by the **loss-anatomy ledger** ([`loss-anatomy.md`](loss-anatomy.md),
   backfilled 2026-07-11 from all frozen ties): every recorded tie/loss distilled to the transcript-level
   reason grep won. A recorded verdict cited to reject or ballast a candidate is a conditional fact: when
   the product or the gate calibration changed since it was recorded, discharge the Class-1 protocol
   (stale-verdict, [`decision-errors.md`](../decision-errors.md)) before citing it. Three laws the backfill added: (a) test DECLARATION SHAPES, not just import/name
   covers - a typed field (`group: Group`), an accessor, a DSL line, or a file-layout convention each
   one-pattern-covers gold the import graph says is scattered; (b) transitive DEPTH alone is never
   grep-hostility - statically-declared hierarchies are $0-enumerable via a throwaway ast.walk script
   (litellm run-2, re-confirmed on filament 2026-07-30 at adversary recall 0.991 through SEVEN hops);
   the edge must be UNDECLARED (implicit satisfaction), not merely far. **This sub-law is now BAR 0
   with a command, because as prose here it did not fire and cost a day;** (c) the
   baseline's real weaknesses are truncation-fragility and salience-drop under noise - gold that
   survives a covering pattern but sits past that horizon is where the margin lives (the sentry law).
   The bar recalibrates from every vertical's losses, not from a one-time backtest.
4. **Blast-retrievable** - candidate deps survive the DEFAULT blast at min-confidence 0.3 AND 0.7, inside
   the output budget (the `resolve_oracle.py` preflight logic, run at admission instead of preflight). This
   also catches product regressions early (a broken anchor, as on saleor, fails here loudly).
5. **Memorization screen** - `memorization_probe.py`: closed-book, no tools, scratch cwd; ask the bench
   model for the anchor's own members and score recall against the index. Recall ≥ 0.30 = recited =
   reject. Calibrated 2026-07-29: laravel `Model` 0.857 and `Dispatcher` 1.0 vs 0.0 on every admitted
   anchor. A marker interface with no members is INAPPLICABLE, not a pass by default.
6. **Slot + pillars fit** - §7.0 composition (`1 framework + 1 big + 2 medium`, or the `2 big + 2 medium`
   variant, never both), two independent win pillars, a same-type backup per slot.
7. **ADVERSARY PROBE - unconditional, and the only bar that has never been wrong.** One frontier
   subagent in the clone, grep and read only, Sense forbidden, headline task only. It either assembles
   the answer or it does not, and admission waits on that verdict (`--adversary`). No threshold gates
   it, because measurement removed the threshold: on 2026-07-30 two anchors were probed and both died
   at control 1.000, and on both the numbers were wrong. akaunting's precision bar admitted it. A
   proposed coverage bar flagged akaunting and predicted filament would SURVIVE. The probe killed both.
   Metrics 0 for 2, probe 2 for 2.

   filament is why no number can replace it: token cover 0.23, no usable covering pattern, and its
   adversary still reached every dependent by grepping the token, then grepping `extends <base>`, then
   writing a class-hierarchy scanner. That is anatomy law #4 (statically-declared hierarchies are
   $0-enumerable at any depth) and bar 3 deliberately does not score hops, so nothing it computes can
   see the shape.

   The other bars are not discarded - they are the PRIOR that tells the probe where to look. They are
   simply no longer the gate. Cost: one $0 subagent per candidate, batched across the slate per the
   run-first ruling's rule 4.

8. **Channel + banner** - bar 7 scans the repo's own docs for a community-channel link and for an
   anti-LLM banner. A banner never rejects: it flags a strip from BOTH arms (the lobsters rule).
   `provision-repos.sh` carries the same scan at clone time.

**The try-harder law (the owner, 2026-07-13):** a slot is NEVER pitched to the human as an
"honest boundary" / ballast row. Since the 2026-07-20 ruling removed the small slot, this
covers ALL FOUR slots - there is no lane the framing is allowed in. A pool whose
candidates all measure BALLAST-ONLY means the CONTRACT
HUNT was too shallow (verdicts are per-contract, not per-repo: enumerate top hubs from the index
and gate each) or the POOL is too narrow - try harder on both before any escalation, and never
present a boundary framing as the slot's goal.

**Slot-awareness:** bars 1-5 gate every slot. The honesty-ballast lane is GONE (ruling 2026-07-20:
the small slot is removed and replaced by a second medium, because a small repo cannot produce a win -
its corpus is readable inside the wall). So a BALLAST-ONLY verdict no longer admits anywhere: it fails
the slot and sends the hunt back to the contract or the pool per the try-harder law.

## A bar may gate only while it admits the wins we already banked

**`bench/lib/gate_backtest.py` is the gate's licence to gate, and `admission_gate.py` reads it at
verdict time.** While the backtest fails, every REJECT is returned as `ADVISORY-REJECT` with the reason
attached: the bars still report, they stop blocking. A missing attestation counts as failing, because an
unverified gate is not a verified one.

Measured 2026-07-30, the first time anyone ran it: the gate rejected **4 of 4 banked go wins** - pebble
(+1.000), dolt (+0.708), nomad (+0.567), consul (+0.538). Three of them it had itself labelled "win
signature" one bar earlier, then rejected on a later bar. dolt it killed outright as BALLAST-ONLY
because the token `DoltDB` covers 92.5% of its dependents: true, and irrelevant, since dolt's baseline
still scored 0.25/0.33 - the question was never "where is the token", it was "what retains this object".

**Why this is machinery and not a paragraph.** The rule "backtest a filter against known positives"
was already implicit in bar 2's own calibration, which was false-negative-checked against the banked
wins ("> 0.50 kills 10 cells and 0 wins"). That standard was never applied to the other six bars, and
the gate then ran for four verticals - python, php, go, php again - without ever producing a win.
The evidence needed to falsify it sat frozen on disk the whole time and cost one command to read.

The general form, which is the expensive lesson of 2026-07-30 and the reason this section exists:

> **On a stalled vertical, the FIRST move is a differential comparison against the banked wins, not a
> deeper analysis of the failure.** A day went into improving gates, probes and metrics on php-laravel
> while four winning go cells sat unexamined. The moment they were compared, the answer took fifteen
> minutes: the wins share a question shape (transitive retention through interface-typed fields) that
> rests on a composition density php does not have - 3.3-5.5% of edges in go, 0.06-0.31% in php.
> Two metrics were proposed and both were wrong before that comparison was run; neither would have
> survived it.

Same rule, stated for anything that filters: **if it has never produced a positive, backtest it against
known positives before using it again.**

## Stop conditions

- **Success:** 4 admitted per §7.0 + backup per slot, SHAs pinned in `verticals/<key>/PINNED_COMMITS.json`, both arms
  cloned, all indexes built, measurements recorded per candidate in `repos.md`.
- **Budget:** indexing wall-clock (a giant repo dominates; log each index's wall-clock and gate on it) and pool
  size. Pool exhausted for a slot → failure path, not more scraping.
- **Failure:** a slot cannot be filled at the bar AFTER the pool is genuinely exhausted - every candidate
  in the stack gated, on more than one anchor ranking, numbers recorded. Then and only then it escalates,
  with the measurements, for: widen the pool, switch the §7.0 variant, or resequence the stack. The loop
  never lowers the bar to fill a slot. Live precedent (php-laravel, 2026-07-29): the big slot's BACKUP is
  unfillable - 28 repos sized, only two clear the big floor and both are in the slate.
- **Every screen and gate verdict discharges the Class-5 protocol** (scope inflation - the claim outruns
  the measurement's reach, [`decision-errors.md`](../decision-errors.md); ratified 2026-07-15). A screen
  bounds exactly what it measured: name the axis / edge kind / tool / anchor set in the same sentence as
  the verdict, and therefore what is NOT concluded. **A candidate that dies on one axis is not a dead
  repo, and a $0 screen never shrinks the program order** - bank its saving, never its verdict.
  Quantitative claims carry their sampling method and interval. Live precedent for why this binds here:
  the Go board's `s07` (survive-at-0.7) was itself measured on a band starved by the declared-receiver gap, so no K5 kill on
  a receiver-heavy anchor is final until a re-gate on the fixed band.

## Human events

**None. This loop is autonomous** (ruling 2026-07-29). the admission sign-off - slate confirm + SHA pin - was one of
the program's four permanent anchors; it is retired here, and the repo slate is now the loop's own
decision. The remaining three anchors (scenario/ground-truth integrity, spend, publish) are untouched:
nothing this loop decides costs a dollar or reaches a reader, and everything it decides is a number on
a real index that a re-run reproduces.

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| - | - | - | - |

What replaces the human: the gate decides, and the decision is written down where it can be audited
after the fact. The slate composes when all seven bars pass mechanically (§ below), the SHAs pin from
the clones, and the `loop2/slate` LEDGER entry records the gate numbers per slot plus the runner-up it
beat. A slot that cannot be filled at the bar does NOT escalate - the loop widens the pool and hunts
again (the try-harder law). It escalates only when the pool is genuinely exhausted for a slot: every
candidate in the stack gated, on more than one anchor ranking, with the numbers recorded.

Backup admission when Loop 3 swaps a repo runs the same way - same gate, same autonomy, no waving in.

## State / memory

- `repos.md` (doc side): candidates, contracts, per-candidate gate measurements, slot map, backups.
- `verticals/<key>/PINNED_COMMITS.json`, `verticals/<key>/repos.txt`, cloned arms + indexes under
  `$SENSE_BENCH_ROOT`.
- **Index-freshness rule (owned here):** if Loop 7 shipped a scan/resolve-layer change since the last
  index, ALL slate indexes rebuild (`sense scan -rebuild -embed`) before any authoritative sweep;
  query-layer changes do not trigger this. **Index candidates with `bench/lib/ensure-index.sh <repo> ...`,
  not `rescan-all.sh`:** the driver scopes its repo list from `verticals/<key>/repos.txt`, which is empty
  until this loop composes the slate, so it cannot reach a pre-slate candidate. `ensure-index.sh` takes
  repo names directly and skips anything whose scan fingerprint already matches. Run its `--check` over
  the pool first - a fingerprint mismatch means every gate number from that index is void
  (php-laravel, 2026-07-29: 17 of 19 pooled indexes predated the last PHP resolver fix).
- **Readability duty:** append `loop2/slate` to `verticals/<vertical>/LEDGER.md` when the slate
  composes; per-candidate numbers stay in `repos.md`, the entry links to them
  (contract in [`00-ledger.md`](00-ledger.md)).

## Un-fakeable check

- The gate numbers on the real index: grep-invisible edge count, covering-pattern scan, blast retrieval at
  both confidences inside budget, banner scan hits, idempotent provisioning against pinned SHAs. Prose
  cannot move any of them.

## Inputs / outputs

- **Consumes:** Loop 1's scaffold; the stack's candidate pool; designed-and-waiting axes staged for the
  stack (for Go: the fan-question axis contracts from the fan-question axis (killed as a class, 2026-07-06)); the
  loss-anatomy ledger (bar 3's recalibration input, appended by Loop 5).
- **Produces:** the pinned, provisioned, indexed slate of 4 + backups; per-candidate measurement records;
  the slot map with the two win pillars named.

## Fixture test (standalone, $0) - doubles as threshold calibration

Run the gate against the frozen rails + python-django indexes; known history is the answer key:

- **Must FAIL:** haystack (bar 2: 0/1256 grep-invisible), pretix (bar 3: `related_name` covering pattern),
  saleor-on-current-main (bar 4: broken anchor, and that catch is the gate working).
- **Must PASS:** sentry (+0.60, 4-arm win), netbox (+0.35), chatwoot-shape repos from rails.
- **Must measure BALLAST-ONLY:** healthchecks (enumerable Transport, benched 18/18 tie). Under the
  2026-07-20 ruling that verdict is a rejection, not a ballast admission; the fixture keeps it because its
  numbers are the calibrated example of the class.
- Calibrate bar 2's threshold as the widest margin that separates the two lists; document it in this file
  when set.

## Built vs missing

- **The loop is one command** (built 2026-07-29): `bash bench/drivers/loop2-hunt.sh <vertical> --write`
  runs pool → clone both arms → size gate → index → rank anchors (`anchor_rank.py`) → gate every
  survivor (`admission_gate.py`, all 7 bars) → probe the survivors (`memorization_probe.py`) →
  compose (`compose_slate.py`) → verify (`slate_check.py`). The pool is DECLARED in
  `verticals/<v>/pool.txt`, not discovered: a pool the loop invents each session is a pool whose
  "exhausted" claim cannot be checked.
- **Re-run it cold before trusting it.** `rm -rf verticals/<v>/.gate-cells` then run twice and diff the
  per-anchor verdicts. That check is what found four bar-5 defects on php-laravel; the deterministic
  bars held 326/326 cells identical, and every disagreement was the one bar that calls a model.
- **Built:** `seam_hunt.py` v3 (`--propose`, `--file`), `structural_surplus.py`, `resolve_oracle.py`,
  `provision-repos.sh` (pin + clone + banner scan), `ensure-index.sh`, the §7.0 composition law, the
  scenario-sourcing runbook.
- **BUILT 2026-07-11:** the gate script (`bench/lib/admission_gate.py`: bars 2/3/4 measured, bars
  1/5/6/7 then printed as a human checklist, measured since 2026-07-29; covering-pattern battery with per-pattern cover AND precision;
  composite `slot_verdict()` = win signature + kill rules K1-K4), calibrated on the 8-cell backtest
  (the admission backtest, internal tree: `bench/results/admission-backtest-2026-07-11.md`, 8/8 vs history; wagtail = the documented
  false-positive that dies downstream in the scout dig). The loss-anatomy ledger is BUILT
  ([`loss-anatomy.md`](loss-anatomy.md); Loop 5 appends from here on).
- **Missing / flagged:** the saleor bar-4 mystery is RESOLVED (2026-07-12 investigation): the #191
  fold fix was already merged (PR #192, in v1.11.17+), AND bar 4 as built could never have caught
  the break - it probes blast-only, and the pre-fix blast output was byte-identical at both
  confidences (the collapse was graph-side: called_by 126→1 on binary swap). **Bar 4 recalibration BUILT
  2026-07-12 (uncommitted):** bar 4 now also probes `graph --direction callers`; FAIL when
  called_by ≤ 5 AND blast direct@0.3 ≥ 10× called_by (collapse = graph tiny while blast numerous;
  healthy-min healthchecks 25/27 vs collapse 1/60). Verified: v1.11.16-saleor now FAILS, 8/8
  backtest verdicts preserved with exact number matches, main-saleor passes; addendum appended to
  the backtest doc. Joint-accessor-family union cover (pretix currently dies via K2, not via its
  accessor family) is a future battery refinement; haystack/pretix backtest cells are HEAD-of-day
  clones, not pinned history.
- **First live use:** Go vertical repo selection. The fan-question axis supplies candidate contracts
  (interface-satisfaction seams; `satisfy.go` verified on gin); gin is already swept for the conventions
  ledger; every other admitted repo sweeps here, in this loop, as it is indexed.
