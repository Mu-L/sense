#!/usr/bin/env python3
"""Behaviour pins for the phase guard.

The pin that carries the rest is `test_a_claimed_artifact_must_exist`: a verdict that
says "written" while nothing is on disk is exactly the failure the guard exists for,
and it is the one a passing exit code cannot catch.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from verdict_check import check

ALLOW = ["SHAPE", "NO-AXIS"]


class CheckTest(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.mkdtemp()
        self.path = os.path.join(self.root, "scout.verdict.json")

    def _write(self, obj, raw=None):
        with open(self.path, "w") as fh:
            fh.write(raw if raw is not None else json.dumps(obj))

    def _artifact(self, rel="shape.md"):
        with open(os.path.join(self.root, rel), "w") as fh:
            fh.write("x")
        return rel

    def _good(self, **over):
        d = {"phase": "scout", "repo": "mastodon", "verdict": "SHAPE",
             "artifact": self._artifact()}
        d.update(over)
        return d

    def _check(self):
        return check(self.path, "scout", "mastodon", ALLOW, self.root)

    def test_a_complete_verdict_passes_and_returns_its_value(self):
        self._write(self._good())
        self.assertEqual(self._check(), ("SHAPE", None))

    def test_a_claimed_artifact_must_exist(self):
        self._write(self._good(artifact="never-written.md"))
        verdict, reason = self._check()
        self.assertIsNone(verdict)
        self.assertIn("does not exist", reason)

    def test_a_missing_file_is_not_an_advance(self):
        verdict, reason = self._check()
        self.assertIsNone(verdict)
        self.assertIn("no verdict on disk", reason)

    def test_unparseable_json_is_not_an_advance(self):
        self._write(None, raw="{not json")
        self.assertIn("does not parse", self._check()[1])

    def test_a_json_array_is_not_a_verdict(self):
        self._write(None, raw="[]")
        self.assertIn("not a JSON object", self._check()[1])

    def test_every_required_field_is_required(self):
        for field in ("phase", "repo", "verdict", "artifact"):
            d = self._good()
            del d[field]
            self._write(d)
            self.assertIn(field, self._check()[1])

    def test_an_empty_field_counts_as_missing(self):
        self._write(self._good(verdict=""))
        self.assertIn("verdict", self._check()[1])

    def test_another_phases_verdict_is_refused(self):
        self._write(self._good(phase="curate"))
        self.assertIn("phase", self._check()[1])

    def test_another_repos_verdict_is_refused(self):
        self._write(self._good(repo="rails"))
        self.assertIn("repo", self._check()[1])

    def test_a_verdict_outside_the_allow_list_is_refused(self):
        self._write(self._good(verdict="PAY"))
        self.assertIn("not one of", self._check()[1])

    def test_the_second_allowed_verdict_also_passes(self):
        """NO-AXIS is a routed lever, not a failure - the guard must let it through."""
        self._write(self._good(verdict="NO-AXIS"))
        self.assertEqual(self._check(), ("NO-AXIS", None))


if __name__ == "__main__":
    unittest.main()
