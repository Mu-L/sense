# LAWS

The standing laws of cycle 2. One line each, statement only. They bind every phase and
they are not re-argued at runtime.

Cycle 2 takes a question that already WON on the headline arm and puts it to the other
models. It is not cycle 1 and several of cycle 1's laws are reversed here. Read these,
not those.

## What this cycle is

- **THE QUESTION IS SETTLED BEFORE YOU ARRIVE.** A cell reaches cycle 2 only as a banked
  WIN on the headline arm. You do not re-author it, re-word it, re-gold it, or improve
  it. If the question looks wrong, say so in one line and stop; a rewrite is cycle 1's
  and it would void the board.
- **THIS CYCLE CAN RECORD A LOSS.** Cycle 1 cannot: it wins, parks, or routes a lever.
  Cycle 2 publishes what it measured. An arm that does not replicate is a result and it
  goes on the page. Suppressing it is how a bench becomes marketing.
- **THE PAGE IS PUBLIC AND IT IS OURS.** It is read on the Sense repository by someone
  who arrived knowing the repo or the model, not the bench. Write in our voice, say
  plainly where Sense helped, and never lead with our own gaps. Honest is not the same as
  self-flagellating.
- **NOT A MODEL RANKING, NOT A REPO RANKING.** Each column is one model's Sense arm
  against that SAME model's own baseline. Different harnesses, different budgets,
  different defaults: absolute scores across models mean nothing and are never set side
  by side. Questions and gold are written per repo, so numbers do not travel between
  pages either.

## What may be concluded

- **QUOTE IT OR YOU HAVE NOT VERIFIED IT.** Before stating a finding, quote the output
  that shows it. Negative claims - cannot, no, never, did not - need a second, DIFFERENT
  probe before they are stated at all.
- **NO FIGURE THAT IS NOT IN THE NUMBERS JSON.** Every number on the page comes from
  `board.py assemble`. You may not compute one, round one differently, or carry one over
  from memory. If a figure you want is absent, say it is absent.
- **A SENSE ARM THAT NEVER CALLED SENSE MEASURED NOTHING ABOUT SENSE.** It is a baseline
  with extra configuration. Its delta is not a result about the product, it is left out
  of the replication count, and it is reported as OUR routing failure, never as the model
  failing or as Sense failing to help.
- **A DEAD SERVER IS NOT A MODEL CHOICE.** No frames in `sense-io.jsonl` means the MCP
  server never came up: discard and re-run. Frames with no `tools/call` means the server
  was there and the model did not use it: that is a real result. The two look identical
  in the score and must never be conflated.
- **CANNOT-FINISH-AT-BUDGET IS A RESULT.** Never raise a watchdog to rescue a stalled
  arm. A failed exam is not an invalid exam.
- **EVERY ARM GETS 2 RUNS.** A third is bought by a flipped VERDICT - the arm's dominant
  cell changes between runs - never by a single gold row flipping. With twenty-plus rows
  a row differs on almost every pair, so "any row" would make a third run unconditional.

## What you may not touch

- **NEVER EDIT A CYCLE 1 SCRIPT.** Verifying such a change costs a full authoring cycle
  and it fails silently, as a slightly different score rather than a crash. Cycle 2 reads
  artifacts off disk and writes its own scripts. Everything you need is already written
  down: `scored.json`, `sense-io.jsonl`, `run_meta.json`, `banked.jsonl`.
- **A SCRIPT IS NOT A RULE UNTIL IT RUNS AND IT GATES.** Before citing one as a
  constraint, establish that something invokes or imports it, and that it exits non-zero
  on the condition rather than printing a sentence about it.
- **THE HEADLINE COLUMN IS READ, NEVER RE-RUN.** It is already banked with its runs and
  its groups. Re-running it spends the most constrained subscription to reproduce a
  number we hold.
- **SAME SENSE BUILD IN EVERY COLUMN.** If the installed build differs from the one the
  headline column was banked at, the board compares two products and reads as a model
  difference. The gate refuses; do not work around it.

## What the page owes its reader

- **THE ASK, VERBATIM.** The framing and every step prompt, exactly as the models
  received them. A result is unreadable without the question that produced it, and a
  paraphrase lets the page drift from what was actually sent.
- **REACH LEADS.** Not "scored higher" but "found what it could not otherwise find": the
  answers the model never reached in any run without Sense. That is the claim the whole
  programme exists to test.
- **COST IN TOKENS, NEVER IN DOLLARS.** Four of the five arms run on flat-rate plans and
  report no price at all. Tokens used is built the same way on every harness and is the
  only cost figure that means the same thing in every column.
