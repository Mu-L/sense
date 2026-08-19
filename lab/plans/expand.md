---
phase: expand
reads: minibench.md
writes: scenario.yaml
emits: [AUTO, REQUESTION]
wall: 25m
---

# expand

## Task

Expand the measured two-step question into the full seven-step work session, without moving the
discriminator the mini-bench just proved.

## Scope

You rewrite one scenario and extend its gold and rubric. **Out of scope:** changing the anchor,
changing the discriminator step's question, re-running either arm, the paid bench, editing a
script, any other repository.

If the mini-bench verdict on disk is not `PROCEED`, write one line in the notes and stop.

## Run

1. Read `minibench.md` and the draft scenario before writing anything.

2. Rewrite the scenario to the seven steps below, then extend the rubric to match.

3. Pull the shown set again for the anchors and specs you are adding.

4. Open every added row and pin the line that actually touches the contract.

5. Render the prompt, check the rubric, check gold confidence on the `dependents` group, then
   stamp and verify the gold audit.

## Decide

### The seven steps

Every banked win in this bench is this session. Only the contract noun and the axis sentence
change between repositories; the skeleton does not.

    1. Orient in the codebase        where the contract sits, how the code is organized
    2. Map the <contract> contract   what it exposes and what it pulls in: concerns, embeds,
                                     associations. file:line for each piece
    3. Trace the write path          how the thing is created, or torn down, and what runs
                                     around it. Which parts run inline and which asynchronously
    4. Audit every dependent         THE DISCRIMINATOR. Carried over VERBATIM
    5. Trace the guards              how code confirms it is acting on a real, current instance
                                     before acting. file:line throughout
    6. Assess the blast radius       which dependents are at risk, ranked, and what must be
                                     re-verified. A missed high-risk dependent is the one that
                                     pages someone
    7. The change and verification   what you will edit, every dependent grouped by area, and
       map                           the specs that must be updated

The steps accumulate: each is worked in order in one session, so by step 4 both arms have already
spent context and wall on steps 1 to 3. **That is part of the measurement, not decoration.**

### The rules that do not bend

- **Step 4 is byte-identical to the probe scenario's step 2**, and its `dependents` gold is
  byte-identical too. Move either and the mini-bench number stops describing this scenario. If you
  believe step 4 needs a rewrite, that is a `REQUESTION`, not an expansion.
- **New gold goes in NON-SCORING groups.** `contract`, `write-path` (or `teardown`) and `specs`
  are what both arms reach. Adding to `dependents` here is untested gold: it never faced a
  baseline.
- **The prompt stays neutral** — no paths, class names, counts, tool names, or answer shape — and
  **every step demands `file:line`.**
- **The ask names the MECHANISM, never the INVENTORY.** The neutrality check reads tokens, so a
  list of functional categories passes it clean and still hands over the answer. Read the finished
  prompt back and quote any phrase that identifies a single gold row's file.
- **Every step carries its checks**: a response richness floor, and tool-reach checks recorded at
  the adoption layer so reach is recorded without scoring the answer.
- **One item per FILE**, hand-audited at its real line, in every group.

## Precedent

Every banked win in this bench came from this seven-step skeleton, and the discriminator has
always been step 4, after three steps of accumulated context and wall. The one recorded attempt
to measure the discriminator cold — the same question asked as step 1 of a short session — is
among the five failures on that repository, where the plain arm scored 0.81, 0.16, 0.09, 1.00 and
1.00 and the gap never opened.

## Artifact

`scenario.yaml`, rewritten to seven steps with `name`, `repo`, `contract_symbol`,
`contract_file`, `description`, `steps`, `scoring`, `gold`. The header comment carries the seam
profile, the axis sentence, the mini-bench figures, and any superseded question with the number
that killed it.

The rubric is extended to seven matching steps, names verbatim and in order, each declaring
`map_quality`, `specificity`, `justification` and `uncertainty`.

The verdict is `AUTO` when the expansion holds, and the scenario is committed once it exists.

`REQUESTION` only if expanding revealed that step 4 cannot survive verbatim: a gold row that does
not hold at its line, or a step that cannot be written without naming a gold file. It carries the
row or the phrase that failed, and it re-enters authoring with the anchor kept.

## Done when

- The scenario has seven steps in the order above; the rubric has seven matching names in order.
- Step 4's prompt and the `dependents` gold are unchanged from the probe scenario, verified by
  diffing rather than by reading.
- Every step's prompt contains `file:line`.
- The rendered prompt shows no path, symbol, count or tool name.
- The rubric check passes.
- The gold confidence check on the `dependents` group passes.
- The gold audit verifies.
- The finished ask has been read back as an inventory check, with any offending phrase quoted and
  rewritten as a mechanism.

## Do not

- Do not add rows to `dependents`, and do not reword step 4. Both invalidate the measurement.
- Do not take a credit from a script tally; open the file and read the line, every added row.
- Do not spawn a subagent. Only the binary spawns.

## Questioned, not changed

The declared graph gave expansion `AUTO` alone. Porting this plan found the hole: the old plan
emits `REQUESTION` when step 4 cannot survive verbatim, and a phase with no verdict for a case it
really meets writes nothing and stalls the loop. `REQUESTION` was added to this phase's enum and
to the lever table, because a lever a recorded campaign used and the table does not carry is a
hole in the test set rather than a plan to bend around it.

The old plan also named its passing verdict `SCENARIO`. That is `AUTO` here, since the graph's
auto phases are the ones that make no judgement, and this one does not: it either carries the
discriminator over or it re-questions.

## Verdict

Write `verdict.json` in this phase's directory, beside the artifact:

```json
{"phase": "expand", "repo": "<repo id>", "cycle": <n>, "verdict": "<one of the emitted>",
 "table": "<why, in one or two sentences>"}
```

The crank reads that file and nothing else to route. A verdict left in prose is a verdict only a
person can act on, and the loop this phase belongs to runs unattended.
