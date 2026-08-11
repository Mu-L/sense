#!/usr/bin/env python3
"""pair_scan.py -- "is there a run of THIS question, on THIS arm, that MEASURED it?"

The driver's own answer to that question used to be an inline walker in
vertical-loop.sh that counted any directory holding a run_meta.json at the right
scenario_version. It counted two things it must not:

  * a PARKED run (`failed-run-*`, `_*`, `dryruns-*`) - a run already declared unfit,
  * a VOID run (`valid: false`) - a harness artifact, not a measurement.

Both let a phase advance on a pair that is not a pair. Measured 2026-08-10:
php-laravel/coolify's baseline died on a provider api_error at 447s of its 550s wall
with zero answer text; the pair read as complete, and the phase spawned a judge on it.

`valid` is NOT "the arm did well" - run_validity.py owns that distinction and this
module delegates to it rather than restating it. An arm that runs out of clock is
VALID (`truncated_at_ceiling`, `never_reached_synthesis`): a censored answer is the
arm's result and, for a baseline, it is the win condition. Re-deriving the rule here
as `rc != 0` would silently re-run every timed-out baseline, which is why there is one
classifier and every caller asks it.
"""
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import run_validity  # noqa: E402


def _load(path):
    try:
        with open(path) as fh:
            return json.load(fh)
    except (OSError, ValueError):
        return None


def measuring_run(root, repo, want_version, arm=""):
    """First transcript under `root` for `repo` at `want_version` that measured `arm`.

    Returns the transcript path, or "" when no such run exists. "No run" is an
    ANSWER, not an error: the caller reads the absence and re-runs the phase.
    """
    for dirpath, _, filenames in os.walk(root):
        if "run_meta.json" not in filenames or "transcript.json" not in filenames:
            continue
        if repo not in dirpath.split(os.sep):
            continue
        # Parked before parsed: a parked run's meta is still well-formed, so the
        # directory name is the only thing that marks it off-board.
        if run_validity.is_parked(os.path.relpath(dirpath, root)):
            continue
        meta = _load(os.path.join(dirpath, "run_meta.json"))
        if meta is None:
            continue
        if meta.get("scenario_version") != want_version:
            continue
        # `tool` is the run's own record of which arm produced it - authoritative
        # where the directory name is only a convention.
        if arm and meta.get("tool") != arm:
            continue
        scored = _load(os.path.join(dirpath, "scored.json")) or {}
        if not run_validity.classify_run(meta, scored).get("valid"):
            continue
        return os.path.join(dirpath, "transcript.json")
    return ""


def main(argv):
    if len(argv) < 4:
        print("usage: pair_scan.py <root> <repo> <scenario_version> [arm]",
              file=sys.stderr)
        return 2
    root, repo, want = argv[1], argv[2], argv[3]
    arm = argv[4] if len(argv) > 4 else ""
    found = measuring_run(root, repo, want, arm)
    if found:
        print(found)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
