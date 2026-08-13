#!/usr/bin/env python3
"""Behaviour pins for the budget-trim audit.

The load-bearing pin is `test_pairs_responses_to_the_tool_that_asked`: the audit's
whole value is saying WHICH tool put the bytes in context. MCP request and response
are separate JSONL records joined only by `id` - lose that join and the report
becomes an undifferentiated byte count that names no trim candidate.

`test_re_read_multiplier` pins the insight the file exists for: cost is injected
size TIMES the turns that re-read it, which is why trimming is a product lever.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from context_cost_audit import CHARS_PER_TOKEN, metric_mean, parse_io, report


def _io(calls):
    """calls = [(id, tool, response_text)] -> sense-io.jsonl lines."""
    out = []
    for cid, tool, text in calls:
        out.append({"dir": "c2s", "msg": {"id": cid, "method": "tools/call",
                                          "params": {"name": tool}}})
        out.append({"dir": "s2c", "msg": {"id": cid,
                                          "result": {"content": [{"text": text}]}}})
    return "\n".join(json.dumps(o) for o in out) + "\n"


class ParseTest(unittest.TestCase):
    def _write(self, body):
        tmp = tempfile.mkdtemp()
        p = os.path.join(tmp, "sense-io.jsonl")
        with open(p, "w") as fh:
            fh.write(body)
        return p

    def test_pairs_responses_to_the_tool_that_asked(self):
        p = self._write(_io([(1, "sense_blast", "x" * 400),
                             (2, "sense_graph", "y" * 100)]))
        self.assertEqual(parse_io(p), [("sense_blast", 400, 0), ("sense_graph", 100, 1)])

    def test_handshake_records_are_not_tool_calls(self):
        """initialize/notifications carry no tool and must not become rows."""
        body = json.dumps({"dir": "c2s", "msg": {"id": 0, "method": "initialize"}}) + "\n"
        self.assertEqual(parse_io(self._write(body + _io([(1, "sense_search", "z")]))),
                         [("sense_search", 1, 0)])

    def test_a_request_with_no_response_counts_zero_not_crash(self):
        body = json.dumps({"dir": "c2s", "msg": {"id": 7, "method": "tools/call",
                                                 "params": {"name": "sense_blast"}}}) + "\n"
        self.assertEqual(parse_io(self._write(body)), [("sense_blast", 0, 0)])

    def test_malformed_lines_are_skipped(self):
        self.assertEqual(parse_io(self._write("not json\n\n" + _io([(1, "t", "ab")]))),
                         [("t", 2, 0)])

    def test_multi_block_responses_sum(self):
        body = json.dumps({"dir": "c2s", "msg": {"id": 1, "method": "tools/call",
                                                 "params": {"name": "t"}}}) + "\n"
        body += json.dumps({"dir": "s2c", "msg": {"id": 1, "result": {
            "content": [{"text": "aa"}, {"text": "bbb"}]}}}) + "\n"
        self.assertEqual(parse_io(self._write(body)), [("t", 5, 0)])


class ReportTest(unittest.TestCase):
    def _cell(self, cache_read_baseline, cache_read_sense, calls):
        root = tempfile.mkdtemp()
        for arm, cr in (("baseline", cache_read_baseline), ("sense", cache_read_sense)):
            d = os.path.join(root, arm, "repo", "run-1")
            os.makedirs(d)
            with open(os.path.join(d, "scored.json"), "w") as fh:
                json.dump({"metrics": {"token_cache_read": cr}}, fh)
        with open(os.path.join(root, "sense", "repo", "run-1", "sense-io.jsonl"), "w") as fh:
            fh.write(_io(calls))
        return root

    def test_re_read_multiplier(self):
        """1000 tokens injected, 10000 extra cache-read paid -> 10x."""
        chars = 1000 * CHARS_PER_TOKEN
        root = self._cell(0, 10_000, [(1, "sense_blast", "x" * chars)])
        import io
        from contextlib import redirect_stdout
        buf = io.StringIO()
        with redirect_stdout(buf):
            report(root, "repo")
        self.assertIn("10.0x", buf.getvalue())

    def test_no_capture_is_a_clean_error(self):
        root = tempfile.mkdtemp()
        with self.assertRaises(SystemExit):
            report(root, "repo")

    def test_metric_mean_of_absent_key_is_zero(self):
        root = self._cell(5, 5, [(1, "t", "x")])
        self.assertEqual(metric_mean(root, "sense", "repo", "nope"), 0)


if __name__ == "__main__":
    unittest.main()
