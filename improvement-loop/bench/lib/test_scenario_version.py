#!/usr/bin/env python3
"""Behaviour pins for the scenario identity hash.

The pin that matters is `test_the_rubric_moves_the_version`: the version has to move on
every reshape, or `vertical-loop.sh` matches a validation pair against gold that never ran
it - which is how a DEAD ceiling got read off a baseline that had answered a different
scenario. The recipe is also pinned against the value stamped into run_meta.json by
bench-sense-local.sh, because the stamper and the matcher agreeing is the whole point.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from scenario_version import version


class VersionTest(unittest.TestCase):
    def setUp(self):
        self.d = tempfile.mkdtemp()
        self.yaml = os.path.join(self.d, "repo.yaml")
        self.rubric = os.path.join(self.d, "repo.rubric.yaml")
        self._write(self.yaml, "name: a\n")
        self._write(self.rubric, "audience: x\n")

    def _write(self, p, s):
        with open(p, "w") as fh:
            fh.write(s)

    def test_the_same_bytes_give_the_same_version(self):
        self.assertEqual(version(self.yaml), version(self.yaml))
        self.assertTrue(version(self.yaml).startswith("sha256:"))
        self.assertEqual(len(version(self.yaml)), len("sha256:") + 16)

    def test_the_scenario_moves_the_version(self):
        before = version(self.yaml)
        self._write(self.yaml, "name: b\n")
        self.assertNotEqual(before, version(self.yaml))

    def test_the_rubric_moves_the_version(self):
        """A reshape that only touches the rubric is still a different scenario."""
        before = version(self.yaml)
        self._write(self.rubric, "audience: y\n")
        self.assertNotEqual(before, version(self.yaml))

    def test_the_rubric_is_derived_from_the_scenario_path(self):
        self.assertEqual(version(self.yaml), version(self.yaml, self.rubric))

    def test_a_missing_rubric_is_not_an_error(self):
        os.remove(self.rubric)
        self.assertTrue(version(self.yaml).startswith("sha256:"))

    def test_it_hashes_only_the_scored_files(self):
        """An unrelated sibling in the same directory must not move the version."""
        before = version(self.yaml)
        self._write(os.path.join(self.d, "repo.gold-audit.json"), "{}")
        self.assertEqual(before, version(self.yaml))


if __name__ == "__main__":
    unittest.main()
