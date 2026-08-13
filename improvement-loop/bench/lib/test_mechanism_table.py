"""Behaviour pins for the cycle 2 mechanism table (the returned x cited 2x2)."""
import json
import os
import unittest

import mechanism_table as mt

GOLD = [
    {"id": "a", "group": "g", "match": ["app/models/a.rb"]},
    {"id": "b", "group": "g", "match": ["app/models/b.rb"]},
]


def _frame(direction, msg):
    return json.dumps({"ts": 0, "dir": direction, "msg": msg})


def _call(cid, name):
    return _frame("c2s", {"id": cid, "method": "tools/call", "params": {"name": name}})


def _result(cid, text):
    return _frame("s2c", {"id": cid, "result": {"content": [{"type": "text", "text": text}]}})


def _write_run(tmp, repo, run, io_lines, cited_ids, arm="sense"):
    d = os.path.join(tmp, arm, repo, run)
    os.makedirs(d, exist_ok=True)
    if io_lines is not None:
        with open(os.path.join(d, "sense-io.jsonl"), "w") as fh:
            fh.write("\n".join(io_lines) + ("\n" if io_lines else ""))
    scored = {"gold_recall": {"details": [{"id": i, "cited": True} for i in cited_ids]}}
    json.dump(scored, open(os.path.join(d, "scored.json"), "w"))
    return d


class RoutingState(unittest.TestCase):
    """A Sense arm that never called Sense is not a measurement of Sense."""

    def test_no_log_at_all_is_a_harness_failure(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            d = _write_run(tmp, "r", "run-1", None, [])
            self.assertEqual(mt.read_run(d)["routing"], "harness-failure")

    def test_handshake_without_a_tool_call_is_never_routed(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            lines = [_frame("c2s", {"id": 1, "method": "initialize"}),
                     _frame("s2c", {"id": 1, "result": {}})]
            d = _write_run(tmp, "r", "run-1", lines, [])
            self.assertEqual(mt.read_run(d)["routing"], "never-routed")

    def test_search_without_a_resolver_is_search_only(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            lines = [_call(1, "sense_search"), _result(1, "{}")]
            d = _write_run(tmp, "r", "run-1", lines, [])
            self.assertEqual(mt.read_run(d)["routing"], "search-only")

    def test_a_blast_call_is_routed(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            lines = [_call(1, "sense_blast"), _result(1, "{}")]
            d = _write_run(tmp, "r", "run-1", lines, [])
            self.assertEqual(mt.read_run(d)["routing"], "routed")


class ReturnedNeedsALine(unittest.TestCase):
    """gold.py credits a citation only with a line, so both sides need one."""

    def test_a_ref_with_a_line_counts_as_returned(self):
        self.assertIn("app/models/a.rb",
                      mt._refs_with_line('{"ref":"app/models/a.rb:12"}'))

    def test_a_bare_path_without_a_line_does_not(self):
        self.assertEqual(mt._refs_with_line('{"file":"app/models/a.rb"}'), set())

    def test_a_suffix_of_the_gold_path_still_matches(self):
        self.assertTrue(mt._path_matches("models/a.rb", {"app/models/a.rb"}))

    def test_an_unrelated_path_does_not(self):
        self.assertFalse(mt._path_matches("app/models/a.rb", {"app/models/z.rb"}))


class TheFourCells(unittest.TestCase):
    def _cells(self, returned, cited):
        run = {"returned_paths": returned, "cited_ids": cited}
        return mt.classify_run(run, GOLD)

    def test_returned_and_cited_is_reach(self):
        self.assertEqual(self._cells(["app/models/a.rb"], ["a"])["a"], mt.REACH)

    def test_returned_and_not_cited_is_ignored(self):
        self.assertEqual(self._cells(["app/models/a.rb"], [])["a"], mt.IGNORED)

    def test_cited_without_sense_returning_it_is_found_anyway(self):
        self.assertEqual(self._cells([], ["a"])["a"], mt.FOUND_ANYWAY)

    def test_neither_is_missed(self):
        self.assertEqual(self._cells([], [])["a"], mt.MISSED)


class RunDisagreement(unittest.TestCase):
    """A third run is bought by a flipped verdict, never by a flipped row.

    Five gold rows, not two: on a two-row table one flipped row IS half the
    table, so it moves the dominant cell and the distinction cannot be shown.
    That is the real-cell shape - discourse flipped 2 rows of 23 and stayed a
    reach in both runs.
    """

    IDS = ["a", "b", "c", "d", "e"]
    GOLD5 = [{"id": i, "group": "g", "match": [f"app/models/{i}.rb"]} for i in IDS]

    def _build(self, tmp, run1_cited, run2_cited):
        refs = "".join(f'{{"ref":"app/models/{i}.rb:1"}}' for i in self.IDS)
        lines = [_call(1, "sense_blast"), _result(1, refs)]
        _write_run(tmp, "r", "run-1", lines, run1_cited)
        _write_run(tmp, "r", "run-2", lines, run2_cited)
        return mt.build(tmp, "r", self.GOLD5)

    def test_a_row_that_flips_is_reported_without_splitting_the_verdict(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            t = self._build(tmp, self.IDS, ["a", "b", "c", "d"])
            self.assertEqual(t["rows_disagreeing"], ["e"])
            self.assertFalse(t["verdict_split"])
            self.assertEqual(t["dominant"], mt.REACH)

    def test_a_flipped_dominant_cell_splits_the_verdict(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            t = self._build(tmp, self.IDS, [])
            self.assertTrue(t["verdict_split"])
            self.assertIsNone(t["dominant"])

    def test_a_harness_failure_is_not_counted_as_a_measured_run(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            lines = [_call(1, "sense_blast"), _result(1, '{"ref":"app/models/a.rb:1"}')]
            _write_run(tmp, "r", "run-1", lines, ["a", "b"])
            _write_run(tmp, "r", "run-2", None, [])
            t = mt.build(tmp, "r", GOLD)
            self.assertEqual(t["measured_runs"], 1)
            self.assertEqual(t["rows_disagreeing"], [])


class DominantTieBreak(unittest.TestCase):
    def test_a_tie_breaks_toward_the_worse_news(self):
        self.assertEqual(mt._dominant({mt.REACH: 2, mt.MISSED: 2}), mt.MISSED)
        self.assertEqual(mt._dominant({mt.REACH: 2, mt.IGNORED: 2}), mt.IGNORED)


class Rendering(unittest.TestCase):
    def test_the_text_table_names_the_split_when_there_is_one(self):
        table = {"repo": "r", "gold_rows": 2, "routing": ["routed"], "measured_runs": 2,
                 "runs": [{"run": "run-1", "routing": "routed", "dominant": mt.REACH,
                           "counts": {mt.REACH: 2, mt.IGNORED: 0,
                                      mt.FOUND_ANYWAY: 0, mt.MISSED: 0}}],
                 "rows_disagreeing": ["b"], "verdict_split": True}
        out = mt.render(table)
        self.assertIn("VERDICT SPLIT", out)
        self.assertIn("b", out)


if __name__ == "__main__":
    unittest.main()
