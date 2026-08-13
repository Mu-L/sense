## What this is

The vertical bench: a loop that measures whether Sense makes a frontier coding agent reach
answers it cannot reach without it. `bench/` is scripts, `plans/` is handed to an agent,
`docs/` is read by a human, `verticals/<key>/` is one campaign's state.

Everything here is bench work. Bench-only commits use `bench(<scope>):`, never `feat`/`fix`.
Product code is never touched from inside a vertical.

## Scope lock

Do the phase you were given and stop. Do not fix an adjacent script, re-shape a neighbouring
scenario, add a knob, or improve a doc you passed through. If you find a real defect outside
your phase, write one line naming it and carry on; the loop routes it, you do not.

**Delegation cap: you spawn nothing.** Every subagent in this system is spawned by
`vertical-loop.sh`. A phase agent that spawns its own judge is grading itself.

**Narration cadence: report at the end, not along the way.** One block when the phase is
done: what you decided, the artifact you wrote, the verdict. No running commentary, no
"let me now", no summary of the summary.

## The five killers

Before reporting ANY pattern as a finding - in chat, a doc, a ledger entry - name the check
that would kill it and **RUN** it. Run, not consider.

| killer | the question | precedent |
|---|---|---|
| **confound** | does X co-vary with vertical / size / model / date? cross-tab it | "declared-family law" was the VERTICAL split; chatwoot 0.09 and redmine 0.77 run the same prompt |
| **per-unit** | does the aggregate hold per row? | castability separated the repos cleanly and predicted the OPPOSITE per row |
| **arithmetic** | is it mechanically possible or necessary? | `delta = sense - base`, `sense <= 1.0`, so `+0.50` REQUIRES `base <= 0.50`: 10 of 20 paid cells were dead before they ran |
| **negative space** | what did I NOT search? | `\.Permission = ` misses `\.Permission, err = `: reported "written in exactly ONE place", was wrong |
| **n** | how many INDEPENDENT observations? is the new number inside the old variance? | n=1 + retro-fits is a story, and it must carry "hypothesis" in its title |

**A confirming check is not evidence; it is the absence of a test.** The failure mode is
running the check that CONFIRMS, calling it a finding, and running the killer afterwards.
That ordering is the whole bug. Checking costs one command; retracting costs a doc rewrite,
a memory rewrite and a wrong path a human has to read.

## RUN vs DECIDE

Every step is one or the other.

**RUN** (index, gates, stamping, rendering, `ledger_check`, the driver's phase order): follow
it exactly. No improvising a substitute, no hand-rolling what a script already does. If the
step looks wrong, SAY SO AND STOP - do not route around it.

**DECIDE** (anchor choice, gold curation, the pay call, diagnosis routing): judgment is the
job. Every factual claim leaving a DECIDE step is **quoted command output** or is labelled an
assumption. Before stating a finding, quote the output that shows it; if you cannot quote it,
you have not verified it. Negative claims - *cannot, no, never, dead, nonexistent, not
reached* - need a second, DIFFERENT probe before they are stated at all.

## The laws are in laws.md, and they bind YOU

`plans/cycle-1-craft-the-scenario/laws.md` is the complete, authoritative set. **Read it before
any verdict, close, swap, kill, pay call, or LEDGER entry** - not only when running a phase.
It binds the operator session too, and the operator session is the one no driver hands a plan
to, so it is the one that has to open the file on purpose.

This page used to paraphrase ten of those laws here. That digest is DELETED, because it read
as the complete set while omitting the ones that bite. Measured 2026-08-11: an operator session
closed a repo on a grep screen, forecast a cell from per-row rates pooled across thirteen
different asks, and recommended authoring for token-darkness - three moves banned by
`NO GREP SCREEN IS A GATE`, `THE FIRST MEASUREMENT IS A REAL BASELINE ARM`, `PRECISION RANKS,
IT NEVER KILLS` and `TOKEN-DARKNESS IS NOT THE MECHANISM`, none of which were in the digest it
was working from. A summary of the laws is not the laws.

Two laws live HERE and are not in `laws.md` - they govern the code around the bench rather than
what may be concluded from a run:

- **A SCRIPT IS NOT A RULE UNTIL YOU PROVE IT RUNS AND IT GATES.** Before citing a script as a
  constraint, or opening it to edit, establish both. **Used** has three states, not two:
  *invoked* (a driver, plan or agent names its path), *imported* (a live module imports it), or
  *cold*. Import probes are UNANCHORED - `scorer.py` imports `grounding` from inside a
  function, and a `^(import|from)` probe misses it. **Enforces** is separate: a live script
  that only builds a report string constrains nothing; the test is whether it exits non-zero on
  the condition. Precedent: "RUNS=2 binds the headline arm only" was quoted as a law from
  `select_final.py` (cold) and `matrix.py` (live, but the line lives in a function that renders
  a sentence). Nothing enforced it.
- **NEVER CHANGE A SCRIPT ANOTHER CYCLE'S PLANS USE.** Verifying such a change costs a full
  cycle - hours and paid sessions - and it fails silently, as a slightly different score rather
  than a crash. A new cycle reads artifacts off disk and writes its OWN scripts. Every shared
  instrument already writes its output down (`scored.json`, `sense-io.jsonl`, `banked.jsonl`),
  so reaching into an older cycle's code is never necessary. `transcript_miss.py` is the
  pattern: read-only, $0, over transcripts that already exist.
  **The carve-out is WRITE-ONLY HUMAN SURFACES** (`render-status.sh` and STATUS.md): they
  decide nothing, no loop reads them, and no measurement can move because of them, so a second
  cycle extends them rather than building a second page nobody thinks to open. The rule
  protects what feeds a decision, not what feeds a reader.

## Where things live

| Need | Read |
|---|---|
| the phase you are running | `plans/cycle-1-craft-the-scenario/<phase>.md` - authority at runtime |
| the laws in one line each | `plans/cycle-1-craft-the-scenario/laws.md` |
| why the design is what it is | `docs/` - authority on design, not at runtime |
| a script | `bench/lib/`, `bench/drivers/`, `bench/bootstrap/` |

**If you open a `docs/` page mid-run, that is a bug report**: what you needed was missing from
the plan. Say so in your report and name the line the plan should carry.
