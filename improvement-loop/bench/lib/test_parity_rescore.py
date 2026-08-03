#!/usr/bin/env python3
"""Behaviour pins for the matched-cost truncation (parity_rescore.py).

The load-bearing pin is `test_untimestamped_rows_survive`: the leading system/init block
carries no timestamp, and dropping it would hand the scorer a transcript with no session
header - a truncation that changes the ANSWER for a reason that has nothing to do with the
budget. The rest pin the boundary, so "at budget" means at-or-under, not under.
"""
import unittest
from datetime import datetime, timedelta

from parity_rescore import truncate, _parse_ts

T0 = datetime.fromisoformat("2026-08-03T12:00:00+00:00")


def row(kind, offset=None, **extra):
    r = {"type": kind}
    if offset is not None:
        r["timestamp"] = (T0 + timedelta(seconds=offset)).isoformat().replace("+00:00", "Z")
    r.update(extra)
    return r


class TruncateTest(unittest.TestCase):
    def test_untimestamped_rows_survive(self):
        """System/init rows have no timestamp and are session setup, not work."""
        rows = [row("system"), row("assistant", 0), row("assistant", 500)]
        kept, dropped = truncate(rows, 100)
        self.assertEqual([r["type"] for r in kept], ["system", "assistant"])
        self.assertEqual(dropped, 1)

    def test_the_boundary_is_inclusive(self):
        """A row landing exactly on the budget was produced within it."""
        rows = [row("assistant", 0), row("assistant", 100), row("assistant", 101)]
        kept, dropped = truncate(rows, 100)
        self.assertEqual(len(kept), 2)
        self.assertEqual(dropped, 1)

    def test_a_run_inside_the_budget_is_untouched(self):
        rows = [row("assistant", 0), row("assistant", 10)]
        kept, dropped = truncate(rows, 300)
        self.assertEqual(kept, rows)
        self.assertEqual(dropped, 0)

    def test_the_clock_starts_at_the_first_timestamp_not_the_epoch(self):
        """Offsets are relative to the session's own start; a late-starting session must
        not be truncated to nothing."""
        rows = [row("assistant", 1000), row("assistant", 1050), row("assistant", 1400)]
        kept, dropped = truncate(rows, 100)
        self.assertEqual(len(kept), 2)
        self.assertEqual(dropped, 1)

    def test_no_timestamps_at_all_keeps_everything(self):
        """Rather than silently scoring an empty transcript."""
        rows = [row("system"), row("system")]
        kept, dropped = truncate(rows, 5)
        self.assertEqual(kept, rows)
        self.assertEqual(dropped, 0)


class ParseTest(unittest.TestCase):
    def test_trailing_z_is_accepted(self):
        self.assertIsNotNone(_parse_ts("2026-08-03T12:00:00Z"))

    def test_garbage_is_not_a_crash(self):
        self.assertIsNone(_parse_ts("not-a-time"))
        self.assertIsNone(_parse_ts(None))


if __name__ == "__main__":
    unittest.main()
