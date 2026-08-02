#!/usr/bin/env python3
"""Behaviour pins for the scored probe gate.

Both directions are pinned, because the gate this replaces could only ever say DEAD.
`test_a_thin_probe_survives` is the one that matters: a probe that pins the contract
centre and two peripheral rows out of twelve is the shape a bench WANTS, and the
self-graded verdict called that ASSEMBLED on the record.
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from probe_score import covers, paths, score, section


def shape(pool_rows):
    return "\n".join([
        "# Contract", "", "`Thing` - app/models/thing.rb:1", "",
        "# Periphery pool", "",
    ] + ["    %s   why it is here" % r for r in pool_rows] + ["", "# Anchors", "",
                                                             "    app/models/thing.rb"])


def probe(result, covered_rows):
    return "\n".join([
        "# Result", "", result, "",
        "# Method", "", "grep, then grep again - app/models/thing.rb:1", "",
        "# Covered", "",
    ] + ["- %s - one phrase" % r for r in covered_rows] + ["", "# Disclaimed", "",
                                                           "I never opened the workers."])


POOL = ["app/lib/feed_manager.rb:104",
        "app/models/feed.rb:41",
        "app/models/link_feed.rb:12",
        "app/services/backup_service.rb:129",
        "app/services/report_service.rb:60",
        "app/presenters/annual_reports_presenter.rb:7",
        "app/workers/email_distribution_worker.rb:9",
        "app/workers/unfilter_notifications_worker.rb:14",
        "app/models/concerns/account/merging.rb:41",
        "app/lib/permalink_redirector.rb:31",
        "app/models/account_warning.rb:45",
        "app/models/media_attachment.rb:33"]


class ScoreTest(unittest.TestCase):
    def test_a_thin_probe_survives(self):
        """2 of 12 pinned leaves a +0.833 ceiling - the shape a bench wants."""
        _, covered, coverage, ceiling = score(
            probe("ASSEMBLED - I pinned the whole contract.", POOL[:2]), shape(POOL))
        self.assertEqual(len(covered), 2)
        self.assertAlmostEqual(coverage, 2 / 12)
        self.assertAlmostEqual(ceiling, 1 - 2 / 12)
        self.assertGreaterEqual(ceiling, 0.50)

    def test_a_covering_probe_is_dead(self):
        """7 of 12 pinned caps the delta at +0.417, under the +0.50 floor."""
        _, covered, coverage, ceiling = score(
            probe("DISCLAIMED - the workers beat me.", POOL[:7]), shape(POOL))
        self.assertEqual(len(covered), 7)
        self.assertAlmostEqual(ceiling, 1 - 7 / 12)
        self.assertLess(ceiling, 0.50)

    def test_the_floor_is_the_boundary(self):
        """Exactly half covered still reaches +0.50, mirroring pay_ceiling's `>=`."""
        _, _, _, ceiling = score(probe("ASSEMBLED", POOL[:6]), shape(POOL))
        self.assertAlmostEqual(ceiling, 0.50)
        self.assertGreaterEqual(ceiling, 0.50)

    def test_the_method_section_does_not_credit_the_probe(self):
        """Only `# Covered` scores; a path walked in Method is not an answer."""
        text = probe("ASSEMBLED", [])
        text += "\n\n" + "\n".join("- %s" % r for r in POOL)  # trailing prose, no heading
        _, _, coverage, _ = score(text, shape(POOL))
        self.assertIsNone(coverage)

    def test_a_bare_basename_does_not_credit_the_probe(self):
        """Loose matching would kill good shapes, so `feed.rb` alone scores nothing."""
        self.assertFalse(covers("app/models/feed.rb", "feed.rb"))
        self.assertTrue(covers("app/models/feed.rb", "models/feed.rb"))
        self.assertTrue(covers("concerns/account/merging.rb",
                               "app/models/concerns/account/merging.rb"))

    def test_a_filename_without_a_line_scores_nothing(self):
        """The grading bar is path:line; a filename alone is not a citation."""
        self.assertEqual(paths("- app/models/feed.rb - hydrates statuses"), set())
        self.assertEqual(paths("- app/models/feed.rb:41"), {"app/models/feed.rb"})

    def test_an_empty_pool_is_not_a_pass(self):
        """A shape with no pool cannot be scored, and unscored is not survived."""
        pool, _, coverage, _ = score(probe("ASSEMBLED", POOL), shape([]))
        self.assertEqual(pool, [])
        self.assertIsNone(coverage)

    def test_section_stops_at_the_next_heading(self):
        body = section(shape(POOL), "Periphery pool")
        self.assertIn("feed_manager.rb", body)
        self.assertNotIn("Anchors", body)
        self.assertNotIn("app/models/thing.rb:1", body)


if __name__ == "__main__":
    unittest.main()
