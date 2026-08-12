# ruby-rails - answer forms

What the ANSWER to a scored step is allowed to be, in THIS stack. Read by `01-author.md`
before choosing a kind of question. Every line is a measurement with its n.

Nothing here gates anything. It ORDERS what to try first.

## Form that WON, four times: the routine-attribution occurrence list

The kind is an occurrence list - "find everything that depends on X" - and it banks. What
makes it win here is not the list, it is what each entry must CARRY:

> for every dependent, name the routine the dependency is inside, give its file:line, and say
> in one phrase what that routine would start getting wrong. A file:line with no routine named
> around it does not count.

Plus a selector no regex expresses: *"the dependents that matter are not the ones whose job is
this record; they are the ones doing some unrelated job that reach for it once in the middle
of it, from a routine named after that other job."*

| repo | anchor | baseline `dependents` | sense | delta |
|---|---|---|---|---|
| mastodon | `Account` | 0.225 | 1.000 | **+0.775** |
| discourse | `Category` | 0.208 | 0.917 | **+0.708** |
| chatwoot | `Conversation` | 0.154 | 0.731 | **+0.577** |
| rails | - | 0.407 | 0.963 | **+0.556** |

All four sit at step 4 of a 7-step session, so both arms arrive with wall and context already
spent. The same kind asked COLD at step 2 of 2 produced plain-arm scores of 0.81, 0.16, 0.09,
1.00, 1.00 - the session is part of why it wins, and that is under test, not established.

## What is NOT the mechanism here

- **Token-darkness.** Every winning gold file names the anchor: mastodon 0 of 20 dark,
  discourse 0 of 12, chatwoot 2 of 13. The dependency is written on the line and the arm
  still cannot close the set, because `Account` appears in 387 app files and the 20 that
  count are chosen semantically.
- **Precision.** These three anchors are the most grep-CLEAN measured anywhere in the bench
  (0.42, 0.82, 0.97 by `seam_hunt`), and they are the biggest wins. Precision does not order
  outcomes.
- **Retention rings.** `ring_sweep.py` measured ZERO rings on both Ruby repos swept, top
  twelve anchors each (2026-08-03). The "what HOLDS X" kind - the strongest form in Go, +0.58
  to +1.00 on four cells - cannot fire in this stack. Do not propose it.

## Known measurement blind spots that bit here

- **Symbol-shaped citations.** An answer that writes `Admin::ActionLogsController#index :7`
  with no path scored zero until the symbol oracle landed (ruled 2026-08-04). Five rows on the
  banked mastodon cell.
- **Elided paths.** `…/friends_of_friends_source.rb:9` is blocked by the suffix anchor that
  otherwise stops `spec/x.rb` pinning `app/x.rb`. Still open.

## Data that does not exist for this vertical

Per-attempt results for the losing cycles were destroyed by the old `FORCE_WIPE` (the driver's
own note: "cycles 1-8 of the Account campaign are unrecoverable"). So the convergence of cycle
1 CANNOT be measured here, and the four wins cannot be separated from the human intervention
that produced them. php-laravel is the first vertical where that is measurable at all.
