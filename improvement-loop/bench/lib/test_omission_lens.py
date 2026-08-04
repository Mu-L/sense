#!/usr/bin/env python3
"""Behaviour pins for the blended cell score (omission_lens.py).

Two pins are load-bearing. `test_the_omission_term_is_todays_headline` records WHY the blend
is worth having: the omission term alone reproduces the current score exactly, so any gain
comes from the completion term and nowhere else - if that ever stops being true, the
justification in the module docstring is stale. `test_the_measured_opus5_cell` pins the cell
the ruling was taken on, so a drift in the arithmetic fails here rather than in a doc.
"""
import json
import os
import shutil
import tempfile
import unittest

from omission_lens import (COMPLETION_WEIGHT, OMISSION_WEIGHT, arm_scores,
                           blended, group_rates, group_rows, is_complete,
                           load_runs)


def scored(groups, completeness=0.0, failed=False):
    """A scored.json body carrying only what this lens reads. groups: {name: (cited, total)}."""
    total = sum(t for _, t in groups.values())
    cited = sum(c for c, _ in groups.values())
    return {
        "failed": failed,
        "completeness": completeness,
        "gold_recall": {
            "cited_recall": (cited / total) if total else 0.0,
            "groups": {n: {"cited": c, "total": t} for n, (c, t) in groups.items()},
        },
    }


class TermsTest(unittest.TestCase):
    def test_the_omission_term_is_todays_headline(self):
        """1.00/16 * 14 and 14/16 are the same calculation. The blend's whole gain therefore
        comes from the completion term; this pin is the reason the blend exists at all."""
        run = scored({"deps": (14, 16)})
        self.assertAlmostEqual(run["gold_recall"]["cited_recall"], 1.00 / 16 * 14)

    def test_the_weights_sum_to_one_so_the_blend_stays_on_the_same_scale(self):
        self.assertAlmostEqual(COMPLETION_WEIGHT + OMISSION_WEIGHT, 1.0)

    def test_blended_weights_omissions_more_than_step_coverage(self):
        run = scored({"deps": (8, 16)}, completeness=1.0)
        self.assertAlmostEqual(blended(run), 0.4 * 1.0 + 0.6 * 0.5)

    def test_a_missing_completeness_field_is_zero_not_a_crash(self):
        """Older scored.json files predate the field; they must read low, never explode."""
        self.assertAlmostEqual(blended({"gold_recall": {"cited_recall": 1.0}}), 0.6)


class CompleteTest(unittest.TestCase):
    def test_complete_needs_every_row_of_every_group(self):
        self.assertTrue(is_complete(scored({"deps": (16, 16), "c": (3, 3)})))
        self.assertFalse(is_complete(scored({"deps": (16, 16), "c": (2, 3)})))

    def test_an_empty_group_is_dropped_not_scored_as_a_free_point(self):
        self.assertEqual(group_rates(scored({"deps": (0, 0), "c": (2, 3)})), {"c": (2, 3)})

    def test_a_run_with_no_scored_group_is_not_complete(self):
        self.assertFalse(is_complete(scored({"deps": (0, 0)})))


class CellTest(unittest.TestCase):
    def test_the_measured_opus5_cell(self):
        """The five pairs the ruling was taken on: today's omission delta +0.087, the
        completion field +0.289, and the blend +0.168."""
        base = [scored({"deps": (c, 16), "specs": (0, 2), "wp": (w, 2), "ct": (3, 3)}, comp)
                for c, w, comp in [(14, 2, 0.688), (14, 2, 0.688), (13, 1, 0.688),
                                   (15, 2, 0.659), (13, 2, 0.688)]]
        sense = [scored({"deps": (c, 16), "specs": (0, 2), "wp": (2, 2), "ct": (3, 3)}, comp)
                 for c, comp in [(16, 0.964), (16, 0.964), (16, 0.964), (14, 0.964),
                                 (16, 1.000)]]
        b, s = arm_scores(base), arm_scores(sense)
        self.assertAlmostEqual(s["omission"] - b["omission"], 0.087, places=3)
        self.assertAlmostEqual(s["completion"] - b["completion"], 0.289, places=3)
        self.assertAlmostEqual(s["blended"] - b["blended"], 0.168, places=3)
        self.assertEqual(b["complete"], [0, 0, 0, 0, 0])
        self.assertEqual(s["complete"], [0, 0, 0, 0, 0])

    def test_a_group_never_cited_by_either_arm_is_flagged_dead(self):
        """specs: 0 of 2 by both arms in every run since June. A broken gold row, not a
        result - and distinct from an arm merely doing badly."""
        base = [scored({"specs": (0, 2), "deps": (2, 16)})]
        sense = [scored({"specs": (0, 2), "deps": (12, 16)})]
        rows = {r["group"]: r for r in group_rows(base, sense)}
        self.assertTrue(rows["specs"]["dead"])
        self.assertFalse(rows["deps"]["dead"])

    def test_a_group_present_in_only_one_arm_is_skipped(self):
        self.assertEqual(group_rows([scored({"a": (1, 2)})], [scored({"b": (1, 2)})]), [])


class LoadRunsTest(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.mkdtemp(prefix="omission-lens-")
        self.addCleanup(shutil.rmtree, self.root, True)

    def write(self, arm, run, body):
        path = os.path.join(self.root, arm, "mastodon", run)
        os.makedirs(path, exist_ok=True)
        with open(os.path.join(path, "scored.json"), "w") as fh:
            json.dump(body, fh)

    def test_a_failed_run_is_dropped_not_scored_as_zero(self):
        """A truncated stream blended as 0.0 manufactures a loss."""
        self.write("sense", "run-1", scored({"deps": (16, 16)}, 1.0))
        self.write("sense", "run-2", scored({"deps": (0, 16)}, 0.0, failed=True))
        self.assertEqual(len(load_runs(self.root, "sense", "mastodon")), 1)

    def test_a_missing_arm_is_empty_not_an_error(self):
        self.assertEqual(load_runs(self.root, "baseline", "mastodon"), [])


if __name__ == "__main__":
    unittest.main()
