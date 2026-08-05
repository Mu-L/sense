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
    """Templated sentences, ordered by cell size. No agent in the render path."""

    def test_reach_says_the_product_worked(self):
        line = rb._why_line(_col(_mech([_counts(reach=18)], rb.REACH)), 20)
        self.assertIn("put 18 of the 20 answers", line)
        self.assertIn("the model used them", line)

    def test_ignored_names_it_as_ours(self):
        line = rb._why_line(_col(_mech([_counts(ignored=15)], rb.IGNORED)), 20)
        self.assertIn("did not make it into the answer", line)
        self.assertIn("ours", line)

    def test_found_anyway_says_sense_was_not_what_got_it_there(self):
        line = rb._why_line(_col(_mech([_counts(found=16)], rb.FOUND_ANYWAY)), 20)
        self.assertIn("reached on its own", line)
        self.assertIn("did not shorten", line)

    def test_missed_says_the_gap_cost_the_answer(self):
        line = rb._why_line(_col(_mech([_counts(missed=14)], rb.MISSED)), 20)
        self.assertIn("reached by neither", line)


class SecondaryCells(unittest.TestCase):
    """One cell alone printed "Sense supplied 21 of 38" over a 32% gap."""

    def test_a_large_second_cell_is_reported_beside_the_dominant_one(self):
        col = _col(_mech([_counts(reach=21, found=12, ignored=4, missed=1)], rb.REACH))
        line = rb._why_line(col, 38)
        self.assertIn("put 21 of the 38 answers", line)
        self.assertIn("12 it reached on its own", line)

    def test_a_small_second_cell_stays_out_of_the_sentence(self):
        col = _col(_mech([_counts(reach=36, found=2)], rb.REACH))
        line = rb._why_line(col, 38)
        self.assertIn("put 36 of the 38 answers", line)
        self.assertNotIn("reached on its own", line)

    def test_the_worst_news_is_said_first_among_secondaries(self):
        col = _col(_mech([_counts(reach=10, missed=5, found=5)], rb.REACH))
        line = rb._why_line(col, 20)
        self.assertLess(line.index("reached by neither"), line.index("reached on its own"))


class RoutingOverridesTheTable(unittest.TestCase):
    """An arm that never asked Sense is not an arm reporting on Sense."""

    def test_never_routed_says_the_delta_measures_configuration(self):
        line = rb._why_line(_col(_mech([], None), routing=("never-routed",)), 20)
        self.assertIn("never called Sense", line)
        self.assertIn("left out of the replication count", line)

    def test_search_only_says_the_question_was_never_asked(self):
        line = rb._why_line(_col(_mech([], None), routing=("search-only",)), 20)
        self.assertIn("never asked it a dependency question", line)

    def test_a_harness_failure_is_not_a_measurement(self):
        line = rb._why_line(_col(_mech([], None), routing=("harness-failure",)), 20)
        self.assertIn("did not come up", line)

    def test_a_verdict_split_asks_for_a_third_run(self):
        col = _col(_mech([_counts(reach=18)], rb.REACH, verdict_split=True))
        self.assertIn("until a third run rules", rb._why_line(col, 20))


class WholeNumbersPrintWhole(unittest.TestCase):
    def test_a_mean_of_twenty_one_is_not_twenty_one_point_zero(self):
        self.assertEqual(rb._n(21.0), "21")
        self.assertEqual(rb._n(21.5), "21.5")


class Replication(unittest.TestCase):
    def test_with_no_routed_arm_the_board_says_it_cannot_tell(self):
        rep = {"routed": [], "replicated": [], "never_routed": [], "search_only": [],
               "not_measured": ["a", "b"], "threshold": 0.5}
        text = "\n".join(rb._replication(rep, [], rb.load_copy()))
        self.assertIn("No confirmation model has been run", text)
        self.assertNotIn("0 of 0", text)

    def test_routed_arms_are_counted_not_ranked(self):
        rep = {"routed": ["a", "b"], "replicated": ["a"], "never_routed": ["c"],
               "search_only": [], "not_measured": [], "threshold": 0.5}
        text = "\n".join(rb._replication(rep, [], rb.load_copy()))
        self.assertIn("1 of the 2 models", text)
        self.assertIn("Never called Sense", text)


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
        self.assertLess(out.index("<!-- reading -->"), out.index("## Model by model"))

    def test_the_page_states_it_is_not_a_model_comparison(self):
        out = rb.render(self._data())
        self.assertIn("not a comparison of the models", out)
        self.assertIn("not a comparison of the repositories", out)

    def test_an_unbenched_arm_says_so_instead_of_showing_a_zero(self):
        out = rb.render(self._data())
        self.assertIn("Not run for this board yet.", out)

    def test_no_em_dashes_reach_a_published_page(self):
        self.assertNotIn("\u2014", rb.render(self._data()))

    def test_the_repo_and_the_models_are_introduced_by_name(self):
        out = rb.render(self._data())
        self.assertIn("Does Sense help an AI understand Discourse?", out)
        self.assertIn("open-source discussion platform", out)
        self.assertIn("Anthropic's Claude Opus 5, run through Claude Code.", out)


class TheTaskIsShown(unittest.TestCase):
    """A result is unreadable without the question that produced it."""

    QUESTION = {"name": "session", "description": "You are a maintainer.",
                "contract_symbol": "Category", "contract_file": "app/models/category.rb",
                "steps": [{"name": "orient", "prompt": "Task: orient yourself."},
                          {"name": "dependents", "prompt": "Now find every dependent."}]}

    def test_the_class_under_rework_is_named(self):
        out = "\n".join(rb._question_block({"question": self.QUESTION}, "Discourse"))
        self.assertIn("`Category`", out)
        self.assertIn("`app/models/category.rb`", out)

    def test_every_step_is_quoted_verbatim(self):
        out = "\n".join(rb._question_block({"question": self.QUESTION}, "Discourse"))
        self.assertIn("The session is 2 steps", out)
        self.assertIn("> Task: orient yourself.", out)
        self.assertIn("> Now find every dependent.", out)

    def test_the_framing_is_quoted_with_its_own_caveat(self):
        out = "\n".join(rb._question_block({"question": self.QUESTION}, "Discourse"))
        self.assertIn("never names the class", out)
        self.assertIn("> You are a maintainer.", out)

    def test_a_board_with_no_scenario_still_renders(self):
        out = "\n".join(rb._question_block({}, "Discourse"))
        self.assertIn("## The question", out)
        self.assertNotIn("Verbatim", out)


if __name__ == "__main__":
    unittest.main()


class Charts(unittest.TestCase):
    """Mermaid beside the tables, never instead of them."""

    def _multi(self, n=3, delta=0.5435):
        cols = []
        for i in range(n):
            cols.append(_col(_mech([_counts(reach=18, ignored=2, found=2, missed=1)],
                                   rb.REACH), model=f"m{i}", delta=delta))
            cols[-1]["overall"] = {"baseline_mean": 0.3, "sense_mean": 0.3 + delta,
                                   "delta": delta}
        return {"repo": "discourse", "vertical": "ruby-rails", "gold_rows": 23,
                "scenario_version": "sha256:abcd", "sense_version": "s",
                "headline": "m0", "question": {}, "columns": cols,
                "replication": {"routed": [], "replicated": [], "never_routed": [],
                                "search_only": [], "not_measured": [], "threshold": 0.5}}

    def test_the_axis_leaves_headroom_above_the_tallest_bar(self):
        out = "\n".join(rb._delta_chart(self._multi(), rb.load_copy()))
        top = float(out.split("0 --> ")[1].split("\n")[0])
        self.assertGreater(top, 0.5435)

    def test_the_axis_never_runs_past_one(self):
        out = "\n".join(rb._delta_chart(self._multi(delta=0.95), rb.load_copy()))
        self.assertIn("0 --> 1.00", out)

    def test_the_chart_plots_deltas_not_absolute_scores(self):
        out = "\n".join(rb._delta_chart(self._multi(), rb.load_copy()))
        self.assertIn("bar [0.5435, 0.5435, 0.5435]", out)
        self.assertNotIn("0.3000", out)

    def test_one_model_alone_gets_no_comparison_chart(self):
        self.assertEqual(rb._delta_chart(self._multi(n=1), rb.load_copy()), [])

    def test_an_arm_that_never_called_sense_is_not_plotted(self):
        data = self._multi(n=2)
        data["columns"][1]["routing"] = ["never-routed"]
        out = "\n".join(rb._delta_chart(data, rb.load_copy()))
        self.assertEqual(out, "")

    def test_each_model_gets_a_pie_of_where_its_answers_came_from(self):
        avg = {rb.REACH: 21, rb.IGNORED: 4, rb.FOUND_ANYWAY: 12, rb.MISSED: 1}
        out = "\n".join(rb._provenance_chart(_col(_mech([], None), model="m0"),
                                             rb.load_copy(), avg))
        self.assertIn("pie showData", out)
        self.assertIn('"From Sense, and used" : 21', out)
        self.assertIn('"Found without Sense" : 12', out)

    def test_an_empty_slice_is_left_out_rather_than_drawn_as_zero(self):
        avg = {rb.REACH: 23, rb.IGNORED: 0, rb.FOUND_ANYWAY: 0, rb.MISSED: 0}
        out = "\n".join(rb._provenance_chart(_col(_mech([], None), model="m0"),
                                             rb.load_copy(), avg))
        self.assertIn('"From Sense, and used" : 23', out)
        self.assertNotIn("not used", out)

    def test_the_page_explains_how_a_number_is_produced(self):
        out = rb.render(self._multi())
        self.assertIn("flowchart LR", out)
        self.assertIn("Blind grader", out)

    def test_every_mermaid_block_is_closed(self):
        out = rb.render(self._multi())
        self.assertEqual(out.count("```mermaid"), out.count("```") - out.count("```mermaid"))
