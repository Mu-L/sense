# Loop 3a - Eligibility

> Stage 1 of 4. Shared laws, the depth-first rule and the ledger namespace live in the parent
> ([`03-per-repo-convergence.md`](03-per-repo-convergence.md)) and are not repeated here.

## Goal

Measure, at $0, whether each admitted repo can arithmetically clear +0.50 - and rank the slate by that
measurement so the paid stages go deep on the strongest cell first. Exit state: every slate repo carries
a per-group control mean and a bound verdict, and the next repo to author is chosen from numbers, not
from order of admission.

## Product duties (per Sense surface)

- **status / index health.** Every probe runs against the pinned index, so this stage is the first place
  a stale or thin index shows itself. Re-index through `bench/drivers/rescan-all.sh` (or
  `bench/lib/ensure-index.sh`) before probing; a probe on a stale index measures a cell that does not
  exist.
- **No other surface is exercised here, by design.** The control arm has Sense forbidden - that is the
  point of the measurement. Sense-side reach is measured in [Run](03-c-run.md), never assumed here.

## Identity

- **Character:** checklist-convergence. Every step is a script with an exit code; there is no judgment in
  this stage and no event for a human to rule on.
- **Unit of work:** one control probe on one repo at that cell's real wall.
- **Position:** consumes Loop 2's admitted slot (pinned repo + index + gate measurements); produces the
  ranked, bound-legal slate that [Authoring](03-b-authoring.md) draws from.

## Actors

| Actor | Who/what | Notes |
|---|---|---|
| Generator | session agent driving `bench/drivers/vertical-loop.sh`, probes batched to cheap agents across the slate | the run-first ruling's rule 4 - batch the hunt |
| Evaluator | `bench/lib/control_bound.py` | arithmetic, not an opinion; no separate adversarial vertex needed |
| Mechanical verifier | `gold.score_gold_recall` - the same scorer the bench uses | a gate with its own scorer would be a second scorer, and two scorers is a bug factory |
| Human | none | a bound kill is arithmetic; there is nothing to rule on |

## The arithmetic bound

`delta = mean(sense) - mean(control)` and `cited_recall <= 1.0`, therefore **a +0.50 delta requires
`mean(control) <= 0.50`.** No term mentions language, framework, size or seam. A cell whose control probe
scores above 0.50 on *every* gold group cannot clear the bar even if Sense scores a perfect 1.00, and it
is killed before spend. `--yes` does not bypass it.

Four properties, each of which cost a mistake to learn. Do not "improve" them:

- **0.50, never tighter.** False-negative check against the banked wins: `> 0.50` kills 10 cells and
  **0 wins**; `> 0.25` kills 14 cells and **saleor**, a real +0.500 win. saleor proves the bound is
  tight, not merely safe: control 0.50 + sense 1.00 = exactly the bar.
- **Per group, killed only when EVERY group is dead.** `pergroup.py` flags a win on ANY group, and
  saleor's banked win is on its `context` group. Judging on one group re-creates the false negative.
- **Mean, over ≥2 probes.** The control's spread is real: one repo swung 0.444 → 0.889 across two
  unconstrained runs of the same arm and prompt. `min()` would pass a cell whose ceiling is +0.333.
- **At the cell's REAL wall.** The same repo scored ~0.00 at a 300s wall and 0.67 at 720s - same gold,
  same question. **The wall IS the control's score**, so a probe at the wrong wall measures a different
  cell.

**The bound is necessary, not sufficient**, and the wall is not a lever for passing it. Tightening the
wall lowers the control (good) and truncates Sense too (measured: sense floored at 0.00–0.33 on the
cells where that was tried). A cell needs BOTH `control <= 0.50` here AND Sense actually reaching
`base + 0.50` in [Run](03-c-run.md) - that second half is a measurement, never an assumption, because
Sense's own floor ranges 0.41–1.00 across a corpus.

## A probe verdict expires when Sense changes (decided 2026-07-30)

**A bound verdict is scoped to the Sense version that produced it.** The control arm has Sense forbidden,
but the *sense-side* half of the cell does not - and the whole point of Loop 7 is that Sense gets better
between verticals. A resolver fix, a new edge kind or a changed default can turn a cell that was
correctly killed into a live candidate. Reusing an old kill after that is citing a stale verdict.

So the Sense version is not decoration on the probe record, it is the **expiry key**:

- Every `loop3/<repo>/probe` entry carries the provenance line - Sense version, pinned repo commit, and
  **the wall used** (the wall IS the control's score, so a verdict without it cannot be re-read).
- When the current binary's version differs from a probe entry's, that verdict is **STALE, not false**.
  It may not rank the slate and may not be cited as a kill until it is re-probed.
- Loop 7 owns the trigger: shipping a product fix marks every probe verdict measured on the previous
  version as stale. A cell killed here is never permanently dead - it is dead *on that version*.

This is the stale-verdict protocol ([`../decision-errors.md`](../decision-errors.md)) mechanized for the
one stage where a $0 kill is cheap enough to be repeated, and therefore cheap enough that there is no
excuse for standing on an expired one.

## Stop conditions

- **Success:** every slate repo has ≥2 scored control probes at its real wall, `control_bound.py` exits 0
  on at least one repo, and `control_bound.py --slate <vertical-dir>` prints the queue. The order is
  **weakest control first** - the most headroom, so the highest-probability win opens first.

The stage in three commands:

```
python3 bench/lib/sense_build.py --stamp verticals/<v>/results/dryrun/<repo>/probe-1.md
python3 bench/lib/control_bound.py verticals/<v>/scenarios/<repo>.yaml <probe files>   # one cell
python3 bench/lib/control_bound.py --slate verticals/<v>                               # the queue
```
- **Budget:** $0 in dollars, but probes ride a subscription. A throttled or capped arm produces a *false*
  low control score, which is a false PASS - re-probe on a healthy arm rather than banking the number.
- **Failure:** every gold group's control mean > 0.50 → `control_bound.py` exits 1 and preflight refuses
  the bench. This is not a loss and not an event: route the cell to [Diagnosis](03-d-diagnosis.md)
  branch 1 (gold re-target) or branch 2 (re-shape the ask), which are both still $0 at this point.

## Human events

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| none | - | - | - |

## State / memory

- Probes: `verticals/<vertical>/results/dryrun/<repo>/probe-<n>.md` (override `CONTROL_PROBE_DIR`), each
  the control arm's deliverable from a walled dry-run: Sense forbidden, **code-capable** (the
  gate-fidelity rule - let the control read and write real code), at the cell's real wall.
- Memorization probe output alongside it (`bench/lib/memorization_probe.py`): a famous repo the model
  already knows inflates the control and hides the seam.
- Ledger: `loop3/<repo>/probe` - the bound verdict, the per-group control means, and a **required
  provenance line: Sense version, pinned repo commit, wall.** The Sense version is what expires the
  verdict; the wall is what makes it re-readable.

## Un-fakeable check

- `control_bound.py` exit code, computed from probe files on disk scored by `gold.score_gold_recall`.
  A verdict asserted without those files is prose. Cross-check with `bench/lib/dryrun_path_oracle.py`
  that the probe actually ran where it claims to have run.
- **Plus the version comparison:** the slate mode refuses to rank on any probe whose recorded Sense
  version is not the current binary's. A ranking that silently reuses an expired probe is the one way
  this stage can lie without anyone noticing.

## Inputs / outputs

- **Consumes:** the pinned slate (`verticals/<vertical>/repos.md`), the built index, the draft gold
  groups, the cell's real wall.
- **Produces:** probe files, per-group control means, a bound verdict per repo, and the ranked paid queue.

## Fixture test (standalone, $0)

Replay the frozen scored runs; the stage must rediscover what the humans found:

- Over 384 frozen scored runs, **10 of 20 paid cells on the bench model were already arithmetically
  dead** - two cells at control 1.00 (ceiling exactly 0.000), the rest 0.60–0.90. Each was killable by
  one comparison. The stage must kill all ten.
- It must **not** kill saleor (control 0.50, delta exactly +0.500 - a real win).
- Necessary-not-sufficient control: a cell at control 0.667 on its weakest group must be killed; the
  same cell re-walled to 0.250 must PASS and then still lose in Run, because the tightened wall
  truncated Sense too. If the stage claims the second case, it is over-claiming its own reach.

## Built vs missing

- **Built:** `control_bound.py` (wired 2026-07-16), `memorization_probe.py`, `dryrun_path_oracle.py`,
  the probe-directory plumbing in `vertical-loop.sh`, `rescan-all.sh` / `ensure-index.sh`.
- **Built 2026-07-30 with the split:** `sense_build.py` (the build identity and the `--stamp` sidecar),
  `loop3/<repo>/probe` in `ledger_check.py` with build + wall required on its Provenance line, and
  `control_bound.py --slate` (the ranked queue with expiry enforcement). Pinned by
  `test_sense_build.py`, `test_ledger_check.py`, and the `SlateRankTest` cases in
  `test_control_bound.py`.
- **Missing:** nothing writes the probe stamp automatically - the operator runs
  `sense_build.py --stamp` after each probe, and an unstamped probe is reported UNRANKABLE rather than
  silently ranked. Wiring it into the dry-run path is the obvious next step.
- **First live use:** the php-laravel slate, before any scenario authoring.
