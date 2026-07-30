#!/usr/bin/env python3
"""sense_build.py -- the identity of a Sense BUILD, and the expiry key for a probe verdict.

    python3 sense_build.py [--bin PATH]           # show this build's identity
    python3 sense_build.py --stamp <probe.md>     # write <probe.md>.build.json beside it

## WHY A BUILD IDENTITY AND NOT A VERSION STRING

A bound verdict (03-a-eligibility.md) is scoped to the Sense build that produced it: Loop 7
ships fixes between verticals, and a fix can turn a correctly-killed cell live. So "has Sense
changed since this probe ran" must be answerable mechanically. The version string CANNOT
answer it, for three separate reasons, each measured on this repo:

  1. `make build` stamped `VERSION ?= dev`, so EVERY local build reported the same string --
     `sense dev`, with no number in it at all. The dev label (release-relative, e.g.
     1.13.3-dev) fixes the human half of this, and only the human half.
  2. Even a correct dev label repeats across every iteration of ONE Loop 7 spike -- and
     Loop 3 invokes Loop 7 more than once per vertical, which is the normal case, not the
     edge case.
  3. `sense_ref` + `sense_dirty` (already stamped into run_meta.json by all three runners)
     cannot separate two DIFFERENT dirty trees on the same commit. A spike-rebuild-reprobe
     loop is exactly that: same commit, still dirty, different bytes.

So the EXPIRY KEY is the sha256 of the binary itself. It changes when, and only when, the
bytes the probe ran against change -- across commits, across dirty edits, across cherry-picks,
across a revert that lands back on an old commit. The version LABEL rides along for humans
answering the other question: WHICH fix is this?

    label -> which enhancement/fix (for the ledger, for a person)
    key   -> is this the same Sense (for the machine, for expiry)

Reading the key is $0 and offline: one file hash, no subprocess. The label needs the binary to
run, so it degrades to None rather than failing -- a missing label never blocks an expiry check.
"""
import hashlib
import json
import os
import subprocess
import sys

STAMP_SUFFIX = ".build.json"
KEY_LEN = 12  # sha256 prefix; collision risk is irrelevant for "did these bytes change"


def default_bin():
    """The binary a bench run would use: SENSE_BIN, else the installed path, else the repo build."""
    if os.environ.get("SENSE_BIN"):
        return os.environ["SENSE_BIN"]
    installed = os.path.expanduser(
        os.environ.get("SENSE_INSTALL_PATH", "~/.local/bin/sense"))
    if os.path.exists(installed):
        return installed
    return "bin/sense"


def binary_key(bin_path):
    """sha256 prefix of the binary's bytes. The expiry key: no subprocess, no git, no clock."""
    h = hashlib.sha256()
    with open(bin_path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()[:KEY_LEN]


def version_label(bin_path):
    """`sense --version`, or None. Advisory: the label names the fix, it never gates."""
    try:
        out = subprocess.run([bin_path, "--version"], capture_output=True,
                             text=True, timeout=10)
    except (OSError, subprocess.SubprocessError):
        return None
    line = (out.stdout or out.stderr or "").strip().splitlines()
    return line[0].strip() if line else None


def build_identity(bin_path=None):
    bin_path = bin_path or default_bin()
    if not os.path.exists(bin_path):
        raise SystemExit(f"sense_build: no binary at {bin_path} (set SENSE_BIN)")
    return {
        "sense_build_key": binary_key(bin_path),
        "sense_build_label": version_label(bin_path),
        "sense_bin": os.path.abspath(bin_path),
        "sense_bin_bytes": os.path.getsize(bin_path),
    }


def stamp_path(probe_path):
    return probe_path + STAMP_SUFFIX


def stamp(probe_path, bin_path=None):
    """Record the build a probe ran against, beside the probe. Written once, never edited."""
    ident = build_identity(bin_path)
    with open(stamp_path(probe_path), "w") as fh:
        json.dump(ident, fh, indent=1, sort_keys=True)
        fh.write("\n")
    return ident


def read_stamp(probe_path):
    """The recorded identity, or None if the probe was never stamped."""
    try:
        with open(stamp_path(probe_path)) as fh:
            return json.load(fh)
    except (OSError, ValueError):
        return None


def freshness(probe_path, current_key):
    """FRESH / STALE / UNSTAMPED for one probe against the build in hand.

    UNSTAMPED is deliberately NOT treated as stale: they are different failures. A stale
    probe has a known, superseded build; an unstamped probe has no provenance at all and
    cannot be trusted in either direction. Collapsing them would let an unstamped probe
    look like a merely-old one and get silently re-probed instead of investigated.
    """
    rec = read_stamp(probe_path)
    if rec is None:
        return "UNSTAMPED"
    return "FRESH" if rec.get("sense_build_key") == current_key else "STALE"


def provenance_fragment(ident):
    """The `sense ...` fragment of a LEDGER Provenance line (see provenance_line.py)."""
    label = ident.get("sense_build_label") or "sense (unknown label)"
    return f"{label} [build {ident['sense_build_key']}]"


def main(argv):
    args = argv[1:]
    bin_path = None
    if "--bin" in args:
        i = args.index("--bin")
        bin_path = args[i + 1]
        del args[i:i + 2]

    if args and args[0] == "--key":
        # The runners' hook: one line, no JSON, empty on any failure so a stamping
        # step can never take a bench run down with it.
        try:
            print(binary_key(bin_path or default_bin()))
        except OSError:
            return 1
        return 0

    if args and args[0] == "--stamp":
        if len(args) != 2:
            raise SystemExit("usage: sense_build.py --stamp <probe.md> [--bin PATH]")
        ident = stamp(args[1], bin_path)
        print(f"stamped {stamp_path(args[1])}")
        print(f"  {provenance_fragment(ident)}")
        return 0

    ident = build_identity(bin_path)
    print(json.dumps(ident, indent=1, sort_keys=True))
    print()
    print(f"provenance fragment: {provenance_fragment(ident)}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
