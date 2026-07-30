#!/usr/bin/env python3
"""Behaviour pins for the ledger's probe-verdict rule (rule 7, extended 2026-07-30).

The load-bearing pin is `test_probe_provenance_needs_the_build`: a probe entry that records
a kill WITHOUT the build it is true of is the exact shape that lets a stale kill read as
current after Loop 7 ships. Rule 7 already sat inert once (nothing used the key shape, so it
never fired and looked green); these tests exist so the probe half cannot repeat that.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from ledger_check import (KEY_CONTRACT, VERDICT_KEY, check_provenance, check_stopper,
                          parse_entries)


def _entry(key, provenance=None, date="2026-07-30"):
    lines = [f"## {date} | {key} | a title\n", "- **What:** x\n"]
    if provenance:
        lines.append(f"- **Provenance:** {provenance}\n")
    entries, findings = parse_entries(lines)
    assert not findings, findings
    return entries


class ProbeKeyTest(unittest.TestCase):
    def test_probe_is_a_contract_key(self):
        self.assertTrue(any(p.match("loop3/snipe-it/probe") for p in KEY_CONTRACT))

    def test_probe_is_a_verdict_key(self):
        """A bound kill is a verdict, so it inherits the Provenance requirement."""
        self.assertTrue(VERDICT_KEY.match("loop3/snipe-it/probe"))

    def test_the_other_loop3_keys_are_unchanged(self):
        for key in ("loop3/r/scenario", "loop3/r/event-b", "loop3/r/event-c"):
            self.assertTrue(any(p.match(key) for p in KEY_CONTRACT), key)
            self.assertIsNone(VERDICT_KEY.match(key), key)
        for key in ("loop3/r/run-2", "loop3/r/swap", "loop3/r/close"):
            self.assertTrue(VERDICT_KEY.match(key), key)


class ProbeProvenanceTest(unittest.TestCase):
    def test_probe_with_no_provenance_fails(self):
        findings = check_provenance(_entry("loop3/snipe-it/probe"))
        self.assertEqual(len(findings), 1)
        self.assertIn("missing **Provenance:**", findings[0])

    def test_probe_provenance_needs_the_build(self):
        """Wall recorded, build missing: the kill has no version it is true OF."""
        findings = check_provenance(_entry(
            "loop3/snipe-it/probe",
            "sense 1.13.3-dev, repo snipe-it@abc1234, wall 300s (2026-07-30)"))
        self.assertEqual(len(findings), 1)
        self.assertIn("missing build", findings[0])

    def test_probe_provenance_needs_the_wall(self):
        """Build recorded, wall missing: the same cell scored 0.00 at 300s and 0.67 at 720s."""
        findings = check_provenance(_entry(
            "loop3/snipe-it/probe",
            "sense 1.13.3-dev [build 4d871f3a31e1], repo snipe-it@abc1234 (2026-07-30)"))
        self.assertEqual(len(findings), 1)
        self.assertIn("missing wall", findings[0])

    def test_a_wrapped_provenance_is_read_whole(self):
        """Regression: the first live probe entry wrapped its provenance over three lines and
        the first-line-only scan called its `wall 480s` missing. A complete probe provenance
        does not fit on one line, so the field is what gets scanned."""
        # Shaped exactly like main(): read_text().splitlines(), no trailing newlines.
        # The date is interpolated, not typed on a line carrying `#`: a heading literal
        # reads as a dated code comment to the placement linter.
        day = "2026-07-30"
        lines = (f"## {day} | loop3/akaunting/probe | kill\n"
                 "- **What:** x\n"
                 "- **Provenance:** sense build d0ffc2a2e062 (stamped at scoring),\n"
                 "  repo akaunting@72fdf8e, scenario akaunting.draft.yaml @sha256:6480936a,\n"
                 f"  wall 480s (derived), arm claude-opus-5 baseline x2 ({day})").splitlines()
        entries, findings = parse_entries(lines)
        self.assertEqual(findings, [])
        self.assertEqual(check_provenance(entries), [])

    def test_complete_probe_provenance_passes(self):
        findings = check_provenance(_entry(
            "loop3/snipe-it/probe",
            "sense 1.13.3-dev [build 4d871f3a31e1], repo snipe-it@abc1234, "
            "wall 300s, scenario snipe-it.yaml @sha256:ba855c96 (2026-07-30)"))
        self.assertEqual(findings, [])

    def test_run_verdicts_do_not_inherit_the_probe_extras(self):
        """Only probes expire on a build change; run entries keep the schema v2 rule."""
        findings = check_provenance(_entry(
            "loop3/snipe-it/run-1",
            "sense 1.13.3-dev, repo snipe-it@abc1234, scenario s.yaml (2026-07-30)"))
        self.assertEqual(findings, [])


class StopperDormancyTest(unittest.TestCase):
    """Rule 10 must not read green when it is blind.

    The load-bearing pin is `test_an_untracked_instrument_is_reported_not_passed`: for as long
    as improvement-loop/ was untracked, `git diff` listed nothing and rule 10 fired for none of
    the seven instruments - including the scorer whose quiet change the rule was written for.
    """

    INSTRUMENT = "improvement-loop/bench/lib/gold.py"

    def _repo(self, tmp, track=False, modify=False):
        import subprocess

        def git(*args):
            subprocess.run(["git", "-C", tmp] + list(args), capture_output=True, text=True)

        git("init", "-q")
        git("config", "user.email", "t@t")
        git("config", "user.name", "t")
        path = os.path.join(tmp, self.INSTRUMENT)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w") as fh:
            fh.write("def score(): return 1\n")
        if track:
            git("add", "-A")
            git("commit", "-qm", "in")
            if modify:
                with open(path, "w") as fh:
                    fh.write("def score(): return 2\n")
        return tmp

    def test_an_untracked_instrument_is_reported_not_passed(self):
        with tempfile.TemporaryDirectory() as t:
            findings = check_stopper([], self._repo(t))
            self.assertEqual(len(findings), 1)
            self.assertIn("BLIND", findings[0])
            self.assertIn(self.INSTRUMENT, findings[0])

    def test_untracked_does_not_demand_a_blast_radius(self):
        """An untracked file has no HEAD to re-score against; demanding one would be noise."""
        with tempfile.TemporaryDirectory() as t:
            findings = check_stopper([], self._repo(t))
            self.assertNotIn("no `stopper/<slug>`", " ".join(findings))

    def test_tracked_and_clean_is_silent(self):
        with tempfile.TemporaryDirectory() as t:
            self.assertEqual(check_stopper([], self._repo(t, track=True)), [])

    def test_tracked_and_modified_demands_a_stopper_entry(self):
        with tempfile.TemporaryDirectory() as t:
            findings = check_stopper([], self._repo(t, track=True, modify=True))
            self.assertEqual(len(findings), 1)
            self.assertIn("no `stopper/<slug>`", findings[0])

    def test_a_stopper_entry_without_a_blast_radius_still_fails(self):
        with tempfile.TemporaryDirectory() as t:
            entries = _entry("stopper/gold-thing", provenance=None)
            findings = check_stopper(entries, self._repo(t, track=True, modify=True))
            self.assertEqual(len(findings), 1)
            self.assertIn("no re-score", findings[0])


if __name__ == "__main__":
    unittest.main()
