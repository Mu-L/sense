#!/usr/bin/env python3
"""confirm_band.py - does this mini-bench cell need a SECOND pair before it routes?

THE DEFECT THIS CLOSES. Cycle 1 routes PROCEED/REQUESTION off ONE run per arm against a
hard 0.50 threshold on the `dependents` group. Measured on php-laravel/coolify, within-arm
spread on the SAME cell across valid runs is 0.077, 0.154, 0.154 and 0.250 - one to three
gold rows of twelve to eighteen. Attempts landing at 0.53, 0.54, 0.57 are therefore coin
flips against the bar, and a coin flip re-enters authoring as if it were a measurement.
Measured the other way too: `cd6a929f` read +0.538 at n=1 and +0.423 at n=2, so a single
run flattered a cell by 0.115 and would have carried it into a paid expansion.

WHAT IT DOES. Reads the cell's VALID runs, takes the baseline's mean `dependents` cited
recall, and says whether one more pair is worth it:

  exit 10  CONFIRM  - the baseline sits inside the band where noise can flip the verdict,
                      and only one valid baseline run exists. Run a second pair.
  exit 0   SETTLED  - either the baseline is clear of the band (no rerun buys anything) or
                      two or more valid baseline runs already exist.
  exit 1   ERROR    - the cell cannot be read; the caller must not route on that.

The band is deliberately narrow and deliberately NOT the whole range: a baseline at 0.85 is
not a coin flip and re-running it spends a pair to learn nothing. Defaults 0.35-0.65 bracket
the 0.50 bar by roughly the largest measured spread. Override with CONFIRM_LO/CONFIRM_HI.

Validity is `run_validity.classify_run`'s call, NEVER the stored `valid` field - see
`_valid` below. An invalid run is IGNORED entirely: averaging one into the mean is how a
dead half-pair turned `3637a637` into a phantom +0.528 (its single valid run reads 0.944).

Scoring: a mini-bench run is written unscored, so a missing `scored.json` is normal and this
script materialises it by calling `scorer.py` - the same call the read agent makes. It never
writes anything else and never edits an instrument.

  python3 confirm_band.py <cell_dir> <repo> <scenario_yaml> [group]
"""

import glob
import json
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import run_validity  # noqa: E402

GROUP_DEFAULT = "dependents"
LO = float(os.environ.get("CONFIRM_LO", "0.35"))
HI = float(os.environ.get("CONFIRM_HI", "0.65"))
LIB = os.path.dirname(os.path.abspath(__file__))


def _valid(run_dir):
    """A run counts only when it measured something - and run_validity OWNS that call.

    NEVER read `run_meta.json`'s stored `valid` field directly. It is written by whichever
    driver produced the run, so every run stamped before a classifier change carries a stale
    value: measured 2026-08-12, 17 baseline runs across coolify and bagisto sit on disk with
    `valid: null` and reclassify as VALID (16 `truncated_at_ceiling`, 1
    `never_reached_synthesis`) - delivered answers of 2,552 to 61,638 characters that a
    reader keying on the raw field throws away. `classify_run` derives the class from what
    the run LEFT BEHIND, which is why `pair_scan.py` delegates to it too.
    """
    meta_path = os.path.join(run_dir, "run_meta.json")
    scored_path = os.path.join(run_dir, "scored.json")
    try:
        with open(meta_path, encoding="utf-8") as fh:
            meta = json.load(fh)
    except (OSError, json.JSONDecodeError):
        return False
    scored = None
    if os.path.exists(scored_path):
        try:
            with open(scored_path, encoding="utf-8") as fh:
                scored = json.load(fh)
        except (OSError, json.JSONDecodeError):
            scored = None
    return bool(run_validity.classify_run(meta, scored, run_dir=run_dir).get("valid"))


def _recall(run_dir, yaml_path, group):
    """The run's cited recall for `group`, scoring it first if it has not been scored."""
    scored = os.path.join(run_dir, "scored.json")
    if not os.path.exists(scored):
        subprocess.run(
            [sys.executable, os.path.join(LIB, "scorer.py"), run_dir, yaml_path, "bench"],
            capture_output=True, text=True, check=False)
    try:
        with open(scored, encoding="utf-8") as fh:
            d = json.load(fh)
    except (OSError, json.JSONDecodeError):
        return None
    groups = (d.get("gold_recall") or {}).get("groups") or {}
    return (groups.get(group) or {}).get("cited_recall")


def arm_values(cell_dir, arm, repo, yaml_path, group):
    out = []
    for run_dir in sorted(glob.glob(os.path.join(cell_dir, arm, repo, "run-*"))):
        if not _valid(run_dir):
            continue
        v = _recall(run_dir, yaml_path, group)
        if v is not None:
            out.append(v)
    return out


def main(argv):
    if len(argv) < 4:
        print(__doc__.strip().splitlines()[-1], file=sys.stderr)
        return 1
    cell_dir, repo, yaml_path = argv[1], argv[2], argv[3]
    group = argv[4] if len(argv) > 4 else GROUP_DEFAULT
    if not os.path.isdir(cell_dir):
        print(f"confirm_band: no cell at {cell_dir}", file=sys.stderr)
        return 1

    base = arm_values(cell_dir, "baseline", repo, yaml_path, group)
    sense = arm_values(cell_dir, "sense", repo, yaml_path, group)
    if not base:
        print("confirm_band: no VALID baseline run to read - not routing on that", file=sys.stderr)
        return 1

    mean = sum(base) / len(base)
    smean = (sum(sense) / len(sense)) if sense else None
    shown = f"baseline {[round(v, 4) for v in base]} mean {mean:.4f}"
    if smean is not None:
        shown += f" | sense {[round(v, 4) for v in sense]} mean {smean:.4f} | delta {smean - mean:+.4f}"
    print(f"confirm_band [{group}] {shown}")

    if len(base) >= 2:
        print(f"confirm_band: SETTLED - {len(base)} valid baseline runs already on disk")
        return 0
    if LO <= mean <= HI:
        print(f"confirm_band: CONFIRM - baseline {mean:.4f} is inside the coin-flip band "
              f"[{LO}, {HI}] at n=1; one more pair before routing")
        return 10
    print(f"confirm_band: SETTLED - baseline {mean:.4f} is outside [{LO}, {HI}]; "
          f"a second pair cannot flip the verdict")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
