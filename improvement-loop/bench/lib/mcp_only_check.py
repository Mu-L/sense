#!/usr/bin/env python3
"""mcp_only_check.py -- no loop check may query Sense through the CLI.

    python3 mcp_only_check.py [root]        # default: the bench/ tree above this file

Exit 0 = every script that queries Sense does it over MCP.
Exit 1 = a script shells a CLI query subcommand.

## WHY

The sense arm is wired to the clone's `.mcp.json` (`bench-sense-local.sh`), so the MCP server
is the only Sense surface the bench ever runs. The CLI diverges from it BY DESIGN - different
defaults, different caps, different budget - so a check that shells the CLI can pass gold the
agent will never see, or fail gold it gets for free. That is not a hypothetical: the gold gate
ran `sense blast` at the CLI's documented `--min-confidence` default and would have failed
every discriminator row in the rails scenario (STOPPER, 2026-07-31).

One law, mechanically enforced: query Sense through `mcp_probe.probe`, never through argv.

`scan`, `mcp` and `version` are not query subcommands - building and indexing a clone is the
CLI's job and stays on it. `status` is not matched either: it collides with the `status` key
used all over the reporting code, and a status call decides nothing about gold.
"""
import os
import re
import sys

QUERY_SUBCOMMANDS = ("blast", "graph", "search", "conventions", "dead")
_SUB = "|".join(QUERY_SUBCOMMANDS)

# A fresh list literal opening on a subcommand: ["blast", sym, ...]. The lookbehind keeps
# `d["status"]`-shaped subscripts out: an identifier or a closing bracket before the `[`
# means this is an index, not a command.
PY_LIST = re.compile(r'(?<![\w\]])\[\s*[\'"](' + _SUB + r')[\'"]')
# A binary token followed by a subcommand: ["sense", "blast", ...] or [SENSE_BIN, "graph"...]
PY_BIN = re.compile(r'(?:[\'"]sense[\'"]|SENSE_BIN|sense_bin|bin_path)\s*,\s*[\'"](' + _SUB
                    + r')[\'"]')
# Shell: "$SENSE_BIN" blast ... / sense graph ...
SH_BIN = re.compile(r'(?:\$\{?SENSE_BIN\}?"?|(?<![\w/-])sense)\s+(' + _SUB + r')\b')

SKIP_BASENAMES = {"mcp_only_check.py", "test_mcp_only_check.py"}


def patterns_for(path):
    if path.endswith(".py"):
        return (PY_LIST, PY_BIN)
    return (SH_BIN,)


def scan_text(path, text):
    """Every (line_no, line, subcommand) where this file queries Sense through the CLI."""
    hits = []
    for lineno, line in enumerate(text.splitlines(), start=1):
        if line.lstrip().startswith("#"):
            continue
        for pat in patterns_for(path):
            found = pat.search(line)
            if found:
                hits.append((lineno, line.strip(), found.group(1)))
                break
    return hits


def scan_tree(root):
    hits = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d != "__pycache__"]
        for name in sorted(filenames):
            if name in SKIP_BASENAMES or not name.endswith((".py", ".sh")):
                continue
            path = os.path.join(dirpath, name)
            with open(path, errors="replace") as fh:
                for lineno, line, sub in scan_text(path, fh.read()):
                    hits.append((os.path.relpath(path, root), lineno, line, sub))
    return hits


def report(hits, root):
    if not hits:
        print(f"## mcp-only check - {root}")
        print("   PASS - every Sense query in this tree goes through MCP.")
        return 0
    print(f"## mcp-only check - {root}")
    print(f"   FAIL - {len(hits)} CLI query call(s). The bench runs the MCP server, so a")
    print("   CLI call measures a surface no arm touches. Use mcp_probe.probe instead.")
    for path, lineno, line, sub in hits:
        print(f"     - {path}:{lineno}  (sense {sub})")
        print(f"         {line[:100]}")
    return 1


def main(argv):
    root = argv[1] if len(argv) > 1 else os.path.dirname(os.path.dirname(
        os.path.abspath(__file__)))
    return report(scan_tree(root), root)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
