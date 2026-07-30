#!/usr/bin/env python3
"""Behaviour pins for the admission gate's adversary-probe requirement.

The load-bearing pin is `test_name_family_cover_blocks_admission`. The gate ALREADY printed
"the ADVERSARY PROBE is REQUIRED" as a note when one composed name family covers the
dependent set, and a note blocks nothing: akaunting was admitted carrying that exact
warning, its scenario was authored, and its control arm then scored 1.000 on every gold
group. The warning had predicted the kill for $0 and nothing acted on it.
"""
import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from admission_gate import COVER_THRESHOLD, adversary_probe_required, admission_verdict


def _passing_out(union_cover, adversary=None, memorization=True):
    """A real gate run, trimmed, with only the variable under test changed.

    Captured from the akaunting anchor rather than hand-built: an invented dict drifts from
    the shape slot_verdict actually reads (it needs bar 2's survivor counts, bar 3's
    battery, bar 4's fold probe), and a fixture that does not match production tests
    nothing. Bars 1/6/7 are emptied so no unrelated kill masks the verdict.
    """
    path = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                        "testdata", "gate_akaunting.json")
    with open(path) as fh:
        out = json.load(fh)
    out["bar3"]["name_family_union_cover"] = union_cover
    if memorization:
        out["bar5"] = {"ok": True}
    if adversary is not None:
        out["adversary"] = adversary
    return out


class AdversaryProbeRequirementTest(unittest.TestCase):
    def test_predicate_fires_at_the_cover_threshold(self):
        self.assertTrue(adversary_probe_required(_passing_out(COVER_THRESHOLD)))
        self.assertTrue(adversary_probe_required(_passing_out(0.912)))  # akaunting's number
        self.assertFalse(adversary_probe_required(_passing_out(COVER_THRESHOLD - 0.01)))
        self.assertFalse(adversary_probe_required({}))

    def test_name_family_cover_blocks_admission(self):
        """akaunting's shape: every bar passes, one composed family covers 0.912, and the
        gate must NOT admit until the probe has run."""
        verdict, why = admission_verdict(_passing_out(0.912))
        self.assertEqual(verdict, "PENDING-ADVERSARY-PROBE")
        self.assertIn("adversary probe", " ".join(why).lower())

    def test_a_probe_that_assembled_the_answer_rejects(self):
        verdict, why = admission_verdict(
            _passing_out(0.912, adversary={"ok": False, "reason": "grep -rn 'use Jobs' found 31 of 34"}))
        self.assertEqual(verdict, "REJECT")
        self.assertIn("31 of 34", " ".join(why))

    def test_a_probe_that_failed_to_assemble_admits(self):
        verdict, _ = admission_verdict(_passing_out(0.912, adversary={"ok": True}))
        self.assertEqual(verdict, "ADMIT")

    def test_anchors_below_the_threshold_are_unaffected(self):
        """filament's shape: no composed cover, so the requirement never fires and the
        verdict is whatever the other bars say. This is the no-regress case."""
        self.assertEqual(admission_verdict(_passing_out(0.23))[0], "ADMIT")
        self.assertEqual(
            admission_verdict(_passing_out(0.23, memorization=False))[0], "PENDING-BAR-5")

    def test_a_blocking_bar_still_wins(self):
        """The new requirement must not mask a real rejection."""
        out = _passing_out(0.912)
        out["bar1"]["fails"] = ["contract too thin"]
        self.assertEqual(admission_verdict(out)[0], "REJECT")
