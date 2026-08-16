# The pre-fix list

Written down BEFORE the comparison, so `scorer version` is a prediction that can be
wrong rather than a story fitted to the numbers afterwards.

The symbol-oracle fix is commit `b90e7f2`, 2026-08-04 13:00:59 +0000:
*bench(ruby-rails): count a class name plus a line as a citation*. It only ever ADDED
credit, so a pre-fix run must rescore HIGHER, never lower. One that rescores lower is
not a scorer-version difference; it is a new defect wearing the wrong label.

Of 122 scored runs, **10 are pre-fix** and 112 follow it.

| when | repo | model | arm | run | dated by |
|---|---|---|---|---|---|
| 2026-08-01 10:38:23 | rails | claude-opus-5 | baseline | run-1 | run_meta.timestamp |
| 2026-08-01 10:41:49 | rails | claude-opus-5 | baseline | run-2 | run_meta.timestamp |
| 2026-08-01 10:45:10 | rails | claude-opus-5 | sense | run-1 | run_meta.timestamp |
| 2026-08-01 10:49:09 | rails | claude-opus-5 | sense | run-2 | run_meta.timestamp |
| 2026-08-04 10:16:30 | rails | claude-opus-5 | sense | run-3 | run_meta.timestamp |
| 2026-08-04 10:22:13 | rails | claude-opus-5 | baseline | run-3 | run_meta.timestamp |
| 2026-08-04 11:13:57 | mastodon | claude-opus-5 | sense | run-1 | run_meta.timestamp |
| 2026-08-04 11:21:24 | mastodon | claude-opus-5 | baseline | run-1 | run_meta.timestamp |
| 2026-08-04 11:29:30 | mastodon | claude-opus-5 | sense | run-2 | run_meta.timestamp |
| 2026-08-04 11:38:01 | mastodon | claude-opus-5 | baseline | run-2 | run_meta.timestamp |

## How each run was dated

The results tree is not in git, so the date is taken from the artifacts themselves:

- `run_meta.json`'s own `timestamp` where it has one (42 runs).
- otherwise the `sense_ref` it records, dated by git (47 runs). Every one of those
  refs is 2026-08-06 or later, which is the cross-model re-run.
- a baseline run with neither carries no clock of its own, because it uses no Sense
  build; it takes the date of the sense run it was paired against in the same cell
  (33 runs). The arms of a cell run together, so this is a fact about the cell
  rather than an estimate.

The boundary is not close. The last pre-fix run is 2026-08-04 11:38:01 and the first
post-fix run is 2026-08-04 21:44:24, ten hours after the fix landed, so nothing sits
near enough to the line for the dating method to decide it.
