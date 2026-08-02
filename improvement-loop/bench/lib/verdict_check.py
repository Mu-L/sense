#!/usr/bin/env python3
"""verdict_check.py - the require() half of the driver's phase guard.

A phase agent's exit code is a claim; the verdict on disk is the fact. Every phase
that hands its judgment back to `vertical-loop.sh` writes one small JSON, and the
driver refuses to advance until this script says the JSON is real:

  - it parses,
  - it names the phase and repo the driver actually ran,
  - its verdict is one the phase is allowed to return,
  - and the artifact it claims to have written EXISTS.

That last one is the whole point. A runner returning 0 with nothing on disk is how a
cell reached a paid bench on an absent artefact; an agent reporting "done" with no
file is the same defect one layer up.

On success the verdict is printed on stdout, so the driver routes on it:

    v=$(python3 verdict_check.py <file> --phase scout --repo mastodon \
          --allow SHAPE,NO-AXIS --root improvement-loop) || exit 1
"""
import argparse
import json
import os
import sys

REQUIRED = ("phase", "repo", "verdict", "artifact")


def check(path, phase, repo, allow, root):
    """Return (verdict, None) when the verdict is usable, else (None, reason)."""
    if not os.path.exists(path):
        return None, "no verdict on disk at %s" % path
    try:
        with open(path) as fh:
            data = json.load(fh)
    except (OSError, ValueError) as exc:
        return None, "the verdict does not parse: %s" % exc
    if not isinstance(data, dict):
        return None, "the verdict is not a JSON object"
    missing = [k for k in REQUIRED if not data.get(k)]
    if missing:
        return None, "the verdict is missing %s" % ", ".join(missing)
    if data["phase"] != phase:
        return None, "the verdict is for phase %r, not %r" % (data["phase"], phase)
    if data["repo"] != repo:
        return None, "the verdict is for repo %r, not %r" % (data["repo"], repo)
    if data["verdict"] not in allow:
        return None, "verdict %r is not one of %s" % (data["verdict"], "/".join(allow))
    artifact = os.path.join(root, data["artifact"])
    if not os.path.exists(artifact):
        return None, "the named artifact does not exist: %s" % data["artifact"]
    return data["verdict"], None


def main(argv):
    ap = argparse.ArgumentParser()
    ap.add_argument("verdict_file")
    ap.add_argument("--phase", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--allow", required=True, help="comma-separated allowed verdicts")
    ap.add_argument("--root", default=".", help="what the artifact path is relative to")
    args = ap.parse_args(argv)

    verdict, reason = check(
        args.verdict_file, args.phase, args.repo,
        [a.strip() for a in args.allow.split(",") if a.strip()], args.root,
    )
    if reason:
        print("verdict_check: %s" % reason, file=sys.stderr)
        return 1
    print(verdict)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
