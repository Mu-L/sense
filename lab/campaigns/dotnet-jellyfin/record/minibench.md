# minibench — dotnet-jellyfin / jellyfin / cycle 1

**Verdict: REQUESTION.** The baseline holds above 0.50.

Every number on this page is `claude-opus-5`, the headline arm, **single-model**. n=1 per arm.

## The numbers, quoted from the credit table

Both arms x1, real wall, unscored. `sense-lab score -group dependents`, checkout pinned at
`ae8723026d97b6d0f926638803edef338919b794`.

```
════════ sense
gold group dependents
cited      14 of 14
recall     1.00
grounding   17 of 38 cited locations do not resolve at the pinned commit

════════ baseline
gold group dependents
cited      13 of 14
recall     0.93
grounding   18 of 42 cited locations do not resolve at the pinned commit
missed     d:dto-service
```

|  | baseline | sense | delta |
|---|---|---|---|
| `dependents` | **0.9286** (13 of 14) | **1.0000** (14 of 14) | **+0.0714** |

- Baseline gate, at or below 0.50: **FAILS** at 0.9286.
- Delta gate, at least +0.50: **FAILS** at +0.0714.
- **n = 1 per arm.** A delta carries the run count that produced it: this one is n=1.
- Side of the window: **BASELINE-TOO-HIGH by 0.43**.

Arithmetic ceiling: recall caps at 1.00, so a baseline at 0.9286 caps the delta at 0.0714. The
sense arm was already at the cap. Nothing about the sense arm is the constraint here.

## Arm health

| | outcome | exit | wall | took | watchdog |
|---|---|---|---|---|---|
| sense | completed | 0 | 480s | 124.7s | none |
| baseline | completed | 0 | 150s | 91.5s | none |

Both walls uncensored. The baseline's 150s wall is `sense wall x 1.2` off the sense arm's 124.7s,
as the rule requires. Neither arm ran out of clock, so CANNOT-FINISH-AT-BUDGET does not apply.

Channel proof, from `sense-lab probe`:

```
  sense arm routes       all present
  baseline arm routes    none reachable
  persisted memory       unreadable by both arms
  Sense in the baseline  no sign of it
  arms differ in         nothing but Sense access
  MCP frames captured    15
SOUND: the two arms differed in Sense access and nothing else.
```

The sense arm did reach: it called `sense_blast`, `sense_graph`, `sense_search`,
`sense_conventions` and `sense_status`, and reached synthesis inside its wall. So this is a
measurement of the baseline, not of an instrument that failed to answer.

## The rows the baseline took, and the route it used

It took 13 of the 14 in **six tool calls and 91 seconds** of a 150 second wall. Missed only
`d:dto-service`. The route, quoted:

```
  Bash  grep -rn "IPathManager" --include=*.cs . | head -60
  Bash  find . -name "PathManager.cs" -o -name "IPathManager.cs" | head
  Bash  grep -rn --include="*.cs" "IPathManager" .
  Bash  cat -n MediaBrowser.Controller/IO/IPathManager.cs && cat -n Emby.Server.Implementations/Library/PathManager.cs
  Bash  for i in ITrickplayManager IChapterManager IExternalDataManager ISubtitleEncoder IAttachmentExtractor; do echo "=== $i"; grep -rn --include="*.cs" "$i" ...
  Bash  grep -rn --include="*.cs" -E "EncodingHelper ..."
```

**The mechanism that killed it.** The second ring was supposed to be unreachable by name,
and it is — none of the 14 gold files contains the string `IPathManager`. But the four
intermediaries are not unreachable by name: the classes that declare `IPathManager` appear in
the anchor's own grep output, so call three hands the agent the via names for free. Once it has
the four names, **one alternation prints the entire ring**. The anchor's darkness bought
nothing, because what had to be dark was the vias, not the anchor.

This is the same family as the `new [A-Za-z]*Tokenable` kill recorded on bitwarden-server — one
regex covering the whole answer set — reached by a different route: there the covering string
was a shared lexical suffix, here it is a four-term list the anchor's own grep hands over.

## What the sweep says, and what it may not say

**It is a LISTING. It orders candidates; it does not close an axis.** NO GREP SCREEN IS A GATE
and PRECISION RANKS, IT NEVER KILLS both bind here, and so does THE FIRST MEASUREMENT IS A REAL
BASELINE ARM: *"a hand estimate of a baseline is not evidence about a baseline"*. One question
has been measured on this repository. One.

What was measured, over MCP at defaults:

**1. The ring is one hop deep.** Every row's `chain` is a single carrier
(`ChapterManager > IPathManager`), and `via_satisfiers` is 1 for all 14. There is no third ring
in the shown payload, so a depth question is unavailable to the *sense* arm as well as to the
baseline — GOLD RAILS requires every blast-sourced row to appear in what the agent is shown.

**2. Ring diversity across the interfaces Sense can currently see.** All 64 interfaces declared
in a file-scoped namespace were swept. Only three carry a ring of eight or more files:

| anchor | vias | ring files | max via share | areas |
|---|---|---|---|---|
| IItemTypeLookup | 5 | 18 | 0.67 | 9 |
| IPathManager | 4 | 14 | 0.43 | 10 |
| IAttachmentExtractor | 1 | 9 | 1.00 | 3 |

**That sweep is scoped to the file-scoped sixteen, and the scoping is not a choice — it is
defect §4.** Block-scoped interfaces return nothing at all, so this listing is taken with the
instrument this same record declares blind on 23 of the 40 interfaces the repository declares.
It therefore cannot support a claim about *every anchor this repository offers*; it supports a
claim about the subset Sense can currently see. The second, different probe that would settle
the rest — sweeping the block-scoped interfaces by hand, or re-sweeping after the fix — **was
not run**.

**3. A comparison that is suggestive and is not a mechanism.** bitwarden-server's banked
mini-bench held its baseline to 0.125 with nine vias over 31 ring files
(`lab/scenarios/bitwarden-server/bitwarden-server.yaml:146`). It is tempting to read via count
as the cause. **The recorded diagnosis of that cell says something else** — same file, lines
20-22: *"The baseline's route was a hand-rolled one-hop interface walk (9 of its 15 calls); it
took the two rows one hop from a direct injector ... and never took a second hop."* The
mechanism on the record is a hop that baseline never took, not an alternation it could not
afford. Our baseline took the second hop in one shell loop. Whether via count or hop-taking is
what separates them is **unmeasured**, and this cycle does not settle it.

## Where this leaves the axis

**Open, with one question measured.** Not closed. A re-question changes the rows and therefore
changes the baseline, and nothing here has measured a second set of rows.

What the cycle does establish is narrower and still worth having: on this anchor, with this
question, the baseline assembled the set in six calls because the anchor's own grep hands over
the four intermediary names. Any re-question on this anchor has to defeat that specific route,
and the ring's one-hop depth removes the obvious way to.

## Why the campaign parked anyway

Not because the axis is closed. Because **its binding constraint is a product defect, and the
fix changes the candidate pool it would be re-questioning against.**

23 of the 40 interfaces this repository declares return an empty blast at any confidence
(`../instrument-defects.md` §4), and they are the large, diverse ones — `ILibraryManager` (121
files declare a field of it), `IServerConfigurationManager` (71), `IFileSystem` (66),
`IUserManager` (65). The sixteen Sense can see are the small ones. Re-questioning five more
times against the small sixteen, when the fix would put the other 23 on the table, is buying
cells against a pool that is about to change.

That is a judgement, it is the operator's, and it is recorded as one. **AXIS-DEAD IS NEVER
REPO-DEAD**, and nothing here rules on jellyfin.

## The grounding rate, decomposed before it is quoted

The scorer reported `17 of 38` sense and `18 of 42` baseline cited locations not resolving at
the pinned commit. AN UNGROUNDED RATE IS A FORM ARTIFACT UNTIL DECOMPOSED forbids quoting that
as a fabrication rate, and it is **not decomposed here** — this cycle did not run the
decomposition, so the raw figures are recorded as raw figures and nothing is concluded from
them.

Two things can be said. The rates are close between the arms (0.447 sense, 0.429 baseline), so
there is no arm asymmetry of the kind that produced the recorded discourse scare. And the
question the decomposition would answer — **whether any of the sense arm's 14 credits rests on
a citation that does not resolve** — is open, so the 1.0000 in the table above carries that
caveat until someone runs it.
