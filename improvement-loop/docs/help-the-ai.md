# What it means for Sense to "help the AI" - the canonical definition

> **Status: canonical.** This is the operational definition of "help the AI / bring value to the AI" - the gate
> every Sense change must pass to earn the right to ship. It graduated here from
> [`goal.md`](goal.md) §"Defining
> 'help the AI'". It **derives from** the bench bible ([`manifesto.md`](manifesto.md)
> §9) and the [judging contract](judging-contract.md); when this doc and the manifesto disagree, **the
> manifesto wins** and this doc is corrected. Thresholds may tighten as verticals accumulate; the shape is settled.

## The premise

Sense's consumer is the coding agent, not the human. So "bringing value" is never measured by how a person reads
the output, but by what the *agent* can reach and assert that it could not have assembled on its own. The baseline
is the same frontier agent without Sense, armed with grep, read, and its own reasoning. Value exists only where
Sense lets that agent reach structure the grep-and-read path leaves on the floor.

## The definition

> **Sense helps the AI when, at held-or-better billed-token cost, the Sense-equipped agent reaches more of the
> codebase's real structure than the same agent reaches by reading and grepping alone - and asserts it correctly.**

"Real structure" is the graph of what calls what, what breaks what, and how the codebase writes itself: callers,
dependents, non-literal edges (associations, polymorphism, concerns), and project conventions. "Reaches" means
cites, grounded in actual file:line locations. "At held-or-better cost" makes reach the headline and cost a side
effect, never the reverse.

## The measurable form (the gate)

A change to Sense proves it helps the AI when, on a bench the change targets, measured ×2 on the frontier model
(the headline arm, judge pinned - `verticals/<key>/arms.txt`):

**Primary signal - reach at token parity:**

- **Cited-recall margin ≥ +0.50** (favored +0.80) on the **discriminator group**: the ~16 heterogeneous,
  non-obvious, scattered dependents that no single grep or directory walk collects (curated from `sense_blast` at
  DEFAULT `min_confidence 0.7`). This floor is non-negotiable. Other groups (`contract`, `context`, `surface`)
  park the obvious/memorized pieces both arms get and do not count toward the margin (`pergroup.py`).
- **Tool-only reach**: at least one dependent the Sense arm cites and the baseline never does (0/3). This is the
  factually-new value - structure that did not exist in the baseline's answer at any token budget.
- **Reach in fewer iterations** (secondary): at held reach, the Sense arm gets there in fewer agent-loop
  round-trips - one indexed call replacing an N-step `grep→read→grep` chain. Round-trips / tool-calls are the
  Sense-attributable speed signal; wall-clock seconds may accompany them but are explicitly noisy (provider
  latency and throttle, not Sense, set the clock), so seconds are softer evidence only and never a parity axis or
  a gate.

**Gates that keep the win honest:**

- **Grounding ≥ baseline** - every citation resolves to a real file at the pinned commit, and no *new confident
  contradiction* is introduced (a stated relation that conflicts with the authored truth;
  `relationship_audit` tri-state `covered`/`related`/`contradicted`). Omissions are allowed; confident-wrong
  claims are penalized. The bar is "≥ baseline," not "zero."
- **Adoption confirmed** - the Sense arm actually fired its tools (`mcp_count > 0`) on the discriminating step. A
  pure grep-fallback that happens to score well does not count. (Forcing adoption backfires - fix the scenario,
  not the steering.)
- **Legit baseline** - the baseline's lower recall is real (transcript-verified, per gold target), not a refusal,
  truncation, or harness artifact.

## Quality beyond reach

Reach is the headline because, on a well-resourced frontier model over a readable repo, both arms reason correctly
over whatever they have - so they tie on reasoning and separate only on what each arm managed to *find*. But
quality-beyond-reach exists in the framework, in three forms:

- **Grounding - a quality dimension expressed as a gate (proven).** "No new confident contradictions" is
  correctness of what's asserted, not breadth. Already in the definition; it lives in the gate rather than the
  headline.
- **Conventions / idiom-match - the product-unique quality lever (candidate).** Not "did the agent find the
  structure" but "does the code the agent *writes* match how this codebase writes." Genuinely orthogonal to reach:
  an agent can reach every dependent and still generate alien-looking code. The one thing nobody else does well.
  Not yet operationalized in the gate because it needs a generation-quality scenario, not a retrieval-recall one.
- **Correctness under pressure - quality when conditions tighten (candidate).** Late in a long session under
  context pressure, or under a tight tool/time budget, one cheap grounded Sense call stays correct where the
  baseline degrades. Quality, not reach.

The latter two are candidate axes, not headline, because they become the discriminator only when you change the
conditions - bespoke conventions the model can't recite, or pressure where cheap-and-grounded beats
expensive-and-derived. Prove such a scenario and quality graduates from gate/candidate to a second headline.

## What it is deliberately NOT

- **Not lower cost.** Token, round-trip, and time savings are side effects of understanding. If LLM inference were
  free tomorrow, the value would be unchanged, because structural answers are more correct than a grep chain. Cost
  and speed are reported only at held recall parity, never as the headline.
- **Not higher completeness.** Step-averaged completeness saturates near 1.0 and dilutes a 6× reach gap into a
  fraction of a point. It is a shape-gate ("did the agent do the task?"), not the metric.
- **Not a better judge score.** The reference-blind LLM judge is blind to omission and rates a 35%-recall answer
  "exhaustive." Truth is the transcripts and objective recall; when the judge disagrees with the transcripts, the
  judge is wrong (and gets fixed).
- **Not text search.** For tasks that are fundamentally grep (find a log message, locate a string literal), grep
  is the right tool and Sense adds nothing and claims nothing.

## Why this shape

Each clause exists to defeat a specific way a number can lie. The recall floor defeats the saturated-completeness
illusion. Tool-only reach defeats memorization (a frontier model recites famous public APIs, so the win must live
in the obscure internals it never memorized). Token parity defeats the cheaper-but-thinner trade that looks like
efficiency but is really a worse answer; the iterations companion captures the round-trip collapse without letting
noisy wall-clock into the gate. The grounding gate defeats hallucinated reach. The adoption gate defeats
accidental wins. Together they make "help the AI" a fact the bench can check, not a flattering composite - which is
the precondition for ever automating the gate (the bench-gate principle, `goal.md`ent-endgame.md`).

## One sentence

**Sense helps the AI when the agent, at equal token cost and in fewer round-trips, cites more of the real
dependency structure - including dependents it would otherwise never find - with grounding at least as honest as
the baseline's, and the bench can prove it on the hard, scattered, non-memorized parts of a real codebase.**

## Provenance

- Headline metric discipline (reach at parity; completeness/F1/cost demotions; adoption + grounding reporting):
  manifesto §9.
- Tri-state grounding (`covered`/`related`/`contradicted`), efficiency-only-at-parity reporting: judging contract.
- Discriminator-group spec (~16 heterogeneous deps at default `min_confidence 0.7`): manifesto §4.
- Definition-of-Done thresholds (≥ +0.50 floor, +0.80 favored, ×2 on the headline arm - the settled run count): manifesto §5/§14.
- The bench-gate principle this definition serves (nothing ships without a value-proving bench):
  `goal.md`.
