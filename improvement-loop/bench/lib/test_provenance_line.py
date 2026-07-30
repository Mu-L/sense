#!/usr/bin/env python3
"""Behaviour pins for the LEDGER Provenance projection.

The load-bearing pin is `test_the_build_key_is_projected`: the build key is the only field in
the line that can answer "is this verdict still true of the Sense in hand". Without it the line
carries a version and a dirty BOOLEAN, which read identically across two different dirty trees
on one commit - the normal shape of a Loop 7 spike.
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from provenance_line import provenance_line

META = {
    "tool_version": "1.13.3-dev+gf220d58",
    "sense_ref": "f220d58",
    "sense_build_key": "4d871f3a31e1",
    "sense_release": "v1.13.2",
    "sense_dirty": True,
    "repo": "dolt",
    "repo_commit": "7e268bf",
    "scenario_file": "/x/scenarios/dolt.yaml",
    "scenario_version": "sha256:ba855c96",
    "timestamp": "2026-07-30T09:00:00Z",
}


class ProvenanceLineTest(unittest.TestCase):
    def test_the_build_key_is_projected(self):
        self.assertIn("[build 4d871f3a31e1]", provenance_line(META))

    def test_a_run_stamped_before_the_key_existed_omits_it_silently(self):
        """Old runs have no key. Print without it; never invent or imply one."""
        old = {k: v for k, v in META.items() if k != "sense_build_key"}
        line = provenance_line(old)
        self.assertNotIn("build", line)
        self.assertIn("1.13.3-dev+gf220d58", line)

    def test_the_full_line_shape(self):
        line = provenance_line(META)
        for part in ("- **Provenance:**", "@f220d58", "release v1.13.2", "dirty tree",
                     "repo dolt@7e268bf", "dolt.yaml", "sha256:ba855c96", "(2026-07-30)"):
            self.assertIn(part, line)

    def test_basename_only_for_the_scenario_path(self):
        self.assertIn("scenario dolt.yaml", provenance_line(META))
        self.assertNotIn("/x/scenarios/", provenance_line(META))

    def test_missing_everything_degrades_without_crashing(self):
        line = provenance_line({})
        self.assertIn("sense (unknown version)", line)
        self.assertIn("repo ?@?", line)


if __name__ == "__main__":
    unittest.main()
