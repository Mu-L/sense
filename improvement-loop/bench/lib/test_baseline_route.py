#!/usr/bin/env python3
"""Behaviour pins for the baseline route reader.

What it has to survive is the transcript format actually on disk: JSONL, one JSON object
per line, tool_use blocks nested inside message content. A reader that only parses a
whole-file JSON silently reports zero calls, and "the baseline made no searches" is the
most misleading thing this could say.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from baseline_route import command_of, is_search, tool_calls


def use(name, payload):
    return {"type": "tool_use", "name": name, "input": payload}


class RouteTest(unittest.TestCase):
    def setUp(self):
        self.d = tempfile.mkdtemp()

    def _write(self, name, lines):
        p = os.path.join(self.d, name)
        with open(p, "w") as fh:
            fh.write("\n".join(json.dumps(x) for x in lines))
        return p

    def test_it_reads_the_jsonl_transcript_the_runner_writes(self):
        p = self._write("t.json", [
            {"type": "assistant", "message": {"content": [use("Bash", {"command": "grep -rn X app"})]}},
            {"type": "assistant", "message": {"content": [use("Read", {"file_path": "app/a.rb"})]}},
        ])
        calls = tool_calls(p)
        self.assertEqual([n for n, _ in calls], ["Bash", "Read"])

    def test_it_reads_a_whole_file_json_transcript(self):
        p = os.path.join(self.d, "whole.json")
        with open(p, "w") as fh:
            json.dump({"messages": [{"content": [use("Bash", {"command": "ls"})]}]}, fh)
        self.assertEqual(len(tool_calls(p)), 1)

    def test_calls_keep_their_order(self):
        p = self._write("o.json", [
            {"c": [use("Bash", {"command": "first"})]},
            {"c": [use("Bash", {"command": "second"})]},
        ])
        self.assertEqual([command_of(n, i) for n, i in tool_calls(p)], ["first", "second"])

    def test_a_malformed_line_does_not_lose_the_rest(self):
        p = os.path.join(self.d, "bad.json")
        with open(p, "w") as fh:
            fh.write("{not json\n")
            fh.write(json.dumps({"c": [use("Bash", {"command": "grep -rn Y app"})]}) + "\n")
        self.assertEqual(len(tool_calls(p)), 1)

    def test_search_commands_are_recognised(self):
        self.assertTrue(is_search('grep -rn "Setting\\." app lib'))
        self.assertTrue(is_search("find . -name '*.rb'"))
        self.assertTrue(is_search("ls app/models"))
        self.assertFalse(is_search("cat -n app/models/setting.rb"))

    def test_command_of_falls_back_across_tool_shapes(self):
        self.assertEqual(command_of("Bash", {"command": "ls"}), "ls")
        self.assertEqual(command_of("Read", {"file_path": "a.rb"}), "a.rb")
        self.assertEqual(command_of("Grep", {"pattern": "Setting"}), "Setting")


if __name__ == "__main__":
    unittest.main()
