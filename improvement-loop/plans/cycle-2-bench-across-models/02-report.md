# PLAN 02-report

## TASK

The board is already rendered and every figure on it is fixed. Write the one section a
script cannot: what this result means, in plain words, at the top of a public page.

## SCOPE

You write the Reading section and nothing else. **Out of scope:** every other section of
the page, which is generated and which you may not edit; the numbers; the charts; the
verbatim task; re-running anything; the scenario, its gold, or any script; any other repo.
If the rendered board disagrees with the numbers JSON, write one line saying so and stop.

## RUN

`$VERTICAL`, `$VDIR`, `$REPO`, `$NUMBERS`, `$BOARD`, `$LOOPDIR` are exported. Work from
`improvement-loop/`.

1. The page as a reader will meet it, top to bottom:

       cat "$BOARD"

2. The numbers behind it. This is the ONLY place a figure may come from:

       python3 -m json.tool "$NUMBERS"

3. The replication count and the routing states, stated plainly:

       python3 -c "import json;d=json.load(open('$NUMBERS'));\
print(json.dumps(d['replication'], indent=2));\
[print(c['model'], c.get('routing'), 'reach', len(c.get('sense_only_reach') or [])) \
for c in d['columns']]"

## DECIDE

What a reader should take away, in three to six sentences. The recipe:

1. **Open with what Sense did**, in the terms the page is about: the answers models
   reached only with Sense. Name the number and the repository. This is the result.
2. **Say whether it held across models.** Use the replication count as it stands. If arms
   never called Sense, say that plainly as our routing gap, in our own words, and do not
   dress it as a model failing.
3. **Name the cost honestly**, in tokens and time, in whichever direction it went.
4. **Name the largest gap the page shows** in one sentence. Not an apology, not a
   roadmap: the reader can see it in the numbers and a page that skips it loses them.
5. **Stop.** No call to action, no roadmap, no comparison to another tool by name.

Voice: plain sentences, no jargon a reader outside this project would have to look up, no
hedging stacks ("appears to somewhat suggest"). Say the thing.

## ARTIFACT

Replace the line `<!-- reading -->` in `$BOARD` with your section. Change nothing else in
that file: not a heading, not a table, not a chart, not a word of the generated prose.

Write `$LOOPDIR/read.verdict.json` - named for the PHASE, which is how the driver looks
it up:

    {
      "phase":    "read",
      "repo":     "<repo>",
      "verdict":  "READING",
      "artifact": "verticals/<vertical>/results/cycle2/<repo>/board.md",
      "figures":  ["<every number you used, as it appears in the numbers JSON>"],
      "notes":    "<one line, or the blocked reason>"
    }

`phase`, `repo` and `artifact` are required by the driver's guard and `artifact` is
checked to EXIST, relative to `improvement-loop/`.

`figures` is how the next gate checks you: every number in your section must appear in it,
and every entry must appear in the numbers JSON.

## DONE WHEN

- `<!-- reading -->` no longer appears in `$BOARD` and your section stands in its place.
- `git diff` on `$BOARD` shows changes to that one section and nowhere else.
- Every figure in your section is listed in `figures` and exists in `$NUMBERS`.
- The section is at most six sentences.

## DO NOT

- Do not write a number you did not read out of `$NUMBERS`, and do not re-round one.
- Do not edit any other part of the page, or any script, or the scenario.
- Do not name a competing tool, promise a fix, or apologise for a gap. State it and move on.
