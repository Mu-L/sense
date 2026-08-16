---
phase: handoff
reads: campaign.json
writes: handoff.md
emits: [AUTO]
---

# handoff

## Task

Six attempts have not produced a payable question on this repository, or a paid cell came back
below the bar. Write the one page, in plain words, that lets a human decide what happens next.

## Scope

You write one summary of work that already happened. **Out of scope:** authoring another question,
running anything, re-scoring, editing a scenario or its gold, recommending a wall change, any other
repository. **You are not a seventh attempt.**

This is a HANDOFF, not a post-mortem. The repository is not dead, the anchor may be fine, and the
loop has simply run out of its own judgment. Say what was measured and what you would do.

## Run

1. **The numbers, one row per attempt.** This is the mechanical spine of the summary. Each row
   carries the cycle number, the phase that rejected it, the verdict, the question's name, and the
   `dependents` recall both arms actually scored.

2. **The reads, in order.** Each names the rows the baseline took and the route it took them by.

3. **What each attempt actually ASKED.** **Do not take this from a title or a `name:` field**:
   measured attempts with opposite outcomes have carried identical ones. Read the real thing: the
   axis sentence in each yaml header and the discriminator step's prompt.

   The archived reads from step 2 are the other half: each one says what the previous question
   failed at, which is what the next attempt was trying to fix. Pair them up, so the table shows a
   sequence of different ideas and not six copies of one title.

4. **The current scenario's seam profile**, for the anchor line of the recommendation.

## Decide

Two things, and only these.

**What the pattern is.** Read the rows together, not one at a time. The useful reading is whether
the baseline's number MOVED across attempts. Six attempts clustered at the same baseline recall is
one finding: the route is the same every time and the questions differ only in words. Six scattered
numbers is a different one: the questions differ but none has found the gap yet. Say which of the
two you are looking at, and quote the numbers that show it.

**What you would do next.** One recommendation, one line of why, and at most one alternative. The
honest options are: keep this anchor and change the kind of question, take a different contract in
this repository, or swap the repository for its declared backup. Pick one. Do not list all three as
equals and leave the human to sort it out.

### How to write it

The reader is not inside this bench and should not have to be. Write for someone who has never
read a plan in this folder.

- **Short sentences. Plain words.** If a six-year-old would not know the word, use a different word
  or explain it in the same sentence.
- **No jargon and no codenames.** Not `dependents`, not `cited recall`, not "the discriminator
  group", not `+0.50`, not phase names. Say "the scattered places in the code that use it", "how
  many of those it found and pointed to exactly", "the bar we need to beat".
- **Every number gets its meaning in words next to it.** "13 out of 16 (about 8 in 10)".
- **Positive and factual.** Every attempt taught something and the summary says what. Nothing here
  says a repository that has banked a win before cannot again. No blame, no hand-wringing, no
  apologising.
- **Lead with the outcome.** The first line says what happened and what is being asked of the
  reader. No preamble.
- **One page.** If it does not fit, cut the detail, never the recommendation.

Every number written is quoted from the attempt history or from a read on disk. If you cannot quote
it, leave it out.

## Precedent

The plain-language rule is not a style preference. The page exists because a resuming session was
once pointed at the plans and the laws and had to discover its position by asking, and because a
reader outside the bench cannot act on a bare delta.

## Artifact

`handoff.md`, with exactly these five headings:

    # What happened     two or three sentences. Opens with: we tried six ways of asking, none
                        beat the bar yet, here is the picture so you can choose
    # The attempts      a table: attempt, what it asked in plain words, what the plain search
                        found, what our tool found, did it beat the bar
    # What worked       the things that went right, named. There will be several
    # What did not      the pattern across the attempts, in one short paragraph
    # What I would do   one recommendation, one line of why, at most one alternative

## Done when

- The table has one row per attempt in the history, in order, none missing.
- No two rows describe the question the same way. If two attempts read alike, you took the title
  instead of the ask; go back to the axis sentence and the discriminator prompt.
- Every number in the file is quoted from the attempt history or a read on disk.
- No bench jargon survives a read-back: no group names, no phase names, no bare deltas.
- `# What worked` is not empty.
- `# What I would do` names ONE recommendation.

## After this page

**The repository is parked until a human re-enters it deliberately.** It is not automatically
re-enterable. The fresh cycle count is what a human gets *if* they choose to resume it, not an
automatic second life, and that difference is a loop that stops rather than one that quietly spends
six more cycles.

## Do not

- Do not author a seventh question, edit a scenario, or run either arm. You summarise.
- Do not call the repository or the anchor dead; the attempts bound what was tried, nothing more.
- Do not spawn a subagent. Only the binary spawns.

## Questioned, not changed

Nothing yet.
