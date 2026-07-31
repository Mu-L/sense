# Bootstrap a vertical

Everything the per-repo loops need, in one command: the scaffold stamped, the repo pool hunted, four repos
screened, pinned, cloned on both arms and indexed. **No human and no model in the path.** It is not a
loop - it converges on the first pass or stops with a named status.

## Requirements

Three things, all checked before anything is created:

1. **The installed `sense` is the latest release.** The indexes it builds are the bench's data; a
   stale binary silently indexes with old rules. Checked by `sense-version-check.sh`.
2. **A next vertical in `verticals.txt`** - one `key|lang|framework|title` per line, in order. The
   pipeline takes the **first key whose `verticals/<key>/` does not exist**. That is the whole
   selection rule: you decide the order by editing the file, the script decides nothing.
3. **`stacks/<key>.conf` for that key** - the manifest marker for the in-vertical screen, the `gh`
   hunt queries, and the framework-role repos. Format and the measurements behind each rule:
   [`../../stacks/README.md`](../../stacks/README.md); validate with
   `python3 bench/bootstrap/stack_profile_check.py <key>`.

Those two files are the only hand-written inputs, and neither is generated on purpose: the queries
are the search a later "the pool is exhausted" claim rests on, and the framework roles are a judgment
about the ecosystem. Everything else is derived.

Also needed in the environment: this repo, python3, `gh` authenticated, and a clone root
(`SENSE_CLONES`).

## Run it

    bash bench/bootstrap/run.sh                  # next in the queue
    bash bench/bootstrap/run.sh ruby-rails       # a named vertical
    bash bench/bootstrap/run.sh --re-hunt        # re-run the repo hunt

Progress goes to **stderr**, the result JSON is the only thing on **stdout**, and every exit path
returns one - so `result=$(run.sh)` is the whole integration for an orchestrator, CI or
a headless agent.

| status | exit | stage | means |
|---|---|---|---|
| `READY-FOR-LOOP3` | 0 | done | the JSON names the slate to read |
| `SENSE-STALE` | 69 | preflight | installed binary is not the latest release |
| `NO-QUEUE` | 66 | select | `verticals.txt` missing |
| `QUEUE-EXHAUSTED` | 3 | select | every queued key already has a directory |
| `NOT-QUEUED` | 66 | select | an explicit key that is not in the queue |
| `NO-STACK-PROFILE` | 66 | prerequisite | `stacks/<key>.conf` missing or invalid |
| `EXTRACTOR-NOT-READY` | 65 | scaffold | no `internal/extract/<lang>` - see Stop below |
| `PREFLIGHT-FAILED` | 70 | hunt | `gh` missing or not authenticated |
| `HUNT-FAILED` | 71 | hunt | every declared query failed |
| `SLATE-INCOMPLETE` | non-zero | admit | no §7.0 slate; widen the hunt and `--re-hunt` |

## The stages

    0 preflight     the installed sense must BE the latest release
    1 select        verticals.txt, first key with no directory
    2 prerequisite  stacks/<key>.conf exists and is well-formed
    3 scaffold      stamp verticals/<key>/, evaluate it, record        scaffold.sh
    4 hunt          run the declared queries -> pool.txt               hunt.py
    5 admit         screen, compose, pin, clone, index, verify         admit.sh
    -> READY-FOR-LOOP3

**Scaffold.** `stamp.sh` writes `verticals/<key>/`: `repos.txt`, `scenarios/`,
`PINNED_COMMITS.json`, `arms.txt`, a README tracker, a `repos.md` slate, an empty `findings/`. Then
`scaffold_check.py --strict` evaluates it as a separate pass, never the stamping step's self-report,
on four file-level facts: every template element present, zero dangling path references, zero stale
previous-stack references, extractor exists with production files. `scaffold_ledger.py` appends
`bootstrap/scaffold` to the vertical's `LEDGER.md` from the facts just measured. No repos are touched
here.

**Hunt.** `hunt.py` runs the queries from `stacks/<key>.conf` and writes `pool.txt`, one
`key|url|framework?|stars|pushed` per line. The repos the conf declares framework-role are **always
seeded**: a declared framework is a candidate by declaration, not by search luck - a 138-candidate
ruby hunt did not contain `rails/rails`, which is not topic-tagged `rails` and loses a keyword search
to its own ecosystem.

**Admit.** Four repo-level screens, none of which needs an anchor:

1. **In the vertical** - the repo declares this stack in its own dependency manifest; the pool file's
   say-so does not count. A **missing** manifest rejects, a **matching** one passes, and a
   **present-but-non-matching** one is UNDECIDED and falls through to the clone, never a reject,
   because a monorepo declares nothing at its root. A repo the conf declares **framework-role passes
   by declaration**: a framework does not depend on itself.
2. **Maintained** - not archived, pushed within 12 months.
3. **Big enough** - prod source files on the clone. **< 1,000 = small = reject** (a small corpus is
   readable inside the context wall, so it cannot produce a win). Size assigns the class: medium ≥
   1,000, big ≥ 4,000.
4. **Used** - ≥ 1,000 stars, and the tie-break when a slot has more qualified candidates than seats.

Phase 1 screens over the API with nothing downloaded; survivors are cloned on both arms and screened
again on the clone. Two things are recorded but never reject: the **anti-LLM banner** (flags a strip
from BOTH arms, the lobsters rule) and the **memorization note** (build gold on internals the model
cannot recite).

`compose.py` then fills §7.0: **`1 framework + 1 big + 2 medium`** or **`2 big + 2 medium`**,
never both, with a **same-type backup per slot** by stars - the framework slot backed only by a
framework, so the framework can never be the campaign's sole win. A repo is used once; everything not
slotted is recorded, not discarded. SHAs are written from the clones, the slate's 4 are indexed
(`ensure-index.sh`, fingerprint-gated), and `slate_check.py` checks the SET.

Nothing here predicts a win. A win is CRAFTED in Loop 1 against a scenario, not detected by a script
before one exists.

## Stop

- **Done:** `READY-FOR-LOOP3`. 4 repos admitted with a backup each, SHAs pinned, both arms at those
  SHAs, indexes built, per-repo numbers in `repos.md`.
- **Extractor not ready:** a **sequencing decision, not a bootstrap task**. The lane parks until
  language support lands, or the gap becomes a Loop 7 work item first. Verticals are parallel lanes,
  so only paid cells block on it. **Bootstrap never builds product code.**
- **A slot will not fill - the try-harder law:** never fill it with a row pitched as an "honest
  boundary" or ballast. It means the **hunt is too narrow**. Add an axis to `stacks/<key>.conf` and
  `--re-hunt`. Measured: a name-and-topic hunt produced exactly ONE big non-framework app across 28
  php candidates, and adding `--size` found three more.
- **A screen bounds exactly what it measured** (Class-5, [`../decision-errors.md`](../decision-errors.md)):
  "rejected: 340 prod files, below the 1,000 floor" is a size verdict and nothing else. A $0 screen
  never shrinks the program order - bank its saving, never its verdict.

## Re-entry

Every stage is idempotent. Re-running on a prepared vertical re-screens and re-verifies without
re-stamping or re-cloning, and leaves exactly one `bootstrap/scaffold` LEDGER entry. `--re-hunt` is the
one destructive flag: it re-runs the queries and overwrites `pool.txt`, which is refused without it
so a hand-curated pool is never silently discarded.

**Index-freshness rule (owned here):** if Loop 7 shipped a scan/resolve-layer change since the last
index, ALL slate indexes rebuild before any authoritative sweep; query-layer changes do not. Index
with `ensure-index.sh <repo> ...`, not `rescan-all.sh`, which scopes its list from `repos.txt`.

## Un-fakeable check

Exit codes and JSON on every path. Stack marker in the manifest, `pushed_at` and `archived` from the
API, prod file count on the clone, star count, banner scan hits, both arms at the pinned SHA, an
index per slate repo.

## Fixture test (standalone, $0)

- **Selection, both directions:** with every queued key stamped it returns `QUEUE-EXHAUSTED`; with
  one unstamped it names that key. An explicit key outside the queue returns `NOT-QUEUED`.
- **Preconditions fire before anything is created:** queue a key with no `stacks/<key>.conf` and it
  stops at `prerequisite` having stamped nothing. Point `SENSE_BIN` at a path that does not exist and
  it stops at `preflight`.
- **Every exit path returns JSON.** A pipeline that fails silently cannot be orchestrated.
- **The scaffold evaluator has teeth:** `scaffold_check.py <stamped-key> --lang <lang>` exits 0,
  `not-a-vertical` exits 1. Citing a path that does not exist FAILS; planting a previous stack's key
  in a stamped `repos.md` FAILS under `--stale <key> --strict`.
- **The screens have teeth:** a repo under the size floor rejects naming the count; an archived repo
  rejects even when large and popular; a repo from another language rejects on the in-vertical
  screen; a monorepo whose root manifest declares nothing must reach the clone, not be rejected in
  phase 1. API unavailable gives UNRUN, never a pass. `slate_check.py` must FAIL a set with a
  framework slot AND a second big, and FAIL a set of 3 naming both reasons.
- **Size calibration:** `prod_source_files()` reproduces the size CLASS of all nine calibration
  repos - verified 9/9 (gin 57, healthchecks 450 small; netbox 1053, october 1753, bookstack 1895,
  statamic 2336 medium; filament 4942, snipe-it 7789, sentry 11645 big).
- **Two counters, deliberate:** `screen.py` counts prod files on the clone and `slate_check.py`
  recounts from the built index. The second independently verifies the first, and the classes must
  agree - they did, 4/4, within 1.2%.

## Built vs missing

- **Built:** everything under `bench/bootstrap/` - `run.sh` (the entry point), `sense-version-check.sh`,
  `stack_profile_check.py`, `scaffold.sh`, `stamp.sh`, `scaffold_check.py`, `scaffold_ledger.py`,
  `hunt.py`, `admit.sh`, `screen.py`, `compose.py`, `slate_check.py`, `provision.sh` - plus
  `bench/lib/ensure-index.sh`, which is shared with the loops.
- **Retired, do not rebuild:** the seven-bar seam gate - `admission_gate.py`, `gate_backtest.py`,
  `anchor_rank.py` AS A GATE. Backtested against the four banked go wins it rejected 4 of 4, after
  running four verticals without producing a win (`LEDGER.md` `bootstrap/gate-backtest`; the general rule
  is trap 5 in [`../cross-cutting/01-methodology.md`](../cross-cutting/01-methodology.md)).
- **Moved to Loop 1, still built:** `anchor_rank.py`, `seam_hunt.py`, `structural_surplus.py`,
  `resolve_oracle.py`, `memorization_probe.py`.
- **Missing:** the extractor check asks whether `internal/extract/<lang>/` exists with production
  files, not whether it covers this stack's dispatch idioms. Those differ.
- **Missing:** nothing verifies the binary that BUILT an existing index. `ensure-index.sh` records
  `sense_version` and `git_head` per repo and never reads them back; the version gate covers the
  entry point only.
- **Missing:** the arm plan is prose in `verticals/<key>/README.md` with no mechanical check; the
  memorization note is not yet written into `repos.md` by any script.
