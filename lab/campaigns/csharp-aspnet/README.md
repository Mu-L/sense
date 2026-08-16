# csharp-aspnet

The arm set as of `547f1bf` (*bench(arms): name the mistral arm in
provider/model form, template included*), **over the one repository that
campaign had reached**, expressed as config.

Two different as-ofs, on purpose, and worth saying plainly: the arms are frozen
at the commit, the repository list is frozen at what had actually run.
`repos.txt` at that commit names four repositories; the campaign had reached
one.

Frozen at that commit on purpose. A mid-flight campaign as a moving target
turns the plan into a bookkeeping exercise, and the point of this file is to
show that the config plans to the cells the campaign actually had.

At that commit the campaign had reached **one repository**, bitwarden-server,
and had banked the headline arm there: baseline and sense on `claude-opus-5`.
The four confirmation arms were declared and had not run.

## The judge is not an arm

`claude-opus-4-7` grades the rubric axes. It produces no cell and it is a
service the scoring layer calls, so it is a pinned field of the campaign rather
than a line in the arm list. It only lived in the arm list before because that
file existed, and leaving it there would mean every consumer skipping it by
convention, forever.

## Adding an arm

One line here, and a model file if the model is new. Nothing else — no dispatch
to edit, no prefix matching to extend. That is the whole point of the catalog,
and it is the thing that failed when an arm's id was rewritten by a runner into
something that did not exist.
