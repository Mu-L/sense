# The mined-to-merged chain

One finding, taken from the miner's material through to a shipped product change, with the
whole chain queryable in `records/`.

| step | record |
|---|---|
| finding | `findings/f-1c56a449e905.json` |
| hypothesis | `candidates/c-4326ee8d9f0b.json`, `hypothesis_at` **2026-08-17T22:17:52Z** |
| candidate | `internal/resolve/resolver.go`, this branch |
| validation | `validations/v-966a78f2a297.json`, `ran_at` **2026-08-17T22:33:01Z** |
| decision | **accepted, partial** |
| residual | `findings/f-eea567433cac.json` |

**The hypothesis was written 16 minutes before the validation ran.** That order is the whole
discipline: a hypothesis written afterwards is a description.

## The finding

`sense_graph` and `sense_blast` return an empty edge set — labelled `completeness: complete`
with the advice *"act on it, do not re-grep"* — for C# types declared in a block-scoped
`namespace X { }`. Of the 40 interfaces jellyfin declares, 17 resolved and 23 returned nothing,
and all 23 were block-scoped. The invisible side was the most-held: `ILibraryManager` (121 files
declare a field of it), `IServerConfigurationManager` (71), `IFileSystem` (66), `IUserManager`
(65).

Surface: **graph**. Found by the first live campaign's author phase, not by an oracle.

## The mechanism

`emitHolds` emits the composition edge with its target set to the type name as written —
`IThing`, bare. Inside a block-scoped namespace the symbol is `X.IThing`, so the exact
`byQualified` lookup misses. Step 3's bare-name fallback is gated to `calls`, `tests` and
`references`, so `composes` had **no** resolution path at all and the edge was dropped rather
than stored.

## The change

`composes` now reaches the lexical-scope walk that already existed for `inherits`. A field's
declared type is resolved through the enclosing scope, exactly as a base type is.

## The numbers

| target cell | before | after |
|---|---|---|
| declared interfaces resolving | 17 of 40 | **23 of 40** |
| `ILibraryManager` | 0 | **5** |
| `IFileSystem` | 0 | 0 |
| `IUserManager` | 0 | 0 |
| `IServerConfigurationManager` | 0 | 0 |
| cells that moved down | — | **0** |

**The prediction is partially falsified, and it is recorded as partial.** It predicted the count
would rise toward 40 and that all four named interfaces would come back non-empty. One of the
four did. The cause is diagnosed rather than guessed: the walk climbs the *source's* enclosing
scopes, so it binds a hold only where holder and target share a namespace prefix, and C# reaches
other namespaces through `using` directives the resolver holds no map of.

Accepted anyway, on the numbers: the change is correct, minimal, in the pattern the file already
uses, and strictly additive — 12 cells moved and every one moved up. It does **not** close the
defect, and the cross-namespace residual is recorded as its own finding rather than folded in.

## What "the regression corpus held" does NOT mean here

**A product change is not in the scoring path**, so re-scoring recorded transcripts cannot detect
a regression from it, and `sense-lab rescore` was not run as evidence. Stated plainly because the
phrase suggests more than was done.

What was actually checked: the product's own gates (`make ci` green, coverage floor with no new
exception, complexity ledger at zero), and the jellyfin sweep itself, where no cell moved down
and no currently-resolving interface went empty — the stated falsifier. **A re-run of the benched
arms was not bought.**

For the corpus itself: 4 cells, of which **1 is live-verified** (cycle 06's replay) and **3 are
recordings** that have never been re-run on this instrument.

## Council

Two rounds, both of which found real defects in the change.

- The kind guard I widened had **no test on either side of it**: replacing it with `if true`
  passed the entire repository. A guard test now kills that mutant.
- The walk bound candidates with **no filter at all**. In C# and TypeScript `.` separates a
  namespace from its types *and* a type from its members, so a source at `Ns.Outer.Inner`
  declaring a field of type `Helper` could bind `Ns.Outer.Helper` — a **method**, i.e. a
  fabricated hold. The walk now filters by language, test direction and type-like kind, and the
  safety fix was re-measured end to end: identical counts, zero cost.

## Instrument defect found by this pitch

**Nothing in `sense-lab` records a finding, a candidate or a validation.** `RecordFinding`,
`RecordCandidate` and `RecordValidation` exist in `lab/internal/record` and are reached only by
the miner and by tests; there is no command. This chain was recorded by a throwaway program
written under `lab/cmd/` and deleted after it ran, which is not a procedure anyone should repeat.

**Route:** cycle 05 or 06 — whichever owns the record store's operator surface.
