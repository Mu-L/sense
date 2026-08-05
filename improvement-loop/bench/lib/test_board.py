"""Behaviour pins for the cycle 2 board: eligibility, the version gate, replication."""
import json
import os
import tempfile
import unittest

import board

SENSE = "sense 1.13.5 (schema v5)"


def _row(repo, verdict="WIN", model="claude-opus-5", version="sha256:aaaabbbbccccdddd",
         sense=SENSE):
    return {"repo": repo, "verdict": verdict, "model": model,
            "scenario_version": version, "sense_version": sense,
            "recorded_at": "2026-08-05T08:02Z"}


def _vertical(tmp, rows):
    with open(os.path.join(tmp, "banked.jsonl"), "w") as fh:
        for r in rows:
            fh.write(json.dumps(r) + "\n")
    return tmp


class Eligibility(unittest.TestCase):
    """A cell earns a board by winning on the headline arm, once."""

    def test_a_banked_win_with_no_report_is_eligible(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse")])
            self.assertEqual([r["repo"] for r in board.eligible(tmp, "claude-opus-5")],
                             ["discourse"])

    def test_a_cell_that_already_has_a_board_is_not(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse")])
            path = board.report_path(tmp, "discourse", "sha256:aaaabbbbccccdddd")
            os.makedirs(os.path.dirname(path))
            open(path, "w").write("# board")
            self.assertEqual(board.eligible(tmp, "claude-opus-5"), [])

    def test_a_sub_floor_cell_is_not_eligible(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse", verdict="NOT-YET")])
            self.assertEqual(board.eligible(tmp, "claude-opus-5"), [])

    def test_a_win_on_a_different_model_is_not_the_headline(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse", model="claude-opus-4-8")])
            self.assertEqual(board.eligible(tmp, "claude-opus-5"), [])

    def test_a_new_question_on_the_same_repo_earns_its_own_board(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse"), _row("discourse", version="sha256:1111222233334444")])
            path = board.report_path(tmp, "discourse", "sha256:aaaabbbbccccdddd")
            os.makedirs(os.path.dirname(path))
            open(path, "w").write("# board")
            self.assertEqual([r["scenario_version"] for r in board.eligible(tmp, "claude-opus-5")],
                             ["sha256:1111222233334444"])


class VersionGate(unittest.TestCase):
    """Every column must be the same Sense build, or the board compares products."""

    def test_the_same_build_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse")])
            self.assertTrue(board.gate(tmp, "discourse", "claude-opus-5", installed=SENSE)["ok"])

    def test_a_newer_build_is_refused_loudly(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse")])
            res = board.gate(tmp, "discourse", "claude-opus-5", installed="sense 1.14.0 (schema v5)")
            self.assertFalse(res["ok"])
            self.assertEqual(res["banked"], SENSE)
            self.assertEqual(res["installed"], "sense 1.14.0 (schema v5)")

    def test_an_unreadable_version_is_refused_not_assumed(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse")])
            self.assertFalse(board.gate(tmp, "discourse", "claude-opus-5", installed="")["ok"])

    def test_a_repo_with_no_banked_win_has_nothing_to_gate(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse", verdict="NOT-YET")])
            self.assertFalse(board.gate(tmp, "discourse", "claude-opus-5", installed=SENSE)["ok"])


class ArmRoots(unittest.TestCase):
    def test_a_model_id_is_sanitised_the_way_bench_paths_does(self):
        root = board.arm_root("/v", "kimi-for-coding/k3", "sha256:aaaabbbbccccdddd")
        self.assertEqual(root, "/v/results/kimi-for-coding_k3/aaaabbbbccccdddd")
        root = board.arm_root("/v", "glm-5.2:cloud", "sha256:aaaabbbbccccdddd")
        self.assertEqual(root, "/v/results/glm-5.2_cloud/aaaabbbbccccdddd")

    def test_the_board_is_named_for_the_repo_and_the_question(self):
        self.assertTrue(board.report_path("/v", "discourse", "sha256:aaaabbbbccccdddd")
                        .endswith("reports/discourse-aaaabbbbccccdddd.md"))


class Replication(unittest.TestCase):
    """Counted, never ranked - and an arm that ignored Sense is not counted."""

    def _col(self, model, delta, routing, measured=True):
        return {"model": model, "measured": measured, "best_group_delta": delta,
                "routing": routing}

    def test_an_arm_over_the_floor_replicates(self):
        rep = board._replication([self._col("gpt", 0.70, ["routed"])])
        self.assertEqual(rep["replicated"], ["gpt"])
        self.assertEqual(rep["routed"], ["gpt"])

    def test_an_arm_under_the_floor_is_routed_but_does_not_replicate(self):
        rep = board._replication([self._col("gpt", 0.20, ["routed"])])
        self.assertEqual(rep["replicated"], [])
        self.assertEqual(rep["routed"], ["gpt"])

    def test_an_arm_that_never_called_sense_is_not_counted_either_way(self):
        rep = board._replication([self._col("kimi", 0.02, ["never-routed"])])
        self.assertEqual(rep["never_routed"], ["kimi"])
        self.assertEqual(rep["routed"], [])
        self.assertEqual(rep["replicated"], [])

    def test_an_arm_that_reached_no_resolver_is_its_own_outcome(self):
        rep = board._replication([self._col("glm", 0.10, ["search-only"])])
        self.assertEqual(rep["search_only"], ["glm"])
        self.assertEqual(rep["routed"], [])

    def test_an_unbenched_arm_is_not_measured(self):
        rep = board._replication([self._col("mistral", 0.0, [], measured=False)])
        self.assertEqual(rep["not_measured"], ["mistral"])
        self.assertEqual(rep["routed"], [])

    def test_the_floor_is_inclusive(self):
        rep = board._replication([self._col("gpt", 0.50, ["routed"])])
        self.assertEqual(rep["replicated"], ["gpt"])


class Assembly(unittest.TestCase):
    def test_assembling_a_repo_with_no_banked_win_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse", verdict="NOT-YET")])
            with self.assertRaises(SystemExit):
                board.assemble(tmp, "discourse", "claude-opus-5", [])

    def test_an_unbenched_arm_degrades_to_not_measured_rather_than_failing(self):
        with tempfile.TemporaryDirectory() as tmp:
            _vertical(tmp, [_row("discourse")])
            out = board.assemble(tmp, "discourse", "claude-opus-5", ["gpt-5.6-sol"])
            self.assertEqual(out["repo"], "discourse")
            self.assertEqual([c["model"] for c in out["columns"]],
                             ["claude-opus-5", "gpt-5.6-sol"])
            self.assertFalse(out["columns"][1]["measured"])
            self.assertEqual(out["replication"]["not_measured"], ["gpt-5.6-sol"])


if __name__ == "__main__":
    unittest.main()
