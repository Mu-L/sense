#!/usr/bin/env python3
"""Behaviour pins for the matched-budget wall in bench-sense-local.sh.

Both pins guard one-line regressions that are INVISIBLE in the output and corrupt the
record rather than crashing:

  * `test_the_enforced_wall_is_the_derived_one` - the bug this file was written for. The
    enforcement used $session_timeout (the default) while run_meta.json recorded the
    derived $run_timeout, so the artifact claimed a budget the run never ran under. A
    scored cell with false provenance is worse than an unimplemented feature.

  * `test_sense_is_ordered_first` - the baseline's wall is derived from its paired sense
    run, so a baseline that runs first would find no partner and silently skip every run.
"""
import os
import re
import unittest

DRIVER = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                      "drivers", "bench-sense-local.sh")


class MatchedBudgetWallTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with open(DRIVER) as fh:
            cls.src = fh.read()

    def test_the_enforced_wall_is_the_derived_one(self):
        """The `timeout` wrapping the measured session must use $run_timeout."""
        launches = re.findall(r'"\$\{TIMEOUT_CMD\[@\]\}"\s+(\S+)\s+claude', self.src)
        self.assertTrue(launches, "no timeout-wrapped claude launch found")
        # The measured session is the one passed a variable; the post-run survey turn uses
        # a literal and is deliberately excluded - it is not scored.
        measured = [a.strip('"') for a in launches if "$" in a]
        self.assertEqual(measured, ["$run_timeout"],
                         f"measured session must be walled by $run_timeout, got {measured}")

    def test_run_meta_records_the_same_wall_it_enforced(self):
        """run_meta is the source of record; recording a different number than the one
        enforced is how a cell acquires false provenance."""
        self.assertIn('"$scenario_version" "$scenario_file" "$run_timeout"', self.src)

    def test_sense_is_ordered_first(self):
        """The pairing table is populated by the sense arm and read by the baseline."""
        self.assertIn('for _t in "${TOOLS[@]}"; do [[ "$_t" == sense ]] && _reordered+=("$_t"); done',
                      self.src)

    def test_a_missing_or_invalid_partner_skips_rather_than_scores(self):
        """A baseline measured against a sense run that never finished says nothing, and
        its wall would be derived from a failure."""
        self.assertIn("RE-RUN THE SENSE ARM", self.src)
        self.assertRegex(self.src, r'paired_valid" != "true"')


if __name__ == "__main__":
    unittest.main()
