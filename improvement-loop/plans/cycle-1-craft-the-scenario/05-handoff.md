# PLAN 05-handoff

## TASK

Six attempts have not produced a payable question on this repo. Write the human one page,
in plain words, that lets them decide what happens next.

## SCOPE

You write one summary of work that already happened. **Out of scope:** authoring another
question, running anything, re-scoring, editing a scenario or its gold, recommending a
watchdog change, any other repo. You are not a seventh attempt.

This is a HANDOFF, not a post-mortem. The repo is not dead, the anchor may be fine, and the
loop has simply run out of its own judgment. Say what was measured and what you would do.

## RUN

`$REPO`, `$VERTICAL`, `$VDIR`, `$CLONE`, `$YAML` are exported. Work from `improvement-loop/`.

1. The numbers, one row per attempt. This is the mechanical spine of your summary:

       cat "$VDIR/results/loop/$REPO/cycles.jsonl"

   Each row carries the cycle number, the phase that rejected it, the verdict, the question's
   name, and the `dependents` recall both arms actually scored.

2. The reads, in order. Each names the rows the baseline took and the route it took them by:

       ls "$VDIR/results/dryrun/$REPO"/minibench.*.md "$VDIR/results/dryrun/$REPO"/validate.*.md
       cat "$VDIR/results/loop/$REPO/minibench.md"      # the most recent, if present

3. What each attempt actually ASKED. **Do not take this from the ledger's `question` field
   or from `name:`** - those are titles, and measured attempts with opposite outcomes have
   carried identical ones. Read the real thing: the axis sentence in each yaml header and the
   discriminator step's prompt.

       for f in "$VDIR/scenarios/$REPO.yaml" "$VDIR/scenarios/$REPO.yaml".*.bak; do
         [ -f "$f" ] && { echo "=== $f"; sed -n '1,45p' "$f"; }
       done

   The archived reads from step 2 are the other half: each one says what the previous
   question failed at, which is what the next attempt was trying to fix. Pair them up, so the
   table shows a sequence of different ideas and not six copies of one title.

4. The current scenario's seam profile, for the anchor line of your recommendation:

       sed -n '1,40p' "$YAML"

## DECIDE

Two things, and only these.

**What the pattern is.** Read the six rows together, not one at a time. The useful reading is
whether the baseline's number MOVED across attempts. Six attempts clustered at the same
baseline recall is one finding (the route is the same every time, and the questions differ
only in words). Six scattered numbers is a different one (the questions differ but none has
found the gap yet). Say which of the two you are looking at, and quote the numbers that show it.

**What you would do next.** One recommendation, one line of why, and at most one alternative.
The honest options are: keep this anchor and change the kind of question, take a different
contract in this repo, or swap the repo for its declared backup. Pick one. Do not list all
three as equals and leave the human to sort it out.

### How to write it

The reader is not inside this bench and should not have to be. Write for someone who has
never read a plan in this folder.

- **Short sentences. Plain words.** If a six-year-old would not know the word, use a different
  word or explain it in the same sentence.
- **No jargon and no codenames.** Not `dependents`, not `cited recall`, not `the discriminator
  group`, not `+0.50`, not phase names. Say "the scattered places in the code that use it",
  "how many of those it found and pointed to exactly", "the bar we need to beat".
- **Every number gets its meaning in words next to it.** "13 out of 16 (about 8 in 10)".
- **Positive and factual.** Every attempt taught something and the summary says what. This is
  a repo that has banked a win before; nothing here says it cannot again. No blame, no
  hand-wringing, no apologising.
- **Lead with the outcome.** The first line says what happened and what is being asked of the
  reader. No preamble.
- **One page.** If it does not fit, cut the detail, never the recommendation.

Every number you write is quoted from `cycles.jsonl` or from a read on disk. If you cannot
quote it, leave it out.

## ARTIFACT

Write `$VDIR/results/loop/$REPO/handoff.md`, with exactly these five headings:

    # What happened      two or three sentences. Opens with: we tried six ways of asking,
                         none beat the bar yet, here is the picture so you can choose.
    # The six attempts   a table: attempt, what it asked (plain words), what the plain
                         search found, what our tool found, did it beat the bar
    # What worked        the things that went right, named. There will be several
    # What did not       the pattern across the six, in one short paragraph
    # What I would do    one recommendation, one line of why, at most one alternative

Then write `$VDIR/results/loop/$REPO/handoff.verdict.json`:

    {
      "phase":    "handoff",
      "repo":     "<repo>",
      "verdict":  "HANDOFF",
      "artifact": "verticals/<vertical>/results/loop/<repo>/handoff.md",
      "notes":    "one line: the recommendation, in plain words"
    }

## DONE WHEN

- The table has one row per attempt in `cycles.jsonl`, in order, none missing.
- No two rows describe the question the same way. If two attempts read alike, you took the
  title instead of the ask; go back to the axis sentence and the discriminator prompt.
- Every number in the file is quoted from `cycles.jsonl` or a read on disk.
- No bench jargon survives a read-back: no group names, no phase names, no bare deltas.
- `# What worked` is not empty.
- `# What I would do` names ONE recommendation.
- The verdict JSON exists and parses.

## DO NOT

- Do not author a seventh question, edit a scenario, or run either arm. You summarise.
- Do not call the repo or the anchor dead; six attempts bound what was tried, nothing more.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
