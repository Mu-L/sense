# Loop 2 - Repo admission

> Admit repos on repo-level facts, compose the §7.0 split, pin, clone, index. Then stop. Nothing here
> predicts a win: a win is CRAFTED in Loop 3 against a scenario, not detected by a script before one
> exists.

## Goal

A slate of 4 repos, pinned, both arms cloned, indexed, composed per manifesto §7.0 with a same-type
backup per slot. **Done when `slate_check.py <vertical>` returns SLATE STANDS.**

## Product duties (per Sense surface)

- **conventions:** the slate sweep is this loop's - it already clones and indexes the admitted repos,
  so `sense conventions` costs nothing extra here. The ledger recording is Loop 5's.
- **blast / graph / search / status:** none. No anchor is chosen in this loop and no agent runs in it.

## Identity

- **Character:** mechanical and cheap. Four repo-level facts, a composition, a pin.
- **Unit of work:** one candidate repo taken through: screen → slot verdict → recorded in `repos.md`.
- **Position:** consumes Loop 1's scaffold and the vertical's declared pool; produces the slate Loop 3
  works one repo at a time. **Re-entered mid-vertical** when Loop 3 swaps a repo: the backup is
  screened the same way, never waved in.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | `bash bench/drivers/loop2-hunt.sh <vertical> --write` | pool → clone → screen → compose → pin → index → verify |
| Evaluator | `bench/lib/repo_screen.py` - the four screens per repo | facts about the repo, never guesses about a scenario |
| Mechanical verifier | `bench/lib/slate_check.py <vertical>` - the SET-level check | composes and verifies are separate scripts by design |
| Human | none (ruling 2026-07-29) - reads `repos.md` + the `loop2/slate` ledger entry after the fact | |

## The screens

Per candidate in `verticals/<v>/pool.txt`. All four are repo-level; none needs an anchor.

1. **In the vertical** - the repo declares this stack in its own dependency manifest. The pool file's
   say-so does not count; the manifest does. Declared as a `# stack: <manifest>:<needle>` directive at
   the top of `pool.txt`, with two properties both learned by running it:
   - **`|`-separated alternatives, matched as ANY.** A stack has more than one way to be declared: a
     Laravel APP requires `laravel/framework`, a Laravel PACKAGE requires `illuminate/*`. A single
     needle silently rejected filament and flarum - the framework slot's entire candidate list. The
     php-laravel marker is `composer.json:laravel/framework|illuminate/support|illuminate/contracts`.
   - **Monorepo-aware.** The manifest is looked for at the root and one or two levels down;
     filament's root `composer.json` declares nothing and every real requirement lives in
     `packages/*/composer.json`.
2. **Maintained** - not archived, pushed within **12 months** (`gh api repos/<owner>/<name>`:
   `archived`, `pushed_at`).
3. **Big enough** - prod source files on the clone. **< 1,000 = small = reject** (the small slot was
   removed 2026-07-20: a small corpus is readable inside the context wall, so it cannot produce a
   win). Size also assigns the class: **medium ≥ 1,000, big ≥ 4,000**. Calibrated on classified
   history - gin 57 and healthchecks 450 small; netbox 1053, october 1753, bookstack 1895, statamic
   2336 medium; filament 4942, snipe-it 7789, sentry 11645 big.
4. **Used** - **≥ 1,000 stars** (`stargazers_count`). A floor on "real code people depend on", and the
   tie-break when a slot has more qualified candidates than seats.

Two things that are recorded but never reject:

- **Anti-LLM banner** - a banner in the repo's docs flags a strip from **BOTH arms** (the lobsters
  rule). `provision-repos.sh` carries the same scan at clone time.
- **Memorization note** - `memorization_probe.py`'s closed-book recall for a symbol the model may
  recite (laravel `Model` 0.857, `Dispatcher` 1.0). Written into `repos.md` as a Loop 3 constraint:
  build gold on internals the model cannot recite. It does not reject a repo.

## Composition

- §7.0: **`1 framework + 1 big + 2 medium`** or **`2 big + 2 medium`**, never both. Framework role is
  the third field in `pool.txt`.
- **A same-type backup per slot**: the next qualified repo of that class, by stars. The framework slot
  is backed only by a framework; the big slot only by a big non-framework app.
- A repo is used once. Everything not slotted is recorded, not discarded.
- Two independent win pillars named in `repos.md` - a description of the slate, not a screen it passed.

## Stop conditions

- **Success:** 4 admitted per §7.0 + a backup per slot, SHAs pinned in `PINNED_COMMITS.json`, both
  arms cloned at those SHAs, the slate's 4 indexes built, per-repo screen numbers in `repos.md`,
  `slate_check.py` green.
- **Budget:** indexing wall-clock. Only the SLATE's 4 are indexed; the screens need a clone, not an
  index, and a backup is indexed when a swap actually promotes it (this loop is re-entered then).
- **Failure - the try-harder law:** a slot is NEVER filled with a row pitched as an "honest boundary"
  or ballast; there is no lane that framing is allowed in. A slot that will not fill means the **pool
  is too narrow** - widen `pool.txt` and screen again. It escalates only when every declared candidate
  has been screened with numbers recorded.
- **Class-5 protocol** (scope inflation, [`decision-errors.md`](../decision-errors.md)): a screen
  bounds exactly what it measured. Name the axis in the verdict - "rejected: 340 prod files, below the
  1,000 floor" is a size verdict and nothing else. A $0 screen never shrinks the program order: bank
  its saving, never its verdict.

## Human events

**None. This loop is autonomous** (ruling 2026-07-29). Nothing it decides costs a dollar or reaches a
reader, and everything it decides is a fact a re-run reproduces. The other three permanent anchors
(scenario/ground-truth integrity, spend, publish) are untouched.

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| - | - | - | - |

## State / memory

- `verticals/<key>/pool.txt` - the DECLARED pool, `repo-key|git-url|framework?` per line, `# stack:`
  directive on top. Discovery-by-API is deliberately not used: a pool the loop invents each session is
  a pool whose "exhausted" claim cannot be checked. A declared file is diffable and widening it is an
  edit someone can review.
- `repos.md` - candidates, per-repo screen numbers, slot map, backups, memorization notes.
- `verticals/<key>/PINNED_COMMITS.json`, `repos.txt`, `slate.json`; cloned arms + indexes under
  `$SENSE_BENCH_ROOT`.
- **Index-freshness rule (owned here):** if Loop 7 shipped a scan/resolve-layer change since the last
  index, ALL slate indexes rebuild (`sense scan -rebuild -embed`) before any authoritative sweep;
  query-layer changes do not. Index with `bench/lib/ensure-index.sh <repo> ...`, not `rescan-all.sh`:
  that driver scopes its list from `repos.txt`, which is empty until this loop writes it.
- **Readability duty:** append `loop2/slate` to `verticals/<vertical>/LEDGER.md` when the slate
  composes; per-repo numbers stay in `repos.md` (contract in [`00-ledger.md`](00-ledger.md)).

## Un-fakeable check

- Stack marker in the manifest, `pushed_at` + `archived` from the API, prod file count on the clone,
  star count from the API, banner scan hits, both arms at the pinned SHA, an index per slate repo.

## Inputs / outputs

- **Consumes:** Loop 1's scaffold; `verticals/<v>/pool.txt`.
- **Produces:** the pinned, provisioned, indexed slate of 4 + backups; per-repo screen records; the
  slot map with the two win pillars named.

## Fixture test (standalone, $0)

- **Known-answer control, both directions:** a repo under the size floor rejects on size and names the
  count; a repo over it admits with its class. A screen that only ever passes is not a screen.
- **Size calibration:** `prod_source_files()` must reproduce the size CLASS of all nine calibration
  repos (gin 57 … sentry 11645). Verified 9/9. The counts themselves drift from the historical
  index-based ones (sentry counts 5581 here); the class is what the screen uses, so the class is what
  must hold.
- **Archived control:** an archived repo rejects on the maintained screen even when it is large and
  popular; a repo pushed >365d ago rejects the same way; API unavailable gives UNRUN, never a pass.
- **Wrong-stack control:** a repo from another language rejects on the in-vertical screen.
- **Composition control:** `slate_check.py` must FAIL a set with a framework slot AND a second big
  (the §7.0 variants are exclusive), and FAIL a set of 3 naming both reasons.
- **Idempotence:** a second `loop2-hunt.sh <vertical> --write` re-composes the same slate and
  re-clones nothing.
- **Note, deliberate:** `repo_screen.py` counts prod files by walking the clone (no index exists yet at
  screen time) and `slate_check.py` recounts from the built index. Two counters for one fact is
  normally a defect; here the second is an independent verification of the first, and the classes must
  agree - they did, 4/4, on counts differing by up to 1.2%.

## Built vs missing

- **Built:** `loop2-hunt.sh`, `repo_screen.py`, `compose_slate.py`, `slate_check.py`,
  `provision-repos.sh` (pin + clone + banner scan), `ensure-index.sh`, the §7.0 composition law.
- **Retired 2026-07-31, do not rebuild:** the seven-bar admission gate - `admission_gate.py`,
  `gate_backtest.py`, `test_admission_gate.py`, and `anchor_rank.py` AS A GATE. Backtested against the
  four banked go wins it rejected 4 of 4, after running four verticals without producing a win
  (`LEDGER.md` `loop2/gate-backtest`; the general rule is trap 5 in
  [`../cross-cutting/01-methodology.md`](../cross-cutting/01-methodology.md)). Bar 6's size classing
  and bar 7's banner scan survived into `repo_screen.py`.
- **Moved to Loop 3, still built:** `anchor_rank.py` (a listing for whoever writes the scenario - its
  docstring records the three rankings that all buried the biggest banked win), `seam_hunt.py`,
  `structural_surplus.py`, `resolve_oracle.py`, `memorization_probe.py`.
