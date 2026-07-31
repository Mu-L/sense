# The vertical ledger - LEDGER.md and STATUS.md (readability contract)

> **What this is.** Phase 1 of the Readability section of `brainstorm.md` ("a human can
> jump in at any moment"), implemented 2026-07-14. It is severable from the rest of that document: it
> demotes no gate, touches no anchor, and changes no spend path. Cross-cutting and loop-agnostic, so
> Loops 4/5/6 inherit it without redesign. On any conflict, the one-pagers and the manifesto win.

## The write-only law (what makes this unbreakable)

`LEDGER.md` and `STATUS.md` are **write-only from the loops' point of view**. No loop, script, or gate
ever reads them to decide anything; position is always recomputed from disk (scaffold files, `repos.md`,
`.loop-state.json`, the results tree), per Loop 4's rule that state is "never trusted from a status
file." These files exist for humans and for fresh-session context loading, full stop.

## The files

| File | What | Discipline |
|---|---|---|
| `verticals/<vertical>/LEDGER.md` | append-only narrative: what happened, why, alternatives, lessons, scores, cost | written by the loop at every transition (table below); entries keyed, never edited after the fact |
| `verticals/<vertical>/STATUS.md` | auto-rendered position: the matrix, where we are, the literal next step | regenerated wholesale by `render-status.sh`; never hand-edited |

`LEDGER.md` is **never committed** (the journal is the private half of the public-by-default
boundary: candid self-directed reasoning, named losses, cost numbers). `STATUS.md` is a render and
is public.

## Entry schema

Every entry, exactly this shape (the checker enforces the fields):

```markdown
## 2026-07-14 | loop2/dolt/event-c | Paid cell approved
- **What:** plain sentences; a stranger understands without opening another file.
- **Why:** the reason, not the mechanism.
- **Alternatives:** considered and rejected, with why. "none tried: <reason>" is legal and
  self-indicting (that is the moment premature closure catches itself).
- **Lesson:** what the next session should know (or "none"). A real lesson ends with its exit
  tag: `Exit: check(<name>)` | `Exit: rule(<one-pager>)` | `Exit: fixture(<path>)` |
  `Exit: parked(<reason>)`. A lesson that exits as prose alone is a repeat waiting to happen
  (the auto-improvement exit law; the checker flags it).
- **Provenance:** REQUIRED on the per-repo verdict entries (`loop2/<repo>/run-<n>`,
  `loop3/<repo>/swap|close`), the
  Class-1 stale-verdict mechanization: sense version, pinned repo SHA, scenario file + date.
  Makes "has anything changed since this verdict" answerable by diff. Other entries omit it.
- **Scores:** before → after, INCLUDING side-effect benches (or "n/a: <why>").
- **Cost:** BOTH currencies, always (schema v3). API dollars as the run artifacts show, AND the
  subscription side: session time, paid arm sessions, and the **fleet as a spawn count by type**
  (`spawn_cost.py`). "$0 API" alone is not a cost record: the weekly reset is the binding
  constraint, and every council seat, probe, and evaluator is free in dollars and expensive in
  quota. "not captured" stays honest for the dollar side; the fleet count is never "not captured",
  it is on disk.
- **Links:** artifacts: run dirs, PRs, pitch files, transcripts.
```

- The heading is `## <date> | <key> | <title>`. Keys make re-runs idempotent: bootstrap and Loops 5/6 may re-run;
  a keyed entry that already exists is skipped, never duplicated.
- **The key contract is enforced forward-only (the owner, 2026-07-15).** Entries dated on or after
  **2026-07-16** must use a key from the write-point table below; `ledger_check.py` rule 9 flags the
  rest. The entries recorded before that date keep their drifted keys (`scenario/`, `overnight/`,
  `step33/`, `loop7/31-13-tail`) and are never rewritten: forward-only is the standing ledger
  strategy, and rewriting recorded entries is a bigger edit than a schema migration. The cutoff is a
  date, not a grandfather list, so the exemption cannot quietly grow.
- Schema v2 (2026-07-14): the Lesson exit tag and the Loop 3 Provenance field were added; the
  pre-existing entries were migrated to conform (a schema migration, noted here, is the one
  sanctioned edit to an existing entry).
- Schema v3 (2026-07-15): the Cost field records BOTH currencies and names the fleet; the
  pre-existing entries were migrated with measured spawn counts.
- **Cost honesty:** three tiers, and the difference matters. (1) Run-level billed tokens exist in
  the results tree: record them. (2) **Spawn counts are free from disk** and always recordable:
  Claude Code's own transcripts carry every `Agent` invocation with its `subagent_type`, so the
  fleet is a measurement, not an estimate (`spawn_cost.py`). Main-session effective tokens come
  from the same place. (3) Per-subagent token spend is **not on disk at all**: subagent turns carry
  no usage record (every record in the store has `isSidechain=false`; measured 2026-07-15). So the
  fleet is counted, never priced, and a spawn count is a cost SIGNAL, not a cost. Estimate session
  time, never invent a token number, and never let "$0 API" stand alone as the answer.

## Write points (the transition list)

Entries follow the natural grain of the stage that writes them, not every heartbeat:

| Owner | Key pattern | Fires at |
|---|---|---|
| bootstrap | `bootstrap/scaffold` | scaffold stamped |
| bootstrap | `bootstrap/slate` | slate composed (the admission sign-off packet; per-candidate numbers stay in `repos.md`, the entry links to it) |
| Loop 1 (authoring) | `loop1/<repo>/{scenario,event-b}` | scenario stamped (with its version hash); gold sign-off |
| Loop 2 (run) | `loop2/<repo>/{event-c,run-<n>}` | spend approval (wall + arm plan); each run verdict WITH the diagnosis branch |
| Loop 3 (diagnosis) | `loop3/<repo>/{swap,close}` | swap dossier; repo closed |
| Loop 4 | `loop4/<arm>/{done,parked}` | per arm (completion or park), matching the per-arm status-format law; never per cell |
| Loop 5 | `loop5/<repo-or-tier>` | each harvest tier done |
| Loop 6 | `loop6/event-e` | vertical close |
| Loop 7 | `loop7/<window>/{open,<fix>-shipped,<fix>-reverted,close}` | window open (worklist); each fix resolution; window close |
| - | `ruling/<slug>` | any out-of-loop consequential decision (a owner ruling, an honesty incident, a park) |
| - | `stopper/<slug>` | **a measurement-instrument bug (rule 10).** Fires the moment it is FOUND, not when it is fixed. MUST carry the re-score blast radius ("N of M runs", `bench/lib/rescore_diff.py`) and what it retro-invalidates. `ledger_check.py` rule 10 blocks a quiet scorer change. |

**Class-6 protocol - read this schema, then RUN `ledger_check.py`, before claiming a session is
recorded** (an available check left unrun, [`decision-errors.md`](../decision-errors.md); ratified
2026-07-15). **"It's written" and "it's recorded" are different claims**, and only the checker settles
the second. Writing entries from a mental model of what a ledger entry looks like is the error: it
produced a malformed key, four invented field names, a codename-law trip, and a Cost field failing rule 8
- eight findings, in a folder where the checker was three directories away and free (2026-07-15, caught
only because the owner asked "is everything recorded?"). The same protocol's sibling clauses: an instrument
whose output will become evidence runs against a **known-answer control first** (a PATH-stripped loop
once printed the exact REVERSE of a finding, 0-fake/31-real against a true 30/31), and **recorded harness
lessons are pre-flight, not trivia** (macOS has no `timeout`; subshell/background PATH lacks
`/opt/homebrew/bin`). This is the program's own motto turned inward - *no un-fakeable check, no
automation*: here the checks existed and the discipline to run them did not.

## Codename law

First mention in an entry always expands: the identifier, then what it means, then a link. A bare gap or
pitch identifier with no expansion is the failure this law names. Mechanical form (heuristic, enforced by
the checker): the first line mentioning a gap or family identifier inside an entry must carry a
parenthetical or comma expansion on that same line.

## Tooling (all $0, all local to this folder)

- **Render:** `bash bench/lib/render-status.sh <vertical-key>` (the doc dir is `docs/<vertical-key>/`).
  Regenerates `STATUS.md` from disk: `report-matrix.sh` for the matrix,
  `repos.md` for the slate, and - only if the vertical's doc dir has one - a per-vertical
  `next-steps.md` for the unchecked steps (the go campaign's frozen copy lives outside this
  folder; fresh verticals run on STATUS + LEDGER alone). Degrades gracefully when a
  source is missing.
- **Measure the fleet:** `python3 spawn_cost.py [--since YYYY-MM-DD] [--until YYYY-MM-DD]` (defaults
  to the last 7 days). Reads the session transcripts and prints, per day, `Agent` spawns by
  subagent_type plus main-session effective tokens by model. This is what fills the Cost field's
  subscription side. Read-only over recorded artifacts, same station as `transcript_miss.py`. Its
  honest gap is printed with every run: the token figure excludes every spawn beside it.
- **Check:** `python3 bench/lib/ledger_check.py <vertical-key>`. Advisory, runs at the publish sign-off
  alongside the closing review; blocks nothing mid-flight. Verifies: every entry has the required
  fields; no duplicate keys; every results cell (`results/<model>/<arm>/<repo>`, including `_invalid-*`
  groups - recorded history is history) is referenced by at least one entry; codename scan passes;
  the Lesson exit tag and Loop 3 Provenance field (v2); that no Cost field answers in dollars
  alone (v3); and the key contract on entries dated from 2026-07-16 (rule 9).

## Honest gaps (stated, not hidden)

- `.doc` is gitignored, so append-only cannot be enforced by git history. The checker verifies coverage
  and shape; immutability stays discipline.
- The codename scan is a heuristic (first-mention-has-expansion); it catches the common case, not prose
  cleverness.

## Phase 2 (RATIFIED severable 2026-07-16; SHIPPED)

The token-reduction payoff, **redesigned 2026-07-15** after measuring the files it stands on. The
original plan (a fresh session reads `LEDGER.md` + `STATUS.md` first; `next-steps.md` shrinks to
build-order only) does not survive its own arithmetic, and shipping it as written would make pickup
*more* expensive by the vertical's close:

- **The saving is in the read ORDER, not the file size.** Pickup costs ~27k tokens because
  `next-steps.md` (106,640 bytes) is read every time, not because it is long. Not reading it at
  pickup captures the whole lever; shrinking it adds nothing.
- **`LEDGER.md` is append-only and unbounded.** 23,832 bytes over 8 entries is ~2,979 bytes/entry.
  A full vertical (4 repos of Loop 3 entries + Loops 2/4/5/6/7 + rulings, call it 36) projects to
  ~142KB, **larger than `next-steps.md` is today**. A LEDGER-first pickup decays to nothing, then
  goes negative.
- **`STATUS.md` is the only bounded file** (4,713 bytes) because `render-status.sh` regenerates it
  wholesale instead of appending. Bounded is the property a pickup file needs.
- **The shrink contradicts the forward-only seed.** `seed/2026-07-14` declined to backfill steps
  1-28 because "the history already lives in `next-steps.md` with its annotations". Shrink that
  file and the history has no home. Dropping the shrink resolves it, with no backfill owed.

**Corrected Phase 2:** `STATUS.md` is the session-pickup file; `LEDGER.md` is read on demand (the
why, when a human or a session needs it); `next-steps.md` keeps BOTH its history and its build
order and is opened on demand too. One line in `how-to-run.md`, reversible in one line.

> **Ratified 2026-07-16 (the owner: "ratify phase 2 alone, it's severable"): SHIPPED.** The pickup line
> in `how-to-run.md` now names `STATUS.md` (re-rendered at pickup); `LEDGER.md` and
> `next-steps.md` are opened on demand. Severed from the brainstorm's ratification pass on the same
> ground as Phase 1: it demotes no gate, touches no anchor, changes no spend path. Condition 1
> (coverage) was NOT met at ratification (2 repos of 6 with real narrative): by the ruling it
> converts from an entry gate to a maturing measure that normal operation closes; the write points
> keep firing at every transition either way, and Loops 4/5/6 write their first entries when the
> vertical reaches them. Reversal is the same one line. LEDGER: `ruling/readability-phase2`.

**Entry conditions as they stood at ratification** (recorded, not rewritten):

1. **Coverage: NOT met.** 2 repos of 6 carry real narrative (dolt, pebble); gitea, go-mysql-server,
   gin and miniflux have 0-3 incidental mentions, and Loops 4/5/6 have never written an entry. The
   gate is "after the Go vertical proves these files in practice".
2. **Key drift: CLOSED FORWARD 2026-07-15 (the owner ruled forward-only), residue noted.** 5 of the
   first 8 keys sat outside the write-point table (`scenario/`, `overnight/`, `step33/`, plus
   `loop7/31-13-tail`), and the drift was not cosmetic: `VERDICT_KEY` matched ZERO entries, so
   schema v2's Provenance rule had never once fired (four entries volunteer the field, which is
   exactly why nobody noticed). Rule 9 now enforces the contract from 2026-07-16 and the legacy
   entries stand. Residue for Phase 2: the pre-cutoff entries remain non-indexable by key, so a
   pickup that maps entries to loop positions covers the vertical from 2026-07-16 forward, not its
   first two days. That is acceptable at 8 entries of a projected ~48; it would not have been if
   the ruling had come later.
3. **`STATUS.md` reliability: MET 2026-07-15.** Its unchecked-step render showed overtaken steps as
   open work (steps 1 and 2, annotated OBE since 2026-07-13) and truncated step 24 mid-sentence.
   Fixed in `render-status.sh`: overtaken steps are labelled, spanning headings are cut cleanly.
4. **The shrink contradiction: resolved** by dropping the shrink.

Tracked in `brainstorm.md` (Readability section, implementation state).
