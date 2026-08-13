#!/usr/bin/env python3
"""Behaviour pins for the gold-shown check.

The load-bearing pin is `test_a_row_the_agent_is_not_shown_fails`: that row is the shape of a
manufactured win. Gold the sense arm never sees earns a delta no agent can reproduce, and the
only surface that can answer "does it see this" is MCP, the one the arm runs.

`test_mcp_is_the_only_surface` pins the engine itself: this check shells `sense mcp`, never a
CLI query subcommand. Measuring the CLI is the defect this file was rewritten for.
"""
import json
import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import gold_confidence_check
from gold_confidence_check import (_paths, classify, gold_rows, parse_args, report,
                                   row_reached)

BLAST_JSON = {
    "symbol": "Repository",
    "direct_callers": [
        {"symbol": "handleBlast", "file": "internal/mcpserver/blast.go",
         "ref": "internal/mcpserver/blast.go:19"},
    ],
    "indirect_callers": [
        {"symbol": "TestX", "ref": "internal/mcpserver/helpers_test.go:1339"},
    ],
    "affected_via_composition": [{"ref": "internal/model/rails.go:8"}],
    "affected_files": 3,
}


def _row(rid, *matches, group="dependents"):
    return {"id": rid, "group": group, "match": list(matches)}


class PathWalkTest(unittest.TestCase):
    def test_walks_file_and_ref_at_any_depth(self):
        """indirect_callers has no `file` key, only `ref` - a fixed key list under-counts."""
        got = _paths(BLAST_JSON)
        self.assertEqual(got, {"internal/mcpserver/blast.go",
                               "internal/mcpserver/helpers_test.go",
                               "internal/model/rails.go"})

    def test_line_numbers_are_stripped(self):
        self.assertEqual(_paths({"ref": "a/b.go:19"}), {"a/b.go"})

    def test_non_path_ints_are_ignored(self):
        self.assertEqual(_paths({"affected_files": 3, "count": 0}), set())


class RowMatchTest(unittest.TestCase):
    def test_suffix_match_but_not_substring(self):
        paths = {"internal/model/rails.go"}
        self.assertTrue(row_reached(_row("a", "internal/model/rails.go"), paths))
        self.assertTrue(row_reached(_row("b", "model/rails.go"), paths))
        # a bare basename that is NOT a path-segment suffix must not credit
        self.assertFalse(row_reached(_row("c", "ails.go"), paths))

    def test_any_of_several_patterns_counts(self):
        self.assertTrue(row_reached(_row("a", "nope.go", "internal/model/rails.go"),
                                    {"internal/model/rails.go"}))


class ClassifyTest(unittest.TestCase):
    def test_a_row_the_agent_is_not_shown_fails(self):
        """The manufactured-win shape: retrievable while curating, absent when benched."""
        rows = [_row("dep:wide", "internal/wide.go")]
        shown, missing = classify(rows, set())
        self.assertEqual((len(shown), len(missing)), (0, 1))

    def test_a_row_in_the_shown_output_passes(self):
        rows = [_row("dep:solid", "internal/solid.go")]
        shown, missing = classify(rows, {"internal/solid.go"})
        self.assertEqual((len(shown), len(missing)), (1, 0))

    def test_report_exit_codes(self):
        rows = [_row("dep:a", "a.go"), _row("dep:b", "b.go")]
        shown, missing = classify(rows, {"a.go"})
        self.assertEqual(report(rows, shown, missing, "Sym", "dependents"), 1)
        self.assertEqual(report(rows, rows, [], "Sym", None), 0)


class SurfaceTest(unittest.TestCase):
    def test_mcp_is_the_only_surface(self):
        """The engine must drive `sense mcp`. A CLI query subcommand is the old defect."""
        captured = {}

        def fake_probe(repo, calls, bin_path):
            captured.update(repo=repo, calls=calls, bin_path=bin_path)
            return [(1, json.dumps({"direct_callers": [{"file": "a/b.rb"}]}))], None

        with tempfile.TemporaryDirectory() as t:
            os.mkdir(os.path.join(t, ".sense"))
            with mock.patch.object(gold_confidence_check, "probe", fake_probe):
                paths = gold_confidence_check.shown_paths("/bin/sense", "Sym",
                                                          file_sub="rel.rb", repo_dir=t)
        self.assertEqual(paths, {"a/b.rb"})
        self.assertEqual(captured["calls"],
                         [{"name": "sense_blast",
                           "arguments": {"symbol": "Sym", "file": "rel.rb"}}])

    def test_no_min_confidence_is_ever_passed(self):
        """The arm passes no threshold, so neither may the gate."""
        captured = {}

        def fake_probe(repo, calls, bin_path):
            captured.update(calls=calls)
            return [(1, "{}")], None

        with tempfile.TemporaryDirectory() as t:
            os.mkdir(os.path.join(t, ".sense"))
            with mock.patch.object(gold_confidence_check, "probe", fake_probe):
                gold_confidence_check.shown_paths("/bin/sense", "Sym", repo_dir=t)
        self.assertNotIn("min_confidence", captured["calls"][0]["arguments"])

    def test_an_unindexed_clone_is_a_clean_error(self):
        with tempfile.TemporaryDirectory() as t:
            with self.assertRaises(SystemExit):
                gold_confidence_check.shown_paths("/bin/sense", "Sym", repo_dir=t)


class ScenarioIoTest(unittest.TestCase):
    def test_group_filter(self):
        with tempfile.TemporaryDirectory() as t:
            p = os.path.join(t, "s.yaml")
            with open(p, "w") as fh:
                fh.write("gold:\n"
                         "  - {id: dep:a, group: dependents, match: [a.go]}\n"
                         "  - {id: ctx:b, group: context, match: [b.go]}\n")
            self.assertEqual(len(gold_rows(p)), 2)
            self.assertEqual([r["id"] for r in gold_rows(p, "context")], ["ctx:b"])

    def test_empty_gold_is_a_clean_error(self):
        with tempfile.TemporaryDirectory() as t:
            p = os.path.join(t, "s.yaml")
            with open(p, "w") as fh:
                fh.write("name: t\n")
            with self.assertRaises(SystemExit):
                gold_rows(p)


class ArgTest(unittest.TestCase):
    def test_flags_parse(self):
        opts = parse_args(["x", "s.yaml", "Repository", "--file", "app/models",
                           "--repo", "/tmp/r"])
        self.assertEqual(opts["symbol"], "Repository")
        self.assertEqual(opts["file_sub"], "app/models")
        self.assertEqual(opts["repo_dir"], "/tmp/r")

    def test_bad_flag_is_a_clean_error(self):
        with self.assertRaises(SystemExit):
            parse_args(["x", "s.yaml", "Sym", "--nope", "v"])

    def test_too_few_args(self):
        with self.assertRaises(SystemExit):
            parse_args(["x", "s.yaml"])


if __name__ == "__main__":
    unittest.main()
