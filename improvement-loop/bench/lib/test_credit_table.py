#!/usr/bin/env python3
"""Behaviour pins for the per-gold-item credit table (the diagnosis read; branches in .claude/agents/bench-evaluator.md).

Two pins carry the weight.

`test_fingerprint_ignores_the_diluter_bucket`: the fingerprint is the movement
detector behind the six-cycle swap rule, so it must not move when gold churns in
and out of `both`. A detector that counts diluter churn as progress would reset
the counter forever and the swap would never fire.

`test_a_validation_run_is_invisible`: validation runs sit one level deeper in the
results tree on purpose, so no scorer had to learn to skip them. If this ever
fails, an unscored x1 run is being averaged into a verdict.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from credit_table import build, classify, fingerprint, load_runs


def _write(root, arm, repo, run, details):
    d = os.path.join(root, arm, repo, run)
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "scored.json"), "w", encoding="utf-8") as fh:
        json.dump({"gold_recall": {"details": details}}, fh)


def _row(gid, cited, group="deps"):
    return {"id": gid, "group": group, "mentioned": True, "cited": cited}


class ClassifyTest(unittest.TestCase):
    def test_the_four_buckets(self):
        runs = {"baseline": 2, "sense": 2}
        cases = {
            "both": {"baseline": 2, "sense": 2},
            "sense-only": {"baseline": 0, "sense": 1},
            "baseline-only": {"baseline": 1, "sense": 0},
            "neither": {"baseline": 0, "sense": 0},
        }
        for want, row in cases.items():
            self.assertEqual(classify(dict(row, group="deps"), runs), want)

    def test_one_run_out_of_two_still_counts_as_reached(self):
        """The struggle read asks reachability; pergroup.py owns the averaging."""
        runs = {"baseline": 2, "sense": 2}
        self.assertEqual(classify({"baseline": 1, "sense": 2, "group": "d"}, runs), "both")


class BuildTest(unittest.TestCase):
    def test_counts_citations_per_arm_across_runs(self):
        with tempfile.TemporaryDirectory() as t:
            _write(t, "baseline", "r", "run-1", [_row("a", True), _row("b", False)])
            _write(t, "baseline", "r", "run-2", [_row("a", True), _row("b", False)])
            _write(t, "sense", "r", "run-1", [_row("a", True), _row("b", True)])
            table, runs = build(t, "r")
            self.assertEqual(runs, {"baseline": 2, "sense": 1})
            self.assertEqual(table["a"], {"group": "deps", "baseline": 2, "sense": 1})
            self.assertEqual(table["b"], {"group": "deps", "baseline": 0, "sense": 1})

    def test_a_validation_run_is_invisible(self):
        """Validation cells live under <root>/validation/<arm>/<repo>/, one level
        deeper, so the glob never reaches them."""
        with tempfile.TemporaryDirectory() as t:
            _write(t, "baseline", "r", "run-1", [_row("a", True)])
            _write(os.path.join(t, "validation"), "baseline", "r", "run-1",
                   [_row("a", False), _row("ghost", False)])
            table, runs = build(t, "r")
            self.assertEqual(runs["baseline"], 1)
            self.assertNotIn("ghost", table)

    def test_legacy_bare_scored_json_is_read(self):
        with tempfile.TemporaryDirectory() as t:
            d = os.path.join(t, "sense", "r")
            os.makedirs(d)
            with open(os.path.join(d, "scored.json"), "w", encoding="utf-8") as fh:
                json.dump({"gold_recall": {"details": [_row("a", True)]}}, fh)
            self.assertEqual(len(load_runs(t, "sense", "r")), 1)

    def test_a_run_with_no_gold_recall_is_skipped_not_crashed(self):
        with tempfile.TemporaryDirectory() as t:
            d = os.path.join(t, "sense", "r", "run-1")
            os.makedirs(d)
            with open(os.path.join(d, "scored.json"), "w", encoding="utf-8") as fh:
                json.dump({"error": "transcript.json not found"}, fh)
            self.assertEqual(load_runs(t, "sense", "r"), [])


class FingerprintTest(unittest.TestCase):
    RUNS = {"baseline": 1, "sense": 1}

    def _fp(self, table):
        return fingerprint(table, self.RUNS)

    def test_fingerprint_ignores_the_diluter_bucket(self):
        """Gold moving in or out of `both` is not movement: those rows cannot
        discriminate either way, so counting them would reset the swap counter
        forever and the six-cycle rule would never fire."""
        base = {"x": {"group": "d", "baseline": 1, "sense": 1},
                "y": {"group": "d", "baseline": 0, "sense": 1}}
        churned = {"x": {"group": "d", "baseline": 0, "sense": 0},  # dropped out of both
                   "y": {"group": "d", "baseline": 0, "sense": 1}}
        self.assertNotEqual(self._fp(base), self._fp(churned))  # x became `neither`: real
        added_diluter = dict(base, z={"group": "d", "baseline": 1, "sense": 1})
        self.assertEqual(self._fp(base), self._fp(added_diluter))  # pure diluter: not

    def test_a_new_discriminator_moves_it(self):
        base = {"y": {"group": "d", "baseline": 0, "sense": 1}}
        moved = dict(base, z={"group": "d", "baseline": 0, "sense": 1})
        self.assertNotEqual(self._fp(base), self._fp(moved))

    def test_it_is_order_independent(self):
        a = {"m": {"group": "d", "baseline": 0, "sense": 1},
             "n": {"group": "d", "baseline": 0, "sense": 0}}
        b = {"n": {"group": "d", "baseline": 0, "sense": 0},
             "m": {"group": "d", "baseline": 0, "sense": 1}}
        self.assertEqual(self._fp(a), self._fp(b))


if __name__ == "__main__":
    unittest.main()
