# sense-main

Sense at `main`, installed the way a user installs it.

Last verified: 2026-08-15.

## The arm must see what a real user sees

Setup uses the product's own `sense setup` surface, never a hand-rolled
imitation. The moment the bench writes its own `.mcp.json`, it is measuring a
configuration nobody has.

That has a consequence worth recording rather than discovering: a change to
`sense setup` changes what this arm sees, which changes the measurement, and
nothing about that is visible in a result months later. Cycle 03 records the
files setup wrote and their hashes into the run.

## Known side effect: indexing configures

`sense scan` runs setup when a repository has no index. On a headless bench that
takes the Claude Code path and writes `.mcp.json`, `CLAUDE.md`, `.claude/`
settings with a pre-tool-use hook, skills and an agent into the subject
repository. **Preparing the repository is therefore a contamination event**, and
the naive preparation hands the baseline arm Sense.

The trigger is `firstRun`, which is true whenever the index directory does not
yet exist (`internal/scan/scan.go`), and setup then writes into the
**repository root** regardless of where that index directory is. So pointing
`SENSE_DIR` outside the repository is **not** enough on its own: the first scan
still contaminates, and only later scans are quiet.

What does suppress it is creating the index directory **before** scanning, so
`firstRun` is false on the very first pass. Verified against the code, not
assumed; an earlier version of this file claimed `SENSE_DIR` alone was
sufficient and that was wrong.

## Install and setup commands are deliberately absent from subject.json

Cycle 08 is the first thing that runs them. A schema for commands nobody
executes is written from imagination; they arrive with the code that runs them.
