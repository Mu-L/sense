#!/usr/bin/env python3
"""Behaviour pins for the arithmetic pay gate.

The pair that matters is `test_a_high_baseline_is_dead` and
`test_the_banked_win_shape_survives`: a gate that only ever says DEAD would block every
cell, and a gate that only ever says OK is the absent check it replaced. Both directions
are pinned with the real shapes - a cell whose baseline holds 10 of 16 scattered rows
(ceiling +0.375) and one whose baseline holds 4 of 9 (ceiling +0.556).
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from pay_ceiling import ceilings, collect


def write_run(root, arm, repo, groups, failed=False, run=1):
    d = os.path.join(root, arm, repo, "run-%d" % run)
    os.makedirs(d, exist_ok=True)
    payload = {"failed": failed, "gold_recall": {"groups": {
        name: {"cited": c, "total": t} for name, (c, t) in groups.items()}}}
    with open(os.path.join(d, "scored.json"), "w") as fh:
        json.dump(payload, fh)


class CeilingTest(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.mkdtemp()

    def _cell(self, base, sense):
        write_run(self.root, "baseline", "r", base)
        write_run(self.root, "sense", "r", sense)
        return ceilings(self.root, "r", 0.50)

    def test_a_high_baseline_is_dead(self):
        """Baseline 10/16 caps the delta at +0.375, under the +0.50 floor."""
        rows, best, ok = self._cell(
            {"dependents": (10, 16), "contract": (4, 4)},
            {"dependents": (15, 16), "contract": (4, 4)})
        self.assertAlmostEqual(best, 0.375)
        self.assertFalse(ok)

    def test_the_banked_win_shape_survives(self):
        """Baseline 4/9 leaves a +0.556 ceiling, so the pair is worth buying."""
        rows, best, ok = self._cell(
            {"dependents": (4, 9), "contract": (6, 7)},
            {"dependents": (9, 9), "contract": (7, 7)})
        self.assertAlmostEqual(best, 1 - 4 / 9)
        self.assertTrue(ok)

    def test_a_maxed_anchor_group_cannot_rescue_the_cell(self):
        """Anchors both arms ace have ceiling 0.0 and must not raise the best."""
        rows, best, ok = self._cell(
            {"dependents": (10, 16), "contract": (4, 4), "write-path": (3, 3)},
            {"dependents": (15, 16), "contract": (4, 4), "write-path": (3, 3)})
        self.assertAlmostEqual(best, 0.375)
        self.assertFalse(ok)

    def test_exactly_at_the_floor_is_reachable(self):
        """Ceiling == floor is not below it; the gate blocks only what cannot reach."""
        _, best, ok = self._cell({"dependents": (5, 10)}, {"dependents": (10, 10)})
        self.assertAlmostEqual(best, 0.50)
        self.assertTrue(ok)

    def test_a_failed_run_is_not_counted_as_zero(self):
        """A failed run blended in as 0.0 manufactures a rescue that is not real."""
        write_run(self.root, "baseline", "r", {"dependents": (10, 16)}, run=1)
        write_run(self.root, "baseline", "r", {"dependents": (0, 16)}, failed=True, run=2)
        self.assertEqual(collect(self.root, "baseline", "r"), {"dependents": (10, 16)})

    def test_no_baseline_runs_reports_no_ceiling(self):
        write_run(self.root, "sense", "r", {"dependents": (9, 9)})
        rows, best, ok = ceilings(self.root, "r", 0.50)
        self.assertIsNone(best)
        self.assertFalse(ok)

    def test_the_delta_matches_pergroups_summed_math(self):
        rows, _, _ = self._cell({"dependents": (10, 16)}, {"dependents": (15, 16)})
        name, bmean, smean, delta, ceiling = rows[0]
        self.assertAlmostEqual(bmean, 0.625)
        self.assertAlmostEqual(smean, 0.9375)
        self.assertAlmostEqual(delta, 0.3125)


if __name__ == "__main__":
    unittest.main()
