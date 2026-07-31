# Loop N - <name> (one-pager template)

> **How to use.** Copy this file to `0N-<slug>.md`, fill every section, delete the guidance comments. A
> one-pager is complete when every section has real content; if "Un-fakeable check" cannot be filled with a
> script or a file-level fact, the loop is not ready to automate and its gates stay blocking. Keep it to one
> page: pointers to scripts and docs, never re-explanations of them.

## Goal

<!-- One or two sentences: what this loop achieves and its exit state. If the goal needs a paragraph, the
     loop is too big; split it. Write it so a reader who stops here knows why the loop exists. -->

## Product duties (per Sense surface)

<!-- What this loop owes the product per goal.md: for each surface it touches (graph, blast, search,
     conventions, status, setup, tool contracts, response shape), what it checks, harvests, or improves.
     "None" must be stated with why; a skipped duty is a finding, not a style choice. -->

## Identity

- **Character:** checklist-convergence | judgment | scheduler | product-touching
  <!-- This decides what "loop engineering" means here. Checklist loops want idempotent scripts; judgment
       loops want the generator/evaluator split; schedulers want cap-aware retry policy; product-touching
       loops want the bench gate + revert discipline. -->
- **Unit of work:** what ONE iteration processes (one scaffold element, one candidate repo, one
  scenario→bench→diagnose turn, one matrix cell, ...).
- **Position:** which loop hands it inputs; which loop consumes its outputs. Loops depend on upstream
  ARTIFACTS, never on upstream loops being built.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | <the agent/script that does the work> | |
| Evaluator | <the SEPARATE adversarial check that decides continue-vs-stop> | never the generator grading itself |
| Mechanical verifier | <the un-fakeable script/fact the evaluator stands on> | |
| Human | <the decisions that are theirs> | mark each: permanent anchor or trust-ledger demotable |

## Stop conditions (all three, explicit)

- **Success:** <the mechanical condition; cite the script that checks it>
- **Budget:** <spend ceiling / turn cap / session cap; what "park" looks like>
- **Failure:** <what "genuinely exhausted" means here, and WHO it escalates to. A loop never records a loss
  or silently gives up; it wins, parks, or hands a decision up.>

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| | | | |

<!-- The three permanent anchors (scenario/ground-truth integrity, spend, publish) never demote; the repo
     slate was a fourth until 2026-07-29, when bootstrap went autonomous.
     Everything else may flip to advisory-logged after a vertical of clean reviews (trust ledger). -->

## State / memory

- <files: state json, ledgers, checklists. "The repo remembers so the agent can forget." Every state file
  is gitignored or committed per the existing conventions; name them exactly.>
- <Readability duty: name this loop's `LEDGER.md` write points (keys + transitions) per
  [`ledger.md`](ledger.md). The ledger is write-only for the loop - never read to decide anything.>

## Un-fakeable check

- <the ONE mechanical thing that keeps this loop honest (pergroup.py on real transcripts, file-existence +
  stale-ref scan, ledger-empty check, exit codes). If this section is empty, stop: do not automate.>

## Inputs / outputs

- **Consumes:** <artifacts, with paths>
- **Produces:** <artifacts, with paths>

## Fixture test (standalone, $0)

- <how to test this loop TODAY against the frozen ruby-rails / python-django artifacts, with pass criteria
  taken from known history. Name the fixture and the expected verdict (e.g. "gate must FAIL haystack, PASS
  sentry"). A loop without a fixture test goes live blind.>

## Built vs missing

- **Built:** <scripts/docs that already exist, with paths>
- **Missing:** <the honest gap list; small wiring vs real design work>
- **First live use:** <which vertical/stage exercises it next>
