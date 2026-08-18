# Instrument defects found by the first live campaign

Campaign `dotnet-jellyfin`, repository `jellyfin`, cycle 1. **Six entries: five instrument
defects routed to cycle 05, and one PRODUCT defect (§4) routed to 07-02.** None is a defect in
the campaign.
Recorded as they were found, with the output that shows them.

## 1. `lab/plans/author.md` RUN step 1 cannot run on a fresh repository — cycle 05

The step reads:

> **What the arms have ACTUALLY cited, per gold row, across every run on disk.** Rarest-cited
> first, scoped to the HEADLINE model's results root

On a repository with no scenario there are no runs on disk, so there is no citation ranking,
no free-row class and no unreachable class. The plan's first RUN step is written for a
re-entry and the plan says nothing about the first cycle on a new repository — which is the
only case 07-01 exercises.

The step is not merely skippable: three of the eight DECIDE rules (7, "drop what the ranking
says is free or unreachable", and the re-question rules) read from its output. This campaign
had to author gold with that class unknown, and nothing in the plan says that is what a first
cycle does.

**Route:** cycle 05, `lab/plans/author.md`. It needs a first-cycle branch saying what stands
in for the ranking when there is no history, and saying plainly that the free-row class is
unmeasured rather than empty.

## 2. `author.md` step 8 names commands that do not exist — cycle 05

The step reads:

> After writing the yaml and the rubric: render the prompt, check the rubric, check gold
> confidence on the `dependents` group, then stamp and verify the gold audit. `stamp` writes
> one TODO row per gold item; you replace each by opening the file and reading the credit.
> `verify` fails while any TODO remains.

And the Done-when list requires "The gold audit verifies: zero TODO rows, gold unchanged under
a finished sheet."

There is no such command:

```
$ sense-lab
Commands:
  catalog  plan  run  probe  score  validate  rescore  status  gate  version
$ grep -rln 'func stampCmd\|"stamp"\|"verify"' lab/internal/cli/
$ echo $?
1                       # no output, no match
```

`sense-lab validate` covers part of it — it audits gold rows against the checkout and reports
quarantines and covering-grep flags — but there is no rubric check, no gold-confidence check,
no prompt render, and no stamp sheet. Four of the nine Done-when bullets in `author.md` are
therefore unverifiable as written.

This one is half-known: the plan's own "Questioned, not changed" section says the RUN steps
name computations rather than commands and binds the fix to **cycle 07**. What that note does
not say is that the *Done-when* list still requires the artifacts of the missing commands, so
the phase cannot report itself done by its own criteria. That is the part this campaign found.

**Route:** cycle 05, `lab/plans/author.md`. Either the commands are built, or the Done-when
list stops requiring their artifacts. It may not stay as it is.

## 3. No answer-forms page exists for a new campaign, and nothing creates one — cycle 05

`author.md` DECIDE says:

> **Read the campaign's answer-forms page FIRST if it exists.**

For `dotnet-jellyfin` it does not, because the campaign is new. The plan handles the absence
("a form absent from it is not forbidden; it is unmeasured, and saying so in the header is the
whole obligation") but no phase in the graph ever *writes* that page, so a campaign's second
cycle reads the same empty absence as its first and the measurement is lost.

**Route:** cycle 05. The obvious owner is `report.md` or `harvest.md` — whichever phase holds
a measured form should append it.

## 4. `sense_blast` and `sense_graph` return an empty, "complete" answer for C# interfaces declared in a block-scoped namespace — routed to 07-02

This is a **product** defect, not an instrument one, and it is the finding 07-02 takes.
Recorded here because it was found by this campaign and it constrained this campaign's anchor.

Measured over MCP against the pinned jellyfin clone. 45 candidates were swept (every interface
held as an injected field in four or more files). **Five of the 45 are not declared in this
repository at all** — `ILogger`, `ILoggerFactory`, `IHttpClientFactory`, `IHttpContextAccessor`,
`IMemoryCache`, which are .NET framework interfaces — and they return `total_affected: null`,
which is *absent from the index*, a different thing from an empty answer. They are excluded, and
the arithmetic below is over the **40 interfaces this repository declares**, which is every one
of the 45 that has a declaration site:

| namespace style | resolve (total_affected > 0) | empty |
|---|---|---|
| file-scoped `namespace X;` | **16 of 16** | 0 |
| block-scoped `namespace X { }` | **1 of 24** | **23** |
| **total declared here** | **17 of 40** | **23** |

**Negative space, checked rather than assumed.** There are zero file-scoped failures, and
exactly one block-scoped success — `IDirectoryService`, at `total_affected` 2. The dichotomy is
one exception away from clean, and that exception is named rather than rounded off.

The empty side includes this repository's most-held interfaces: `ILibraryManager` (121 files
declare a field of it), `IServerConfigurationManager` (71), `IFileSystem` (66), `IUserManager`
(65). For `ILibraryManager`, `sense_graph` returns every edge list empty — including
`inherited_by`, against a literal `Emby.Server.Implementations/Library/LibraryManager.cs:65:
public class LibraryManager : ILibraryManager` — and reports:

```
"completeness":{"verdict":"complete","resolved":0,
 "advice":"Complete resolvable edge set — act on it, do not re-grep."}
```

An agent told the empty set is complete, and told not to re-grep, will act on nothing.

The assembly confound is dead: both groups span `MediaBrowser.Controller`,
`MediaBrowser.Model` and `MediaBrowser.Common`, and the empty side is the *more* used side, so
it is not a story about unused symbols.

**Not the confidence gate.** The killer was run before the finding was stated. At
`min_confidence: 0.0` both tools are still empty:

```
graph ILibraryManager @ min_confidence 0.0 -> edges {} , low_confidence_hidden None
blast ILibraryManager @ min_confidence 0.0 -> total_affected 0, direct 0, ring None
```

So the edges are absent from the index rather than hidden by the default floor.

**Reproduced in twelve lines.** Two directories, the same code in each, differing only in
namespace syntax, scanned and queried over MCP:

```
=== IBlockThing ===   namespace Demo.Block { ... }
 qualified: Demo.Block.IBlockThing
 edges: ALL EMPTY
=== IFileThing ===    namespace Demo.File;
 qualified: IFileThing
 edges: {'composed_by': 1}
```

**Where it lives.** `internal/extract/csharp/csharp.go:71` lists `namespace_declaration` in
`isClass` — the block-scoped form — and never `file_scoped_namespace_declaration`, which
appears nowhere under `internal/`. So the block-scoped form opens a naming scope and its
symbols are namespace-qualified, while the file-scoped form is walked straight through and its
symbols stay bare. References are written bare in both cases. Which of the two is the defect is
07-02's design call; that both exist and disagree is the finding.

**Effect on this campaign:** the anchor had to come from the sixteen. On a block-scoped anchor
the retention question could not have been asked at all.

## 5. A measured-dead axis has no route to a park, and the verdict that names it does not exist — cycle 05

This is the defect this campaign exists to find, and it is the most consequential one here.

Cycle 1 returned `REQUESTION`, correctly: the baseline held at 0.9286, above the 0.50 gate. The
plan's instruction for that verdict is *"The lever is the QUESTION, not the anchor"*.

But the sweep that followed measured that **no question on this anchor can move cited recall**:
the ring is one hop deep (`via_satisfiers` 1 on all 14 rows, every `chain` a single carrier), and
across all 64 file-scoped interfaces in the repository the best via diversity is five, so every
candidate ring is covered by a short alternation the anchor's own grep hands over. The answer set
*is* the ring, and the ring *is* what the alternation prints.

The graph has no transition for that state:

- `REQUESTION` routes to `author`, which is the lever that has just been measured empty.
- `NO-ANCHOR` is defined narrowly — *"only when the shown blast set itself cannot carry twelve
  dependent files"* — and this anchor carries fourteen. So the verdict that would allow a
  re-anchor is unavailable by its own criterion, even though the anchor demonstrably cannot carry
  any question of this form.
- `handoff` is reachable only from the authoring ceiling (six cycles) or from `report` with a
  `DIAGNOSIS`, and neither is reachable from here without buying five more mini-benches on an
  axis the arithmetic has already closed.

So the loop's only declared move is to spend five more cells confirming a result the sweep
already proves. **A lever the verdict called for that has no route in the graph** is one of the
five failure conditions 07-01 enumerates, and this is one.

**What is missing is a verdict, not a transition.** `NO-ANCHOR` counts rows and areas; nothing in
the enum expresses *"the ring is large enough and is covered by one cheap alternation"*, which is
the property that actually decides whether a retention question can discriminate. The
mini-bench's own kill record on bitwarden-server (`new [A-Za-z]*Tokenable`, one regex printing
the whole answer set) is the same property, and it had no verdict there either.

**Route:** cycle 05, `lab/internal/phase/phase.go` and `lab/plans/minibench.md`. The candidate
shape is a verdict that says the axis is closed on this anchor with the numbers attached, routing
to `handoff` rather than to `author`.

**What this campaign did, stated plainly rather than hidden.** It parked at cycle 1 and wrote a
handoff, which is **not** a transition the declared graph offers from `minibench`. That was an
operator decision taken on the measurement.

**And it is the pitch's FIRST failure condition, not its third.** "A gate that had to be
bypassed" would be the lesser reading, and saying "no gate was bypassed because no gate exists
on this edge" is self-serving: the operator did not slip past a gate, the operator manufactured
a transition the graph does not offer. That is **"a phase that could not be run"**, and the
Done-means bullet *"one repository goes from no scenario to a verdict without a human
hand-running a phase"* is therefore **NOT MET**. It is marked unmet in the pitch rather than
left reading as satisfied with a defect filed beside it.

The same reading applies to the sweep that justified stopping: it is hand-run analysis standing
in for the `author` phase the graph declares, and the pitch's rabbit hole warns against
hand-running a phase to keep things moving. This hand-ran one to *stop* things, which is the
mirror image and no better.

## 6. The run tree that holds every phase artifact is gitignored, and no plan says to keep a copy — cycle 05

`/runs/` is ignored wholesale, for a good and documented reason: one cell of a real repository is
hundreds of megabytes and tens of thousands of files, and the same line stops `sense scan`
indexing them.

But the phase artifacts live in that same tree — `index.json`, `minibench.md`, `handoff.md`, the
scenario, the gold, the rubric. They are kilobytes of markdown and yaml, and they are the entire
record of what a campaign measured. Nothing in the phase graph or in any plan says to copy them
anywhere durable, so on a fresh clone a completed campaign has no record at all and 07-01's
"recorded with its numbers and its route" is satisfied only until someone runs `git clean`.

This campaign copied them by hand into `lab/campaigns/dotnet-jellyfin/record/`, which follows the
convention `lab/campaigns/csharp-aspnet/` already sets for tracked campaign material.

**Route:** cycle 05. Either a phase writes the record out, or the ignore rule narrows to the
heavy directories (`*/home/`, `*/repo/`, `*/raw/`) and keeps the artifacts.
