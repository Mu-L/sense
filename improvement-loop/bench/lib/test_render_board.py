"""Behaviour pins for the cycle 2 board renderer (deterministic, no agent)."""
import unittest

import render_board as rb


def _mech(counts_per_run, dominant, routing=("routed",), **kw):
    runs = [{"routing": routing[0], "counts": c} for c in counts_per_run]
    out = {"runs": runs, "measured_runs": len(runs), "dominant": dominant,
           "verdict_split": False, "rows_disagreeing": [], "gold_rows": 20}
    out.update(kw)
    return out


def _col(mech, routing=("routed",), model="m", delta=0.6):
    return {"model": model, "source": "benched", "measured": True,
            "runs": {"baseline": 2, "sense": 2},
            "overall": {"baseline_mean": 0.30, "sense_mean": 0.90, "delta": delta},
            "best_group_delta": delta, "billed_tokens": {},
            "routing": list(routing), "mechanism": mech}


def _counts(reach=0, ignored=0, found=0, missed=0):
    return {rb.REACH: reach, rb.IGNORED: ignored,
            rb.FOUND_ANYWAY: found, rb.MISSED: missed}


class TheWhyLine(unittest.TestCase):
    """One templated sentence per dominant cell. No agent in the render path."""

    def test_reach_says_the_product_worked(self):
        line = rb._why_line(_col(_mech([_counts(reach=18)], rb.REACH)), 20)
        self.assertIn("returned 18 of 20 rows", line)
        self.assertIn("product working", line)

    def test_ignored_names_it_as_ours(self):
        line = rb._why_line(_col(_mech([_counts(ignored=15)], rb.IGNORED)), 20)
        self.assertIn("went unused", line)
        self.assertIn("our side", line)

    def test_found_anyway_says_sense_was_not_what_got_it_there(self):
        line = rb._why_line(_col(_mech([_counts(found=16)], rb.FOUND_ANYWAY)), 20)
        self.assertIn("no Sense call returned", line)
        self.assertIn("coverage gap", line)

    def test_missed_says_the_gap_cost_the_answer(self):
        line = rb._why_line(_col(_mech([_counts(missed=14)], rb.MISSED)), 20)
        self.assertIn("neither returned by Sense nor cited", line)


class SecondaryCells(unittest.TestCase):
    """A dominant cell alone printed 'the product working' over a 32% gap."""

    def test_a_large_second_cell_is_reported_beside_the_dominant_one(self):
        col = _col(_mech([_counts(reach=21, found=12, ignored=4, missed=1)], rb.REACH))
        line = rb._why_line(col, 38)
        self.assertIn("product working", line)
        self.assertIn("cited 12 rows that no Sense call returned", line)

    def test_a_small_second_cell_stays_out_of_the_sentence(self):
        col = _col(_mech([_counts(reach=36, found=2)], rb.REACH))
        line = rb._why_line(col, 38)
        self.assertIn("product working", line)
        self.assertNotIn("no Sense call returned", line)

    def test_the_worst_news_is_said_first_among_secondaries(self):
        col = _col(_mech([_counts(reach=10, missed=5, found=5)], rb.REACH))
        line = rb._why_line(col, 20)
        self.assertLess(line.index("neither returned"), line.index("by reading"))


class RoutingOverridesTheTable(unittest.TestCase):
    """An arm that never asked Sense is not an arm reporting on Sense."""

    def test_never_routed_says_the_delta_measures_configuration(self):
        line = rb._why_line(_col(_mech([], None), routing=("never-routed",)), 20)
        self.assertIn("never called Sense", line)
        self.assertIn("excluded from the replication count", line)

    def test_search_only_says_the_question_was_never_asked(self):
        line = rb._why_line(_col(_mech([], None), routing=("search-only",)), 20)
        self.assertIn("never reached a resolver", line)

    def test_a_harness_failure_is_not_a_measurement(self):
        line = rb._why_line(_col(_mech([], None), routing=("harness-failure",)), 20)
        self.assertIn("Not a measurement", line)

    def test_a_verdict_split_asks_for_a_third_run(self):
        col = _col(_mech([_counts(reach=18)], rb.REACH, verdict_split=True))
        self.assertIn("A third run rules", rb._why_line(col, 20))


class WholeNumbersPrintWhole(unittest.TestCase):
    def test_a_mean_of_twenty_one_is_not_twenty_one_point_zero(self):
        self.assertEqual(rb._n(21.0), "21")
        self.assertEqual(rb._n(21.5), "21.5")


class Replication(unittest.TestCase):
    def test_with_no_routed_arm_the_board_says_it_cannot_tell(self):
        rep = {"routed": [], "replicated": [], "never_routed": [], "search_only": [],
               "not_measured": ["a", "b"], "threshold": 0.5}
        text = "\n".join(rb._replication_block(rep, 2))
        self.assertIn("cannot say whether the win replicates", text)
        self.assertNotIn("0 of 0", text)

    def test_routed_arms_are_counted_not_ranked(self):
        rep = {"routed": ["a", "b"], "replicated": ["a"], "never_routed": ["c"],
               "search_only": [], "not_measured": [], "threshold": 0.5}
        text = "\n".join(rb._replication_block(rep, 3))
        self.assertIn("**1 of 2**", text)
        self.assertIn("counts, not a ranking", text)
        self.assertIn("`c`", text)


class ThePage(unittest.TestCase):
    def _data(self):
        return {"repo": "discourse", "vertical": "ruby-rails", "gold_rows": 20,
                "scenario_version": "sha256:abcd", "sense_version": "sense 1.13.5",
                "headline": "claude-opus-5",
                "columns": [_col(_mech([_counts(reach=18)], rb.REACH),
                                 model="claude-opus-5"),
                            {"model": "gpt", "source": "benched", "measured": False,
                             "reason": "no scored runs under this arm's root"}],
                "replication": {"routed": [], "replicated": [], "never_routed": [],
                                "search_only": [], "not_measured": ["gpt"],
                                "threshold": 0.5}}

    def test_the_same_numbers_render_the_same_bytes(self):
        self.assertEqual(rb.render(self._data()), rb.render(self._data()))

    def test_the_reading_sits_above_the_detail(self):
        out = rb.render(self._data())
        self.assertLess(out.index("<!-- reading -->"), out.index("## Per model"))

    def test_the_page_states_it_is_not_a_model_comparison(self):
        self.assertIn("audits **Sense**, not the models", rb.render(self._data()))

    def test_an_unbenched_arm_says_so_instead_of_showing_a_zero(self):
        out = rb.render(self._data())
        self.assertIn("Not measured: no scored runs under this arm's root.", out)

    def test_no_em_dashes_reach_a_published_page(self):
        self.assertNotIn("—", rb.render(self._data()))


if __name__ == "__main__":
    unittest.main()
