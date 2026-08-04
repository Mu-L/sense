# STOPPER: the gold matcher is path-only, and it cost the winning arm 5 rows

**Status:** RULED and CLOSED (Luc, 2026-08-04): *"count class name plus line number."*
Implemented as the symbol oracle in `gold.py`; the re-score diff is at the bottom of this
page. The line is no longer halted.
**Source:** reading the sense 15/20 transcript on the banked mastodon cell
(`sha256:27dfc6000a5e98f3`, `claude-opus-5`), the third item of the mastodon harvest.

## What the pickup page expected, and what is actually there

Expected: *"Five rows Sense found in one run and missed in the next... our own tool failing
with the evidence on disk."* It is not that. Sense returned all five in run-2, and the agent
named all five in its answer. **The scorer did not recognise the form the agent wrote them
in.**

Sense returned them, in server-to-client tool results, twice (`sense_blast` id=6 and
`sense_graph` id=7); run-1 got them once (`sense_blast` id=7). Then the run-2 answer:

| gold row | what the answer says |
|---|---|
| `d:admin-audit-scope` | `` `Admin::ActionLogsController#index` `:7` `` |
| `d:favourited-by-accounts` | `` `Api::V1::Statuses::FavouritedByAccountsController#default_accounts` `:21` `` |
| `d:reblogged-by-accounts` | `` `…RebloggedByAccountsController#default_accounts` `:21` `` |
| `d:space-usage-avatar-sum` | `` `Admin::Metrics::Dimension::SpaceUsageDimension#media_size` `:39` `` |
| `d:friends-of-friends` | `` `…/friends_of_friends_source.rb:9` `` |

All five scored `missed_mention` AND `missed_cite`.

## The mechanism, reproduced

`gold.score_gold_recall` matches on the gold row's `match:` value, which is a file **path**.
Re-scoring run-2 with the path-compaction oracle ON (9,844 repo files + full transcript, the
same inputs `score.sh` passes) reproduces 15/20 exactly, and shows why:

    d:admin-audit-scope       min_unique_suffix='admin/action_logs_controller.rb'            in_answer=False
    d:favourited-by-accounts  min_unique_suffix='statuses/favourited_by_accounts_controller.rb' in_answer=False
    d:reblogged-by-accounts   min_unique_suffix='statuses/reblogged_by_accounts_controller.rb' in_answer=False
    d:space-usage-avatar-sum  min_unique_suffix='dimension/space_usage_dimension.rb'         in_answer=False
    d:friends-of-friends      min_unique_suffix='account_suggestions/friends_of_friends_source.rb' in_answer=False

Two distinct blind spots:

1. **Four rows are symbol-shaped.** The answer gives a class, a method and a line, and no
   path at all. No path matcher can credit that.
2. **One row is an elided path.** The answer writes `…/friends_of_friends_source.rb:9`. The
   oracle needs a >=2-segment suffix, and the basename rule uses `_SUFFIX_ANCHOR =
   (?<![\w./\-])`, which a preceding `/` deliberately blocks - that anchor is what stops
   `spec/x.rb` pinning `app/x.rb`. The ellipsis is collateral of a rule that is otherwise
   right.

## The killer: does this cut both ways?

**No, and that is checked, not assumed.** Every baseline miss on this cell was probed for the
class-name form the sense arm used:

    baseline run-1/run-2, over its 17-19 missed rows:
      ActionLogs 0/0   FavouritedBy 0/0   RebloggedBy 0/0   SpaceUsage 0/0
      FriendsOfFriends 0/0   set_user 0/0   single_user_mode 0/0   SelfDestruct 0/0
      feed_manager 1/2  -> both hits are `clean_feed_manager` in `account.rb`, not
                           `app/lib/feed_manager.rb`. A true miss.

The baseline's misses are absences. The blind spot only ever fires on an answer that
compresses into symbols, and on these 10 opus-5 runs only run-2 did. Rails (n=3, both arms)
recovers nothing under the same probe.

## What it moves

| | as scored | if symbol+line counts as cited |
|---|---|---|
| sense dependents | 20/20, 15/20 | 20/20, 20/20 |
| baseline dependents | 4/20, 5/20 | unchanged |
| discriminator | **+0.65** | **+0.775** |

It suppressed the win; it cannot manufacture one. The cell stays banked either way.

## The ruling asked for

Is `Admin::ActionLogsController#index :7` a **citation**?

- **For crediting it:** the row is identified unambiguously, with a line; Rails class-to-path
  is deterministic; and Sense's own output is symbol-shaped (`symbol` + `file` + `ref`), so a
  path-only oracle systematically under-credits the arm that reads Sense.
- **Against:** the headline is `cited_recall`, and the whole point is a location an agent can
  jump to. A class name is not a path. Crediting it lowers the standard for both arms
  forever, on every future cell, to buy back one run.

A middle option exists: credit an elided path (`…/x.rb:9`) as cited - it names the file and
the line, and the ellipsis is the only thing in the way - while leaving symbol-only forms as
misses. That recovers 1 of the 5 and does not touch the citation standard.

## The ruling, and what shipped

Ruled option 1: **a class name plus a line is a citation.**

`gold._symbol_match` credits it, gated behind the same two oracle checks as the path
compaction (the gold pattern grounds to exactly one real repo file; that file appears in the
run's transcript) plus a third of its own: the short form counts only when exactly one repo
file derives that constant, otherwise the fully qualified constant is required, so a bare
`Source` can never earn a credit. The constant is derived from the PATH alone -
`app/controllers/admin/action_logs_controller.rb` -> `admin::actionlogscontroller` - never
from an index or a tool result, so scoring stays arm-blind and needs no inflection table.
The line pin may sit up to 30 chars past the class name (room for `#method` and backticks),
and `;`/newline are hard stops so a pinless class in a list cannot borrow the next row's line.

Tests: `test_gold.py::SymbolOracleTest` and `::PathToConstantTest`, written from the verbatim
answer forms above. Suite green, 429 passed.

## Re-score diff: every scored run in the vertical

27 scored runs re-scored under the new matcher, **4 moved**:

| run | cited | note |
|---|---|---|
| opus-5 `27dfc…` sense mastodon run-2 | 33 -> 38 | the banked cell, the five rows above |
| opus-5 minibench `f5335…` sense mastodon run-1 | 24 -> 27 | not verdict-bearing |
| opus-5 validation `f576e…` sense mastodon run-4 | 19 -> 20 | not verdict-bearing |
| **opus-4.8** validation `f576e…` **baseline** mastodon run-1 | 6 -> 8 | historical, and it moves the BASELINE |

That last row is the arm-blindness check passing on real data: the rule credits whoever
writes symbols, and on that run it was the baseline. Rails (n=3, both arms) does not move at
all.

Only the banked cell was re-scored ON DISK (`score.sh` at its own versioned root, same
scenario version, backup taken first). The other three live at older scenario versions where
`score.sh` would apply the CURRENT question to them - the known hazard - so their `scored.json`
is left as-is deliberately.

## What it did to the cell

    dependents  baseline 4/20, 5/20   sense 20/20, 20/20   +0.78   WIN   (was +0.65)
    write-path  baseline 2/4,  2/4    sense 4/4,   4/4     +0.50   WIN
    overall     baseline 0.53         sense 1.00           +0.47

**The n=3 run is no longer owed.** It was called for by the two-runs-disagree rule, and the
disagreement was the matcher: the sense arm scored 20/20 twice. Two runs per arm, in
agreement, satisfies the standing law.
