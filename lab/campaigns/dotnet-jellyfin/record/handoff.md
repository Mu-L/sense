# handoff — dotnet-jellyfin / jellyfin

**The campaign parked at cycle 1 with the retention axis OPEN and one question measured, and
hands up a product change with the numbers attached.**

Every number here is `claude-opus-5`, the headline arm, **single-model**, n=1 per arm. Cycle 08
brings the confirmation arms.

**A phase could not be run, and the pitch's first Done-means bullet is NOT met.** The campaign
did not go from no scenario to a verdict without a human hand-running a phase: reaching this
handoff required a transition the declared graph does not offer, and the sweep that justified
stopping is hand-run analysis standing in place of the `author` phase. That is recorded as
instrument defect §5 and as a failed bullet, not filed as a defect and left reading as passed.

This is the first campaign driven end to end on sense-lab against a repository nobody had
benched. Its verdict is a park, and **a park is a result**: the instrument was what was under
test, not the repository.

## Position

| | |
|---|---|
| campaign | `dotnet-jellyfin` |
| repository | jellyfin @ `ae8723026d97b6d0f926638803edef338919b794` |
| cycles used | 1 of 6 |
| phases run | index → author → minibench |
| verdicts | `AUTO` → `DRAFT` → `REQUESTION` |
| paid runs | 2 (one cell, both arms, x1, unscored) |
| ceiling | 40 per campaign. **Untested**: 40 is `defaultCeiling` in `lab/internal/cli/statuscmd.go:14`, whose own comment says the page only reports it. At 2 runs, nothing could have intervened and nothing did. Reported as untested, not as held |
| published | nothing |

## What was measured

**index.** 2137 files, 11410 symbols, 23698 edges, embeddings complete, quoted from
`sense_status` over MCP.

**author.** Anchor `IPathManager`, chosen on ring diversity after sweeping 45 candidate
interfaces over MCP. Fourteen `dependents` rows, one file each, ten areas, every credit opened
and read by hand, every chain audited at its construction site. `sense-lab validate`: 17 rows, 0
quarantined, and none of the fourteen scoring rows inside the covering grep.

**minibench.** Both arms, real wall, unscored, channel proof SOUND.

| | baseline | sense | delta |
|---|---|---|---|
| `dependents`, n=1 per arm | **0.9286** (13 of 14) | **1.0000** (14 of 14) | **+0.0714** |

`REQUESTION`: the baseline held above 0.50, so the question does not discriminate. Its route was
six tool calls and 91 seconds — grep the anchor, read the four intermediary names out of that
output, then one shell loop over the four. The anchor was dark; the vias were not, and the vias
are what had to be dark.

## Why it parked here rather than re-questioning five more times

**Not because the axis is closed.** One question has been measured, and a re-question changes the
rows and therefore the baseline. What follows is a listing that ORDERS candidates; under NO GREP
SCREEN IS A GATE and THE FIRST MEASUREMENT IS A REAL BASELINE ARM it may not kill a draft, and it
does not kill this one.

- The ring is **one hop deep** — `via_satisfiers` is 1 on all fourteen rows and every `chain` is
  a single carrier — so there is no depth question available to *either* arm.
- Across **all 64 interfaces declared in a file-scoped namespace** in this repository, the best
  via diversity is five (`IItemTypeLookup`, 18 files) and the next is four (`IPathManager`, 14
  files). bitwarden-server's banked mini-bench, baseline 0.125, had **nine vias over 31 ring
  files**. A four-term alternation is one shell loop; a nine-term one over twelve areas is not.

**And that sweep is scoped to the sixteen interfaces Sense can currently see**, which is defect
§4 rather than a scoping choice — so it cannot speak for the 23 it cannot see. The second,
different probe that would settle those was not run.

The comparison to bitwarden's nine vias is suggestive and is **not** a mechanism: that cell's own
recorded diagnosis attributes its 0.125 to a baseline that *never took the second hop*
(`lab/scenarios/bitwarden-server/bitwarden-server.yaml:20-22`), not to an alternation it could
not afford. Ours took the second hop in one loop. Which of the two separates them is unmeasured.

**So the reason for parking is not that the axis is dead.** It is that the campaign's binding
constraint is a product defect whose fix changes the pool a re-question would run against: 23 of
the 40 interfaces this repository declares are invisible to the instrument, and they are the
large, diverse ones. Buying five more cells against the small sixteen, when the fix puts the
other 23 on the table, spends against a pool that is about to change. That is an operator
judgement and it is recorded as one.

**AXIS-DEAD IS NEVER REPO-DEAD**, and nothing here rules on jellyfin.

## The swap handed up

**Sense returns an empty, `"complete"` answer for every C# type declared in a block-scoped
`namespace X { }`.** File-scoped `namespace X;` resolves 16 of 16; block-scoped resolves 1 of 24.
Reproduced in twelve lines of C# where the only difference is the namespace syntax. Not the
confidence gate — still empty at `min_confidence: 0.0`. Located at
`internal/extract/csharp/csharp.go:71`, where `isClass` lists `namespace_declaration` and never
`file_scoped_namespace_declaration`, a node kind that appears nowhere under `internal/`.

**Why it is the swap and not a side note.** The 24 interfaces Sense cannot see are precisely the
large, diverse ones whose rings a grep alternation could not cover — `ILibraryManager` (121 files
declare a field of it), `IServerConfigurationManager` (71), `IFileSystem` (66), `IUserManager`
(65). The sixteen it can see are the small ones. **The campaign's binding constraint and the
campaign's finding are the same fact**, and the honest reading of this park is that the axis is
closed *at the product's current behaviour* and worth re-opening after the fix.

Full detail, with the killers that were run before the finding was stated, in
`../instrument-defects.md` §4. Routed to 07-02.

## Instrument defects, routed to cycle 05

Five, in `../instrument-defects.md`. The one that matters most is §5: **a measured-dead axis has
no route to a park**, because `REQUESTION` routes to authoring, `NO-ANCHOR` is defined by row
count rather than by whether the ring is coverable, and `handoff` is reachable only from the
six-cycle ceiling or from a report. This handoff was therefore written by operator decision on
the measurement, not by a declared transition. No gate was bypassed — no gate exists on this
edge, and that is the defect.

## Publishing: the scenario and its gold are deliberately NOT committed

This repository is public, so committing gold publishes it, and PUBLISHED GOLD LEAKS, SO A
HELD-OUT SET IS PERMANENT makes that irreversible.

**What is committed:** this handoff, the defect list, the index summary, the campaign config and
the README — the record of what was measured and what was found.

**What is not:** `scenario.draft.yaml`, `scenario.draft.gold.yaml` and
`scenario.draft.rubric.yaml`. They stay in the untracked run tree at
`runs/campaigns/dotnet-jellyfin/jellyfin/1/author/`.

That is a deliberate hold, not an oversight, and it is deliberately the *reversible* choice: gold
can be committed later, it cannot be un-published. The handoff above says this axis may be worth
re-opening after the C# fix, and re-opening a scenario whose gold is public is exactly what the
law prevents.

**It also flags an inconsistency for a human.** `lab/scenarios/*/` gold is already tracked on
`main` for five scenarios, so either "published" is a term of art here meaning an article and
those are fine, or the same law applies to them and it has already been broken five times. This
campaign took the conservative reading and did not resolve the question. Cycle 02's `published` /
`held_out` fields do not exist in `lab/internal/`, so the declared leakage check has nothing to
read either way.

No numbers and no article are published anywhere outside this repository.
