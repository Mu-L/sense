#!/usr/bin/env python3
"""Behaviour pins for the mcp-only law.

The two pins that matter pull against each other: `test_a_cli_query_is_caught` (the law has
teeth) and `test_a_dict_subscript_is_not_a_command` (it has no false positives). A checker
that cries wolf on `d["status"]` gets muted, and a muted checker is the CLI leak all over
again.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from mcp_only_check import report, scan_text, scan_tree


class ScanTest(unittest.TestCase):
    def test_a_cli_query_is_caught(self):
        hits = scan_text("x.py", 'args = ["blast", symbol, "--json"]')
        self.assertEqual([h[2] for h in hits], ["blast"])

    def test_a_binary_token_form_is_caught(self):
        hits = scan_text("x.py", 'run(["sense", "graph", symbol])')
        self.assertEqual([h[2] for h in hits], ["graph"])
        self.assertEqual([h[2] for h in scan_text("x.py", "run([SENSE_BIN, 'dead'])")],
                         ["dead"])

    def test_a_shell_query_is_caught(self):
        self.assertEqual([h[2] for h in scan_text("x.sh", '"$SENSE_BIN" search "$q"')],
                         ["search"])
        self.assertEqual([h[2] for h in scan_text("x.sh", "sense conventions --json")],
                         ["conventions"])

    def test_a_dict_subscript_is_not_a_command(self):
        """`d["status"]`-shaped reads are all over the reporting code and are not calls."""
        self.assertEqual(scan_text("x.py", 's = d["status"]'), [])
        self.assertEqual(scan_text("x.py", 'if row["blast"] > 0:'), [])
        self.assertEqual(scan_text("x.py", 'x = results[0]["graph"]'), [])

    def test_mcp_and_scan_are_allowed(self):
        """Building and indexing a clone is the CLI's job and stays on it."""
        self.assertEqual(scan_text("x.py", 'subprocess.run([bin_path, "mcp"])'), [])
        self.assertEqual(scan_text("x.sh", '"$SENSE_BIN" scan -rebuild -embed'), [])

    def test_a_comment_is_not_a_call(self):
        self.assertEqual(scan_text("x.py", '# was: ["blast", symbol]'), [])

    def test_one_hit_per_line(self):
        hits = scan_text("x.py", 'run(["sense", "blast", x]) or ["graph", y]')
        self.assertEqual(len(hits), 1)


class TreeTest(unittest.TestCase):
    def _tree(self, files):
        tmp = tempfile.mkdtemp()
        for name, body in files.items():
            with open(os.path.join(tmp, name), "w") as fh:
                fh.write(body)
        return tmp

    def test_clean_tree_passes(self):
        root = self._tree({"a.py": 'probe(repo, calls, bin_path)\n',
                           "b.sh": '"$SENSE_BIN" scan\n'})
        hits = scan_tree(root)
        self.assertEqual(hits, [])
        self.assertEqual(report(hits, root), 0)

    def test_dirty_tree_fails_with_the_path(self):
        root = self._tree({"a.py": 'x = 1\nargs = ["blast", sym]\n'})
        hits = scan_tree(root)
        self.assertEqual([(h[0], h[1], h[3]) for h in hits], [("a.py", 2, "blast")])
        self.assertEqual(report(hits, root), 1)

    def test_non_script_files_are_skipped(self):
        root = self._tree({"notes.md": 'run `sense blast Foo` by hand\n'})
        self.assertEqual(scan_tree(root), [])


if __name__ == "__main__":
    unittest.main()
