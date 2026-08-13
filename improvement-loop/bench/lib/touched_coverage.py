#!/usr/bin/env python3
"""touched_coverage.py - the 94% gate on the files a branch actually touched.

`make ci` enforces the repository's 92% floor over the whole production tree. The
product window demands more of its own diff: every file AND function created or
modified must hold above 94% line and function coverage. Nothing enforced that. It was
read off `go tool cover` by the agent that wrote the code and reported in its artifact,
which is a claim, not a gate - and an agent grading its own coverage is the shape this
loop exists to remove.

So this reads the same `coverage.txt` the repository's own gate reads, resolves the
touched set from git, and exits non-zero when any touched file misses the floor.

Definitions, matched to the repository's gate so two numbers never disagree:
  line     covered statements / total statements, duplicate blocks merged by MAX count
  function functions exercised at all (any count > 0) / functions in the file

Usage:
  touched_coverage.py --root <repo> --base main --branch feat/x --profile coverage.txt
  touched_coverage.py ... --floor 94.0        # the default
"""
import argparse
import collections
import os
import subprocess
import sys

MODULE = "github.com/luuuc/sense/"


def touched_files(root, base, branch):
    """Production .go files added or modified between base and branch."""
    out = subprocess.run(
        ["git", "-C", root, "diff", "--name-only", "--diff-filter=d",
         "%s...%s" % (base, branch)],
        capture_output=True, text=True, check=True).stdout
    return sorted(
        p for p in out.splitlines()
        if p.endswith(".go") and not p.endswith("_test.go")
    )


def parse_profile(path):
    """profile -> {repo-relative file: (covered_stmts, total_stmts)}.

    Go writes one line per block: `<module>/<file>:s.c,e.c <nstmt> <count>`. The same
    block appears once per test binary that touched it, so blocks are merged by MAX
    count before anything is summed - taking the last line instead reports a block as
    uncovered because some other package's run did not reach it.
    """
    blocks = {}
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("mode:"):
                continue
            head, _, rest = line.partition(":")
            parts = rest.split()
            if len(parts) != 3:
                continue
            span, nstmt, count = parts[0], int(parts[1]), int(parts[2])
            name = head[len(MODULE):] if head.startswith(MODULE) else head
            key = (name, span)
            blocks[key] = (nstmt, max(count, blocks.get(key, (0, 0))[1]))
    totals = collections.defaultdict(lambda: [0, 0])
    for (name, _), (nstmt, count) in blocks.items():
        totals[name][1] += nstmt
        if count > 0:
            totals[name][0] += nstmt
    return {k: tuple(v) for k, v in totals.items()}


def parse_funcs(root, path):
    """`go tool cover -func` -> {repo-relative file: (funcs_exercised, funcs_total)}."""
    out = subprocess.run(["go", "tool", "cover", "-func=%s" % path],
                         capture_output=True, text=True, cwd=root).stdout
    totals = collections.defaultdict(lambda: [0, 0])
    for line in out.splitlines():
        parts = line.split("\t")
        if len(parts) < 3 or not parts[-1].endswith("%"):
            continue
        loc = parts[0].split(":")[0]
        if loc == "total":
            continue
        name = loc[len(MODULE):] if loc.startswith(MODULE) else loc
        totals[name][1] += 1
        if float(parts[-1].rstrip("%")) > 0:
            totals[name][0] += 1
    return {k: tuple(v) for k, v in totals.items()}


def pct(covered, total):
    return 100.0 if total == 0 else 100.0 * covered / total


def evaluate(files, lines, funcs, floor):
    """-> (rows, failures). A file with no statements is skipped, not failed."""
    rows, failures = [], []
    for f in files:
        lc, lt = lines.get(f, (0, 0))
        fc, ft = funcs.get(f, (0, 0))
        if lt == 0 and ft == 0:
            rows.append((f, None, None, "no statements"))
            continue
        line_pct, func_pct = pct(lc, lt), pct(fc, ft)
        ok = line_pct >= floor and func_pct >= floor
        rows.append((f, line_pct, func_pct, "" if ok else "BELOW FLOOR"))
        if not ok:
            failures.append(f)
    return rows, failures


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", required=True)
    ap.add_argument("--base", default="main")
    ap.add_argument("--branch", required=True)
    ap.add_argument("--profile", default="coverage.txt")
    ap.add_argument("--floor", type=float, default=94.0)
    args = ap.parse_args(argv)

    profile = args.profile
    if not os.path.isabs(profile):
        profile = os.path.join(args.root, profile)
    if not os.path.exists(profile):
        print("no coverage profile at %s - run `make ci` first" % profile)
        return 2

    files = touched_files(args.root, args.base, args.branch)
    if not files:
        print("no production .go files touched between %s and %s" % (args.base, args.branch))
        return 0

    rows, failures = evaluate(files, parse_profile(profile),
                              parse_funcs(args.root, profile), args.floor)
    print("touched-set coverage gate: floor %.1f%% (line AND function), %d file(s)"
          % (args.floor, len(files)))
    for name, line_pct, func_pct, note in rows:
        if line_pct is None:
            print("  %-52s %s" % (name, note))
        else:
            print("  %-52s line %6.1f%%  func %6.1f%%  %s"
                  % (name, line_pct, func_pct, note))
    if failures:
        print("FAIL: %d file(s) below the floor" % len(failures))
        return 1
    print("PASS: every touched file meets the floor")
    return 0


if __name__ == "__main__":
    sys.exit(main())
