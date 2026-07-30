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
    def test_the_probe_is_required_unconditionally(self):
        """No threshold gates it. filament is why: token cover 0.23, no covering pattern,
        and its adversary still reached every dependent via a two-hop declared chain. A
        number that cannot see that shape cannot be trusted to decide when to look."""
        self.assertTrue(adversary_probe_required(_passing_out(0.912)))  # akaunting
        self.assertTrue(adversary_probe_required(_passing_out(0.23)))   # filament
        self.assertTrue(adversary_probe_required(_passing_out(0.0)))
        self.assertTrue(adversary_probe_required({}))

    def test_an_otherwise_clean_anchor_is_still_pending(self):
        """akaunting's shape: every other bar passes and the gate must still not admit
        until the probe has run. This is the case that was admitted for real, authored,
        and then killed at control 1.000."""
        verdict, why = admission_verdict(_passing_out(0.912))
        self.assertEqual(verdict, "PENDING-ADVERSARY-PROBE")
        self.assertIn("probed before admission", " ".join(why))

    def test_a_probe_that_assembled_the_answer_rejects(self):
        verdict, why = admission_verdict(
            _passing_out(0.912, adversary={"ok": False, "reason": "grep -rn 'use Jobs' found 31 of 34"}))
        self.assertEqual(verdict, "REJECT")
        self.assertIn("31 of 34", " ".join(why))

    def test_a_probe_that_failed_to_assemble_admits(self):
        verdict, _ = admission_verdict(_passing_out(0.912, adversary={"ok": True}))
        self.assertEqual(verdict, "ADMIT")

    def test_a_low_cover_anchor_is_also_pending(self):
        """filament's shape. Under the old threshold this admitted; it then died at control
        1.000, which is the measurement that removed the threshold."""
        verdict, why = admission_verdict(_passing_out(0.23))
        self.assertEqual(verdict, "PENDING-ADVERSARY-PROBE")
        self.assertNotIn("Prior:", " ".join(why), "no cover prior should be claimed at 0.23")

    def test_a_high_cover_anchor_carries_the_prior(self):
        _, why = admission_verdict(_passing_out(0.912))
        self.assertIn("Prior:", " ".join(why))

    def test_the_probe_gates_before_the_memorization_bar(self):
        """Both are pending-shaped; the probe is $0 and comes first, so it is the one
        reported when neither has run."""
        self.assertEqual(
            admission_verdict(_passing_out(0.23, memorization=False))[0],
            "PENDING-ADVERSARY-PROBE")

    def test_a_blocking_bar_still_wins(self):
        """The new requirement must not mask a real rejection."""
        out = _passing_out(0.912)
        out["bar1"]["fails"] = ["contract too thin"]
        self.assertEqual(admission_verdict(out)[0], "REJECT")
