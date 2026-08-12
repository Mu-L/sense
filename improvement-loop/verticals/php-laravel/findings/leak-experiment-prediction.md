# Experiment: does an unscored task leaking the scored question lift the baseline?

Written BEFORE the run. Scenario `sha256:ac8a8d6a59b720a5` (coolify, 7 tasks, 25 gold rows).

ONE variable changed against `sha256:7e71ec7ad6b1c084`: task 3 "Trace the write path" no
longer asks for the LINK-vs-COPY classification of each seeded record, nor for BOTH ends of
each deferred hand-off located at file:line. Tasks 1, 2, 4, 5, 6, 7 are byte-identical and
the gold is byte-identical (13 `dependents` rows).

## Controls, all four arms clean (exit 0, no watchdog, valid: true), same 13 gold rows

| cell | shape | baseline | sense | delta |
|---|---|---|---|---|
| `cd6a929f53916573` | 2-task probe | 0.154 | 0.692 | +0.538 |
| `7e71ec7ad6b1c084` | 7 tasks, leak present | 0.308 | 0.692 | +0.385 |
| `adba790068ed0412` | 7 tasks, leak present | 0.308 | 0.615 | +0.308 |

## Prediction

- baseline `dependents` cited_recall **<= 0.20** (back to the 2-task probe's 0.154)
- delta **>= +0.50**

## Falsifier

- baseline **>= 0.30**, i.e. unmoved from the leaked 7-task cells. Then task-3 leakage is NOT
  what lifts the baseline in the long session, and the "no unscored task may ask for the
  scored set" recommendation is dead.

## Standing

n=1 per arm. This is a probe with the same standing as a mini-bench: it may kill the
hypothesis or support it. It may NOT settle a win (EVERY ARM GETS 2 RUNS PER CELL).
Both arms are checked for wall, exit code, watchdog and API share before any number is read.

---

# RESULT: the prediction FAILED

Run 2026-08-12, scenario `sha256:ac8a8d6a59b720a5`, both arms x1, unscored root.

| arm | wall/ceiling | exit | watchdog | valid | `dependents` cited |
|---|---|---|---|---|---|
| sense | 477/480 (**99.4%**; first attempt hit the cap at 481/480 and was parked) | 0 | none | true | **0.3846** |
| baseline | 518/573 | 0 | none | true | **0.2308** |

**delta +0.154.** Predicted >= +0.50. FAILED.

- The baseline moved in the predicted direction, 0.308 -> 0.231, but by 0.077. There is NO
  measurement of baseline run-to-run variance anywhere in this vertical (no cell has two
  VALID baseline runs), and the one valid same-arm sense pair on this repo differs by 0.250
  (`cfe0c23a`: 0.917 and 0.667). A 0.077 move is not readable against that.
- Sense fell much further, 0.692 -> 0.385, and that is what killed the delta.

## What the three session lengths say, on the same 13 gold rows

| shape | sense wall | sense cited | baseline cited | delta |
|---|---|---|---|---|
| 2 tasks (`cd6a929f`) | 289s of 480 | 0.692 | 0.154 | +0.538 |
| 7 tasks, leak present (`7e71ec7a`, `adba7900`) | 466s, 432s | 0.692, 0.615 | 0.308, 0.308 | +0.385, +0.308 |
| 7 tasks, leak removed (`ac8a8d6a`) | 477s (cap hit once at 481s) | 0.385 | 0.231 | +0.154 |

Sense's recall on the scored group falls monotonically as the session lengthens, and it falls
FASTER than the baseline's. Removing the leak did not reverse it.

## Conclusion

The leak is not the mechanism, or not the dominant one. The dominant one is that the
seven-task session does not fit the sense arm's 480s ceiling on this repo: the cap was hit on
this scenario's first attempt (481/480) and on `e733f4d2fda1787b` (481/480) before it, and the
run that did finish used 99.4% of the ceiling. The scored task is last-but-three in a session
the arm cannot afford, so it gets what is left.

`CANNOT-FINISH-AT-BUDGET IS A RESULT`. The loop already owns the correct lever for this
(`out_of_clock` -> `requeue_expand`: "the ceiling is never raised; the SCENARIO is what
gives"). It did not fire here only because the retry landed 3 seconds inside the cap.

---

# SECOND RUNS on the two probe cells (2026-08-12)

Both cells re-run as FULL PAIRS, not baseline-only: `sense_run_lookup` keys the matched budget
on the run INDEX and `SENSE_RUNS` is in-memory, so a baseline-only invocation finds no paired
sense run and is SKIPPED. A second baseline needs its own second sense run or its wall is not
derived from a paired one. Timeout law verified applied on both: 298s -> 358s, 319s -> 383s.

| cell | arm | run | wall/ceiling | exit | watchdog | valid | `dependents` |
|---|---|---|---|---|---|---|---|
| cd6a929f | sense | 1 | 289/480 | 0 | - | true | 0.6923 |
| cd6a929f | sense | 2 | 298/480 | 0 | - | true | 0.6154 |
| cd6a929f | baseline | 1 | 292/347 | 0 | - | true | 0.1538 |
| cd6a929f | baseline | 2 | 349/358 | 0 | - | true | 0.3077 |
| 55ee5f19 | sense | 1 | 328/480 | 0 | - | true | 1.0000 |
| 55ee5f19 | sense | 2 | 319/480 | 0 | - | true | 0.8462 |
| 55ee5f19 | baseline | 1 | 357/394 | 0 | - | true | 0.3077 |
| 55ee5f19 | baseline | 2 | 384/383 | 124 | hard_cap_timeout | **false** | VOID |

## Verdicts

- **`cd6a929f` FAILS the bar at n=2.** baseline mean 0.2308, sense mean 0.6539, **delta +0.423**.
  Its n=1 reading was +0.538. The cell is not a win; the single run flattered it by 0.115.
- **`55ee5f19` still passes at +0.615** (baseline 0.3077 n=1, sense mean 0.9231), but its
  baseline is n=1 and the second attempt VOIDED. Not settled.

## The void, and a question it raises

`55ee5f19` baseline run-2 spent **98.6% of its wall inside the provider** (376.9s API of 382.2s,
45 turns) and was cut off having written **42 chars**. `run_validity.classify` returns
`no_output_hang` because 42 < MIN_ANSWER_CHARS 200, so it is void and cannot be scored 0.0.

That collides with `ONE SENSE RETRY...`: *"A baseline that runs out of `sense_wall x 1.2` is the
measurement, not a malfunction - it is the win condition itself."* Here a baseline exhausted a
correctly-derived budget doing real work and produced nothing, which is what that law calls the
win condition, and the classifier voids it. Both readings cannot be right. NOT resolved here -
this is a human ruling, and until it is made `55ee5f19` cannot be closed either way.

## Noise floor, now measured (was: unmeasured)

Within-arm spread on the same cell, coolify, valid runs only: 0.077, 0.154, 0.154, 0.250
(n=4 pairs) - roughly 1 to 3 rows of 12-13. Any effect smaller than ~0.20 on this repo is not
readable at n=2.
