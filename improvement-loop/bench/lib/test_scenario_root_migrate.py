#!/usr/bin/env python3
"""Behaviour pins for moving cells under their scenario-version root.

The load-bearing pin is `test_an_already_versioned_root_is_left_alone`: re-running the
migration must not bury one question's root inside another's. That is the same nesting bug
the validation-root append learned once already, and here it would move real transcripts.

`test_a_run_that_cannot_be_attributed_goes_to_unversioned` matters because the alternative -
guessing a version for a run whose metadata is missing - would file a killed run under a
question it may never have answered.
"""
import json
import os
import shutil
import tempfile
import unittest

from scenario_root_migrate import plan, run_version


class MigrateTest(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.mkdtemp(prefix="root-migrate-")
        self.addCleanup(shutil.rmtree, self.root, True)

    def run_dir(self, rel, version="sha256:aaaabbbbccccdddd", meta=True):
        d = os.path.join(self.root, rel)
        os.makedirs(d, exist_ok=True)
        if meta:
            with open(os.path.join(d, "run_meta.json"), "w") as fh:
                json.dump({"tool": "sense", "scenario_version": version}, fh)
        return d

    def dests(self, moves):
        return {os.path.relpath(dest, self.root) for _s, dest, _v in moves}

    def test_a_run_moves_under_its_own_version(self):
        self.run_dir("sense/mastodon/run-1")
        self.assertEqual(self.dests(plan(self.root)),
                         {"aaaabbbbccccdddd/sense/mastodon/run-1"})

    def test_two_questions_in_one_root_split(self):
        """The defect this exists to undo: ten runs of one question and two of another,
        pooled in a single cell and averaged by every reader."""
        self.run_dir("sense/mastodon/run-1", "sha256:1111111111111111")
        self.run_dir("sense/mastodon/run-2", "sha256:2222222222222222")
        self.assertEqual(self.dests(plan(self.root)), {
            "1111111111111111/sense/mastodon/run-1",
            "2222222222222222/sense/mastodon/run-2"})

    def test_an_already_versioned_root_is_left_alone(self):
        """Re-running must be a no-op, not another level of nesting."""
        self.run_dir("aaaabbbbccccdddd/sense/mastodon/run-1")
        self.assertEqual(plan(self.root), [])

    def test_a_run_that_cannot_be_attributed_goes_to_unversioned(self):
        self.run_dir("baseline/mastodon/run-1", meta=False)
        self.assertEqual(self.dests(plan(self.root)),
                         {"unversioned/baseline/mastodon/run-1"})

    def test_a_phase_root_is_not_mistaken_for_an_arm(self):
        """validation/ and minibench/ are results roots in their own right; descending into
        them would file one root's runs under another's version."""
        self.run_dir("validation/sense/mastodon/run-1")
        self.run_dir("minibench/sense/mastodon/run-1")
        self.assertEqual(plan(self.root), [])

    def test_reports_beside_the_cells_are_untouched(self):
        os.makedirs(os.path.join(self.root, "variance"), exist_ok=True)
        open(os.path.join(self.root, "report.json"), "w").close()
        self.run_dir("sense/mastodon/run-1")
        self.assertEqual(len(plan(self.root)), 1)

    def test_a_non_run_directory_in_a_cell_is_skipped(self):
        os.makedirs(os.path.join(self.root, "sense/mastodon/notes"), exist_ok=True)
        self.run_dir("sense/mastodon/run-1")
        self.assertEqual(len(plan(self.root)), 1)


class VersionTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="root-migrate-v-")
        self.addCleanup(shutil.rmtree, self.tmp, True)

    def test_the_sha256_prefix_is_stripped(self):
        with open(os.path.join(self.tmp, "run_meta.json"), "w") as fh:
            json.dump({"scenario_version": "sha256:abcdef0123456789"}, fh)
        self.assertEqual(run_version(self.tmp), "abcdef0123456789")

    def test_missing_or_unreadable_metadata_is_not_a_crash(self):
        self.assertIsNone(run_version(self.tmp))
        with open(os.path.join(self.tmp, "run_meta.json"), "w") as fh:
            fh.write("{not json")
        self.assertIsNone(run_version(self.tmp))

    def test_metadata_with_no_version_is_not_attributed(self):
        with open(os.path.join(self.tmp, "run_meta.json"), "w") as fh:
            json.dump({"tool": "sense"}, fh)
        self.assertIsNone(run_version(self.tmp))


if __name__ == "__main__":
    unittest.main()
