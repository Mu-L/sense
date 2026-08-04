#!/usr/bin/env python3
"""Behaviour pins for the scenario archive and the difficulty-by-path ranking.

The load-bearing pin is `test_two_questions_naming_one_file_accumulate_together`: it is the
whole point of the archive. A re-gold renames a row and mints a new scenario version, and the
old ranking then reported the file as freshly unseen. Keyed on the gold path, the two
questions' runs add up.

`test_a_version_with_no_archived_scenario_is_dropped_and_named` is the guard on that: two
questions whose rows happen to share an id are NOT the same row, so an unresolvable version
must be excluded loudly rather than folded in under its ids.
"""
import io
import json
import os
import shutil
import sys
import tempfile
import unittest
from contextlib import redirect_stdout

import inverse_frequency
import scenario_archive

SCENARIO = """\
name: probe
repo: mastodon
steps: []
gold:
  - {id: %s, group: dependents, match: [app/lib/feed_manager.rb]}
  - {id: sym-only, group: contract, match: [SomeClassName]}
  - {id: no-slash, group: dependents, match: [annual_reports_presenter.rb]}
"""


class ArchiveTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="scen-archive-")
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.store = os.path.join(self.tmp, ".versions")

    def write(self, name, body):
        path = os.path.join(self.tmp, name)
        with open(path, "w") as fh:
            fh.write(body)
        return path

    def test_the_same_bytes_archive_once(self):
        path = self.write("a.yaml", SCENARIO % "d:feeds")
        ver, new = scenario_archive.add(path, store=self.store)
        self.assertTrue(new)
        again, new2 = scenario_archive.add(path, store=self.store)
        self.assertEqual(ver, again)
        self.assertFalse(new2)

    def test_changed_bytes_are_a_different_version_and_both_survive(self):
        """Nothing is overwritten: a version can never hold two different golds."""
        v1, _ = scenario_archive.add(self.write("a.yaml", SCENARIO % "d:feeds"), store=self.store)
        v2, _ = scenario_archive.add(self.write("b.yaml", SCENARIO % "dep:feeds"), store=self.store)
        self.assertNotEqual(v1, v2)
        self.assertEqual(len(scenario_archive.versions(self.store)), 2)

    def test_the_rubric_is_part_of_the_identity(self):
        """scenario_version.py hashes the rubric sibling too, so the archive must store it -
        the pair that produced the twelve scored runs was a .bak plus the COMMITTED rubric."""
        scen = self.write("a.yaml", SCENARIO % "d:feeds")
        rub = self.write("r1.yaml", "weights: {a: 1}\n")
        v1, _ = scenario_archive.add(scen, rub, store=self.store)
        v2, _ = scenario_archive.add(scen, self.write("r2.yaml", "weights: {a: 2}\n"),
                                     store=self.store)
        self.assertNotEqual(v1, v2)
        self.assertTrue(os.path.isfile(os.path.join(
            self.store, v1.split(":")[1], "rubric.yaml")))

    def test_get_returns_none_for_an_unknown_version(self):
        self.assertIsNone(scenario_archive.get("sha256:deadbeefdeadbeef", self.store))

    def test_a_row_with_no_file_pattern_keeps_its_id(self):
        path = self.write("a.yaml", SCENARIO % "d:feeds")
        ver, _ = scenario_archive.add(path, store=self.store)
        keys = scenario_archive.gold_paths(scenario_archive.get(ver, self.store))
        self.assertEqual(keys["d:feeds"], "app/lib/feed_manager.rb")
        self.assertEqual(keys["sym-only"], "sym-only")

    def test_a_filename_without_a_slash_is_still_a_file(self):
        """gold.py's rule, not a second copy of it: annual_reports_presenter.rb has no slash."""
        path = self.write("a.yaml", SCENARIO % "d:feeds")
        ver, _ = scenario_archive.add(path, store=self.store)
        keys = scenario_archive.gold_paths(scenario_archive.get(ver, self.store))
        self.assertEqual(keys["no-slash"], "annual_reports_presenter.rb")


class ByPathTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="by-path-")
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.store = os.path.join(self.tmp, ".versions")
        self.results = os.path.join(self.tmp, "results")

    def scenario(self, name, row_id):
        path = os.path.join(self.tmp, name)
        with open(path, "w") as fh:
            fh.write(SCENARIO % row_id)
        return scenario_archive.add(path, store=self.store)[0]

    def run_dir(self, arm, idx, version, row_id, cited, model="claude-opus-5"):
        d = os.path.join(self.results, arm, "mastodon", f"run-{idx}")
        os.makedirs(d, exist_ok=True)
        with open(os.path.join(d, "run_meta.json"), "w") as fh:
            json.dump({"tool": arm, "scenario_version": version, "model": model}, fh)
        with open(os.path.join(d, "scored.json"), "w") as fh:
            json.dump({"gold_recall": {"details": [
                {"id": row_id, "group": "dependents", "cited": cited}]}}, fh)

    def rank(self, *extra):
        argv = ["inverse_frequency.py", "mastodon", self.results,
                "--by-path", self.store] + list(extra)
        old, buf = sys.argv, io.StringIO()
        sys.argv = argv
        try:
            with redirect_stdout(buf):
                rc = inverse_frequency.main()
        finally:
            sys.argv = old
        return rc, buf.getvalue()

    def test_two_questions_naming_one_file_accumulate_together(self):
        """The whole point: a re-gold renames the row and mints a new version, and the file's
        history must survive it. Keyed on ids these would be two rows at 1 run each."""
        v1 = self.scenario("q1.yaml", "d:feeds")
        v2 = self.scenario("q2.yaml", "dep:feeds")
        self.run_dir("baseline", 1, v1, "d:feeds", False)
        self.run_dir("baseline", 2, v2, "dep:feeds", True)
        rc, out = self.rank()
        self.assertEqual(rc, 0)
        self.assertIn("2 scenario version(s) merged", out)
        self.assertRegex(out, r"app/lib/feed_manager\.rb\s+dependents\s+1/2")

    def test_a_version_with_no_archived_scenario_is_dropped_and_named(self):
        v1 = self.scenario("q1.yaml", "d:feeds")
        self.run_dir("baseline", 1, v1, "d:feeds", True)
        self.run_dir("baseline", 2, "sha256:notarchived0000", "d:feeds", False)
        rc, out = self.rank()
        self.assertEqual(rc, 0)
        self.assertIn("DROPPED", out)
        self.assertIn("sha256:notarchived0000", out)
        self.assertRegex(out, r"app/lib/feed_manager\.rb\s+dependents\s+1/1")

    def test_a_second_model_is_excluded_and_named(self):
        """laws.md scopes this ranking to the headline model; mixing generations reorders
        the middle of the table. The first run of --by-path blended two without saying so."""
        v1 = self.scenario("q1.yaml", "d:feeds")
        self.run_dir("baseline", 1, v1, "d:feeds", True)
        self.run_dir("sense", 1, v1, "d:feeds", True)
        self.run_dir("baseline", 2, v1, "d:feeds", False, model="claude-opus-4-8")
        rc, out = self.rank()
        self.assertEqual(rc, 0)
        self.assertIn("# model claude-opus-5", out)
        self.assertIn("claude-opus-4-8 (1 run(s))", out)
        self.assertRegex(out, r"app/lib/feed_manager\.rb\s+dependents\s+2/2")

    def test_two_equally_represented_models_refuse_rather_than_pick(self):
        """Breaking the tie alphabetically picks claude-opus-4-8 over claude-opus-5, i.e. a
        default that silently prefers the older generation."""
        v1 = self.scenario("q1.yaml", "d:feeds")
        self.run_dir("baseline", 1, v1, "d:feeds", True)
        self.run_dir("baseline", 2, v1, "d:feeds", False, model="claude-opus-4-8")
        rc, out = self.rank()
        self.assertEqual(rc, 1)
        self.assertIn("pass --model", out)
        rc2, out2 = self.rank("--model", "claude-opus-5")
        self.assertEqual(rc2, 0)
        self.assertIn("# model claude-opus-5", out2)

    def test_an_explicit_model_that_never_ran_is_an_error_not_an_empty_table(self):
        v1 = self.scenario("q1.yaml", "d:feeds")
        self.run_dir("baseline", 1, v1, "d:feeds", True)
        rc, out = self.rank("--model", "gpt-5.6-sol")
        self.assertEqual(rc, 1)
        self.assertIn("no runs on model", out)

    def test_nothing_archived_explains_itself_rather_than_printing_an_empty_table(self):
        self.run_dir("baseline", 1, "sha256:notarchived0000", "d:feeds", True)
        rc, out = self.rank()
        self.assertEqual(rc, 1)
        self.assertIn("scenario_archive.py add", out)


if __name__ == "__main__":
    unittest.main()
