#!/usr/bin/env python3
"""Behaviour pins for the banked-cell index.

The load-bearing one is `test_agrees_with_pergroup`: banked.py re-reads the same
scored.json that pergroup.py decides on, so the danger is not that it crashes but
that it quietly computes a DIFFERENT number and a reader believes it. That test
runs the real pergroup.py as a subprocess over the same fixture and asserts the
per-group deltas and the verdict match, which is what stops the two derivations
drifting apart.
"""
import json
import os
import re
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import banked

LIB = os.path.dirname(os.path.abspath(__file__))


def write_run(root, arm, repo, groups, run=1, cited=None, failed=False, meta=None):
    d = os.path.join(root, arm, repo, "run-%d" % run)
    os.makedirs(d, exist_ok=True)
    total_c = sum(c for c, _ in groups.values())
    total_t = sum(t for _, t in groups.values())
    payload = {
        "failed": failed,
        "gold_recall": {
            "cited_recall": cited if cited is not None else (total_c / total_t if total_t else 0.0),
            "groups": {n: {"cited": c, "total": t} for n, (c, t) in groups.items()},
        },
        "metrics": {"token_total_billed": 1000},
    }
    with open(os.path.join(d, "scored.json"), "w") as fh:
        json.dump(payload, fh)
    with open(os.path.join(d, "run_meta.json"), "w") as fh:
        json.dump(meta or {"model": "claude-opus-5",
                           "scenario_version": "sha256:1def723310067e48"}, fh)


def _discourse_shape(root):
    """The real banked discourse cell: dependents 4/12,1/12 -> 10/12,12/12."""
    write_run(root, "baseline", "discourse", {"dependents": (4, 12), "guards": (2, 3)}, run=1)
    write_run(root, "baseline", "discourse", {"dependents": (1, 12), "guards": (1, 3)}, run=2)
    write_run(root, "sense", "discourse", {"dependents": (10, 12), "guards": (3, 3)}, run=1)
    write_run(root, "sense", "discourse", {"dependents": (12, 12), "guards": (3, 3)}, run=2)


def test_the_banked_win_shape(tmp_path):
    root = str(tmp_path)
    _discourse_shape(root)
    row = banked.build_row(root, "discourse")
    # dependents: (4+1)/24 = 0.2083 -> (10+12)/24 = 0.9167, delta +0.7083
    assert row["groups"]["dependents"]["delta"] == 0.7083
    assert row["groups"]["guards"]["delta"] == 0.5
    assert row["verdict"] == "WIN"
    assert row["verdict_source"] == "rebuilt"
    assert row["runs"] == {"baseline": 2, "sense": 2}


def test_a_cell_under_the_floor_is_not_a_win(tmp_path):
    root = str(tmp_path)
    write_run(root, "baseline", "r", {"dependents": (8, 12)})
    write_run(root, "sense", "r", {"dependents": (11, 12)})
    row = banked.build_row(root, "r")
    assert row["best_group_delta"] == 0.25
    assert row["verdict"] == "NOT-YET"


def test_the_phase_verdict_wins_over_the_rule(tmp_path):
    """A verdict passed in is recorded as given: this index never second-guesses."""
    root = str(tmp_path)
    write_run(root, "baseline", "r", {"dependents": (8, 12)})
    write_run(root, "sense", "r", {"dependents": (11, 12)})
    row = banked.build_row(root, "r", verdict="WIN")
    assert (row["verdict"], row["verdict_source"]) == ("WIN", "driver")


def test_a_failed_run_is_skipped_not_scored_as_zero(tmp_path):
    root = str(tmp_path)
    write_run(root, "baseline", "r", {"dependents": (4, 12)}, run=1)
    write_run(root, "sense", "r", {"dependents": (12, 12)}, run=1)
    write_run(root, "sense", "r", {"dependents": (0, 12)}, run=2, failed=True)
    row = banked.build_row(root, "r")
    assert row["runs"]["sense"] == 1
    assert row["groups"]["dependents"]["sense_mean"] == 1.0


def test_an_arm_with_no_runs_banks_nothing(tmp_path):
    root = str(tmp_path)
    write_run(root, "sense", "r", {"dependents": (12, 12)})
    assert banked.build_row(root, "r") is None


def test_recording_the_same_cell_replaces_its_row(tmp_path):
    root = str(tmp_path / "cell")
    out = str(tmp_path / "banked.jsonl")
    _discourse_shape(root)
    row = banked.build_row(root, "discourse")
    banked.upsert(out, row)
    banked.upsert(out, row)
    rows = [json.loads(x) for x in open(out) if x.strip()]
    assert len(rows) == 1
    other = dict(row, repo="mastodon")
    banked.upsert(out, other)
    assert len({json.loads(x)["repo"] for x in open(out) if x.strip()}) == 2


def test_rebuild_skips_unscored_and_doubled_roots(tmp_path):
    """validation/ and minibench/ are unscored by construction; the doubled dirs
    the path bug left behind are not cells either."""
    vert = tmp_path / "v"
    results = vert / "results" / "claude-opus-5"
    for bad in ["validation/1def723310067e48", "minibench/1def723310067e48",
                "validation/1def723310067e48/validation/1def723310067e48"]:
        _discourse_shape(str(results / bad))
    _discourse_shape(str(results / "1def723310067e48"))
    roots = list(banked.paid_roots(str(vert)))
    assert [os.path.basename(r) for r in roots] == ["1def723310067e48"]
    assert "validation" not in roots[0] and "minibench" not in roots[0]


def _pergroup(root, repo):
    env = dict(os.environ, RESULTS_DIR=root)
    out = subprocess.run([sys.executable, os.path.join(LIB, "pergroup.py"), repo, "0.50"],
                         capture_output=True, text=True, env=env)
    return out.stdout


def test_agrees_with_pergroup(tmp_path):
    """The killer: same fixture, same numbers, same verdict as the real instrument."""
    root = str(tmp_path)
    _discourse_shape(root)
    printed = _pergroup(root, "discourse")
    row = banked.build_row(root, "discourse")

    for name, group in row["groups"].items():
        line = next(x for x in printed.splitlines() if x.startswith(name))
        printed_delta = float(re.search(r"([+-]\d+\.\d+)", line).group(1))
        assert abs(printed_delta - group["delta"]) < 0.005, (name, line, group)

    overall = re.search(r"mean ([\d.]+)\s*\|\s*sense .* mean ([\d.]+)\s+delta ([+-][\d.]+)",
                        printed)
    assert overall, printed
    assert abs(float(overall.group(3)) - row["overall"]["delta"]) < 0.005
    assert ("VERDICT: WIN" in printed) == (banked.verdict_for(row) == "WIN")
