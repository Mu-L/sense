#!/usr/bin/env python3
"""baseline_route.py - how did the BASELINE actually assemble its answer?

The pay decision keeps asking "did the baseline reach the gold" and answering it from a
recall number. The number says how much it reached; only the route says HOW, and the how
is what tells you whether the scenario is sound. A baseline that runs one grep and then
opens exactly the files the ask enumerated has not been beaten by a hard task - it has been
handed a checklist. That is a defect in the ASK, and recall alone cannot see it.

Measured, the run this exists for: 24 tool calls, all Bash, of which command 4 was one
`grep -rn "Setting\\."` over the whole repo and the rest were `cat -n` of the files the
step-4 ask had named by function. Cited recall 22 of 22.

  baseline_route.py <transcript.json> [--full]

Prints every tool call in order, search commands first-class, with a one-line shape
summary. Reads the JSONL-per-line transcripts the runner writes; a whole-file JSON
transcript also works.
"""
import json
import sys
from collections import Counter

SEARCH = ("grep", "rg ", "find ", "ls ", "ag ")


def tool_calls(path):
    """Every tool_use in the transcript, in order, as (name, input dict)."""
    calls = []

    def walk(node):
        if isinstance(node, dict):
            if node.get("type") == "tool_use":
                calls.append((node.get("name", "?"), node.get("input") or {}))
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)

    with open(path) as fh:
        body = fh.read()
    try:
        walk(json.loads(body))
    except ValueError:
        for line in body.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                walk(json.loads(line))
            except ValueError:
                continue
    return calls


def command_of(name, payload):
    for key in ("command", "pattern", "file_path", "path", "query"):
        if payload.get(key):
            return str(payload[key])
    return json.dumps(payload)[:200]


def is_search(text):
    return any(token in text for token in SEARCH)


def main(argv):
    if not argv:
        print("usage: baseline_route.py <transcript.json> [--full]", file=sys.stderr)
        return 64
    full = "--full" in argv
    try:
        calls = tool_calls(argv[0])
    except OSError as exc:
        print("baseline_route: %s" % exc, file=sys.stderr)
        return 64
    if not calls:
        print("baseline_route: no tool calls in %s" % argv[0], file=sys.stderr)
        return 1

    kinds = Counter(name for name, _ in calls)
    searches = [(i, command_of(n, p)) for i, (n, p) in enumerate(calls, 1)
                if is_search(command_of(n, p))]
    print("tool calls: %d   %s" % (len(calls), dict(kinds)))
    print("search commands: %d of %d\n" % (len(searches), len(calls)))
    width = 190 if not full else 10_000
    for i, cmd in searches:
        print("%3d. %s" % (i, cmd[:width].replace("\n", " ")))
    if full:
        print("\n--- every call in order ---")
        for i, (name, payload) in enumerate(calls, 1):
            print("%3d. %-8s %s" % (i, name, command_of(name, payload)[:width].replace("\n", " ")))
    print("\nRead this against the ask. If one search returned the candidate set and the "
          "rest are\nreads of files the ask named by function, the ask is an inventory and "
          "the cell is\nmeasuring the prompt, not the tool.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
