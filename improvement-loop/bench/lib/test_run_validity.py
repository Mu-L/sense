"""Behavior tests for run_validity: a timed-out run is a RESULT, not a void."""
import json

import run_validity as rv


def _c(rc, chars, tokens, **kw):
    return rv.classify(rc, chars, tokens, **kw)


def test_clean_run_with_an_answer_is_completed():
    r = _c(0, 34374, 23232)
    assert r["valid"] is True
    assert r["outcome"] == "completed"
    assert r["void_reason"] is None


def test_watchdog_cut_a_real_answer_stays_valid():
    """The 2026-07-21 ruling: an arm out of clock failed the exam, not the exam."""
    r = _c(124, 38649, 18994)
    assert r["valid"] is True
    assert r["outcome"] == "truncated_at_ceiling"
    assert r["watchdog_kind"] == "hard_cap_timeout"


def test_never_reaching_synthesis_is_a_real_failure_not_an_artifact():
    """Tokens and tool calls burned, 83 chars of mid-work narration: a true 0.0."""
    r = _c(124, 83, 21886)
    assert r["valid"] is True
    assert r["outcome"] == "never_reached_synthesis"


def test_a_silent_arm_that_never_got_its_wall_is_starved_not_a_zero():
    """php-laravel/coolify, 2026-08-11: a baseline held 486s of wall with 34s of
    provider time, 9 turns and a 71-char answer, and scored 0.0 on every group.
    Read as never_reached_synthesis it is a real 0.0 and manufactures the delta
    for whatever sense run it is paired against."""
    r = _c(124, 71, 1842, api_seconds=34.3, wall_seconds=486)
    assert r["valid"] is False
    assert r["void_reason"] == "starved_session"


def test_a_silent_arm_that_held_its_wall_stays_a_real_zero():
    """The two other 0.0 baselines in the same campaign, at 0.89 and 0.96 of wall:
    they ground through their budget and failed. That failure IS the measurement."""
    assert _c(124, 83, 21886, api_seconds=470, wall_seconds=526)["outcome"] \
        == "never_reached_synthesis"
    assert _c(124, 83, 21886, api_seconds=425.7, wall_seconds=443)["outcome"] \
        == "never_reached_synthesis"


def test_starvation_never_touches_a_delivered_answer():
    """truncated_at_ceiling is a delivery the clock cut: the arm answered, so how
    it spent the wall does not change that it did."""
    r = _c(124, 38649, 18994, api_seconds=10, wall_seconds=500)
    assert r["valid"] is True
    assert r["outcome"] == "truncated_at_ceiling"


def test_unknown_api_time_classifies_exactly_as_before():
    """Every run on disk predates the stamp, and a missing number must not void
    them wholesale: no api time means the question was not asked."""
    assert _c(124, 71, 1842)["outcome"] == "never_reached_synthesis"
    assert _c(124, 71, 1842, wall_seconds=486)["outcome"] == "never_reached_synthesis"
    assert _c(124, 71, 1842, api_seconds=34.3)["outcome"] == "never_reached_synthesis"
    assert _c(124, 71, 1842, api_seconds=34.3, wall_seconds=0)["outcome"] \
        == "never_reached_synthesis"


def test_watchdog_before_any_output_measures_nothing():
    r = _c(124, 0, 0)
    assert r["valid"] is False
    assert r["void_reason"] == "no_output_hang"


def test_non_watchdog_failure_is_a_harness_crash():
    """rc=1 at 188s under a 300s ceiling: the session fell over, no measurement."""
    r = _c(1, 203, 5904)
    assert r["valid"] is False
    assert r["void_reason"] == "harness_crash"


def test_clean_exit_with_a_degenerate_stream_is_invalid():
    r = _c(0, 12, 400)
    assert r["valid"] is False
    assert r["void_reason"] == "empty_final_answer"


def test_provider_cap_outranks_the_answer_length_gate():
    r = _c(0, 94, 120, provider_error=True)
    assert r["valid"] is False
    assert r["void_reason"] == "provider_cap_error"


def test_offloaded_answer_is_an_artifact():
    r = _c(0, 5000, 9000, offloaded=True)
    assert r["valid"] is False
    assert r["void_reason"] == "answer_offloaded_to_file"


def test_stall_and_cold_start_codes_are_watchdogs_too():
    assert _c(125, 9000, 5000)["watchdog_kind"] == "stalled_midrun"
    assert _c(126, 9000, 5000)["watchdog_kind"] == "no_first_output_hang"


def test_min_answer_chars_is_tunable():
    assert _c(124, 300, 5000, min_answer_chars=1000)["outcome"] == "never_reached_synthesis"
    assert _c(124, 300, 5000, min_answer_chars=200)["outcome"] == "truncated_at_ceiling"


def test_classify_run_reads_claude_evidence_from_scored_json():
    """The claude driver records no answer_chars; the scorer does."""
    meta = {"claude_exit_code": 124}
    scored = {"metrics": {"answer_chars": 38649, "token_output": 18994}}
    r = rv.classify_run(meta, scored)
    assert r["valid"] is True
    assert r["outcome"] == "truncated_at_ceiling"


def test_classify_run_prefers_run_meta_evidence_when_present():
    meta = {"opencode_exit_code": 124, "answer_chars": 22327, "output_tokens": 16435}
    r = rv.classify_run(meta, {"metrics": {"answer_chars": 0, "token_output": 0}})
    assert r["outcome"] == "truncated_at_ceiling"


def test_classify_run_carries_a_recorded_provider_cap_forward():
    meta = {"opencode_exit_code": 0, "answer_chars": 94,
            "output_tokens": 30, "error": "provider_cap_error"}
    assert rv.classify_run(meta)["void_reason"] == "provider_cap_error"


def test_classify_run_defaults_a_missing_exit_code_to_clean():
    """session-run.sh records no exit code; judge it on the answer alone."""
    r = rv.classify_run({}, {"metrics": {"answer_chars": 9000, "token_output": 4000}})
    assert r["outcome"] == "completed"


def _starved_cell(tmp_path, api_ms, wall):
    """One run directory shaped like the coolify baseline that started this."""
    run = tmp_path / "baseline" / "coolify" / "run-1"
    run.mkdir(parents=True)
    (run / "run_meta.json").write_text(json.dumps(
        {"claude_exit_code": 124, "wall_time_seconds": wall}))
    (run / "scored.json").write_text(json.dumps(
        {"metrics": {"answer_chars": 71, "token_output": 1842}}))
    (run / "transcript.json").write_text("\n".join([
        json.dumps({"type": "assistant", "message": {"role": "assistant"}}),
        json.dumps({"type": "result", "duration_api_ms": api_ms, "num_turns": 9}),
    ]))
    return run


def test_api_time_is_recovered_from_the_transcript_when_no_driver_stamped_it(tmp_path):
    """No runner records api_duration_ms today, so the class has to follow the
    artifact the session already wrote."""
    run = _starved_cell(tmp_path, 34290, 486)
    meta = json.loads((run / "run_meta.json").read_text())
    scored = json.loads((run / "scored.json").read_text())
    assert rv.classify_run(meta, scored)["outcome"] == "never_reached_synthesis"
    assert rv.classify_run(meta, scored, run_dir=str(run))["outcome"] == "starved_session"


def test_a_stamped_api_time_wins_over_the_transcript(tmp_path):
    run = _starved_cell(tmp_path, 34290, 486)
    meta = json.loads((run / "run_meta.json").read_text())
    meta["api_duration_ms"] = 470000
    scored = json.loads((run / "scored.json").read_text())
    assert rv.classify_run(meta, scored, run_dir=str(run))["outcome"] \
        == "never_reached_synthesis"


def test_a_starved_run_leaves_the_scored_set(tmp_path):
    run = _starved_cell(tmp_path, 34290, 486)
    assert rv.measured_runs(str(run.parent)) == []
    _starved_cell(tmp_path / "held", 460000, 486)
    assert len(rv.measured_runs(str(tmp_path / "held" / "baseline" / "coolify"))) == 1


def test_a_transcript_without_a_result_record_asks_nothing(tmp_path):
    """codex and opencode transcripts carry no duration_api_ms."""
    run = tmp_path / "run-1"
    run.mkdir()
    (run / "transcript.json").write_text(json.dumps({"type": "message"}))
    assert rv._api_ms_from_transcript(str(run)) is None
    assert rv._api_ms_from_transcript(str(tmp_path / "absent")) is None


def test_parked_and_probe_dirs_are_off_board():
    assert rv.is_parked("claude-opus-4-8/dryruns-20260719/sense/pebble/run-1")
    assert rv.is_parked("claude-opus-4-8/dropped-cells-20260720/baseline/miniflux")
    assert rv.is_parked("claude-fable-5/_voided-sense-run2/run-2")
    assert rv.is_parked("claude-opus-4-8/baseline/dolt/failed-run-2-claude-session")


def test_a_normal_run_is_on_board():
    assert not rv.is_parked("claude-opus-4-8/sense/dolt/run-4")
    assert not rv.is_parked("gpt-5.5/baseline/consul/run-1")


def test_a_superseded_run_leaves_the_scored_set(tmp_path):
    """park_superseded (lib/bench-paths.sh) renames run-N -> failed-run-N, and that
    rename IS the whole mechanism: every reader globs `run-*`. A retried sense arm used
    to leave both the replaced attempt and its replacement in the cell - 3 sense runs
    against a 2-run baseline, with the superseded 0.0 still in the mean."""
    cell = tmp_path / "sense" / "mastodon"
    for name in ("run-1", "failed-run-2", "run-3"):
        (cell / name).mkdir(parents=True)
        (cell / name / "scored.json").write_text("{}")
    kept = [p.split("/")[-2] for p in rv.measured_runs(str(cell))]
    assert kept == ["run-1", "run-3"]
