"""Behaviour pins for the cycle 2 ledger entry: derived from numbers, never invented."""
import unittest

import cycle2_ledger as cl


def _col(model, measured=True, base=0.30, sense=0.90, reach=5):
    if not measured:
        return {"model": model, "measured": False}
    return {"model": model, "measured": True,
            "overall": {"baseline_mean": base, "sense_mean": sense,
                        "delta": round(sense - base, 4)},
            "sense_only_reach": ["g"] * reach}


def _data(columns, **rep):
    base = {"routed": [], "replicated": [], "never_routed": [], "search_only": [],
            "not_measured": [], "threshold": 0.5}
    base.update(rep)
    return {"repo": "discourse", "vertical": "ruby-rails",
            "scenario_version": "sha256:1def723310067e48",
            "sense_version": "sense 1.13.5", "headline": "claude-opus-5",
            "columns": columns, "replication": base}


class TheLessonIsNeverManufactured(unittest.TestCase):
    """A loop that always claims a lesson is manufacturing them."""

    def test_no_routed_arm_yet_says_so(self):
        out = cl._lesson(_data([_col("claude-opus-5")]))
        self.assertIn("none yet", out)

    def test_a_clean_sweep_claims_no_lesson(self):
        out = cl._lesson(_data([], routed=["a", "b"], replicated=["a", "b"]))
        self.assertTrue(out.startswith("none."))

    def test_an_arm_that_never_routed_is_named_as_our_failure(self):
        out = cl._lesson(_data([], routed=["a"], replicated=["a"], never_routed=["kimi"]))
        self.assertIn("routing failure of ours", out)
        self.assertIn("kimi", out)

    def test_a_win_that_travels_to_nobody_is_stated_plainly(self):
        out = cl._lesson(_data([], routed=["a", "b"], replicated=[]))
        self.assertIn("did not travel", out)

    def test_a_partial_replication_is_not_dressed_up(self):
        out = cl._lesson(_data([], routed=["a", "b"], replicated=["a"]))
        self.assertIn("travels unevenly", out)
        self.assertIn("1 of 2", out)


class TheEntry(unittest.TestCase):
    def _entry(self, **kw):
        cols = [_col("claude-opus-5"), _col("gpt-5.6-sol", **kw)]
        return cl.entry(_data(cols, routed=["gpt-5.6-sol"],
                              replicated=["gpt-5.6-sol"]), "2026-08-05")

    def test_the_key_is_the_one_the_write_point_table_carries(self):
        self.assertIn("| cycle2/discourse/board |", self._entry())

    def test_every_required_field_is_present(self):
        out = self._entry()
        for field in ("Provenance", "What", "Why", "Alternatives", "Lesson",
                      "Scores", "Cost", "Links"):
            self.assertIn(f"- **{field}:**", out)

    def test_the_headline_column_is_recorded_as_reused_not_re_run(self):
        self.assertIn("NOT re-run", self._entry())

    def test_reach_is_carried_into_the_scores(self):
        self.assertIn("5 reached only with Sense", self._entry())

    def test_an_unbenched_arm_says_not_run_rather_than_a_number(self):
        # Normalised: the field is wrapped, so the phrase can straddle two lines.
        out = " ".join(self._entry(measured=False).split())
        self.assertIn("gpt-5.6-sol not run", out)

    def test_a_singular_arm_reads_singular(self):
        self.assertIn("1 confirmation arm at", self._entry())

    def test_no_line_exceeds_the_ledger_width(self):
        for line in self._entry().splitlines():
            self.assertLessEqual(len(line), 100, line)

    def test_a_backticked_path_is_never_split_across_lines(self):
        for line in self._entry().splitlines():
            self.assertEqual(line.count("`") % 2, 0, line)


if __name__ == "__main__":
    unittest.main()
