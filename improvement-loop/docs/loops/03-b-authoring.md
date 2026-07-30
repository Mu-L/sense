# Loop 3b - Authoring

> Stage 2 of 4. Shared laws, the depth-first rule and the ledger namespace live in the parent
> ([`03-per-repo-convergence.md`](03-per-repo-convergence.md)) and are not repeated here.

## Goal

Author one scenario and its gold for the top-ranked bound-legal repo, such that a code-capable baseline
with Sense forbidden cannot assemble the answer. Exit state: a stamped scenario plus rubric, gold
hand-audited per dependency, an adversary probe that failed to shortcut it, and the scenario-integrity gate signed.

## Product duties (per Sense surface)

- **blast:** the gold must survive **both** `min_confidence` 0.3 and 0.7. A gold set that only holds at
  0.3 turns a documented default into a scoring artifact, and the contract defect around that param is a
  known live one - gold curated blind to it manufactures a win.
- **graph:** the target hop is chosen for structural surplus, not for grep-hostility alone. If a plain
  two-hop grep reaches the answer, the scenario is not measuring Sense, it is measuring patience.
- **search / status:** record at authoring time whether the shape *needs* them. If the winning shape
  never asks for either, that is a feature-coverage blind spot to hand Loop 5, not a verdict.
- **conventions:** nothing here. The slate sweep belongs to Loop 2.

## Identity

- **Character:** judgment. This is where a win can be manufactured by accident, so the adversary vertex
  and the human ground-truth anchor both sit inside this stage.
- **Unit of work:** one scenario plus its gold for one repo.
- **Position:** consumes the ranked bound-legal queue from [Eligibility](03-a-eligibility.md); produces
  the frozen scenario artifacts [Run](03-c-run.md) spends against.

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

**Only probe-disclaimed shapes proceed to gold.** This is the design-time kill; the sense-arm dry-run in
[Run](03-c-run.md) is the separate spend-time go/no-go. One does not substitute for the other: a shape
can survive the probe and still fail to reach at the cell's wall.

## Gold curation rules

The starting point is the mini-gold [Eligibility](03-a-eligibility.md) hand-verified
(`scenarios/<repo>.draft.yaml`). This stage widens it to the full set, audits it per dependency, and
writes `scenarios/<repo>.yaml` - at which point the audited set supersedes the draft everywhere, including
in the bound. **Re-targeting the draft is expected, not a failure:** it was built to answer "can this cell
clear the bar", not "what exactly does this cell measure".

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
  [Diagnosis](03-d-diagnosis.md) branch 2 (re-shape) or branch 6 (seam nonexistent) - with the probe
  transcript attached. This stage never writes a tie or a boundary framing; the parent's try-harder law
  applies at proposal time, not after the fact.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Scenario integrity (batchable across the slate) | scenario + gold drafted and pressure-tested, before any spend | yes | **never** - ground-truth anchor |

## State / memory

- `verticals/<vertical>/scenarios/<repo>.yaml` + `<repo>.rubric.yaml`.
- Adversary probe transcript alongside the control probes in
  `verticals/<vertical>/results/dryrun/<repo>/`.
- **`scenario_version` is a sha256 of the WHOLE scenario file, comments included.** Editing a comment
  drifts the hash and orphans every run pinned to it - grep the drivers for the current hash before
  touching a benched scenario.
- Ledger: `loop3/<repo>/scenario` (stamped, with the version hash) and `loop3/<repo>/event-b` (gold
  sign-off).

## Un-fakeable check

- `scenario.py --prompt` proves no gold leaked into the prompt, and the per-dependency hand audit proves
  each credit points at the file it claims. Both are required: the leak check cannot see a mis-credited
  dependency, and the audit cannot see a leak.

## Inputs / outputs

- **Consumes:** the ranked bound-legal queue and per-group control means from Eligibility; the
  scenario-crafting law ([`../scenarios/crafting.md`](../scenarios/crafting.md),
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
  `compose_slate.py`, `anchor_rank.py`, the crafting and sourcing docs.
- **Built 2026-07-30:** `gold_confidence_check.py` (pinned by `test_gold_confidence_check.py`) runs
  `sense blast` at both `min_confidence` values and fails any gold row that exists only at 0.3 - the
  manufactured-win shape, since agents pass the documented 0.7. Rows reachable by neither threshold are
  reported, never failed: graph/search/hand-sourced gold is legitimately not in a blast set, and failing
  it would narrow the bench to one tool. It was prioritised over the blast trim audit because a fake win
  propagates into Loops 4/5/6 and a published article before anyone re-reads it.
- **First live use:** the top-ranked php-laravel repo, after its eligibility probe passes.
