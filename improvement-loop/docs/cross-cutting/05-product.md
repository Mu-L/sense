---
slug: what-the-bench-bought-sense
role: "cross-cutting fact pack - the product Sense gained by benching against real community repos (the byproduct that pays for itself)"
status: seeded (Rails)
data: git log (PRs #136–#161) + verticals/*/results/ + manifesto §12
byline: "Luc B. Perussault-Diallo"
rigor: factual (shipped commits); cumulative across verticals
---
# What the bench bought Sense

**The finding.** Benchmarking against a community's real repos is not just a marketing exercise; it is a
product-discovery engine. Every vertical surfaces concrete gaps in Sense that no synthetic test would, and
shipping the fix makes the tool better for *every* user, not just the bench. The manifesto's rule (§12) is that
product fixes are **byproducts, never the goal** - but they are real, and this pack is the running ledger of
them. "The same benchmark that proved the wins also improved the tool" is itself one of the most credible
stories the campaign tells.

## Why this is a cross-cutting article, not a per-repo note

A fix flagged by Chatwoot helps Django users too. The product byproducts are a property of *benchmarking against
real code*, not of any one stack. So they accumulate here, across verticals, the same way the methodology and
provider findings do.

## The ledger (one row per shipped byproduct)

| vertical | fix | what it was | flagged by | commit / PR |
|---|---|---|---|---|
| Rails | `file:` disambiguator | `sense_blast`/`graph` + MCP take a `file` param to disambiguate reopened-class / cross-language name collisions; the MCP hint steers the agent to retry | Mastodon (`Status` ambiguous; first run lost) | `aee5f86`, #157-era |
| Rails | deterministic high-fan-out blast cap | `capResults` was nondeterministic on high-fan-out symbols and evicted direct callers; conf/direct/id tiebreak | GitLab (`MergeRequest`, biggest fan-out; deps +0.38→+0.62) | `3002e1b`, #153 |
| Rails | `acts_as_*` mixin resolver | resolve `acts_as_*` dynamic mixin dependents to collaborator classes (+ nil-source guard) | Solidus (cross-gem promotion deps) | `3fc5717`, `3737a00` |
| Rails | config-string edge resolver | resolve `class_name`-attribute config-string edges (the 29-02 macro idiom) | Solidus / deep-namespace work | `279e280`, #151 |
| Rails | relative-superclass resolution | resolve relatively-named superclasses via lexical scope | deep-namespace resolution | `7eff3e1`, #151 |
| Rails | conventions de-noise + re-rank | filter synthetic-prefix symbols, category re-rank, Ruby significance refiner (192→39 naming lines on maket) | the weak `sense_conventions` axis | `15f5615`, #161 (v1.11.1) |
| Rails | conventions label/order determinism | qualify colliding bases, exclude test scaffolding, disambiguate repeated labels, deterministic example order | conventions oracle instability | `cde9b50` #154, `cfad6b2` #143, `b47c291` #142 |
| Rails | right-size blast output | area-stratified enumeration so structural output survives a narrow channel | the cheap-arm throttle (`02-providers.md`) | `371d2f9` |
| Rails | reach-at-parity reporting | headline unbounded reach, demote completeness to a shape-gate | the completeness-dilutes finding | `83070ab` |

## Per-vertical detail

### Ruby/Rails (seed)

- **Three "headline" product fixes** map one-to-one to three wins: the `file:` disambiguator flipped Mastodon
  (lost on the ambiguous `Status` first run, won after), the determinism cap drove GitLab's deps from +0.38 to
  +0.62, and the `acts_as`/config-string resolver pair unlocked Solidus's cross-gem promotion dependents that
  no grep reaches.
- **A whole conventions sub-campaign:** `sense_conventions` was the weakest axis (Rails conventions drowned by
  synthetic `i18n:`/`route:`/`partial:` symbols, 192 naming lines on the maket test repo). The fix pack
  (de-noise + re-rank + significance refiner + label/order determinism, PRs #142/#143/#154/#161) made
  project-specific bases lead over memorized framework idioms. Product DONE; the winning bench scenario on
  conventions is now SCHEDULED, not hunted per-vertical (DECIDED 2026-07-06): it runs at the local-law build
  gate (write-task bench, `01-production-spec.md` (private tree) §8)
  and again as an end-of-program cross-stack pass, with every vertical harvesting conventions material for $0
  into `02-bench-harvest.md` (private tree). The direction doc remains
  `00-principles.md` (private tree) (turning
  `sense_conventions` from descriptive into normative/enforceable).
- **Two output-shape fixes that came straight from the bench's pain:** right-sizing `sense_blast`
  (area-stratified, `371d2f9`) is the real fix behind the cheap-arm throttle, and the reach-at-parity reporting
  change (`83070ab`) corrected a metric that was hiding Sense's own advantage.
- **The discipline (§12):** before shipping any fix for user value, re-assess the gap is still factual in
  current `main`, and after the fix test against repos of all sizes (one run each) for side effects. A resolver
  change must not regress blast/graph/dead on unrelated symbols.

## Data points by vertical

| vertical | when | byproducts shipped | theme |
|---|---|---|---|
| Ruby/Rails | 2026-06 | 9 (3 resolver, 4 conventions, 2 output-shape) | dynamic-dispatch resolution + conventions denoise + output right-sizing |
| Python/Django | TBD | … | (typed-er decorators / ORM magic likely surface different gaps) |
| Go | 2026-07 | Harvest over 39 already-paid sense runs, $0. **Two instruments converged on one surface:** a mechanical transcript mine found 7 discriminator-group CITED-NOT-RETURNED misses across consul (4) and nomad (3) - every one an interface-NARROWED retention field the agent had to find by reading - while the agent survey channel independently produced, from 4 models across 8 repos, the same top ask: give `retained_via_interfaces`/`composed_by` the retaining field's own name and line, plus the assignment site. Unprompted self-report and mechanical mining agreeing is the strongest product signal this program has produced. Also parked: same-symbol/same-index returns varying across runs INCLUDING collapse to zero (`sense_graph:dsess.SqlDatabase` [0, 48], `pebble.Batch` [0, 19, 29]) - either a regression of the fixed high-fan-out determinism defect or a Go variant, and which one changes the fix. Blind spot recorded: `sense_conventions` was called 11 times across 39 runs (0.28/run), so this campaign says NOTHING about conventions quality on Go, in either direction. | **refines** |

## Open questions (carry forward)

- Which byproducts **generalize** (output right-sizing, determinism) vs which are **stack-specific** (the
  `acts_as` resolver)? The generalizing ones are the strongest article material and the best auto-improvement
  candidates (`../goal.md`).
- Residual resolver gaps Rails logged but did NOT fix (pub-sub `DiscourseEvent.trigger`/`on`; some
  dynamic/data-flow edges) - the roadmap, and candidate byproducts for a future vertical.
