#!/usr/bin/env python3
"""Which runs the driver may treat as a measurement.

The load-bearing assertion is the watchdog one: an arm that runs out of clock is a
RESULT, and for a baseline it is the win condition. A "skip the failures" rule written
as `rc != 0` passes every other test here and silently re-runs the thing the bench
exists to measure.
"""
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import pair_scan  # noqa: E402

VERSION = "sha256:deadbeef"


def _run(root, arm, repo, name, meta_extra, scored=None):
    d = os.path.join(root, arm, repo, name)
    os.makedirs(d)
    meta = {"tool": arm, "scenario_version": VERSION}
    meta.update(meta_extra)
    with open(os.path.join(d, "run_meta.json"), "w") as fh:
        json.dump(meta, fh)
    with open(os.path.join(d, "transcript.json"), "w") as fh:
        fh.write("{}\n")
    if scored is not None:
        with open(os.path.join(d, "scored.json"), "w") as fh:
            json.dump(scored, fh)
    return d


def test_healthy_run_counts(tmp_path):
    root = str(tmp_path)
    _run(root, "sense", "coolify", "run-1",
         {"valid": True, "claude_exit_code": 0})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "sense")


def test_parked_run_does_not_count(tmp_path):
    """A run already declared unfit is off-board by its directory name alone."""
    root = str(tmp_path)
    _run(root, "baseline", "coolify", "failed-run-1",
         {"valid": True, "claude_exit_code": 0})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "baseline") == ""


def test_maintainer_parked_underscore_does_not_count(tmp_path):
    root = str(tmp_path)
    _run(root, "baseline", "coolify", "_pre-harness-fix-coolify",
         {"valid": True, "claude_exit_code": 0})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "baseline") == ""


def test_harness_crash_does_not_count(tmp_path):
    """The harness decided this outcome, so it measured nothing."""
    root = str(tmp_path)
    _run(root, "baseline", "coolify", "run-1",
         {"valid": False, "void_reason": "harness_crash", "claude_exit_code": 1})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "baseline") == ""


def test_watchdog_with_an_answer_still_counts(tmp_path):
    """OUT OF CLOCK IS A RESULT - the regression guard for `rc != 0`.

    rc 124 with a real answer is `truncated_at_ceiling`: the arm failed the exam and
    the exam counts. For a baseline this is the WIN CONDITION; dropping it here would
    make the driver re-run it and delete the measurement.
    """
    root = str(tmp_path)
    _run(root, "baseline", "coolify", "run-1",
         {"claude_exit_code": 124, "answer_chars": 38649, "output_tokens": 9000})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "baseline")


def test_watchdog_without_output_does_not_count(tmp_path):
    """rc 124 with ZERO output is a hang, not an arm result."""
    root = str(tmp_path)
    _run(root, "baseline", "coolify", "run-1",
         {"claude_exit_code": 124, "answer_chars": 0, "output_tokens": 0})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "baseline") == ""


def test_never_reached_synthesis_counts(tmp_path):
    """Tokens burned, no answer: a real 0.0, not an artifact."""
    root = str(tmp_path)
    _run(root, "baseline", "coolify", "run-1",
         {"claude_exit_code": 124, "answer_chars": 12, "output_tokens": 8000})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "baseline")


def test_other_scenario_version_does_not_count(tmp_path):
    root = str(tmp_path)
    _run(root, "sense", "coolify", "run-1",
         {"valid": True, "claude_exit_code": 0, "scenario_version": "sha256:other"})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "sense") == ""


def test_parked_void_beside_a_healthy_retry_finds_the_retry(tmp_path):
    """The shape this fix exists for: park the void, re-run, read the retry."""
    root = str(tmp_path)
    _run(root, "baseline", "coolify", "failed-run-1",
         {"valid": False, "void_reason": "harness_crash", "claude_exit_code": 1})
    good = _run(root, "baseline", "coolify", "run-2",
                {"valid": True, "claude_exit_code": 0})
    assert pair_scan.measuring_run(root, "coolify", VERSION, "baseline") == \
        os.path.join(good, "transcript.json")
