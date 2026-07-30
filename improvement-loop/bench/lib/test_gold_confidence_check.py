#!/usr/bin/env python3
"""Behaviour pins for the 0.3-vs-0.7 gold check.

The load-bearing pin is `test_a_row_that_only_exists_at_0_3_fails`: that row is the shape of a
manufactured win. `sense blast` documents min_confidence 0.7 and agents pass the documented
default, so a gold row visible only in a 0.3 sweep earns a delta no agent can reproduce.

The negative pin matters just as much: `test_rows_unreachable_at_both_thresholds_pass`. Gold
sourced from graph, search or a hand read is not in ANY blast set, and failing it would push
curators toward blast-only gold - narrowing the bench to one tool.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from gold_confidence_check import _paths, classify, gold_rows, parse_args, row_reached

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
    def test_a_row_that_only_exists_at_0_3_fails(self):
        """The manufactured-win shape: retrievable while curating, gone when benched."""
        rows = [_row("dep:wide", "internal/wide.go")]
        both, low_only, neither = classify(rows, {"internal/wide.go"}, set())
        self.assertEqual((len(both), len(low_only), len(neither)), (0, 1, 0))

    def test_a_row_present_at_0_7_survives(self):
        rows = [_row("dep:solid", "internal/solid.go")]
        both, low_only, neither = classify(
            rows, {"internal/solid.go"}, {"internal/solid.go"})
        self.assertEqual((len(both), len(low_only), len(neither)), (1, 0, 0))

    def test_rows_unreachable_at_both_thresholds_pass(self):
        """Graph/search/hand-sourced gold is not in any blast set and is not a defect."""
        rows = [_row("ctx:doc", "docs/architecture.md")]
        both, low_only, neither = classify(rows, {"internal/x.go"}, {"internal/x.go"})
        self.assertEqual((len(both), len(low_only), len(neither)), (0, 0, 1))

    def test_a_high_only_row_counts_as_surviving(self):
        """0.7 is the threshold that matters; being absent at 0.3 is not a defect."""
        rows = [_row("dep:strict", "internal/strict.go")]
        both, low_only, _ = classify(rows, set(), {"internal/strict.go"})
        self.assertEqual((len(both), len(low_only)), (1, 0))


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
