#!/usr/bin/env python3
"""Behaviour pins for appending authoring-cycle runs (vertical-loop.sh).

The load-bearing pin is `test_neither_unscored_root_is_wiped`. Passing FORCE_WIPE=1 to
runs-variance.sh disables its refuse-to-destroy-scored-runs guard, and both unscored call
sites did exactly that so a second authoring cycle could re-run at the same run-1. The cost
was invisible until it was checked: cycles 1-8 of the Account campaign are unrecoverable, and
with five SCORED validation pairs on disk under a scenario version that no longer matches the
live scenario, the next validation cycle would have deleted them too.

The rest pin `next_run`, whose only job is to never hand back an index that is already filed.
"""
import os
import re
import subprocess
import unittest

LIB = os.path.dirname(os.path.abspath(__file__))
DRIVER = os.path.join(os.path.dirname(LIB), "drivers", "vertical-loop.sh")


def driver_text():
    with open(DRIVER) as fh:
        return fh.read()


def driver_code():
    """The driver with comment lines stripped.

    The pins below assert on what the script RUNS. Matching raw text would also match the
    comments that explain why the wipe was removed, so removing it would look like leaving
    it in place - a pin that fails on its own explanation is worse than no pin.
    """
    return "\n".join(line for line in driver_text().splitlines()
                     if not line.lstrip().startswith("#"))


def next_run(root, repo="mastodon"):
    """Run the driver's own next_run against a tree, by extracting the function body.

    Sourcing vertical-loop.sh would execute a campaign, so the function is lifted out
    instead. The extraction is anchored on the same `name() {` ... `}` shape the whole
    file uses, and fails loudly rather than silently testing nothing.
    """
    text = driver_text()
    match = re.search(r"^next_run\(\) \{\n(.*?)^\}", text, re.S | re.M)
    assert match, "next_run() not found in vertical-loop.sh - the pin below tests nothing"
    script = f'REPO="{repo}"\nnext_run() {{\n{match.group(1)}}}\nnext_run "{root}"'
    out = subprocess.run(["bash", "-c", script], capture_output=True, text=True, check=True)
    return int(out.stdout.strip())


class WipeTest(unittest.TestCase):
    def test_neither_unscored_root_is_wiped(self):
        """FORCE_WIPE=1 turns off the guard that refuses to destroy scored runs."""
        self.assertNotIn("FORCE_WIPE=1", driver_code())

    def test_both_unscored_call_sites_append(self):
        text = driver_code()
        self.assertEqual(text.count("KEEP_RUNS=1"), 2)
        self.assertIn('START_RUN="$(next_run "$MINIBENCH_DIR")"', text)
        self.assertIn('START_RUN="$(next_run "$VALIDATION_DIR")"', text)


class NextRunTest(unittest.TestCase):
    def setUp(self):
        self.root = os.path.join(
            os.environ.get("TMPDIR", "/tmp"), f"cycle-append-{os.getpid()}")
        os.makedirs(self.root, exist_ok=True)
        self.addCleanup(subprocess.run, ["rm", "-rf", self.root])

    def fill(self, *rels):
        for rel in rels:
            os.makedirs(os.path.join(self.root, rel), exist_ok=True)

    def test_an_empty_root_starts_at_one(self):
        self.assertEqual(next_run(os.path.join(self.root, "never-created")), 1)

    def test_the_next_index_is_past_the_highest_filed(self):
        self.fill("sense/mastodon/run-1", "baseline/mastodon/run-1")
        self.assertEqual(next_run(self.root), 2)

    def test_arms_out_of_step_take_the_higher(self):
        """A cycle whose second arm died mid-run must not refile over the first arm."""
        self.fill("sense/mastodon/run-2", "baseline/mastodon/run-3")
        self.assertEqual(next_run(self.root), 4)

    def test_double_digit_runs_are_compared_as_numbers(self):
        """Lexical comparison would rank run-10 below run-2 and overwrite it."""
        self.fill("sense/mastodon/run-2", "sense/mastodon/run-10")
        self.assertEqual(next_run(self.root), 11)

    def test_a_non_numeric_run_dir_is_ignored(self):
        self.fill("sense/mastodon/run-1", "sense/mastodon/run-old")
        self.assertEqual(next_run(self.root), 2)

    def test_another_repo_in_the_same_root_does_not_shift_this_one(self):
        """Roots are shared across repos; rails' run-7 must not push mastodon to 8."""
        self.fill("sense/rails/run-7")
        self.assertEqual(next_run(self.root), 1)


if __name__ == "__main__":
    unittest.main()
