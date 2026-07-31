# Loop 1 - Repo authoring

> First of the three per-repo loops (authoring → [run](02-repo-run.md) →
> [diagnosis](03-repo-diagnosis.md)). The laws all three share live in
> [`campaign-laws.md`](campaign-laws.md) and are not repeated here.

**One repo at a time, to a verdict.** A repo enters the sequence here and does not leave it until it
wins, parks or is swapped. No second repo's scenario is authored while the first is mid-diagnosis.

**Which repo: the first in `repos.txt` order with no verdict on disk.** There is no ranking, because
any ordering claim made before a scenario exists is a predictor and predictors are banned
([`campaign-laws.md`](campaign-laws.md)). Slate order is arbitrary and honest; a ranking would be
arbitrary and dressed up. After a swap this loop does not choose either: [Diagnosis](03-repo-diagnosis.md)
hands over the slot's declared backup from `slate.json`, cycle counter reset to zero.

## Goal

Author one scenario and its gold for one repo off the slate, such that a code-capable baseline
with Sense forbidden cannot assemble the answer. Exit state: a stamped scenario plus rubric, gold
hand-audited per dependency, an adversary probe that failed to shortcut it, and the scenario-integrity gate signed.

The scenario is CRAFTED, and that is the whole job: nothing scores this repo before the scenario
exists. Diagnosis feeds the next draft, so a repo usually reaches its verdict through several
passes through here ([`03-repo-diagnosis.md`](03-repo-diagnosis.md)).

## Product duties (per Sense surface)

- **blast:** the gold must survive **both** `min_confidence` 0.3 and 0.7. A gold set that only holds at
  0.3 turns a documented default into a scoring artifact, and the contract defect around that param is a
  known live one - gold curated blind to it manufactures a win.
- **graph:** the target hop is chosen for structural surplus, not for grep-hostility alone. If a plain
  two-hop grep reaches the answer, the scenario is not measuring Sense, it is measuring patience.
- **search / status:** record at authoring time whether the shape *needs* them. If the winning shape
  never asks for either, that is a feature-coverage blind spot to hand Loop 5, not a verdict.
- **conventions:** nothing here. The slate sweep belongs to bootstrap.

## Identity

- **Character:** judgment. This is where a win can be manufactured by accident, so the adversary vertex
  and the human ground-truth anchor both sit inside this loop.
- **Unit of work:** one scenario plus its gold for one repo.
- **Position:** consumes one repo off bootstrap's slate, or the same repo back from
  [Diagnosis](03-repo-diagnosis.md) with a named lever; produces the frozen scenario artifacts
  [Run](02-repo-run.md) spends against.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | session agent, authoring serially - never forked onto shared files | fork swarms on one scenario file confabulate |
| Evaluator | the **adversary probe**: one frontier subagent in the baseline clone, grep and read only, Sense forbidden, headline task only | separate from the author; it is trying to beat the scenario, not improve it |
| Mechanical verifier | `scenario.py --prompt` leak check, `audit_scenarios.py`, `gold_confidence_check.py` (0.3 vs 0.7), per-dependency hand audit of gold credits | the basename false-credit trap is real: a script tally alone has passed wrong gold before |
| Human | signs off scenario and ground-truth integrity | **permanent anchor, never demotable** |

## The adversary probe (design-time kill, $0)

Run it **before** gold curation, not after. Its two outputs are both load-bearing:

- Its **method** is the list of dead shapes - anything it reached without Sense is a shape that cannot be
  benched.
- Its **honesty disclaimer** is the discriminator axis, verbatim. What the probe says it could not
  establish is what the scenario must ask for.

**Only probe-disclaimed shapes proceed to gold.** This is the design-time kill; the validation run in
[Run](02-repo-run.md) is the separate spend-time go/no-go. One does not substitute for the other: a shape
can survive the probe and still fail to reach at the cell's wall.

## Memorization is a gold-time constraint, never an admission verdict

A famous repo is not disqualified; a memorized gold row is. The model recites what it has seen, so
every gold target must be either churn-dated after the model's snapshot or a line-level structural
fact (callers, edges) the probe demonstrably cannot produce. Run `bench/lib/memorization_probe.py`
per item at gold time and cut what comes back recited.

Two measured shapes, so neither gets rediscovered. On an evergreen-famous repo the weights are simply
CORRECT, so the only discriminators are recency-dated internals and structural facts. Where the weights
are stale they are confidently WRONG, which sets what may be credited but is **not** a win axis in
itself: with repo access a strong agent self-corrects in about two moves.

## Gold curation rules

Gold is built here from scratch for the shape the adversary probe disclaimed, and it is written to
`scenarios/<repo>.yaml`.

- **One item per FILE.** A gold group that lists several symbols from one file rewards a single read.
- **Hand-audit every credit.** Basename matching has awarded credit for the wrong file; run the tally,
  then read the credits.
- **Watch the context radius, not the hit count.** A judgment repeated across many hits is batchable by a
  baseline if each hit is decidable locally. The litmus for a batchable, therefore unwinnable, ask is a
  *narrow per-hit context radius* - hit count is irrelevant.
- **Gold is never rendered into a prompt**, and both arms get the identical prompt.

## Stop conditions

- **Success:** scenario plus rubric stamped, `scenario.py --prompt` leak check clean, `audit_scenarios.py`
  clean, every gold dependency hand-audited, gold verified at 0.3 and 0.7, adversary probe failed to
  assemble the answer, the scenario-integrity gate signed.
- **Budget:** $0 in dollars; capped by session turns. Park with the draft on disk - a half-authored
  scenario resumes, it never restarts.
- **Failure:** the adversary probe assembled the answer and no undisclaimed axis remains after the
  contract hunt was deepened and the pool widened. Hand the repo to
  [Diagnosis](03-repo-diagnosis.md) branch 2 (re-shape) or branch 6 (seam nonexistent) - with the probe
  transcript attached. This loop never writes a tie or a boundary framing; the try-harder law
  ([`campaign-laws.md`](campaign-laws.md)) applies at proposal time, not after the fact.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Scenario integrity (batchable across the slate) | scenario + gold drafted and pressure-tested, before any spend | yes | **never** - ground-truth anchor |

## State / memory

- `verticals/<vertical>/scenarios/<repo>.yaml` + `<repo>.rubric.yaml`.
- Adversary probe transcript in `verticals/<vertical>/results/dryrun/<repo>/`.
- **`scenario_version` is a sha256 of the WHOLE scenario file, comments included.** Editing a comment
  drifts the hash and orphans every run pinned to it - grep the drivers for the current hash before
  touching a benched scenario.
- Ledger: `loop1/<repo>/scenario` (stamped, with the version hash) and `loop1/<repo>/event-b` (gold
  sign-off).

## Un-fakeable check

- `scenario.py --prompt` proves no gold leaked into the prompt, and the per-dependency hand audit proves
  each credit points at the file it claims. Both are required: the leak check cannot see a mis-credited
  dependency, and the audit cannot see a leak.

## Inputs / outputs

- **Consumes:** one repo off bootstrap's slate (pinned repo, index, contract) or the same repo back
  from Diagnosis with a named lever and its credit table; the scenario-crafting law ([`../scenarios/crafting.md`](../scenarios/crafting.md),
  [`../scenarios/sourcing-runbook.md`](../scenarios/sourcing-runbook.md)); the empirical laws in
  [`campaign-laws.md`](campaign-laws.md).
- **Produces:** the stamped scenario + rubric, the adversary probe transcript, the audited gold, the
  the scenario-integrity gate record.

## Fixture test (standalone, $0)

- **Positive:** the chatwoot shape - re-author a fan-shaped ask into a chain and the discriminator moves
  from sub-floor to +0.60. Hand the stage the pre-shape scenario; it must propose the chain.
- **Kill fixture:** the 2026-07-13 dry-run that killed five framework shapes only *after* two days of
  battery reasoning had cleared them. The adversary probe must kill them in one $0 pass - that incident
  is why this vertex exists.
- **Negative control:** hand it a scenario whose gold is already sound. It must sign off and stop; an
  authoring vertex that always finds something to re-shape is over-tuned.

## Built vs missing

- **Built:** `scenario.py` (with `--prompt`), `audit_scenarios.py`, `gold.py` (with the basename guard),
  `anchor_rank.py` (a LISTING for whoever writes the scenario, never a gate; its docstring records
  the three rankings that all buried the biggest banked win), the crafting and sourcing docs.
  `compose.py` no longer emits an anchor: choosing it is this stage's judgment call, made
  while the scenario is written.
- **Built 2026-07-30:** `gold_confidence_check.py` (pinned by `test_gold_confidence_check.py`) runs
  `sense blast` at both `min_confidence` values and fails any gold row that exists only at 0.3 - the
  manufactured-win shape, since agents pass the documented 0.7. Rows reachable by neither threshold are
  reported, never failed: graph/search/hand-sourced gold is legitimately not in a blast set, and failing
  it would narrow the bench to one tool. It was prioritised over the blast trim audit because a fake win
  propagates into Loops 4/5/6 and a published article before anyone re-reads it.
- **First live use:** the first php-laravel repo off the slate.
