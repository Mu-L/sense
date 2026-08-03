# PLAN 04-validate

## TASK

Read the unscored validation run the driver just produced against the full seven-step scenario
and rule on one thing: does this cell go to the paid bench, or back to the question.

## SCOPE

You read a run that already exists and issue one verdict. **Out of scope:** re-running
anything, running the paid bench, editing the scenario or its gold, re-scoring, raising a
watchdog, any other repo. You do not diagnose a paid loss - a separate phase owns that. You do
not fix the harness.

## RUN

`$REPO`, `$VERTICAL`, `$VDIR`, `$RDIR` (the validation results root) are exported. Work from
`improvement-loop/`.

1. The credit table, both arms, per gold item - this is your one mechanical input:

       RESULTS_DIR="$RDIR" python3 bench/lib/credit_table.py "$REPO"

2. Arm health, before believing any floored number:

       find "$RDIR" -name run_meta.json -exec python3 -c \
         'import json,sys; d=json.load(open(sys.argv[1])); print(sys.argv[1], d["tool"], \
          "wall", d["wall_time_seconds"], "of", d["session_timeout_seconds"], \
          "exit", d["claude_exit_code"], "watchdog", d["watchdog_kind"], \
          "valid", d["valid"], d["void_reason"])' {} \;

   A non-null `watchdog_kind`, a `valid: false`, or a wall clock at the timeout means the arm
   was cut off. A throttled or timed-out arm produces a false loss.

3. What the sense arm did with its tools:

       python3 bench/lib/tool_use_audit.py "$RDIR"/sense/"$REPO"/run-1/sense-io.jsonl

4. What it cited but was never returned, and where it fell back:

       python3 bench/lib/transcript_miss.py --stack "$VERTICAL" --repo "$REPO"

5. How the BASELINE assembled its answer - the route, not the number:

       python3 bench/lib/baseline_route.py "$RDIR"/baseline/"$REPO"/run-1/transcript.json

6. The session row. The mini-bench measured this same discriminator step in ISOLATION; this
   run measures it after three steps of accumulated context and wall. Put both baseline
   `dependents` figures side by side:

       grep -A4 '^# The two numbers' "$VDIR/results/loop/$REPO/minibench.md"

   State which way the session moved it. One row per cell is how the loop learns whether the
   isolated step is a fair predictor of the session, and it is free.

## DECIDE

**The bar is +0.50 on the `dependents` group, not a visible gap.** A cell can discriminate
clearly and still be worth nothing: baseline 0.625 against sense 0.938 is a real, tool-driven
gap of +0.31 and it is a LOSS, because the floor is +0.50. Quote the delta and compare it to
0.50 explicitly before writing a verdict.

Three readings, and only one of them pays.

- **The baseline assembled too much of the set.** It cited the scattered residue at
  `path:line`, not just the anchors, and the group delta is under +0.50. DO NOT PAY. The lever
  is the QUESTION: name the rows the baseline took and the route it used, exactly as the
  mini-bench does, so the next draft can defeat that route on the same anchor.
- **Neither arm reached.** The sense arm never called `sense_blast`, dropped what came back, or
  never reached synthesis inside the wall. DO NOT PAY. A scenario the instrument cannot answer
  is not a measurement of the baseline.
- **The sense arm reached what the baseline did not, by at least the floor.** The gap is in
  `dependents`, at `path:line`, and the group delta is at or above +0.50. PAY.

**Cannot-finish-at-budget IS a result.** If the sense arm ran out of wall, that is the verdict,
not an obstacle. Never propose raising the watchdog to rescue it.

Every claim in `notes` is quoted output. A number you did not read out of the credit table or a
`run_meta.json` does not go in the file. This run is x1 and unscored: it may not settle a win,
a tie or a loss, and its number may never be cited anywhere. It decides one thing, which is
whether money moves.

## ARTIFACT

Write `$VDIR/results/loop/$REPO/validate.verdict.json`:

    {
      "phase":    "validate",
      "repo":     "<repo>",
      "verdict":  "PAY" | "DO-NOT-PAY",
      "artifact": "verticals/<vertical>/results/loop/<repo>/validate.md",
      "notes":    "one line, with the two cited-recall figures in it"
    }

And `$VDIR/results/loop/$REPO/validate.md`, five headings:

    # Verdict           PAY or DO-NOT-PAY, and the single sentence that decides it
    # Credit table      the sense-only rows, the shared rows, the neither rows
    # Arm health        watchdog code + wall clock per arm, quoted
    # Session row       baseline dependents recall: isolated step vs full session
    # If DO-NOT-PAY     which of the three readings, the rows the baseline took, and what
                        the next question must defeat

A `DO-NOT-PAY` keeps the anchor and the scenario on disk. The lever is the question, and the
authoring phase rewrites it in place against this credit table.

## DONE WHEN

- Both arms' transcripts were read from disk, not inferred from an exit code.
- The credit table output is quoted in `validate.md`.
- The `dependents` delta is stated as a number and compared to +0.50 in writing.
- Arm health is quoted per arm, and no floored score is believed without it.
- The session row carries both baseline figures and says which way the session moved it.
- On `DO-NOT-PAY`, every row the baseline cited is listed by id.
- The verdict JSON exists and parses.

## DO NOT

- Do not pay on a hand-grep, a proxy, or a scenario that "looks strong". The run decides.
- Do not propose raising a watchdog, extending a wall, or re-running to get a better sample.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
