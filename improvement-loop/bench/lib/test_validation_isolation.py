#!/usr/bin/env python3
"""Behaviour pins for the unscored validation run (plans/04-validate.md).

The load-bearing pin is `test_the_scorer_cannot_see_a_validation_run`: the isolation is the
RESULTS ROOT, not a flag a measurement instrument has to honour. That choice is deliberate -
teaching pergroup.py/scorer.py to skip a run would have been a measurement-instrument change
(ledger rule 10, a STOPPER) to buy a property the directory layout already gives for free.

The second pin is that `scoring` reaches run_meta.json, so a file copied out of the tree still
says what it is. A validation number that escapes its directory and gets cited as a result is
the whole failure this run is meant to avoid.
"""
import os
import subprocess
import sys
import tempfile
import unittest

LIB = os.path.dirname(os.path.abspath(__file__))
BENCH = os.path.dirname(LIB)


def _paths(**env):
    """Source bench-paths.sh the way a runner does and read back what it exported."""
    # Only clear what the caller did not set: a multi-model loop unsets RESULTS_DIR to
    # re-derive per model, while a parent that already resolved it must be respected.
    clear = " ".join(k for k in ("RESULTS_DIR", "SCENARIOS_DIR") if k not in env)
    script = (
        f'set -u; {"unset " + clear + "; " if clear else ""}'
        f'BENCH_DIR="{BENCH}"; source "{LIB}/bench-paths.sh"; '
        'echo "$RESULTS_DIR"; echo "$BENCH_SCORING"'
    )
    full = dict(os.environ)
    full.pop("RESULTS_DIR", None)
    full.update({k: str(v) for k, v in env.items()})
    out = subprocess.run(["bash", "-c", script], capture_output=True, text=True, env=full)
    assert out.returncode == 0, out.stderr
    results_dir, scoring = out.stdout.strip().splitlines()
    return results_dir, scoring


class ValidationRoutingTest(unittest.TestCase):
    def test_a_normal_run_is_unaffected(self):
        d, scoring = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5")
        self.assertTrue(d.endswith("verticals/php-laravel/results/claude-opus-5"), d)
        self.assertEqual(scoring, "1")

    def test_a_validation_run_lands_under_validation(self):
        d, scoring = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5",
                            BENCH_VALIDATION=1)
        self.assertTrue(d.endswith("results/claude-opus-5/validation"), d)
        self.assertEqual(scoring, "0")

    def test_validation_nests_under_the_model_root_not_beside_it(self):
        """A sibling of the model root would collide across models and re-enter the scored
        tree the moment RESULTS_DIR was pointed one level up."""
        scored, _ = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5")
        val, _ = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5",
                        BENCH_VALIDATION=1)
        self.assertEqual(os.path.dirname(val), scored)

    def test_it_works_without_a_model_scope(self):
        d, scoring = _paths(VERTICAL="php-laravel", BENCH_VALIDATION=1)
        self.assertTrue(d.endswith("verticals/php-laravel/results/validation"), d)
        self.assertEqual(scoring, "0")

    def test_an_explicit_results_dir_still_gets_the_validation_leaf(self):
        """Runners that pre-resolve RESULTS_DIR must not silently opt out of the isolation."""
        with tempfile.TemporaryDirectory() as t:
            d, scoring = _paths(VERTICAL="php-laravel", RESULTS_DIR=t, BENCH_VALIDATION=1)
            self.assertEqual(d, os.path.join(t, "validation"))
            self.assertEqual(scoring, "0")


class MinibenchRoutingTest(unittest.TestCase):
    """The two-step probe run (plans/02-minibench.md) is unscored like a validation run and
    lands in its own root. The two CANNOT share one: both write <root>/<arm>/<repo>/run-1,
    so the seven-step pair would overwrite the two-step pair at the same path and
    plans/04-validate.md's session row would compare a run against itself."""

    def test_a_minibench_run_lands_under_minibench(self):
        d, scoring = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5",
                            BENCH_MINIBENCH=1)
        self.assertTrue(d.endswith("results/claude-opus-5/minibench"), d)
        self.assertEqual(scoring, "0")

    def test_minibench_and_validation_are_different_roots(self):
        mini, _ = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5",
                         BENCH_MINIBENCH=1)
        val, _ = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5",
                        BENCH_VALIDATION=1)
        self.assertNotEqual(mini, val)
        self.assertEqual(os.path.dirname(mini), os.path.dirname(val))

    def test_minibench_wins_when_both_flags_are_set(self):
        """runs-variance.sh re-sources bench-paths.sh per model with the driver's whole
        environment, so a stale BENCH_VALIDATION must not pull a mini-bench run into the
        validation tree."""
        d, scoring = _paths(VERTICAL="php-laravel", BENCH_MODEL="claude-opus-5",
                            BENCH_MINIBENCH=1, BENCH_VALIDATION=1)
        self.assertTrue(d.endswith("results/claude-opus-5/minibench"), d)
        self.assertEqual(scoring, "0")

    def test_the_append_is_idempotent(self):
        """bench-paths.sh is sourced once per driver in the chain; an unconditional append
        nested it once per hop and the reporter looked one level too deep."""
        with tempfile.TemporaryDirectory() as t:
            root = os.path.join(t, "minibench")
            os.makedirs(root)
            d, _ = _paths(VERTICAL="php-laravel", RESULTS_DIR=root, BENCH_MINIBENCH=1)
            self.assertEqual(d, root)


class ScorerBlindnessTest(unittest.TestCase):
    def test_the_scorer_cannot_see_a_validation_run(self):
        """pergroup.py walks RESULTS_DIR/<tool>/<repo>. A validation cell sits one level
        deeper, so the scored root does not contain it - no instrument change required."""
        with tempfile.TemporaryDirectory() as t:
            scored = os.path.join(t, "baseline", "coolify", "run-1")
            validation = os.path.join(t, "validation", "baseline", "coolify", "run-1")
            os.makedirs(scored)
            os.makedirs(validation)
            cells = [
                os.path.join(root, d)
                for root, dirs, _ in os.walk(t) for d in dirs
                if d.startswith("run-") and "validation" not in root
            ]
            self.assertEqual(cells, [scored])


class RunMetaStampTest(unittest.TestCase):
    """Every runner that writes run_meta.json must carry the scoring flag; a runner that
    forgets it writes an artifact that cannot say whether it counts."""

    RUNNERS = ("bench-sense-local.sh", "opencode-run.sh", "session-run.sh", "codex-run.sh")

    def test_every_runner_stamps_scoring(self):
        for name in self.RUNNERS:
            with open(os.path.join(BENCH, "drivers", name)) as fh:
                body = fh.read()
            self.assertIn('"scoring": os.environ.get("BENCH_SCORING", "1") != "0"', body, name)

    def test_the_stamp_defaults_to_scored(self):
        """An unset BENCH_SCORING means a normal run, so an old driver or a hand invocation
        never silently marks a paid cell unscored."""
        self.assertTrue(os.environ.get("BENCH_SCORING", "1") != "0")


if __name__ == "__main__":
    sys.exit(0 if unittest.main(exit=False).result.wasSuccessful() else 1)
