#!/usr/bin/env python3
"""Behaviour pins for the authoring-time rubric gate.

`test_the_invented_shape_is_caught` is the one that pays for the file: a rubric keyed
`{scenario, judge}` instead of `{audience, steps}` reached judge time and took a full
validation pair with it. The gate has to fail that at $0 or it buys nothing.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from rubric_check import check, default_rubric_path

SCENARIO = """
name: T
repo: r
description: d
steps:
  - name: One
    prompt: p
    checks:
      - {type: word, value: x, required: true}
  - name: Two
    prompt: p
    checks:
      - {type: word, value: y, required: true}
"""

GOOD_RUBRIC = """
audience: an engineer
steps:
  - name: One
    criteria: {map_quality: a, specificity: b, justification: c, uncertainty: d}
  - name: Two
    criteria: {map_quality: a, specificity: b, justification: c, uncertainty: d}
"""


class CheckTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp()
        self.scen = os.path.join(self.dir, "t.yaml")
        with open(self.scen, "w") as fh:
            fh.write(SCENARIO)

    def _rubric(self, body):
        with open(default_rubric_path(self.scen), "w") as fh:
            fh.write(body)

    def test_a_matching_rubric_passes(self):
        self._rubric(GOOD_RUBRIC)
        ok, msg = check(self.scen)
        self.assertTrue(ok, msg)
        self.assertIn("2 steps", msg)

    def test_the_invented_shape_is_caught(self):
        self._rubric("scenario: t\njudge:\n  notes: whatever\n")
        ok, msg = check(self.scen)
        self.assertFalse(ok)
        self.assertIn("audience", msg)

    def test_a_step_count_mismatch_is_caught(self):
        self._rubric(GOOD_RUBRIC.replace("""  - name: Two
    criteria: {map_quality: a, specificity: b, justification: c, uncertainty: d}
""", ""))
        ok, msg = check(self.scen)
        self.assertFalse(ok)
        self.assertIn("steps", msg)

    def test_a_renamed_step_is_caught(self):
        """Names must match verbatim or the judge scores the wrong step."""
        self._rubric(GOOD_RUBRIC.replace("- name: Two", "- name: Deux"))
        ok, msg = check(self.scen)
        self.assertFalse(ok)
        self.assertIn("does not match", msg)

    def test_a_missing_criterion_is_caught(self):
        self._rubric(GOOD_RUBRIC.replace(", uncertainty: d}", "}"))
        ok, msg = check(self.scen)
        self.assertFalse(ok)
        self.assertIn("uncertainty", msg)

    def test_an_absent_rubric_is_caught(self):
        ok, msg = check(self.scen)
        self.assertFalse(ok)
        self.assertIn("missing rubric", msg)

    def test_an_unparseable_scenario_is_reported_as_such(self):
        with open(self.scen, "w") as fh:
            fh.write("repo: r\n")
        self._rubric(GOOD_RUBRIC)
        ok, msg = check(self.scen)
        self.assertFalse(ok)
        self.assertIn("scenario does not parse", msg)

    def test_the_default_rubric_path_is_the_sibling(self):
        self.assertEqual(default_rubric_path("/a/b.yaml"), "/a/b.rubric.yaml")


if __name__ == "__main__":
    unittest.main()
