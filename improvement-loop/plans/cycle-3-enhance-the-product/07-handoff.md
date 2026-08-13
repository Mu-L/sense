# PLAN 07-handoff

## TASK

Write the one page a human reads before opening the pull request: what this lane does, what
it was measured on, and what it does not cover.

## SCOPE

You write a page. **Out of scope:** product code, tests, probes, re-running anything,
pushing, opening a pull request, deleting or renaming the branch, `verticals/`. The window
stops here by design and you are the last phase in it.

You are writing for someone who was not in this window and does not know this bench. They
have two questions - is this safe to merge, and what did it actually buy - and the page
answers both before it explains anything.

## RUN

`$KEY`, `$LANG`, `$FRAMEWORK`, `$TITLE`, `$WDIR`, `$SENSE_ROOT`, `$BRANCH` are exported.
Work from `improvement-loop/`.

1. The window, in order, both council rounds included:

       cat "$WDIR/worklist.md" "$WDIR/proposal.md" "$WDIR/truth.md" "$WDIR/build.md" \
           "$WDIR/prove.md" "$WDIR/review.md"

1b. The gates, as the driver ran them:

       cat "$WDIR/gates.txt"

   Three rows: `make ci`, the touched-set 94% floor, and qlty. Every one is the driver's
   own run, not an agent's report, and the page states them as such. A row reading `UNRUN`
   is reported as UNRUN and never as a pass - a check that did not run is not evidence
   about the code, and a reader who assumes three greens where there were two is exactly
   who this page exists to protect.

2. The diff, as a reviewer will meet it:

       cd "$SENSE_ROOT" && git log --oneline main.."$BRANCH" && git diff --stat main.."$BRANCH"

3. The version the change would carry:

       cd "$SENSE_ROOT" && git-cliff --bumped-version

4. The tree is clean and everything is committed:

       cd "$SENSE_ROOT" && git status --short

   If anything is uncommitted, commit it on `$BRANCH` with a conventional subject before
   you write the page.

## DECIDE

There is one verdict, `HANDOFF`, and the judgment is what goes on the page.

**Open with one plain sentence stating the outcome.** Not a heading, not a table, not a
codename: "Sense now resolves ASP.NET controller actions and injected services in C#,
measured on two real repositories." Someone skimming for fifteen seconds gets that sentence
and nothing else, so it has to be the true one.

**State what is not covered, in the same voice.** Every lane has an edge: an idiom dropped
in `03-truth`, a row left undone in `04-build`, a probe that passed narrowly, a repo where
the language sits at a low symbol count. A page that implies completeness costs the next
person a day finding the hole; a page that names its own edges makes the next window cheap.

**The pull-request line is not decoration.** The PR body opens by naming where this came
from, and the page carries the line ready to paste:

    Found through the Sense Improvement Loop while preparing the <title> vertical.

That provenance is the loop's whole claim to exist: the difference between someone noticing
a missing case and a bench surfacing a stack the product could not read.

**No branding, no tool names, no people's names, no em-dashes**, on the page or in the
commit subjects you are about to hand over.

## ARTIFACT

Write `$WDIR/handoff.md`, six headings, the plain sentence FIRST and above them all:

    # What this is        what a reviewer is looking at: the branch, the commits, the diffstat
    # What it resolves    per worklist row, what Sense returns now that it did not, with the
                          probe that shows it
    # What it does not    dropped rows, undone rows, narrow passes, low corpus reach
    # How it was measured the corpus repos, the probe count, and the control repos that held
    # Before the PR       git-cliff --bumped-version, the gates row by row from
                          gates.txt with UNRUN stated as UNRUN, both council verdicts,
                          and the provenance line to paste into the PR body
    # If you would rather not merge it
                          what to delete and what is lost, in two lines

Then `$WDIR/handoff.verdict.json`:

    {
      "phase":    "handoff",
      "repo":     "<key>",
      "verdict":  "HANDOFF",
      "artifact": "product-window/<key>/handoff.md",
      "notes":    "one line: the branch name and the bumped version"
    }

## DONE WHEN

- The page opens with one plain sentence naming the outcome.
- Every worklist row appears under either `# What it resolves` or `# What it does not`.
- The branch name, the commit subjects, the diffstat and the bumped version are quoted.
- Every row of `gates.txt` is on the page, verbatim, and both council verdicts are named.
- `git status --short` is empty in `$SENSE_ROOT`.
- The provenance line is on the page, ready to paste.
- The verdict JSON exists and parses.

## DO NOT

- Do not push, do not open a pull request, do not delete or rename the branch. The human
  gate is the pull request and this page is what it reads.
- Do not claim coverage the probes did not show, and do not soften a dropped row into a
  future intention.
- Do not spawn a subagent. Every agent in this system is spawned by the driver.
