# LAWS

The standing laws of cycle 3. One line each, statement only. They bind every phase and
they are not re-argued at runtime.

Cycle 3 is the only cycle that touches Sense's own code. It takes a stack Sense cannot
handle yet and makes it handleable, on a branch, and stops before the pull request. It is
not cycle 1 and not cycle 2, and several of their laws are reversed here. Read these, not
those.

## What this cycle is

- **THE TRUTH HERE IS A FACT, NEVER A JUDGEMENT.** The only question this cycle asks is
  "this relationship exists in the source; does Sense return it?". No score, no model, no
  baseline, no gold curation, no delta. That is what lets it run unattended where a
  fix-for-value window could not.
- **THIS CYCLE CAN RECORD A LOSS.** Cycle 1 wins, parks or routes a lever. Cycle 3 may
  end in `CANNOT-BUILD` or `REVERT`, and a clean revert closes a dead end permanently.
  A half-shipped extractor is the only forbidden exit.
- **NO REPRO, NO CODE.** Every claim about what Sense misses is a failing test on current
  `main` before a line of the fix exists. A gap that cannot go red has not been found.
- **A FIXTURE IS NOT A REPO.** Tests going green prove the parser; they prove nothing
  about ASP.NET as it is actually written. Every lane is also proven on cloned real code,
  over MCP, against relationships hand-audited at `path:line` in that clone.
- **MCP IS THE ONLY SURFACE.** Every check that queries Sense goes through the MCP server
  (`bench/lib/mcp_probe.py`). The CLI diverges by design - different defaults, caps and
  budget - so a CLI call measures a surface no agent touches.
- **QUOTE IT OR YOU HAVE NOT VERIFIED IT.** Before stating a finding, quote the output
  that shows it. Negative claims - cannot, no, never, missing, unsupported - need a
  second, DIFFERENT probe before they are stated at all.

## What may be built

- **THE THREE LANES BIND.** New language or framework support, dead-code fine-graining,
  AI-tool integration. Nothing else. Sense is feature-complete for v1: a new command, a
  new output format, a config knob, a fifth tool or a performance rewrite is
  `OUT-OF-SCOPE` and is recorded as such, never forced through as part of a language lane.
- **A DIRECTORY IS NOT SUPPORT, AND ITS ABSENCE IS NOT ITS ABSENCE.** The trigger tests
  whether `internal/extract/<lang>/` exists. A language can be partly served by
  `internal/extract/langspec/` with no such directory, and a directory can exist while
  the stack's dispatch idioms resolve to nothing. Measure what is missing; never infer it
  from the exit code that opened the window.
- **PER-LANGUAGE LOGIC IS NAMESPACED.** `internal/extract/<lang>/`,
  `internal/conventions/detectors_<lang>.go`, `internal/model/<framework>.go`,
  `internal/resolve/<framework>.go`. Reuse shared helpers; never generalise one language's
  heuristic into a generic detector, and never edit another language's files to make yours
  pass.
- **THE PRODUCTION DISCIPLINE APPLIES IN FULL.** `make ci` green, the per-file coverage
  floor, zero complexity suppressions, gofmt and goimports, no side effects in the pure
  core. Decompose the function; you cannot `//nolint` your way out.

## What you may not touch

- **NO BENCH STATE, IN EITHER DIRECTION.** Cycle 3 never reads or writes `verticals/`,
  never reads a scored run, never spawns a benched arm, never spends a paid token. Its
  whole input is one request file and its whole output is a branch plus this window's
  pages.
- **NEVER EDIT A CYCLE 1 OR CYCLE 2 SCRIPT.** Verifying such a change costs a full
  authoring cycle and it fails silently, as a slightly different number rather than a
  crash. Cycle 3 invokes them and reads what they already write down.
- **A NEW LANGUAGE CANNOT INVALIDATE ANOTHER LANGUAGE'S INDEX, AND THAT IS PROVEN, NOT
  ASSUMED.** The php holder-composition fix was verified php-only by rescanning a Go, a
  Ruby and a Python repo and finding identical counts. Run the same control here. If a
  control repo moves, the change is not per-language and it stops until it is.
- **BOOTSTRAP OWNS THE SLATE.** This cycle does not stamp a vertical, hunt repos, pin
  SHAs or fill a slot. It clones what it needs to prove itself and leaves the queue alone.

## Where it stops

- **THE HUMAN GATE IS THE PULL REQUEST, AND IT IS THE ONLY ONE.** The window runs to a
  committed local branch and a page a human can read in two minutes, then stops. Nothing
  is pushed, no PR is opened, no branch is deleted.
- **THE PAGE STATES WHAT IS NOT COVERED.** A lane that resolves controllers and not
  minimal APIs is a good lane honestly described. The handoff naming its own edges is
  what makes the next window cheap; a page that implies completeness costs someone a day
  finding the hole.
