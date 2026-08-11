# PLAN 02-minibench

## TASK

Read the unscored two-arm run the driver just produced against the two-step probe scenario,
and rule on one thing: is this question worth expanding into a full session.

## SCOPE

You read a run that already exists and issue one verdict. **Out of scope:** re-running
anything, expanding the scenario, editing the yaml or its gold, re-scoring, raising a watchdog,
the paid bench, any other repo. You do not re-author - a routed `REQUESTION` sends the credit
table back to the authoring phase, which owns the rewrite.

**Both arms are guaranteed to be MEASUREMENTS.** Run validity is mechanical, not a judgment:
the runner classifies every run (`lib/run_validity.py`), retries a void arm once and parks the
void one, and the phase halts rather than reaching you if a pair is still incomplete. So the
three verdicts below are always issuable, and none of them is a comment on arm health. If you
are ever handed a void arm anyway (`valid: false`, or a parked `failed-run-*`), that is a
defect in the driver: **write it down and STOP - do not invent a fourth verdict, and do not
average a harness artifact into a number.** Note that OUT OF CLOCK IS NOT VOID: a watchdogged
arm is valid, it is the arm's own result, and for the baseline it is the win condition.

## RUN

`$REPO`, `$VERTICAL`, `$VDIR`, `$YAML`, `$RDIR` (the mini-bench results root) are exported.
Work from `improvement-loop/`.

1. The credit table, both arms, per gold item - this is your one mechanical input:

       RESULTS_DIR="$RDIR" python3 bench/lib/credit_table.py "$REPO"

2. Arm health, before believing any floored number:

       find "$RDIR" -name run_meta.json -exec python3 -c \
         'import json,sys; d=json.load(open(sys.argv[1])); print(sys.argv[1], d["tool"], \
          "wall", d["wall_time_seconds"], "of", d["session_timeout_seconds"], \
          "exit", d["claude_exit_code"], "watchdog", d["watchdog_kind"], \
          "valid", d["valid"], d["void_reason"])' {} \;

   A non-null `watchdog_kind`, a `valid: false`, or a wall clock at the timeout means the arm
   was cut off. A throttled or timed-out arm produces a false result in either direction.

3. What the sense arm did with its tools:

       python3 bench/lib/tool_use_audit.py "$RDIR"/sense/"$REPO"/run-1/sense-io.jsonl

4. How the BASELINE assembled its answer - the route, not the number:

       python3 bench/lib/baseline_route.py "$RDIR"/baseline/"$REPO"/run-1/transcript.json

   Quote the route in your artifact. One search that returned the candidate set, followed by
   reads of files the ask named by function, means the question measured the PROMPT and not
   the contract. The precedent: nine tool calls, one `grep -rn "Setting\."`, 17 of 18
   dependents, 157 seconds of a 480 second wall.

5. What the sense arm cited that was never returned to it:

       python3 bench/lib/transcript_miss.py --stack "$VERTICAL" --results-dir "$RDIR" \
         --repo "$REPO"

   `--results-dir "$RDIR"` scopes the mine to THIS cycle's pair. Without a root it exits
   non-zero, and a model-wide root would blend the cycles this question already replaced.

## DECIDE

**Both arms must pass, and the bar is arithmetic.** Quote the `dependents` group figures from
the credit table and compare them in writing:

- `PROCEED` - the baseline holds **at or below 0.50** of `dependents` AND the sense arm beats
  it by **at least +0.50**. Both conditions, quoted. This is the shape of every banked win on
  the record: a mini run at baseline 0/5 against sense 5/5 preceded a +1.00 cell.
- `REQUESTION` - the baseline holds **above 0.50**. The question does not discriminate however
  clean the scenario looks, because recall caps at 1.00 and a baseline at B caps the delta at
  `1.00 - B`. The lever is the QUESTION, not the anchor: name, in your artifact, the rows the
  baseline took and the route it used, so the next draft can ask something that route cannot
  answer.
- `REQUESTION` - the sense arm did not reach either. It never called `sense_blast`, dropped
  what came back, or never reached synthesis inside the wall. A question the instrument cannot
  answer is not a measurement of the baseline. Say which of the three it was.
- `NO-ANCHOR` - only when the shown blast set itself cannot carry twelve dependent files, which
  is a defect in the authoring phase, not a result. Everything else routes back for a rewrite.

**Cannot-finish-at-budget IS a result.** If an arm ran out of wall, that is the finding, not an
obstacle. Never propose raising the watchdog.

Every claim in `notes` is quoted output. A number you did not read out of the credit table or a
`run_meta.json` does not go in the file. This run is x1 and unscored: it may not settle a win,
a tie or a loss, and its number may never be cited anywhere. It decides one thing, which is
whether this question gets a full session.

## ARTIFACT

Write `$VDIR/results/loop/$REPO/minibench.md`, five headings:

    # Verdict              PROCEED, REQUESTION or NO-ANCHOR, and the sentence that decides it
    # Credit table         quoted, with the sense-only rows, the shared rows, the neither rows
    # The two numbers      baseline dependents recall vs 0.50, and the delta vs +0.50
    # Baseline route       how it assembled the set, quoted from baseline_route.py
    # If REQUESTION        the rows the baseline took, and what the next question must defeat

Then write `$VDIR/results/loop/$REPO/minibench.verdict.json`:

    {
      "phase":    "minibench",
      "repo":     "<repo>",
      "verdict":  "PROCEED" | "REQUESTION" | "NO-ANCHOR",
      "artifact": "verticals/<vertical>/results/loop/<repo>/minibench.md",
      "notes":    "one line, carrying both figures"
    }

## DONE WHEN

- Both arms' transcripts were read from disk, not inferred from an exit code.
- The credit table output is quoted in `minibench.md`.
- The baseline's `dependents` recall is stated as a number and compared to 0.50 in writing.
- The delta is stated as a number and compared to +0.50 in writing.
- Arm health is quoted per arm, and no floored score is believed without it.
- On `REQUESTION`, every row the baseline cited is listed by id.
- The verdict JSON exists and parses.

## DO NOT

- Do not re-author, re-gold or edit the yaml. Name the lever; the authoring phase pulls it.
- Do not propose raising a watchdog, extending a wall, or re-running to get a better sample.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
