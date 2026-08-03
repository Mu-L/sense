# Plan steps, in order

What each phase actually runs, and why. The plan files in `plans/` are the authority at
runtime; this page is the map a human reads to see the loop end to end.

Authoring is a **cycle**, and the driver runs it unattended. `NO-ANCHOR`, `REQUESTION` and
`DO-NOT-PAY` all re-enter `01-author.md` immediately with the credit table that rejected the
question, and the anchor stays. The scenario is rewritten in place, never deleted and
re-scouted. The loop parks for a human only after six cycles on one repo (`AUTH_CYCLE_MAX`),
and the next run gets a fresh budget.

    author -> minibench -> expand -> preflight -> validate -> bench
        ^         |            |                     |
        +-- REQUESTION/NO-ANCHOR ----------- DO-NOT-PAY

One cycle is one author agent plus one unscored two-arm pair, so the bound is the spend
control: roughly an hour of wall for the six.

At the ceiling the loop does not just stop. It writes `handoff.md` - what each attempt asked,
what both arms scored, what worked, what did not, and one recommendation - in plain words, off
a per-cycle ledger (`cycles.jsonl`) the driver stamps while each run is still on disk. The runs
themselves are overwritten every cycle, so that ledger is the only place the six numbers
survive.

## 01-author - anchor, question, probe scenario, gold

| # | Step | Purpose |
|---|---|---|
| 0 | Read `minibench.md` if present | You were routed back from a measured run. Keep the anchor, change the question |
| 1 | `anchor_rank.py --top 20` | Candidate anchors as a listing, not a ranking to defer to |
| 2 | `seam_hunt.py <Symbol> --propose` | The seam profile. `PRECISION` is dependents over grep hits, lower is better. It ranks candidates; it never kills one |
| 3 | `mcp_probe.py` blast at defaults | What the sense arm is actually shown. Every dependents row must appear here |
| 4 | Open every candidate row | The blast gives the enclosing `def`; pin the line that touches the contract |
| 5 | `memorization_probe.py` | Cut rows the model recites from memory |
| 6a | `scenario.py --prompt` | Prompt is neutral: no path, symbol, count or tool name |
| 6b | `rubric_check.py` | Rubric is judgeable before anything runs |
| 6c | `gold_confidence_check.py --group dependents` | Every row really appears in what MCP shows the arm |
| 6d-e | `gold_audit.py stamp` / `verify` | One TODO per row, replaced by a hand-read credit; verify fails while any remains |
| - | ARTIFACT | A **two-step** `$YAML` (map the contract, audit every dependent) + `$RUBRIC` + `author.verdict.json` |

The discriminator is step 2 of the two. Nothing in this phase may kill a draft with a grep.

## 02-minibench - is this question worth a full session

| # | Step | Purpose |
|---|---|---|
| 1 | `credit_table.py` | The one mechanical input: per-gold-item credit, both arms |
| 2 | `run_meta.json` sweep | Arm health. A cut-off arm is a false result in either direction |
| 3 | `tool_use_audit.py` | What the sense arm did with its tools |
| 4 | `baseline_route.py` | How the baseline assembled the set. The route is the lever |
| 5 | `transcript_miss.py` | What the sense arm cited that was never returned to it |
| - | DECIDE | `PROCEED` needs both: baseline at or below 0.50 of `dependents`, and sense ahead by at least +0.50. Anything else is `REQUESTION` |
| - | ARTIFACT | `minibench.md` (verdict, credit table, the two numbers, baseline route, if REQUESTION) + verdict JSON |

This replaces the simulated adversary probe. Same cost, a real baseline instead of a guess.

## 03-expand - the seven-step session

| # | Step | Purpose |
|---|---|---|
| 1 | Read `minibench.md` and `$YAML` | The measured question is the thing being preserved |
| 2 | Rewrite to seven steps | Orient, map the contract, trace the write path, **audit every dependent**, trace the guards, blast radius, change map |
| 3 | `mcp_probe.py` blast | The anchors and specs being added |
| 4 | Open every added row | Same hand audit as authoring |
| 5 | The five gates | `scenario.py --prompt`, `rubric_check.py`, `gold_confidence_check.py`, `gold_audit.py stamp` / `verify` |
| - | DECIDE | Step 4 and its `dependents` gold are byte-identical to the probe scenario. New gold goes only in non-scoring groups |
| - | ARTIFACT | Seven-step `$YAML` + `$RUBRIC` + gold-audit sheet + `expand.verdict.json` |

The steps accumulate in one session, so by step 4 both arms have already spent context and
wall. That is part of the measurement.

## 05-handoff - the human's page after six cycles

| # | Step | Purpose |
|---|---|---|
| 1 | `cat cycles.jsonl` | The mechanical spine: one row per attempt, both arms' scores |
| 2 | The archived reads | What the baseline took each time, and the route it took |
| 3 | The question names | What each attempt actually asked, including the `.bak` versions |
| 4 | The seam profile | The anchor line of the recommendation |
| - | DECIDE | Did the baseline's number MOVE across attempts? Then one recommendation: keep the anchor and change the kind of question, take a different contract, or swap the repo |
| - | ARTIFACT | `handoff.md` (what happened, the six attempts, what worked, what did not, what I would do) + verdict JSON |

Written for someone who has never opened this folder: short sentences, no jargon, every number
explained in words, positive and factual. It is a handoff, not a post-mortem.

## 04-validate - rule on whether money moves

| # | Step | Purpose |
|---|---|---|
| 1 | `credit_table.py` | Per-gold-item credit, both arms, on the full scenario |
| 2 | `run_meta.json` sweep | Arm health, quoted per arm |
| 3 | `tool_use_audit.py` | What the sense arm did with its tools |
| 4 | `transcript_miss.py` | What it cited that was never returned, and where it fell back |
| 5 | `baseline_route.py` | How the baseline assembled its answer |
| 6 | The session row | Baseline `dependents` recall isolated (mini-bench) against in-session. Free calibration, one row per cell |
| - | DECIDE | Quote the delta against +0.50. Baseline held too much, or neither arm reached, is `DO-NOT-PAY` and the lever is the question |
| - | ARTIFACT | `validate.verdict.json` + `validate.md` (verdict, credit table, arm health, session row, if DO-NOT-PAY) |

All four phases close on the same three bans: no subagent spawning, no adjacent fixes, no
negative claim from a single probe.
