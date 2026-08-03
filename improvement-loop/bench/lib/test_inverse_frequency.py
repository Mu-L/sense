#!/usr/bin/env python3
"""Behaviour pins for the cross-run difficulty ranking (inverse_frequency.py).

The ranking decides which gold rows the NEXT scenario is built from, so the pins guard
the two ways it could quietly mislead: counting a row as hard when it was simply seen in
fewer runs, and losing the per-arm split that separates "hard for everyone" from "the
discriminator we are looking for".
"""
import json
import os
import tempfile
import unittest

from inverse_frequency import runs_for, tally


def make_run(root, arm, repo, run, rows):
    d = os.path.join(root, arm, repo, run)
    os.makedirs(d)
    json.dump({"tool": arm, "repo": repo, "wall_time_seconds": 1},
              open(os.path.join(d, "run_meta.json"), "w"))
    json.dump({"gold_recall": {"details": rows}},
              open(os.path.join(d, "scored.json"), "w"))
    return d


class TallyTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        # sense cites the hard row in both runs; baseline never does.
        for run in ("run-1", "run-2"):
            make_run(self.tmp, "sense", "mastodon", run, [
                {"id": "hard", "group": "dependents", "cited": True},
                {"id": "free", "group": "dependents", "cited": True},
                {"id": "dead", "group": "specs", "cited": False},
            ])
            make_run(self.tmp, "baseline", "mastodon", run, [
                {"id": "hard", "group": "dependents", "cited": False},
                {"id": "free", "group": "dependents", "cited": True},
                {"id": "dead", "group": "specs", "cited": False},
            ])

    def test_it_finds_every_scored_run(self):
        self.assertEqual(len(runs_for("mastodon", [self.tmp])), 4)

    def test_another_repo_is_not_counted(self):
        make_run(self.tmp, "sense", "rails", "run-1",
                 [{"id": "hard", "group": "dependents", "cited": True}])
        self.assertEqual(len(runs_for("mastodon", [self.tmp])), 4)

    def test_the_per_arm_split_survives(self):
        """Collapsing to one overall rate would hide the discriminator: `hard` and a row
        both arms get half the time score the same overall."""
        t = tally(runs_for("mastodon", [self.tmp]))
        self.assertEqual(t["hard"]["sense"], [2, 2])
        self.assertEqual(t["hard"]["baseline"], [0, 2])

    def test_a_row_no_arm_cites_is_zero_not_absent(self):
        """An unreachable row must appear in the ranking; dropping it is how a dead gold
        row survives unnoticed across cycles."""
        t = tally(runs_for("mastodon", [self.tmp]))
        self.assertIn("dead", t)
        self.assertEqual(t["dead"]["sense"], [0, 2])

    def test_a_run_with_no_scored_json_is_skipped_not_counted(self):
        """run_meta without scored.json is an in-flight or failed run; counting it as a
        miss would make every row look harder than it is."""
        d = os.path.join(self.tmp, "sense", "mastodon", "run-9")
        os.makedirs(d)
        json.dump({"tool": "sense"}, open(os.path.join(d, "run_meta.json"), "w"))
        self.assertEqual(len(runs_for("mastodon", [self.tmp])), 4)

    def test_unreadable_scored_json_does_not_crash_the_ranking(self):
        d = make_run(self.tmp, "sense", "mastodon", "run-8", [])
        open(os.path.join(d, "scored.json"), "w").write("{not json")
        t = tally(runs_for("mastodon", [self.tmp]))
        self.assertEqual(t["hard"]["sense"], [2, 2])


if __name__ == "__main__":
    unittest.main()
