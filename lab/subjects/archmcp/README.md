# archmcp

A competing local MCP server that generates architectural snapshots of a
repository. `github.com/dejo1307/archmcp`, read at `986d9dc` (2026-03-06).

**Operational facts only, dated and tied to a version.** Nothing here is a
judgement about the product, and nothing here was read out of its
documentation: every line below is what a discovery run observed.

## Discovery run, 2026-08-18

Installed from source with `go install ./cmd/archmcp`, in a disposable HOME with
nothing in it, on Go 1.25.12 / darwin-arm64. The declared cleanup was then run
and what survived it was compared against what appeared.

| | paths |
|---|---|
| written under the arm's HOME | 2,185 |
| removed by the declared cleanup | 1 |
| surviving it | 2,184 |

The one removed is the binary, `home/bin/archmcp`. What survives is the Go
toolchain's own state, in three trees:

| tree | paths | what it is |
|---|---|---|
| `home/gocache` | 1,615 | the build cache |
| `home/go` | 562 | the module cache, including the archmcp module and its dependencies |
| `home/Library/Application Support/go` | 7 | the toolchain's telemetry counters |

**Attribution, stated because it matters.** Those three trees are written by the
Go toolchain during the install, not by archmcp's own code. That does not make
them nothing: state left in a HOME is state every later run against that HOME
reads, whoever wrote it.

**What actually contains them is the disposable HOME, not the cleanup.** Each
arm gets its own, and releasing an arm destroys it, so nothing here survives a
run. The declaration above says so explicitly rather than leaving a cleanup that
removes one file in 2,185 looking complete.

**A subject installed from source cannot prove its own cleanup**, and that is
the finding this run produced. It is the input to the escalation question in
08-05: a subject that cannot be contained by a disposable home goes in a
container, and this one is contained by the disposable home — which is why it
does not.

## Nothing from the host reaches it

Its commands run with the arm's own environment: its repository, its own HOME,
and its own PATH. There is no path through which the host's credentials or the
host's agent configuration could arrive, and that is structural rather than a
policy — the type that carries the environment carries nothing else.

## Not yet benched

This subject has been installed, run and removed. It has not been scored against
anything, and no comparison has been published. That is 08-05.
