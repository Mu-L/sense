#!/usr/bin/env python3
"""Known-answer control for the touched-set coverage gate.

An instrument whose output becomes evidence is run against one row known true and one
known false before it is trusted: a tool that cannot fail visibly cannot be trusted
quietly. Every case here is hand-computed from the fixture above it.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import touched_coverage as tc  # noqa: E402

M = "github.com/luuuc/sense/"

# a.go: 10 statements, 9 covered = 90.0%   (below a 94% floor)
# b.go: 10 statements, 10 covered = 100.0% (above it)
PROFILE = """mode: atomic
{m}internal/x/a.go:1.1,2.2 9 1
{m}internal/x/a.go:3.1,4.2 1 0
{m}internal/x/b.go:1.1,2.2 10 3
""".format(m=M)

# The same block reported twice by two test binaries: uncovered in one, covered in the
# other. Merging by MAX is the only reading that does not invent a gap.
PROFILE_DUP = """mode: atomic
{m}internal/x/b.go:1.1,2.2 10 0
{m}internal/x/b.go:1.1,2.2 10 4
""".format(m=M)


def write(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


class ParseProfile(unittest.TestCase):
    def test_counts_covered_statements_per_file(self):
        got = tc.parse_profile(write(PROFILE))
        self.assertEqual(got["internal/x/a.go"], (9, 10))
        self.assertEqual(got["internal/x/b.go"], (10, 10))

    def test_duplicate_blocks_merge_by_max_not_by_last(self):
        got = tc.parse_profile(write(PROFILE_DUP))
        self.assertEqual(got["internal/x/b.go"], (10, 10))

    def test_module_prefix_is_stripped_to_repo_relative(self):
        self.assertNotIn(M + "internal/x/a.go", tc.parse_profile(write(PROFILE)))


class Evaluate(unittest.TestCase):
    LINES = {"internal/x/a.go": (9, 10), "internal/x/b.go": (10, 10)}
    FUNCS = {"internal/x/a.go": (2, 2), "internal/x/b.go": (2, 2)}

    def test_a_file_below_the_floor_fails(self):
        rows, failures = tc.evaluate(["internal/x/a.go"], self.LINES, self.FUNCS, 94.0)
        self.assertEqual(failures, ["internal/x/a.go"])
        self.assertEqual(rows[0][1], 90.0)

    def test_a_file_above_the_floor_passes(self):
        _, failures = tc.evaluate(["internal/x/b.go"], self.LINES, self.FUNCS, 94.0)
        self.assertEqual(failures, [])

    def test_function_coverage_fails_independently_of_line_coverage(self):
        funcs = {"internal/x/b.go": (1, 2)}  # 50% of functions exercised
        _, failures = tc.evaluate(["internal/x/b.go"], self.LINES, funcs, 94.0)
        self.assertEqual(failures, ["internal/x/b.go"])

    def test_the_floor_is_inclusive_at_exactly_the_bar(self):
        lines = {"f.go": (94, 100)}
        _, failures = tc.evaluate(["f.go"], lines, {"f.go": (100, 100)}, 94.0)
        self.assertEqual(failures, [])

    def test_a_file_with_no_statements_is_skipped_not_failed(self):
        rows, failures = tc.evaluate(["doc.go"], {}, {}, 94.0)
        self.assertEqual(failures, [])
        self.assertEqual(rows[0][3], "no statements")


class Main(unittest.TestCase):
    def test_a_missing_profile_exits_2_rather_than_passing(self):
        self.assertEqual(
            tc.main(["--root", ".", "--branch", "x", "--profile", "/nope/none.txt"]), 2)


if __name__ == "__main__":
    unittest.main()
