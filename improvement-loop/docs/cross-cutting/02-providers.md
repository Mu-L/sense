---
slug: the-cheap-arm-law-channel-not-model
role: "cross-cutting fact pack - why a weak benchmark result is usually a subscription/channel artifact, not a model verdict"
status: seeded (Rails)
data: verticals/ruby-rails/results/report.md (Qwen, Kimi) + manifesto §10/§11
byline: "Luc B. Perussault-Diallo"
rigor: breadth arms (open models on metered subs); never a headline-tier claim
---
# The cheap-arm law: it is the channel, not the model

**The finding (counterintuitive, and we have not seen it written anywhere else).** When a code-intelligence
tool "loses" on a cheap model, the cause is almost always the **delivery channel** (the subscription's
throttle), not the model's capability. We proved it the expensive way: the fix to a weak *result* was a stronger
*subscription*, not a stronger *model*.

## The mechanism

A structural tool's output is **additive**: the agent reads the tool's JSON *and* still reads files on top. So
the sense arm carries roughly **2× the token weight** of the baseline arm. On a token-metered small
subscription it is the heavier sense stream that crosses the throttle and gets **cut mid-answer**. The truncated
answer still clears a lenient length gate, so it scores as a real run, producing a **false loss**. That is the
exact mirror of the judge's false-tie blind spot: a measurement artifact dressed as a finding.

The tell: an uncontended re-run of the same scenario scored **0.83** where the throttled run scored **0.15**.
The model was never the problem.

## The proof (Rails, the Qwen → Kimi move)

We tried nearly everything on the Ollama-cloud Qwen arm (watchdog knobs, run spacing, sweep-resume), then moved
to Kimi for Coding. It helped **not because Kimi is a smarter model** but because the subscription throttles
less. Same tool, opposite efficiency sign across the two cheap subscriptions:

- **Kimi K2.7:** sense arm **−12% billed tokens** (the tool is cheaper here).
- **Qwen3-coder-next:** sense arm **+18% billed tokens** (the additive stream costs more on this channel).

Same local index, same tool, opposite sign, explained entirely by the channel.

## The discipline this forces

- **A "loss" under a metered sub is a load artifact until truncation is ruled out.** Never a tool-quality or
  model-capability finding.
- **Controls:** space the runs; run the heavier (sense) arm into a *fresh* window, not baseline-first into a
  spent one; tighten the gate to discard step-incomplete answers; re-run any throttled repo uncontended ×2.
- **The real product fix behind the symptom:** right-size structural output (count + by-area + top-N actionable
  with file:line) so it survives a narrow channel. Locus `mcpio.ApplyBlastBudget`. This helps every provider,
  not just the cheap arm.
- The headline frontier arm never throttles; this law governs the cheap/breadth arms only.

## Data points by vertical

| vertical | when | the channel data point | agrees / refines / reverses |
|---|---|---|---|
| Ruby/Rails | 2026-06 | Qwen false losses traced to throttle (0.15 throttled vs 0.83 uncontended); fixed by a stronger sub (Kimi), not a stronger model; efficiency sign flips Kimi −12% / Qwen +18% on the same tool. | seed |
| Python/Django | TBD | does the additive-stream multiplier hold on a more verbose stack? | … |
| Go | 2026-07 | **A retired model id is indistinguishable from a usage cap.** The kimi arm was recorded as "cap-blocked, 0 cells" for days; the provider had in fact RETIRED k2p5/k2p6/k2p7, and a dead id returns a generic `UnknownError` at ~21s with 0 tokens - the exact signature of a cap. Killer: k2p7 errors and k3 answers on the SAME subscription seconds apart. The arm then completed 4/4. Rule: before recording a cap, call a second id on the same subscription. Second data point, on trusting an arm's self-report: `survey_verify.py` transcript-verification puts confabulation at 38% (opus), 30% (kimi k3), 34% (glm-5.2) and **100% (mistral-large-3, 28/28 instances)** - and mistral self-scores HIGHEST of the four. A weak arm's self-assessment is anti-correlated with its evidence. | **refines** |

## Per-vertical detail

### Ruby/Rails (seed) - the cheap-arm saga in full

- **The Qwen arm was where we learned the law, the hard way.** Symptoms we chased before naming the channel as
  the cause: a stochastic *file-offload* (qwen3-coder-next wrote its detailed answer to a file and returned a
  ~3k-char stub → scored ~0, a harness artifact not a real loss); watchdog mis-calibration (STALL_IDLE=150
  killed throttled-but-healthy runs; corrected to 300 with MAX_SECS=1200); and a quota reality (mastodon
  baseline ~42M tokens/run ≈ 34% of a weekly quota, so win-order affords only 2–3 big repos per window).
- **The move to Kimi proved the thesis.** We tried nearly everything on Qwen, then switched to Kimi for Coding
  (Anthropic-compat, opencode ToS-sanctioned, ~$31–39/mo). It helped because the *subscription* throttles less,
  not because Kimi is a better model - research + a raix smoke (mcp=12, substantive answers) showed the ollama
  tool-call defects were serving-layer, not model.
- **The provider landscape under ~$40/mo** (the cheap-arm shortlist): GLM Coding Plan (cheapest, Lite $18, tight
  rate limits), Kimi Allegretto (chosen), MiniMax, Qwen, DeepSeek. The choice is a subscription-strength
  decision, not a model-quality one.
- **Operational scar:** never broad-`pkill -f sweep-resume.sh` - it once killed a *different* session's GPT-5.5
  sweep. Concurrent sweeps from other sessions are fine (own root + provider); only same-model + same-repo
  conflicts. A "Say OK" probe proves the rolling window, not the weekly quota.
- **The structural answer we don't have yet:** dedicated strong subscriptions per provider, or enough Sense
  popularity that providers grant access. For now: patience, advance with what we have.

## Open questions (carry forward)

- Provider ranking under ~$40/mo as the cheap arm (GLM Coding Plan, Kimi Allegretto, MiniMax, Qwen, DeepSeek):
  is the right cheap arm a per-vertical choice or a constant?
- Once `ApplyBlastBudget` is right-sized (`05-product.md`), does the Qwen +18% sign flip to neutral/negative?
  (A product win that this pack would record as a reversal-in-our-favor.)
