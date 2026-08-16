# The checkpoint, held

The count trigger fired: **253 of 430 group-scores differ (59%)**, well past the
"about a third" that says stop and look at the shape rather than work the queue.

The checkpoint is not an abort. Cycle 02 finishes. What follows is the three
questions answered in writing, with the distribution that justifies each answer.

## 1. Is it one cause or many?

**One.** Of the rows the old scorer credited and the new one does not, the split is:

| rows | share | what it is |
|---|---|---|
| 544 | 94% | the file is cited, at a line gold does not name |
| 33 | 6% | a bare basename with a line, which the matcher refuses on purpose |
| 1 | 0% | the file named with no line anywhere |

The 6% is the same mechanism wearing a different coat. `"OrganizationService.cs:36"`
against gold at `:835` is a citation to the right file at the wrong line, written
short. Hand-read, all six sampled were that. So the class is one, at essentially
100%: **the old scorer credited the right file at the wrong line.**

That is not a difference to reconcile. It is the thing this metric exists to
refuse — the right file at the wrong line is a teammate sent to the wrong place —
and it is why the recomputed numbers are roughly half the recorded ones.

So: a bug to name, not a queue to grind. The work continues.

## 2. Is per-row still the right bar, or is per-class enough?

**Per-class, and the classes were named before the table was read.** The closed
set is in `rescore.Cause`, and every one of the 253 differences lands inside it:

| rows | cause |
|---|---|
| 190 | old defect: the right file at a line gold does not name |
| 61 | gold change |
| 2 | old defect: a row the answer never located |
| **0** | **unexplained** |

Per-row on 253 rows sharing one mechanism would be an expensive way to write the
same sentence 253 times. Per-class is not a lowered standard here, because
nothing lands outside the classes and the classes were fixed in advance.

The one row-level obligation was kept: the differences were hand-audited before
the class was named, not after, and that audit is what found the new defect below.

## 3. Does parity on the banked cells suffice?

It did not have to be decided, because **zero rows are unexplained** — the pitch's
closing condition — and the accounting covers all 430 comparable group-scores
rather than a chosen subset. Narrowing to the 4 banked cells would have discarded
the finding, since the file-level class is visible only across the corpus.

## What the hand-audit found: one new defect, fixed

A `new defect` blocks the cycle until fixed, and there was one.

The winning question in this bench asks the arm to *name the routine the
dependency is inside, give its file:line*. The answer to it is one item written in
two tokens:

```
`ImportScripts::Base#create_category` — `script/import_scripts/base.rb:465`
```

The matcher read that as two citations. The symbol carried no line, so it could
never match, and the path was held to the gold line. On one run alone that cost 18
rows, every one a case where the arm named the right routine and pinned its
definition line while gold cites a use inside it.

Both rows checked by hand were hits **by the gold rows' own words**:
`d:import-scripts-base` says the location is "inside
`ImportScripts::Base#create_category`", and the answer named exactly that routine.

Fixed, corroborated the same way an inherited file is — the symbol has to name
that path under Ruby's own rule, so an unrelated constant beside an unrelated path
stays two citations. Worth **+219 rows** across the corpus, and it moved the
checked-in discourse figure from 2 of 12 to 4 of 12. That published number was
wrong, and this is the correction.

## The direction prediction, and what happened to it

Written down before the comparison: the symbol-oracle fix only ever ADDED credit,
so a pre-fix run must rescore **higher, never lower**.

**No run rescored higher. Not one, out of 430.** So no difference could be labelled
`scorer version`, and none was — the classifier tests direction first, precisely so
the comfortable label cannot be reached for.

That is the prediction failing in the honest direction. It was not wrong about the
fix; it was wrong about which effect would dominate. Both effects are present in
the 10 pre-fix runs, and the file-level one is much larger, so it buries the
symbol-oracle gain underneath it. A prediction that can be wrong is worth
something only if you say so when it is.

## The gold-change prediction, and what happened to it

02-05 handed over 33 rails runs as the runs its quarantine would move. Every one
of the 61 `gold change` rows is rails, and rails carries no other cause. The
prediction was met exactly.
