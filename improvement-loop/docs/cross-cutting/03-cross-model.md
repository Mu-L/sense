---
slug: the-maps-value-scales-with-the-model
role: "cross-cutting fact pack - how much the same local index helps across the LLM ladder"
status: current (Rails full 5×13 matrix + Python/Django full 5×6 matrix)
data: verticals/ruby-rails/results/report.{md,json} + verticals/python-django/results/report.{md,json} + 00-campaign-scorecard.md (private tree)
byline: "Luc B. Perussault-Diallo"
rigor: Opus 4.8 hardened ×2 both verticals (headline); Kimi/Qwen/Devstral ×2 (exceptions flagged OPEN); Rails GPT-5.5 mostly ×1 (OPEN)
---
# The map's value scales with the model - *where the stack is grep-hostile*

**The finding (revised by the second vertical).** The Rails seed said "the frontier model extracts the most."
The full two-vertical matrix says the claim is **stack-conditional**: on a grep-hostile stack (Rails) the
frontier headline still converts the most (deps Δ +0.48, the top of both matrices), but on a textual,
declared-surface stack (Django) the ordering **inverts** - the weakest model gets the biggest lift and the
frontier's advantage shows up as efficiency-at-parity instead of recall. The durable, two-vertical constant:
**mean deps Δ is positive on every arm in both matrices** (10 of 10 rows). The map's value scales with the
*headroom* between what the model's baseline can reach by reading and how much of the dependent set hides in
the stack's dynamic seams - not with raw capability alone.

## The numbers (cited-recall lift, sense − baseline)

### Ruby/Rails (13 repos, matrix complete)

| arm | repos | overall Δ | deps Δ (discriminator) | billed Δ% | tier |
|---|---|---|---|---|---|
| Claude Code · Opus 4.8 | 13 | **+0.26** | **+0.48** | +0% | hardened ×2 (headline) |
| OpenCode · Devstral-Small-2 | 13 | +0.25 | +0.36 | −22% | breadth ×2 (chatwoot ×1 - OPEN) |
| OpenCode · Qwen3-coder-next | 13 | +0.18 | +0.24 | +14% | breadth ×2 |
| Codex · GPT-5.5 | 13 | +0.13 | +0.29 | −9% | cross-agent frontier; ×1 on 10/13 repos - OPEN |
| OpenCode · Kimi K2.7 | 13 | +0.14 | +0.18 | −10% | breadth ×2 (matrix completed post-cap) |

The frontier headline still tops the deps discriminator (**+0.48** vs +0.36/+0.29/+0.24/+0.18), but the
Rails-final ladder already breaks the seed's clean story: the *weakest* model (Devstral) is second, and the
*second frontier* model (GPT-5.5) lands mid-pack.

### Python/Django (6 repos, matrix complete)

| arm | repos | overall Δ | deps Δ (discriminator) | billed Δ% | tier |
|---|---|---|---|---|---|
| OpenCode · Devstral-Small-2 | 6 | **+0.17** | **+0.24** | +20% | breadth ×2 |
| OpenCode · Kimi K2.7 | 6 | +0.06 | +0.18 | −12% | breadth (sentry + saleor wins ×1 - OPEN) |
| Codex · GPT-5.5 | 6 | +0.08 | +0.17 | −9% | cross-agent frontier ×2 |
| Claude Code · Opus 4.8 | 6 | +0.05 | +0.16 | +3% | hardened ×2 (headline; board 3W/3T/0L) |
| OpenCode · Qwen3-coder-next | 6 | +0.08 | +0.05 | −14% | breadth ×2 |

On Django the ordering inverts: the campaign's weakest model has its biggest mean lift (+0.17), and the
frontier headline has the smallest (+0.05) - because its baseline already reads a textual stack nearly to the
ceiling, so its win migrates to efficiency (sentry ◆ −7% billed at held recall) and to the deps group
(netbox 0.67→1.00, saleor 0.50→1.00).

## The honesty caveats (mandatory, every time)

1. **Not monotonic, in either direction.** Rails: Devstral (weakest) beats GPT-5.5 and Qwen; Kimi (mid open)
   sits last on deps. Django: Qwen's deps mean (+0.05) is the honest outlier at the bottom while its overall
   ties GPT-5.5. State the jagged ladder; do not smooth it into a clean ranking either way.
2. **The open and cross-agent arms carry real losses/negative cells** (×2 means from the final matrices):
   Rails - Devstral raix −0.20, Qwen redmine −0.03, GPT-5.5 langchainrb −0.03. Django - Kimi netbox −0.04
   (tie at ceiling; its baseline memorizes netbox). Most cheap-arm noise traces to the channel, not the map
   (`02-providers.md`); **this pack depends on `02-providers.md`, read/write that one first.**
3. **Run-count flags.** RUNS=2 is the settled standard; ×1 cells are OPEN: Rails GPT-5.5 is ×1 on 10 of 13
   repos (×2 only discourse/gitlabhq/langchainrb); Rails Devstral chatwoot (+0.74, its biggest cell) is ×1;
   Django Kimi sentry +0.24 and saleor +0.15 are ×1 win-bar-cleared.

Recall is comparable across agents; **cost is not** (report cost only within one agent). The cross-agent lift
is **conditional on adoption** (no tool calls → sense arm ≈ baseline arm; see `04-harness.md`).

## Why it scales with headroom, not raw capability (the revised mechanism)

Two forces pull in opposite directions. (a) A stronger model adopts the tools more readily and converts a
structural result into correct, cited answers more reliably - that is why Opus tops the Rails deps ladder.
(b) A stronger model's *baseline* also closes more of the gap unaided - on a textual stack like Django it
greps and reads its way to near-ceiling recall, leaving the map little recall headroom and pushing the
frontier's win into efficiency. The weak arms have headroom everywhere, so their lift survives even on the
textual stack (Devstral +0.35 on netbox, +0.29 on sentry). The map substitutes for the navigation ability
the small model lacks, and multiplies the ability the big model has - whichever margin is open.

## Data points by vertical

| vertical | when | the cross-model data point | agrees / refines / reverses |
|---|---|---|---|
| Ruby/Rails | 2026-06 | Final 5×13 matrix: frontier deps Δ +0.48 still ≫ the rest, but Devstral (weakest) is 2nd overall (+0.25/+0.36) and GPT-5.5 (2nd frontier) mid-pack (+0.13/+0.29, mostly ×1 - OPEN); every arm's deps Δ positive. | seed, self-refined at matrix close |
| Python/Django | 2026-07 | Full 5×6 matrix: ordering inverts - Devstral biggest mean lift (+0.17/+0.24), Opus smallest (+0.05/+0.16, board 3W/3T/0L, sentry ◆ eff-at-parity −7%); GPT-5.5 sentry +0.35 at −23% billed; every arm's deps Δ positive, 0 losses on all 5 arms. | **reverses** the seed's ladder; refines the claim to headroom |
| Go | 2026-07 | 6 models x 4 repos (consul/dolt/nomad/pebble). **Sense-arm recall is bimodal by MODEL, and by nothing else:** of 38 sense-arm runs, 33 sit at cited_recall >= 0.92 (opus 20/20 and kimi-k3 4/4 at 1.00, fable 2/2 at 1.00, gpt-5.5 min 0.92) and ALL FIVE sub-bar cells belong to the two ollama-cloud arms. Killed hypothesis, recorded so it is not re-raised: "Sense wins as a targeting layer, arms that query it then stop score worst" REVERSES on the full board - cells >= 0.90 average 6.6 Sense calls and 18.5 follow-up reads, cells < 0.50 average 8.6 calls and 26.6 follow-ups. gpt-5.5/dolt scores 1.00 on 9 calls and ZERO follow-up; glm/pebble scores 1.00 on 3 calls and 109 follow-ups. Tool mix and adoption score do not predict recall. Not a flat ceiling either: both ollama arms reach 1.00 on one repo each. | **agrees** (lift holds down the ladder) **+ refines** (the mechanism is not tool-mix) |

## Per-vertical detail

### Ruby/Rails (seed, closed) - the per-repo cross-model texture

- **The matrix closed at 5×13** (the product repo's `bench/verticals/ruby-rails/results/report.md`): Opus +0.26/+0.48 ×2;
  Devstral +0.25/+0.36 ×2 (chatwoot +0.74 ×1 - OPEN - and redmine +0.58, but raix −0.20, its one real loss);
  Qwen +0.18/+0.24 ×2 (redmine −0.03 the one negative left after settling); GPT-5.5 +0.13/+0.29 with
  langchainrb −0.03 (mostly ×1 - OPEN); Kimi +0.14/+0.18 ×2, no negative cell (floor +0.00 on redmine).
- **The mid-campaign loss cells settled out.** The seed-era caveats (Qwen mastodon −0.09, Kimi discourse
  −0.12, Kimi's 12-repo partial) do not survive the final ×2 means: mastodon is +0.07 on Qwen, discourse
  +0.10 on Kimi, and Kimi is 13/13. Cheap-arm channel noise (`02-providers.md`) explains why the partial
  matrix looked worse than the settled one.
- **The GPT-5.5 answer to the seed's open question:** a second frontier model on a different harness did
  *not* replicate Opus-level extraction (+0.13 vs +0.26 overall). Part capability, part harness: its
  adoption had to be wired via `AGENTS.md` (`04-harness.md`), and its Rails cells are mostly ×1 (OPEN). The
  seed claim shifted from "scales with the model" toward "scales with headroom × adoption" already at Rails
  close.
- **The Devstral floor test passed loudly:** the weakest model's lift stayed not just positive but
  near-headline (+0.25 overall at −22% billed), foreshadowing the Django inversion.

### Python/Django (2026-07, closed) - the inversion, arm by arm

Sources: the product repo's `bench/verticals/python-django/results/report.md` (5×6 matrix + efficiency),
`00-campaign-scorecard.md` (private tree) (canonical boards, hand-verified
2026-07-11), the product repo's `bench/verticals/python-django/matrix-plan.yaml` (retrospective plan).

- **Opus 4.8 (headline, ×2, board 3W/3T/0L):** saleor WIN +0.15 (deps 0.50→1.00), netbox sense-ahead +0.11
  (deps 0.67→1.00), sentry ◆ efficiency-at-parity (+0.03 at −7% billed, deps 0.90→1.00), and three control
  ties by design (wagtail/healthchecks/litellm). Mean +0.05/+0.16 - the smallest lift on the board because
  its baseline scores 0.80–1.00 cited recall everywhere on this textual stack.
- **GPT-5.5 (×2):** sentry WIN **+0.35 at −23% billed**, five ties, 0 losses; sense never lost recall in any
  of the 12 runs and billed less in 9 of 12. The second frontier model's one big Django cell is exactly the
  grep-dark repo (contract-embedding deps).
- **Kimi K2.7:** sentry +0.24 (deps +0.60) and saleor +0.15 (deps +0.50), both **×1 - OPEN**; litellm +0.04
  ×2; netbox −0.04 (tie at ceiling - its baseline memorizes netbox at 0.96–1.0). Mirrors the Opus shape with
  bigger deltas.
- **Devstral (weakest, ×2):** netbox **+0.35**, sentry +0.29 (billed −30% on that repo), saleor +0.15,
  wagtail +0.15, 0 losses - the biggest mean lift of the campaign (+0.17) from the weakest model. Its +20%
  arm-mean billed is thrash-draw inflation on an off-platform meter, not comparable across agents.
- **Qwen (×2):** wagtail +0.19 (the control row that separates *only* on a weak model), netbox +0.13,
  litellm +0.13, sentry an honest tie (+0.00 - the cell that broke sentry's four-arm streak), 0 losses.
- **The repo shape decides who wins, not just the model:** sentry (unclosed dispatch) separates on every
  cross-agent arm while Opus converts it to efficiency; netbox/saleor (Django's dynamic edges) separate on
  the headline; the three textual controls tie on the frontier and separate, if at all, on the weak arms.
  Memorization is a real confound on famous repos (GPT-5.5's baseline recites ~87% of the netbox set).

## Open questions (carry forward)

- ~~Where does GPT-5.5 land relative to Opus?~~ Answered at Rails close: below Opus (+0.13 vs +0.26), and
  the story did shift toward headroom × adoption. Still OPEN in rigor: 10 of its 13 Rails cells are ×1.
- ~~Does the lift stay positive at the bottom of the ladder?~~ Answered twice: Devstral is second (Rails)
  and first (Django) on mean lift. New question: is "the weakest model gains the most" *specific to textual
  stacks*, or does Go (vertical #3) show the same inversion?
- Does the headroom framing predict Go? Prediction on record: a statically-typed, tool-rich stack should
  compress *both* baselines - watch whether the deps Δ stays positive on all arms (the two-vertical
  constant) even if overall Δ compresses toward Django-sized margins.
