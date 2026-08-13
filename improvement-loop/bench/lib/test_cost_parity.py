#!/usr/bin/env python3
"""Behaviour pins for the cost-parity routing signal.

The load-bearing pin is `test_the_axis_is_priced_not_billed`: the 2026-08-01 defect
was reading `token_total_billed` (uncached remainder) as a cost measure, which
turned a 1.30x premium into 1.07x "near parity". If this check ever reads that
field again, a premium can hide behind it and the harvest lane never fires.

`test_a_miss_never_fails_the_phase` is the other half: a parity miss is a PRODUCT
finding routed to harvest, not a gate that halts the loop. Exiting non-zero here
would recreate the stop-instead-of-learn error the file exists to prevent.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import cost_parity
from cost_parity import DEFAULT_THRESHOLD, main, run_priced, verdict


def _metrics(uncached, output, cache_read, cache_write, priced=None):
    m = {"token_input_uncached": uncached, "token_output": output,
         "token_cache_read": cache_read, "token_cache_write": cache_write}
    if priced is not None:
        m["token_total_priced"] = priced
    return {"metrics": m}


class AxisTest(unittest.TestCase):
    def _scored(self, **kw):
        tmp = tempfile.mkdtemp()
        p = os.path.join(tmp, "scored.json")
        with open(p, "w") as fh:
            json.dump(_metrics(**kw), fh)
        return p

    def test_the_axis_is_priced_not_billed(self):
        """Cache tokens are billed; ignoring them is what hid a 30% premium."""
        # Real rails baseline run-2 shape: tiny uncached, huge cache read.
        p = self._scored(uncached=30, output=16959, cache_read=554577, cache_write=27702)
        got = run_priced(p)
        self.assertGreater(got, 150_000)          # counts the cache
        self.assertNotEqual(got, 30 + 16959)      # is NOT token_total_billed

    def test_stored_priced_value_wins(self):
        p = self._scored(uncached=1, output=1, cache_read=1, cache_write=1, priced=99_999)
        self.assertEqual(run_priced(p), 99_999)

    def test_recomputes_for_cells_scored_before_the_field_existed(self):
        p = self._scored(uncached=0, output=1000, cache_read=0, cache_write=0)
        self.assertEqual(run_priced(p), 5000)  # output prices at 5x input


class VerdictTest(unittest.TestCase):
    def test_held_cost_passes(self):
        ratio, line = verdict(100_000, 103_000, DEFAULT_THRESHOLD)
        self.assertIn("PASS", line)
        self.assertAlmostEqual(ratio, 1.03, places=2)

    def test_a_premium_misses(self):
        _, line = verdict(177_757, 223_525, DEFAULT_THRESHOLD)
        self.assertIn("MISS", line)
        self.assertIn("ratio=1.26", line)

    def test_cheaper_than_baseline_passes(self):
        _, line = verdict(200_000, 100_000, DEFAULT_THRESHOLD)
        self.assertIn("PASS", line)

    def test_missing_arm_is_a_skip_not_a_miss(self):
        ratio, line = verdict(None, 100, DEFAULT_THRESHOLD)
        self.assertIsNone(ratio)
        self.assertIn("SKIP", line)


class RoutingTest(unittest.TestCase):
    def _tree(self, baseline_priced, sense_priced):
        root = tempfile.mkdtemp()
        for arm, val in (("baseline", baseline_priced), ("sense", sense_priced)):
            d = os.path.join(root, arm, "repo", "run-1")
            os.makedirs(d)
            with open(os.path.join(d, "scored.json"), "w") as fh:
                json.dump(_metrics(0, 0, 0, 0, priced=val), fh)
        return root

    def test_a_miss_never_fails_the_phase(self):
        """A parity miss ROUTES to harvest; it must not halt the loop."""
        os.environ["RESULTS_DIR"] = self._tree(100_000, 200_000)
        self.assertEqual(main(["x", "repo"]), 0)

    def test_a_pass_also_exits_zero(self):
        os.environ["RESULTS_DIR"] = self._tree(100_000, 100_000)
        self.assertEqual(main(["x", "repo"]), 0)

    def test_threshold_is_overridable(self):
        _, strict = verdict(100_000, 105_000, 1.01)
        _, loose = verdict(100_000, 105_000, 1.10)
        self.assertIn("MISS", strict)
        self.assertIn("PASS", loose)

    def test_missing_results_dir_is_a_clean_error(self):
        os.environ.pop("RESULTS_DIR", None)
        with self.assertRaises(SystemExit):
            main(["x", "repo"])


class ContractTest(unittest.TestCase):
    def test_threshold_absorbs_noise_but_not_a_real_premium(self):
        """10% must sit well below the 26-30% the rails cell showed."""
        self.assertLess(DEFAULT_THRESHOLD, 1.26)
        self.assertGreater(DEFAULT_THRESHOLD, 1.0)

    def test_verdict_line_is_greppable(self):
        """vertical-loop.sh routes on this prefix."""
        _, line = verdict(1, 2, DEFAULT_THRESHOLD)
        self.assertTrue(line.startswith("COST_PARITY: "))
        self.assertIs(cost_parity.priced_tokens.__module__.endswith("scorer"), True)


if __name__ == "__main__":
    unittest.main()
