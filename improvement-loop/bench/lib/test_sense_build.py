#!/usr/bin/env python3
"""Behaviour pins for the probe-expiry key.

The load-bearing pin is `test_same_commit_different_bytes_expires`: the whole reason this
module exists is that a version string and a commit+dirty pair BOTH fail to separate two
builds of one Loop 7 spike. If someone "simplifies" the key back to a version label, that
test goes red and says what it costs.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from sense_build import (binary_key, build_identity, freshness, provenance_fragment,
                         read_stamp, stamp, stamp_path)


def _fake_bin(tmp, content, name="sense"):
    p = os.path.join(tmp, name)
    with open(p, "wb") as fh:
        fh.write(content)
    return p


class BuildKeyTest(unittest.TestCase):
    def test_same_bytes_same_key(self):
        with tempfile.TemporaryDirectory() as t:
            a = _fake_bin(t, b"binary-v1", "a")
            b = _fake_bin(t, b"binary-v1", "b")
            self.assertEqual(binary_key(a), binary_key(b))

    def test_same_commit_different_bytes_expires(self):
        """Two dirty builds of ONE spike: same commit, same label, different bytes.

        This is the case the version string and sense_ref+sense_dirty both miss, and it is
        the normal case when Loop 3 invokes Loop 7 more than once.
        """
        with tempfile.TemporaryDirectory() as t:
            first = _fake_bin(t, b"spike-attempt-1", "a")
            second = _fake_bin(t, b"spike-attempt-2", "b")
            self.assertNotEqual(binary_key(first), binary_key(second))

    def test_one_byte_changes_the_key(self):
        with tempfile.TemporaryDirectory() as t:
            a = _fake_bin(t, b"x" * 4096, "a")
            b = _fake_bin(t, b"x" * 4095 + b"y", "b")
            self.assertNotEqual(binary_key(a), binary_key(b))

    def test_missing_binary_is_a_clean_error(self):
        with self.assertRaises(SystemExit):
            build_identity("/nonexistent/sense")

    def test_label_degrades_to_none_and_never_blocks(self):
        """A non-executable file still yields a usable key: the label is advisory."""
        with tempfile.TemporaryDirectory() as t:
            ident = build_identity(_fake_bin(t, b"not-executable"))
            self.assertEqual(len(ident["sense_build_key"]), 12)
            self.assertIsNone(ident["sense_build_label"])
            self.assertIn("unknown label", provenance_fragment(ident))


class StampTest(unittest.TestCase):
    def test_stamp_roundtrips(self):
        with tempfile.TemporaryDirectory() as t:
            b = _fake_bin(t, b"build-A")
            probe = os.path.join(t, "probe-1.md")
            with open(probe, "w") as fh:
                fh.write("control probe answer")
            ident = stamp(probe, b)
            self.assertTrue(os.path.exists(stamp_path(probe)))
            self.assertEqual(read_stamp(probe)["sense_build_key"], ident["sense_build_key"])

    def test_freshness_three_states(self):
        with tempfile.TemporaryDirectory() as t:
            old = _fake_bin(t, b"build-A", "old")
            new = _fake_bin(t, b"build-B", "new")
            probe = os.path.join(t, "probe-1.md")
            with open(probe, "w") as fh:
                fh.write("answer")

            unstamped = os.path.join(t, "probe-2.md")
            with open(unstamped, "w") as fh:
                fh.write("answer")

            stamp(probe, old)
            self.assertEqual(freshness(probe, binary_key(old)), "FRESH")
            self.assertEqual(freshness(probe, binary_key(new)), "STALE")
            # UNSTAMPED is its own state, never folded into STALE: no provenance at all is
            # a different problem from superseded provenance.
            self.assertEqual(freshness(unstamped, binary_key(old)), "UNSTAMPED")

    def test_corrupt_stamp_reads_as_unstamped(self):
        with tempfile.TemporaryDirectory() as t:
            probe = os.path.join(t, "probe-1.md")
            with open(probe, "w") as fh:
                fh.write("answer")
            with open(stamp_path(probe), "w") as fh:
                fh.write("{not json")
            self.assertIsNone(read_stamp(probe))
            self.assertEqual(freshness(probe, "abc123"), "UNSTAMPED")


if __name__ == "__main__":
    unittest.main()
