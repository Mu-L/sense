#!/usr/bin/env python3
"""Behaviour pins for the ledger's per-repo verdict rule (rule 7) and the key contract.

The load-bearing pin is `test_a_verdict_without_provenance_fails`: a verdict recorded WITHOUT
the build it is true of is the exact shape that lets a stale kill read as current after Loop 7
ships. Rule 7 already sat inert once (nothing used the key shape, so it never fired and looked
green), so the key patterns are pinned here alongside it.

Keys follow the per-repo loop numbering: authoring writes `loop1/`, run writes `loop2/`,
diagnosis writes `loop3/`.
"""
import datetime
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from ledger_check import (KEY_CONTRACT, VERDICT_KEY, results_cells,
                          check_provenance, check_stopper,
                          parse_entries)


def _entry(key, provenance=None, date="2026-07-30"):
    lines = [f"## {date} | {key} | a title\n", "- **What:** x\n"]
    if provenance:
        lines.append(f"- **Provenance:** {provenance}\n")
    entries, findings = parse_entries(lines)
    assert not findings, findings
    return entries


class PerRepoKeyTest(unittest.TestCase):
    def test_every_per_repo_key_is_under_contract(self):
        for key in ("loop1/r/scenario", "loop1/r/event-b", "loop2/r/event-c",
                    "loop2/r/run-2", "loop3/r/swap", "loop3/r/close"):
            self.assertTrue(any(p.match(key) for p in KEY_CONTRACT), key)

    def test_only_the_outcome_keys_are_verdicts(self):
        """Scenario stamps and gate approvals are records, not verdicts: no Provenance owed."""
        for key in ("loop1/r/scenario", "loop1/r/event-b", "loop2/r/event-c"):
            self.assertIsNone(VERDICT_KEY.match(key), key)
        for key in ("loop2/r/run-2", "loop3/r/swap", "loop3/r/close"):
            self.assertTrue(VERDICT_KEY.match(key), key)

    def test_the_retired_eligibility_key_is_rejected(self):
        """The probe key died with the eligibility stage; a rule that still accepts it
        would let a bound kill be recorded against a loop that no longer exists."""
        self.assertFalse(any(p.match("loop3/r/probe") for p in KEY_CONTRACT))
        self.assertIsNone(VERDICT_KEY.match("loop3/r/probe"))

    def test_the_old_flat_loop3_shape_is_rejected(self):
        """Before the split every per-repo key was loop3/; those shapes must not linger."""
        for key in ("loop3/r/scenario", "loop3/r/event-c", "loop3/r/run-1"):
            self.assertFalse(any(p.match(key) for p in KEY_CONTRACT), key)


class VerdictProvenanceTest(unittest.TestCase):
    def test_a_verdict_without_provenance_fails(self):
        findings = check_provenance(_entry("loop2/snipe-it/run-1"))
        self.assertEqual(len(findings), 1)
        self.assertIn("missing **Provenance:**", findings[0])

    def test_a_verdict_with_provenance_passes(self):
        findings = check_provenance(_entry(
            "loop2/snipe-it/run-1",
            "sense 1.13.3-dev, repo snipe-it@abc1234, scenario s.yaml (2026-07-30)"))
        self.assertEqual(findings, [])

    def test_diagnosis_outcomes_carry_the_same_requirement(self):
        for key in ("loop3/snipe-it/swap", "loop3/snipe-it/close"):
            self.assertEqual(len(check_provenance(_entry(key))), 1, key)

    def test_a_wrapped_provenance_is_read_whole(self):
        """Regression: a live entry wrapped its provenance over three lines and a
        first-line-only scan called its tail missing. The field is what gets scanned."""
        # Shaped exactly like main(): read_text().splitlines(), no trailing newlines.
        # The date is interpolated, not typed on a line carrying `#`: a heading literal
        # reads as a dated code comment to the placement linter.
        day = "2026-07-30"
        lines = (f"## {day} | loop2/akaunting/run-1 | sub-floor\n"
                 "- **What:** x\n"
                 "- **Provenance:** sense build d0ffc2a2e062 (stamped at scoring),\n"
                 "  repo akaunting@72fdf8e, scenario akaunting.yaml @sha256:6480936a,\n"
                 f"  arm claude-opus-5 x2 ({day})").splitlines()
        entries, findings = parse_entries(lines)
        self.assertEqual(findings, [])
        self.assertEqual(check_provenance(entries), [])


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
        # Dated TODAY on purpose: rule 10 only accepts a stopper entry from today, so a
        # hardcoded date makes this test rot into passing for the wrong reason.
        with tempfile.TemporaryDirectory() as t:
            entries = _entry("stopper/gold-thing", provenance=None,
                             date=datetime.date.today().isoformat())
            findings = check_stopper(entries, self._repo(t, track=True, modify=True))
            self.assertEqual(len(findings), 1)
            self.assertIn("no re-score", findings[0])


if __name__ == "__main__":
    unittest.main()


class ResultsCellsAtAnyDepth(unittest.TestCase):
    """Rule 4 globbed a fixed depth and went inert when a segment was inserted."""

    def _cells(self, *paths):
        import pathlib
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            for p in paths:
                (root / p).mkdir(parents=True)
            return results_cells(root)

    def test_a_versioned_cell_is_seen(self):
        self.assertEqual(self._cells("m/aaaa/sense/repo/run-1"), {"m/aaaa/sense/repo"})

    def test_a_legacy_cell_without_a_version_is_still_seen(self):
        self.assertEqual(self._cells("m/sense/repo/run-1"), {"m/sense/repo"})

    def test_a_validation_root_adds_its_own_segment_and_is_still_seen(self):
        self.assertEqual(self._cells("m/validation/aaaa/sense/repo/run-2"),
                         {"m/validation/aaaa/sense/repo"})

    def test_every_shape_at_once(self):
        self.assertEqual(len(self._cells("m/aaaa/sense/r1/run-1", "m/sense/r2/run-1",
                                         "m/minibench/bbbb/baseline/r3/run-3")), 3)

    def test_a_tree_with_no_runs_yields_nothing(self):
        self.assertEqual(self._cells("m/aaaa/sense/repo"), set())
