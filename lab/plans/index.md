---
phase: index
reads: repo.json
writes: index.json
emits: [AUTO]
---

# index

## Task

Clone the repository at its pinned revision, run `sense scan` over it, and record what the index
holds. Nothing here judges anything.

## Scope

You prepare one repository. **Out of scope:** choosing an anchor, writing a scenario, running
either arm, any other repository. If the clone or the scan fails, that is this phase's failure
and it is recorded as one; it is never worked around.

## Run

1. Clone at the revision named in `repo.json`. A moving revision is not a measurement: two runs
   months apart must see the same tree, and a floating branch means nothing on the board is
   comparable to anything else on it.

2. `sense scan` over the clone, then `sense_status` through the MCP server.

3. Record, in `index.json`: the revision, the file and symbol counts, the language breakdown, the
   edge count, and whether embeddings are present.

## Decide

Nothing. This phase emits `AUTO` and either writes its artifact or does not.

The one thing it must not do is soften a failure. A scan that indexed half the tree produces an
`index.json` that says so, and the authoring phase reads it. An index quietly short of its
symbols is how a repository gets called dark when the tool simply did not run.

## Artifact

`index.json`, carrying the revision, the counts, and the `sense_status` output as it was
returned.

## Done when

- The clone is at the pinned revision, and the revision is in `index.json`.
- `sense scan` exited zero and `sense_status` was read through the MCP server, never by opening
  the index file.
- The counts in `index.json` are quoted from `sense_status`, not estimated.

## Do not

- Do not read the SQLite index directly. Sense is reached through the MCP server, because that
  is the channel the benched agent actually has.
- Do not spawn a subagent. Only the binary spawns.
