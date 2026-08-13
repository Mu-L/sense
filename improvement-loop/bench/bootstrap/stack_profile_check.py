#!/usr/bin/env python3
"""Check a stack profile before anything is stamped, cloned or hunted.

    stack_profile_check.py <key> [--conf stacks/<key>.conf]

A stack profile is a hand-written bootstrap prerequisite (stacks/README.md), so
it is checked the way the vertical queue is: a missing or incomplete one stops
the pipeline with a named reason. Exit 0 = usable, 1 = not, and the findings say
which key is wrong.

It checks SHAPE, never content: whether `rails/rails` belongs in the framework
list, or whether a needle actually matches anything, is not knowable from the
file. What is knowable is that a `hunt:` line written as a query string will be
rejected by gh at run time, and that costs a whole hunt - so that one is caught
here.
"""

import argparse
import os
import sys

WARN_QUERY_STRING = ("looks like a query string, not gh ARGV: a pure-qualifier "
                     "search collapses to one search TERM and gh rejects it. "
                     "Write `--language php --size \">40000\"`.")


def parse(path):
    keys = {"stack": [], "hunt": [], "framework": [], "repo": []}
    unknown = []
    with open(path, encoding="utf-8") as fh:
        for n, line in enumerate(fh, 1):
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            head, sep, rest = line.partition(":")
            head, rest = head.strip(), rest.strip()
            if not sep:
                unknown.append((n, line))
            elif head in keys:
                keys[head].append((n, rest))
            else:
                unknown.append((n, line))
    return keys, unknown


def check(path):
    if not os.path.exists(path):
        return [f"no stack profile at {path} - write one, see stacks/README.md"]
    keys, unknown = parse(path)
    out = []

    if len(keys["stack"]) != 1:
        out.append(f"expected exactly one `stack:` line, found {len(keys['stack'])}")
    else:
        _, marker = keys["stack"][0]
        manifest, sep, needles = marker.partition(":")
        if not sep or not manifest.strip():
            out.append("`stack:` must be `<manifest>:<needle>|<needle>...`")
        elif not [n for n in needles.split("|") if n.strip()]:
            out.append("`stack:` has a manifest but no needles")

    if not keys["hunt"]:
        out.append("no `hunt:` query - the pool would have no declared source, "
                   "and 'the pool is exhausted' could never be re-checked")
    for n, q in keys["hunt"]:
        # The failure that cost 3 of 4 queries: qualifiers written as a string.
        if ":" in q.split("--")[0] and not q.lstrip().startswith("-"):
            out.append(f"line {n}: {WARN_QUERY_STRING}")

    for n, line in unknown:
        out.append(f"line {n}: unrecognised, expected stack:/hunt:/framework:/repo: - {line[:60]}")
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("key")
    ap.add_argument("--conf", default=None)
    args = ap.parse_args()
    root = os.path.normpath(os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    conf = args.conf or os.path.join(root, "stacks", f"{args.key}.conf")

    findings = check(conf)
    if findings:
        print(f"stack profile {args.key}: {len(findings)} finding(s)")
        for f in findings:
            print(f"  - {f}")
        return 1
    keys, _ = parse(conf)
    print(f"stack profile {args.key}: OK - 1 marker, {len(keys['hunt'])} "
          f"hunt query/queries, {len(keys['framework'])} framework-role repo(s), "
          f"{len(keys['repo'])} listed repo(s)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
