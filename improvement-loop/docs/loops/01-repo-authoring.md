# Loop 1 - Repo authoring

> First of the three per-repo loops (authoring → [run](02-repo-run.md) →
> [diagnosis](03-repo-diagnosis.md)). The laws all three share live in
> [`campaign-laws.md`](campaign-laws.md) and are not repeated here.

**Start by running the driver, not by reading this page.** The loop is an executable state machine
(`vertical-loop.sh`, phases `index → scout → preflight → validate → bench → report → harvest`, state
in `verticals/<vertical>/.loop-state.json`), and it names the phase you are actually in:

```
VERTICAL=<vertical> REPO=<repo> bash bench/drivers/vertical-loop.sh
```

Read this page for the judgment the driver cannot make; take the ORDER from the driver. Hand-walking
the steps from the docs is how a session ends up working whatever blocker shouts loudest instead of
the loop - the 2026-08-01 session hand-ran Loop 1 from this page, never started the driver (there was
no state file for the vertical at all), and spent itself on an instrument fix while the draft's gold
sat unaudited.

**RUN:** `index`, the gold-audit gate, `scenario.py --prompt`, `gold_confidence_check.py`, the
driver's phase order. **DECIDE:** the anchor, the axis, the ask, and every gold row - judgment is
the job here, and every claim leaving it is quoted output or a labelled assumption
([`campaign-laws.md`](campaign-laws.md), RUN vs DECIDE).

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
hand-audited per dependency, and an adversary probe that failed to shortcut it.

The scenario is CRAFTED, and that is the whole job: nothing scores this repo before the scenario
exists. Diagnosis feeds the next draft, so a repo usually reaches its verdict through several
passes through here ([`03-repo-diagnosis.md`](03-repo-diagnosis.md)).

## Product duties (per Sense surface)

- **blast:** every blast-sourced gold row must appear in the output the agent is **shown over MCP**, at
  the arm's defaults, budget truncation included. A row the arm never sees earns a delta no agent can
  reproduce. Check the CLI instead and the gate measures a surface the bench never runs - that is the
  2026-07-31 stopper, and `mcp_only_check.py` now blocks it.
- **graph: how to draft a scenario that wins.** The bar is CITED recall - `path:line`.
  A baseline greps a filename cheaply and cannot afford to open sixteen files to pin
  sixteen lines. That gap is the whole win (banked mastodon: baseline found 0.61, cited
  0.26). So:
  1. **Pick the repo's central contract** - the model or type everything hangs off
     (`Status`, `Inbox`, `Upload`, `MergeRequest`, `Order`). Not a clever corner.
  2. **Ask for a teardown audit**: "what depends on this before I rework it". Real,
     recurring, and it forces enumeration rather than a single lookup.
  3. **Take the dependents from `sense_blast` and keep the boring ones a hand audit
     never thinks of** - the backup exporter, the annual-report presenter, the permalink
     redirector. One row per FILE.
  4. **Split the gold**: `contract` + `write-path` are the obvious pieces both arms get
     (they are anchors, NOT the discriminator); `dependents` is the scattered residue
     that decides the cell.
  5. **Demand `file:line` in every step prompt.** Without it the task grades at mention
     level and both arms tie.
  Do NOT screen anchors by whether grep finds the files - that reads a discovery bar on
  a citation metric, and it is how a session talked itself out of a banked +0.54 repo.- **search / status:** record at authoring time whether the shape *needs* them. If the winning shape
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
| Mechanical verifier | `scenario.py --prompt` leak check, `gold_confidence_check.py` (shown over MCP), `gold_audit.py` per-dependency hand audit of gold credits | the basename false-credit trap is real: a script tally alone has passed wrong gold before |
| Human | none - the loop authors and checks its own scenario | ground truth is held by the mechanical verifiers above, not by a reviewer |

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

- **Success:** scenario plus rubric stamped, `scenario.py --prompt` leak check clean, every gold
  dependency hand-audited (`gold_audit.py verify` green), every blast-sourced row verified shown
  over MCP, adversary probe failed to assemble the answer.
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
| none | - | - | - |

The scenario-integrity sign-off was REMOVED 2026-07-31. What stood in its place is mechanical and
must all pass before the scenario leaves this loop: the adversary probe failing to assemble the
answer, `scenario.py --prompt` proving no leak, `gold_confidence_check.py` over MCP, and the
per-dependency hand audit (`gold_audit.py`). **The audit is the load-bearing one** - a
script tally alone has passed wrong gold before - so it is done here, in full, with nobody
downstream to catch it.

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
  the audit trail for each of them.

## Fixture test (standalone, $0)

- **Positive:** the chatwoot shape - re-author a fan-shaped ask into a chain and the discriminator moves
  from sub-floor to +0.60. Hand the stage the pre-shape scenario; it must propose the chain.
- **Kill fixture:** the 2026-07-13 dry-run that killed five framework shapes only *after* two days of
  battery reasoning had cleared them. The adversary probe must kill them in one $0 pass - that incident
  is why this vertex exists.
- **Negative control:** hand it a scenario whose gold is already sound. It must sign off and stop; an
  authoring vertex that always finds something to re-shape is over-tuned.

## Built vs missing

- **Built:** `scenario.py` (with `--prompt`), `gold.py` (with the basename guard),
  `anchor_rank.py` (a LISTING for whoever writes the scenario, never a gate; its docstring records
  the three rankings that all buried the biggest banked win), the crafting and sourcing docs.
  `compose.py` no longer emits an anchor: choosing it is this stage's judgment call, made
  while the scenario is written.
- **Built 2026-07-30, re-aimed 2026-08-01:** `gold_confidence_check.py` (pinned by
  `test_gold_confidence_check.py`) calls `sense_blast` over MCP at the arm's defaults and fails any gold
  row missing from the output the agent is SHOWN. It shipped measuring the CLI at `min_confidence` 0.7
  on the premise that agents pass the documented default; the arm runs MCP at 0.3 and is told not to
  raise it, so the gate was failing true gold (STOPPER, 2026-07-31). Scope it with `--group` to the
  blast-sourced group: handed the whole gold it would fail every graph/search/hand-sourced row and
  narrow the bench to one tool.
- **NOT a Loop 1 tool:** `audit_scenarios.py` reads "both sense and baseline scored/transcript/
  judged for one scenario" - it is a POST-run tool and cannot pass before a run exists. It was
  listed here as a pre-run verifier and a stop condition until 2026-08-01, i.e. a gate that
  could only ever be skipped or faked. It belongs to [Diagnosis](03-repo-diagnosis.md).
- **Built 2026-08-01:** `gold_audit.py` (pinned by `test_gold_audit.py`) makes the per-dependency
  hand audit a recorded artefact: `stamp` writes one row per gold item marked TODO, `verify`
  fails while any TODO remains or when the gold has changed under a finished sheet. `do_scout`
  blocks on it, so a draft found on disk is no longer treated as a draft that was checked.
- **DELETED 2026-08-01:** `assembly_cost.py`, a scout that scored anchors on grep
  reachability before a scenario existed. It violated NO PREDICTORS and produced the
  false kill that law predicts, twice, within an hour of being written. The judgment it
  tried to automate is the drafting guide above; the loop keeps the guide and drops the
  script.
- **Built 2026-08-01:** `mcp_only_check.py` (pinned by `test_mcp_only_check.py`) fails any script in
  `bench/` that shells a CLI query subcommand, so the surface defect above cannot recur silently.
- **First live use:** `rails`, the first ruby-rails repo off the slate.
