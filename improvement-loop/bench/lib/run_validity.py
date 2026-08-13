#!/usr/bin/env python3
"""run_validity.py -- one classifier for "is this run a MEASUREMENT?", shared by
every runner and by select_final.

`valid` answers ONE question: did this run measure the arm? It never answers
"did the arm do well". An arm that runs out of wall clock FAILED the exam, and
the exam still counts (a standing rule; full rule in
scorer.py -> TIME_CEILINGS). Collapsing those two questions into `rc == 0` is
what made a 38,649-char timed-out baseline read as "no result".

Same watchdog exit code, OPPOSITE meanings -- the tell is the answer text, not
the exit code:

  completed              rc 0, a real answer                    -> VALID
  truncated_at_ceiling   watchdog cut a real answer short       -> VALID (failed exam)
  never_reached_synthesis tokens+tool calls burned, no answer   -> VALID (real 0.0)
  starved_session        watchdog, but the arm never got its wall -> invalid
  empty_final_answer     clean exit, degenerate/empty stream    -> invalid
  no_output_hang         watchdog cut it before any output      -> invalid
  provider_cap_error     metered sub refused mid-delivery       -> invalid
  answer_offloaded       answer written to a file, stub returned-> invalid
  harness_crash          non-watchdog failure                   -> invalid

The five invalid classes are measurement ARTIFACTS: the harness, not the arm,
decided the outcome, so they are re-run rather than scored.
"""
import glob
import json
import os

# Watchdog exits: the runner's own clock stopped the session. Everything else
# non-zero is the harness falling over, which measures nothing.
WATCHDOG_CODES = {
    124: "hard_cap_timeout",
    125: "stalled_midrun",
    126: "no_first_output_hang",
}

# Final assistant text shorter than this is mid-work narration, not an answer.
MIN_ANSWER_CHARS = 200

# A watchdogged run that spent less than this fraction of its wall INSIDE the
# provider is starved, not slow: the arm was queued or retried, never given the
# budget the cell derives from. Measured 2026-08-11 across the 26 runs of
# php-laravel/coolify, api-seconds over wall-seconds:
#
#   0.07  baseline, 9 turns, 71-char answer, scored 0.0 on every group  <- starved
#   0.46  0.60  0.65  0.89  0.96  baselines that greped locally, scored 0.27-0.92
#   0.98-1.00  every sense run (MCP calls return in milliseconds)
#
# So the floor separates the one starved run from the honest lows with a wide
# margin either side. It is NOT a slowness gate: the two 0.0 baselines that DID
# get their wall sit at 0.89 and 0.96 and stay valid, which is the point - a real
# failure to answer is the result the bench exists to record.
STARVED_API_RATIO = 0.15

_ARTIFACTS = {
    "starved_session",
    "empty_final_answer",
    "no_output_hang",
    "provider_cap_error",
    "answer_offloaded_to_file",
    "harness_crash",
}


def classify(rc, answer_chars, output_tokens,
             provider_error=False, offloaded=False,
             min_answer_chars=MIN_ANSWER_CHARS,
             api_seconds=None, wall_seconds=None):
    """Return {"valid", "outcome", "void_reason", "watchdog_kind"} for one run.

    answer_chars is the FINAL assistant text length; output_tokens is what the
    session generated. Their ratio is the classification -- see the module
    docstring. rc alone is never the verdict.

    api_seconds/wall_seconds are optional and only ever RESCUE a run from being
    counted: they split the one class that reads a silent arm as a real 0.0
    (never_reached_synthesis) from the run that was never given its wall. When
    either is missing the split is not made and nothing changes, so a runner or
    a transcript that cannot report API time classifies exactly as before.
    """
    rc = int(rc)
    answer_chars = int(answer_chars or 0)
    output_tokens = int(output_tokens or 0)
    watchdog_kind = WATCHDOG_CODES.get(rc)

    outcome = _outcome(rc, answer_chars, output_tokens,
                       provider_error, offloaded, min_answer_chars,
                       watchdog_kind is not None,
                       _starved(api_seconds, wall_seconds))
    valid = outcome not in _ARTIFACTS
    return {
        "valid": valid,
        "outcome": outcome,
        "void_reason": None if valid else outcome,
        "watchdog_kind": watchdog_kind,
    }


def _starved(api_seconds, wall_seconds):
    """True when this run spent almost none of its wall inside the provider.

    Unknown is not starved: both numbers must be present and the wall positive,
    or the question was not asked and the run keeps whatever class it had.
    """
    try:
        api = float(api_seconds)
        wall = float(wall_seconds)
    except (TypeError, ValueError):
        return False
    return wall > 0 and (api / wall) < STARVED_API_RATIO


def _outcome(rc, answer_chars, output_tokens,
             provider_error, offloaded, min_answer_chars, is_watchdog,
             starved=False):
    """Which of the nine classes this run landed in."""
    # Provider cap first: its error blob is short enough to also trip the
    # answer-length gate, but the cap is the actionable diagnosis.
    if provider_error:
        return "provider_cap_error"
    if offloaded:
        return "answer_offloaded_to_file"
    if is_watchdog:
        if output_tokens == 0:
            return "no_output_hang"
        # Tokens were spent. A long answer means the clock cut a real delivery;
        # a short one means the arm never got to the answer at all. Both are the
        # arm's result, not the instrument's failure.
        if answer_chars >= min_answer_chars:
            return "truncated_at_ceiling"
        # ...unless the arm never got the wall it was silent through. A silent
        # arm that HELD the provider for its budget failed the exam honestly; one
        # that was queued or retried outside it sat out the exam, and scoring that
        # as a 0.0 manufactures the delta for whatever it is paired against.
        return "starved_session" if starved else "never_reached_synthesis"
    if rc != 0:
        return "harness_crash"
    return ("completed" if answer_chars >= min_answer_chars
            else "empty_final_answer")


def classify_run(meta, scored=None, min_answer_chars=MIN_ANSWER_CHARS,
                 run_dir=None):
    """Classify from a run's on-disk records, newest evidence first.

    Reads the exit code and answer evidence out of run_meta.json (`meta`) and,
    when the runner did not record them there, out of the sibling scored.json
    (`scored`). This derives the class from what the run LEFT BEHIND, so a run
    stamped by an older driver reclassifies correctly with no rewrite.

    `run_dir` lets the API time be recovered from transcript.json for runs no
    driver stamped it into - which is every run on disk today. Same reason as
    above: the evidence is already written down, so the class follows the
    artifacts rather than needing the campaign re-run.
    """
    scored = scored or {}
    metrics = scored.get("metrics") or {}
    rc = _first(meta, ("claude_exit_code", "codex_exit_code", "opencode_exit_code"))
    answer_chars = _first(meta, ("answer_chars",))
    if answer_chars is None:
        answer_chars = metrics.get("answer_chars")
    output_tokens = _first(meta, ("output_tokens",))
    if output_tokens is None:
        output_tokens = metrics.get("token_output")
    error = meta.get("error")

    # No answer evidence anywhere (an unscored run, or a runner that records
    # neither): there is nothing to classify FROM, so defer to whatever the
    # runner stamped rather than inventing a verdict from silence.
    if answer_chars is None:
        return _from_stored_flag(meta)
    api_ms = _first(meta, ("api_duration_ms",))
    if api_ms is None and run_dir:
        api_ms = _api_ms_from_transcript(run_dir)
    wall = _first(meta, ("wall_time_seconds",))
    if wall is None:
        wall = metrics.get("wall_time_seconds")
    return classify(
        rc if rc is not None else 0,
        answer_chars, output_tokens,
        provider_error=(error == "provider_cap_error"),
        offloaded=(error == "answer_offloaded_to_file"),
        min_answer_chars=min_answer_chars,
        api_seconds=(None if api_ms is None else float(api_ms) / 1000.0),
        wall_seconds=wall,
    )


def _api_ms_from_transcript(run_dir):
    """Provider time in ms out of the session's own result record, or None.

    Only the tail is read: the result event is the last line of a stream-json
    transcript, and these files run to megabytes. A transcript that carries no
    such record (codex, opencode) returns None, which reads as "not asked".
    """
    path = os.path.join(run_dir, "transcript.json")
    try:
        with open(path, "rb") as f:
            f.seek(0, os.SEEK_END)
            f.seek(max(0, f.tell() - 65536))
            tail = f.read().decode("utf-8", "replace")
    except OSError:
        return None
    for line in reversed(tail.splitlines()):
        line = line.strip()
        if not line.startswith("{") or '"duration_api_ms"' not in line:
            continue
        try:
            return json.loads(line).get("duration_api_ms")
        except ValueError:
            return None
    return None


# Directory-name conventions for runs that are ON DISK but NOT part of the
# published board: maintainer-parked (`_voided-*`, `_invalid-*`), superseded
# (`failed-run-*`), and pre-campaign probes (`dryruns-*`, `dropped-cells-*`).
# One list, because "which runs are published" was being answered differently by
# select_final (which had started selecting dryrun probes) and by the report
# generator (which counted a dryrun's judge model as a board-wide judge split).
_PARKED_PREFIXES = ("_", "failed-", "dryruns-", "dropped-cells-")


def is_parked(rel_path):
    """True if any path segment marks this run as off-board."""
    return any(seg.startswith(_PARKED_PREFIXES)
               for seg in rel_path.replace(os.sep, "/").split("/"))


def measured_runs(repo_dir):
    """This arm's scored.json paths, MEASUREMENT runs only.

    The single answer to "which runs count", shared by every instrument that
    publishes a number. Three instruments used to carry their own copy of this
    glob -- matrix.py, scoreboard.py and check_findings_stats.py -- and when one
    learned to drop measurement artifacts and the others did not, they published
    two different headlines for the same cell (dolt +0.50 vs +0.38, because a
    203-char harness crash was averaged in by two of the three).
    """
    paths = sorted(glob.glob(os.path.join(repo_dir, "run-*", "scored.json")))
    if not paths and os.path.exists(os.path.join(repo_dir, "scored.json")):
        paths = [os.path.join(repo_dir, "scored.json")]
    return [p for p in paths if measured(p)]


def measured(scored_path):
    """True unless this run is a measurement artifact."""
    run_dir = os.path.dirname(scored_path)
    meta_path = os.path.join(run_dir, "run_meta.json")
    if not os.path.exists(meta_path):
        return True
    try:
        with open(meta_path) as f:
            meta = json.load(f)
        with open(scored_path) as f:
            scored = json.load(f)
    except (OSError, ValueError):
        return True
    return classify_run(meta, scored, run_dir=run_dir)["valid"]


def _from_stored_flag(meta):
    """Fall back to the runner's own stamp when the run left no answer evidence.

    An unstamped run is treated as a measurement: `valid` was absent from the
    opencode and session runners entirely, and defaulting those to invalid
    silently deleted four whole arms from the final selection.
    """
    valid = meta.get("valid")
    if valid is None:
        valid = True
    reason = meta.get("void_reason") or meta.get("error") or "invalid"
    return {
        "valid": bool(valid),
        "outcome": "unclassified" if valid else reason,
        "void_reason": None if valid else reason,
        "watchdog_kind": WATCHDOG_CODES.get(int(meta.get("claude_exit_code") or 0)),
    }


def _first(meta, keys):
    """First key present in meta with a non-None value."""
    for k in keys:
        if meta.get(k) is not None:
            return meta[k]
    return None
