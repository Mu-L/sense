"""Behaviour pins for the publish gate: it fails closed on anything unaccounted for."""
import unittest

import board_check as bc

NUMBERS = {"repo": "chatwoot", "gold_rows": 38,
           "columns": [{"model": "claude-opus-5",
                        "overall": {"baseline_mean": 0.6053, "sense_mean": 0.8684,
                                    "delta": 0.2632},
                        "sense_only_reach": ["a"] * 11,
                        "session": {"sense": {"wall_time_seconds": 414.0,
                                              "token_total_all": 1748326},
                                    "baseline": {"token_total_all": 905453.5}}}]}


def _board(reading, tail="\n## Model by model\n\nbody\n"):
    return f"# board\n\n## Reading\n\n{reading}\n{tail}"


class ThePlaceholder(unittest.TestCase):
    def test_a_board_still_carrying_the_marker_is_refused(self):
        out = bc.check(_board(bc.READING_MARKER), NUMBERS)
        self.assertIn("the reading placeholder is still in the page", out)

    def test_an_empty_reading_is_refused(self):
        self.assertIn("the Reading section is empty", bc.check(_board("   "), NUMBERS))

    def test_a_written_reading_with_no_figures_passes(self):
        self.assertEqual(bc.check(_board("Sense helped here."), NUMBERS), [])


class Figures(unittest.TestCase):
    """Every number in the prose has to be derivable from the numbers JSON."""

    def test_a_list_length_counts_as_a_figure(self):
        self.assertEqual(bc.check(_board("It found 11 answers."), NUMBERS), [])

    def test_a_plain_leaf_counts(self):
        self.assertEqual(bc.check(_board("Out of 38 answers."), NUMBERS), [])

    def test_a_million_scale_token_total_counts(self):
        self.assertEqual(bc.check(_board("It moved 1,748,326 tokens."), NUMBERS), [])

    def test_a_two_run_mean_quoted_to_its_own_decimal_counts(self):
        self.assertEqual(bc.check(_board("It moved 905,453.5 tokens."), NUMBERS), [])

    def test_a_rounded_delta_counts(self):
        self.assertEqual(bc.check(_board("It gained +0.26 overall."), NUMBERS), [])

    def test_minutes_count_because_the_page_prints_minutes(self):
        self.assertEqual(bc.check(_board("It took 6.9 min."), NUMBERS), [])

    def test_a_share_stated_as_a_percentage_counts(self):
        self.assertEqual(bc.check(_board("It reached 87% of them."), NUMBERS), [])

    def test_a_version_inside_a_model_name_is_not_a_figure(self):
        out = bc.check(_board("Claude Opus 5 led the board."), NUMBERS,
                       declared=[])
        self.assertEqual(out, [])

    def test_an_invented_figure_is_refused(self):
        out = bc.check(_board("It found 29 answers."), NUMBERS)
        self.assertIn("figure '29' is not derivable from the numbers JSON", out)

    def test_a_figure_the_agent_did_not_declare_is_refused(self):
        out = bc.check(_board("It found 11 answers."), NUMBERS, declared=["38"])
        self.assertTrue(any("not declared in the verdict" in p for p in out))

    def test_a_declared_figure_passes(self):
        self.assertEqual(bc.check(_board("It found 11 answers."), NUMBERS,
                                  declared=["11"]), [])

    def test_a_figure_outside_the_reading_is_not_checked(self):
        page = _board("Sense helped.", tail="\n## Model by model\n\n999999 tokens\n")
        self.assertEqual(bc.check(page, NUMBERS), [])


class Leaks(unittest.TestCase):
    """The results tree holds someone else's source. None of it is publishable."""

    def test_a_results_path_is_refused(self):
        out = bc.check(_board("See results/claude-opus-5/x."), NUMBERS)
        self.assertTrue(any("results/" in p for p in out))

    def test_a_run_directory_is_refused(self):
        out = bc.check(_board("In /run-1 it did better."), NUMBERS)
        self.assertTrue(any("/run-1" in p for p in out))

    def test_a_raw_artifact_name_is_refused_anywhere_on_the_page(self):
        page = _board("Sense helped.", tail="\n## Model by model\n\nsense-io.jsonl\n")
        self.assertTrue(any("sense-io.jsonl" in p for p in bc.check(page, NUMBERS)))


class Parsing(unittest.TestCase):
    def test_a_version_hash_is_not_read_as_a_number(self):
        page = _board("The question is sha256:24f720898c0385b9 and it held.")
        self.assertEqual(bc.check(page, NUMBERS), [])

    def test_a_model_name_with_digits_is_not_read_as_a_number(self):
        self.assertEqual(bc._numbers_in("GPT-5.6 and GLM-5.2 replicated"), set())

    def test_commas_and_percent_signs_normalise_away(self):
        self.assertEqual(bc._numbers_in("1,748,326 tokens and 87%"), {"1748326", "87"})

class MissingVerdict(unittest.TestCase):
    """This runs while someone is being told why a page cannot ship."""

    def test_a_missing_verdict_file_is_a_refusal_not_a_traceback(self):
        import subprocess
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            import json
            import os
            board = os.path.join(tmp, "b.md")
            numbers = os.path.join(tmp, "n.json")
            open(board, "w").write(_board("Sense helped."))
            json.dump(NUMBERS, open(numbers, "w"))
            out = subprocess.run(
                ["python3", os.path.join(os.path.dirname(bc.__file__), "board_check.py"),
                 board, numbers, "--verdict", os.path.join(tmp, "nope.json")],
                capture_output=True, text=True)
            self.assertEqual(out.returncode, 1)
            self.assertIn("no usable report verdict", out.stderr)
            self.assertNotIn("Traceback", out.stderr)


if __name__ == "__main__":
    unittest.main()
