#!/usr/bin/env python3
"""Behaviour pins for the hand-audit sheet.

The two that carry the check: `test_a_todo_row_fails` (the sheet cannot be satisfied by the
keystroke that creates it) and `test_retargeted_gold_voids_the_sheet` (an audit of gold that
no longer exists is not an audit). Without the second, stamping once would immunise a
scenario forever.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from gold_audit import gold_digest, sheet_path, stamp, verify

GOLD = """gold:
  - {id: dep:a, group: dependents, match: [a.rb]}
  - {id: dep:b, group: dependents, match: [b.rb]}
"""


class SheetTest(unittest.TestCase):
    def _scenario(self, body=GOLD):
        tmp = tempfile.mkdtemp()
        path = os.path.join(tmp, "repo.yaml")
        with open(path, "w") as fh:
            fh.write(body)
        return path

    def _fill(self, path, verdict="reads on the Relation contract, credit is the right file"):
        with open(sheet_path(path)) as fh:
            sheet = json.load(fh)
        sheet["rows"] = {k: verdict for k in sheet["rows"]}
        with open(sheet_path(path), "w") as fh:
            json.dump(sheet, fh)

    def test_no_sheet_fails(self):
        self.assertEqual(verify(self._scenario()), 1)

    def test_a_todo_row_fails(self):
        """Stamping must not satisfy the check it creates."""
        path = self._scenario()
        stamp(path)
        self.assertEqual(verify(path), 1)

    def test_a_filled_sheet_passes(self):
        path = self._scenario()
        stamp(path)
        self._fill(path)
        self.assertEqual(verify(path), 0)

    def test_retargeted_gold_voids_the_sheet(self):
        path = self._scenario()
        stamp(path)
        self._fill(path)
        self.assertEqual(verify(path), 0)
        with open(path, "w") as fh:
            fh.write(GOLD.replace("b.rb", "c.rb"))
        self.assertEqual(verify(path), 1)

    def test_a_new_row_is_reported_unlisted(self):
        path = self._scenario()
        stamp(path)
        self._fill(path)
        with open(path, "w") as fh:
            fh.write(GOLD + "  - {id: dep:c, group: dependents, match: [c.rb]}\n")
        self.assertEqual(verify(path), 1)

    def test_restamp_carries_finished_verdicts_over(self):
        """Re-stamping after adding a row must not wipe the audits already done."""
        path = self._scenario()
        stamp(path)
        self._fill(path, "audited: correct file")
        with open(path, "w") as fh:
            fh.write(GOLD + "  - {id: dep:c, group: dependents, match: [c.rb]}\n")
        stamp(path)
        with open(sheet_path(path)) as fh:
            rows = json.load(fh)["rows"]
        self.assertEqual(rows["dep:a"], "audited: correct file")
        self.assertEqual(rows["dep:c"], "TODO")

    def test_sidecar_never_touches_the_scenario(self):
        path = self._scenario()
        before = open(path).read()
        stamp(path)
        self.assertEqual(open(path).read(), before)
        self.assertTrue(sheet_path(path).endswith("repo.gold-audit.json"))

    def test_digest_ignores_key_order(self):
        self.assertEqual(gold_digest([{"id": "a", "match": ["x"]}]),
                         gold_digest([{"match": ["x"], "id": "a"}]))

    def test_empty_gold_is_a_clean_error(self):
        path = self._scenario("name: t\n")
        with self.assertRaises(SystemExit):
            verify(path)


if __name__ == "__main__":
    unittest.main()
