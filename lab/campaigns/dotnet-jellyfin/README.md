# dotnet-jellyfin

The first campaign authored on sense-lab against a repository nobody had benched.

Its purpose was to test the instrument, not the repository: take a repo with no scenario and try
to drive it to a measured result without a human hand-running a phase and without a gate being
bypassed. **Any verdict counts** — a win, a park, or an honest refusal to pay — because the loop
cannot record a loss.

**That condition was not met, and the failure is the finding.** Reaching the handoff required a
transition the declared phase graph does not offer, and the sweep that justified stopping is
hand-run analysis standing in place of the `author` phase. Recorded as instrument defect §5 and
as a failed Done-means bullet, not filed quietly.

## Result

**Parked at cycle 1** with the retention axis **open and one question measured**, and a product
change handed up.

| | |
|---|---|
| repository | jellyfin @ `ae8723026d97b6d0f926638803edef338919b794` |
| anchor | `IPathManager` |
| phases | index → author → minibench |
| verdicts | `AUTO` → `DRAFT` → `REQUESTION` |
| mini-bench, n=1 per arm, `claude-opus-5` (headline, **single-model**) | baseline **0.9286** (13 of 14) · sense **1.0000** (14 of 14) · delta **+0.0714** |
| cells | 1, unscored, both arms, channel proof SOUND |
| spend ceiling | 40 per campaign, **untested** at 2 runs |
| published | no article, no numbers outside this repository; the scenario and its gold are deliberately withheld from git — see the handoff |

The baseline held above the 0.50 gate, so this question does not discriminate. Its route was six
tool calls and 91 seconds: grep the anchor, read the four intermediary names out of that output,
then one shell loop over the four. The anchor was dark; the intermediaries were not, and they are
what had to be dark.

**The axis is not closed.** A re-question changes the rows and therefore the baseline, and only
one set of rows has been measured. The campaign parked for a different reason, below.

## Why one arm only

Single-model by design. The headline arm is `claude-opus-5`; the confirmation arms arrive with
cycle 08's adapters, which was the deliberate trade for running this campaign a week earlier.

## What it found

A **product defect** that turned out to be the campaign's own binding constraint: Sense returns
an empty, `"complete"` answer for every C# type declared in a block-scoped `namespace X { }`. Of
the 40 interfaces this repository declares, 17 resolve and 23 return nothing at any confidence —
every one of the 23 block-scoped, with a single block-scoped exception. The invisible side is the
most-held: `ILibraryManager` (121 files declare a field of it), `IServerConfigurationManager`
(71), `IFileSystem` (66), `IUserManager` (65). Those are exactly the anchors whose rings a grep
alternation could not have covered.

**That is why it parked.** Not because the axis is dead, but because re-questioning five more
times against the sixteen small interfaces Sense can see — when the fix puts the other 23 on the
table — spends against a pool that is about to change. An operator judgement, recorded as one.

Plus **five instrument defects**, routed to cycle 05. The sharpest is that a measured-dead axis
has no route to a park: `REQUESTION` routes back to authoring, `NO-ANCHOR` is defined by row
count rather than by whether the ring is coverable, and `handoff` is reachable only from the
six-cycle ceiling.

Two of the five were written back into `lab/plans/author.md` during the campaign: a first-cycle
branch for the RUN step that has no input on a repository with no history, and the removal of
three Done-when bullets requiring `stamp`, `verify` and a rubric check — commands that do not
exist, so the phase could never report itself done by its own criteria.

## The record

`record/` carries the campaign as it was measured: the index summary, the mini-bench with its
quoted figures and the baseline's route, the handoff, and the defect list with the killer checks
that were run before each finding was stated — and the ones that were not, named as not run.

It is a copy. The live tree is under `runs/campaigns/dotnet-jellyfin/`, which is gitignored
because a cell of a real repository is hundreds of megabytes — see defect §6. The scenario, its
gold and its rubric stay there deliberately and are not committed.
