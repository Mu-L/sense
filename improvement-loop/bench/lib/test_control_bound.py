#!/usr/bin/env python3
"""Behaviour pins for the arithmetic bound gate.

The load-bearing pin is `test_saleor_shaped_cell_survives`: a control at exactly the bound
MUST pass. saleor (control 0.50, sense 1.00, delta exactly +0.500) is a banked WIN, and every
threshold tighter than 0.50 throws it away. If someone "improves" this gate by tightening the
number, that test goes red and tells them what it costs.
"""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import control_bound
from control_bound import BAR, BOUND, evaluate, main
from sense_build import binary_key, stamp


def _scenario(tmp, rows):
    p = os.path.join(tmp, "s.yaml")
    with open(p, "w") as fh:
        fh.write("name: t\nrepo: t\ngold:\n")
        for r in rows:
            fh.write(f"  - {{id: {r['id']}, group: {r['group']}, match: [{r['match']}]}}\n")
    return p


def _probe(tmp, name, text):
    p = os.path.join(tmp, name)
    with open(p, "w") as fh:
        fh.write(text)
    return p


GOLD4 = [{"id": f"dep:d{i}", "group": "dependents", "match": f"pkg/d{i}.go"} for i in range(4)]


class ArithmeticBoundTest(unittest.TestCase):
    def test_the_bound_is_the_bar_complement(self):
        # the whole gate in one line: sense <= 1.0, so delta >= BAR needs control <= 1 - BAR
        self.assertEqual(BOUND, 1.0 - BAR)
        self.assertEqual(BOUND, 0.50)

    def test_control_above_the_bound_kills(self):
        with tempfile.TemporaryDirectory() as t:
            s = _scenario(t, GOLD4)
            # cites 3 of 4 -> control 0.75 -> ceiling +0.25 -> dead
            a = _probe(t, "p1.md", "pkg/d0.go:1 pkg/d1.go:2 pkg/d2.go:3")
            b = _probe(t, "p2.md", "pkg/d0.go:1 pkg/d1.go:2 pkg/d2.go:3")
            _, means = evaluate(s, [a, b])
            self.assertAlmostEqual(means["dependents"], 0.75)
            self.assertEqual(main(["x", s, a, b]), 1)

    def test_saleor_shaped_cell_survives(self):
        """A control at EXACTLY the bound must PASS. saleor is a banked win at this point.

        control 0.50 + sense 1.00 = delta +0.500 = the bar. Tightening this gate below 0.50
        throws saleor away: measured, control > 0.25 kills 14 cells INCLUDING that win.
        """
        with tempfile.TemporaryDirectory() as t:
            s = _scenario(t, GOLD4)
            a = _probe(t, "p1.md", "pkg/d0.go:1 pkg/d1.go:2")   # 2/4 = 0.50, exactly the bound
            b = _probe(t, "p2.md", "pkg/d0.go:1 pkg/d1.go:2")
            _, means = evaluate(s, [a, b])
            self.assertAlmostEqual(means["dependents"], 0.50)
            self.assertEqual(main(["x", s, a, b]), 0, "a control at the bound is WINNABLE")

    def test_a_floored_control_passes(self):
        with tempfile.TemporaryDirectory() as t:
            s = _scenario(t, GOLD4)
            a = _probe(t, "p1.md", "nothing relevant here")
            self.assertEqual(main(["x", s, a]), 0)

    def test_kill_needs_EVERY_group_dead(self):
        """pergroup flags a win on ANY group, so one live group keeps the cell alive.

        saleor's banked win is on its `context` group, not `dependents`. A gate that judged
        one group would re-create exactly the false negative BAR=0.50 exists to avoid.
        """
        gold = GOLD4 + [{"id": f"ctx:c{i}", "group": "context", "match": f"pkg/c{i}.go"}
                        for i in range(4)]
        with tempfile.TemporaryDirectory() as t:
            s = _scenario(t, gold)
            # dependents aced (1.00, dead) but context floored (0.00, alive) -> PASS
            txt = "pkg/d0.go:1 pkg/d1.go:2 pkg/d2.go:3 pkg/d3.go:4"
            a, b = _probe(t, "p1.md", txt), _probe(t, "p2.md", txt)
            _, means = evaluate(s, [a, b])
            self.assertAlmostEqual(means["dependents"], 1.0)
            self.assertAlmostEqual(means["context"], 0.0)
            self.assertEqual(main(["x", s, a, b]), 0, "one live group keeps the cell alive")

    def test_all_groups_dead_kills(self):
        gold = GOLD4 + [{"id": f"ctx:c{i}", "group": "context", "match": f"pkg/c{i}.go"}
                        for i in range(4)]
        with tempfile.TemporaryDirectory() as t:
            s = _scenario(t, gold)
            txt = ("pkg/d0.go:1 pkg/d1.go:2 pkg/d2.go:3 pkg/d3.go:4 "
                   "pkg/c0.go:1 pkg/c1.go:2 pkg/c2.go:3")
            a, b = _probe(t, "p1.md", txt), _probe(t, "p2.md", txt)
            self.assertEqual(main(["x", s, a, b]), 1)

    def test_mean_not_min_decides_because_the_verdict_uses_the_mean(self):
        """dolt's real shape: 0.444 and 0.889 -> mean 0.667 -> DEAD (ceiling +0.333).

        min() would read 0.444 and pass a cell whose delta ceiling is +0.333, because
        pergroup's delta is computed on MEANS. This pins the aggregation.
        """
        gold = [{"id": f"dep:d{i}", "group": "dependents", "match": f"pkg/d{i}.go"}
                for i in range(9)]
        with tempfile.TemporaryDirectory() as t:
            s = _scenario(t, gold)
            a = _probe(t, "p1.md", " ".join(f"pkg/d{i}.go:1" for i in range(4)))   # 4/9=.444
            b = _probe(t, "p2.md", " ".join(f"pkg/d{i}.go:1" for i in range(8)))   # 8/9=.889
            _, means = evaluate(s, [a, b])
            self.assertAlmostEqual(means["dependents"], (4 / 9 + 8 / 9) / 2, places=3)
            self.assertEqual(main(["x", s, a, b]), 1, "dolt-shaped control is DEAD on the mean")


class SlateRankTest(unittest.TestCase):
    """Pins for --slate: the ranking, and the expiry the ranking depends on.

    The load-bearing pin is `test_a_stale_probe_never_ranks`. Loop 7 ships fixes between
    verticals, so a kill is only true OF a build; a ranking that silently reuses a probe
    from an older binary is this gate lying with no one noticing.
    """

    def _vertical(self, tmp, repos, bin_bytes=b"build-A"):
        """Build a vertical tree: scenarios/<repo>.yaml + results/dryrun/<repo>/probe-N.md."""
        binp = os.path.join(tmp, "sense")
        with open(binp, "wb") as fh:
            fh.write(bin_bytes)
        for repo, cited in repos.items():
            sdir = os.path.join(tmp, "scenarios")
            os.makedirs(sdir, exist_ok=True)
            path = _scenario(tmp, GOLD4)
            os.replace(path, os.path.join(sdir, f"{repo}.yaml"))
            pdir = os.path.join(tmp, "results", "dryrun", repo)
            os.makedirs(pdir, exist_ok=True)
            for n in (1, 2):
                p = os.path.join(pdir, f"probe-{n}.md")
                with open(p, "w") as fh:
                    fh.write(" ".join(f"pkg/d{i}.go:1" for i in range(cited)))
                stamp(p, binp)
        return binp

    def test_weakest_control_ranks_first(self):
        """The repo with the most headroom goes first: that IS the depth-first queue."""
        with tempfile.TemporaryDirectory() as t:
            # strong-baseline cites 2/4 (control .50); weak-baseline cites 0/4 (control .00)
            binp = self._vertical(t, {"strong-baseline": 2, "weak-baseline": 0})
            rows = [control_bound.assess_repo(
                t, r, os.path.join(t, "results", "dryrun", r),
                binary_key(binp))
                for r in ("strong-baseline", "weak-baseline")]
            queue = sorted((r for r in rows if control_bound.rankable(r)),
                           key=lambda r: r["best_control"])
            self.assertEqual([r["repo"] for r in queue],
                             ["weak-baseline", "strong-baseline"])
            self.assertEqual(control_bound.slate(t, binp), 0)

    def test_a_stale_probe_never_ranks(self):
        """Same probe, different binary: the verdict is superseded, not reusable."""
        with tempfile.TemporaryDirectory() as t:
            self._vertical(t, {"repo-a": 0}, bin_bytes=b"build-A")
            newer = os.path.join(t, "sense-next")
            with open(newer, "wb") as fh:
                fh.write(b"build-B-after-a-loop7-fix")
            row = control_bound.assess_repo(
                t, "repo-a", os.path.join(t, "results", "dryrun", "repo-a"),
                binary_key(newer))
            self.assertTrue(row["alive"], "the cell is bound-legal on its own numbers")
            self.assertFalse(control_bound.rankable(row), "but it may not rank: STALE")
            self.assertEqual(control_bound.slate(t, newer), 1)

    def test_an_unstamped_probe_never_ranks(self):
        """No build recorded is not the same as an old build; neither one ranks."""
        with tempfile.TemporaryDirectory() as t:
            binp = self._vertical(t, {"repo-a": 0})
            os.remove(os.path.join(t, "results", "dryrun", "repo-a",
                                   "probe-1.md.build.json"))
            row = control_bound.assess_repo(
                t, "repo-a", os.path.join(t, "results", "dryrun", "repo-a"),
                binary_key(binp))
            self.assertIn("UNSTAMPED", row["freshness"])
            self.assertFalse(control_bound.rankable(row))

    def test_a_bound_killed_repo_never_ranks(self):
        with tempfile.TemporaryDirectory() as t:
            binp = self._vertical(t, {"dead-repo": 3})   # 3/4 -> control .75 -> dead
            row = control_bound.assess_repo(
                t, "dead-repo", os.path.join(t, "results", "dryrun", "dead-repo"),
                binary_key(binp))
            self.assertFalse(row["alive"])
            self.assertFalse(control_bound.rankable(row))
            self.assertEqual(control_bound.slate(t, binp), 1)

    def test_probe_dir_without_any_gold_is_unrankable_not_dead(self):
        with tempfile.TemporaryDirectory() as t:
            binp = self._vertical(t, {"repo-a": 0})
            os.remove(os.path.join(t, "scenarios", "repo-a.yaml"))
            row = control_bound.assess_repo(
                t, "repo-a", os.path.join(t, "results", "dryrun", "repo-a"),
                binary_key(binp))
            self.assertIn("no gold", row["note"])
            self.assertFalse(control_bound.rankable(row))

    def test_draft_gold_is_used_when_nothing_is_authored_yet(self):
        """Eligibility runs BEFORE authoring, so it scores against the mini-gold."""
        with tempfile.TemporaryDirectory() as t:
            binp = self._vertical(t, {"repo-a": 0})
            os.replace(os.path.join(t, "scenarios", "repo-a.yaml"),
                       os.path.join(t, "scenarios", "repo-a.draft.yaml"))
            path, kind = control_bound.scenario_for(t, "repo-a")
            self.assertEqual(kind, "draft")
            row = control_bound.assess_repo(
                t, "repo-a", os.path.join(t, "results", "dryrun", "repo-a"),
                binary_key(binp))
            self.assertEqual(row["gold_kind"], "draft")
            self.assertTrue(control_bound.rankable(row), "a draft cell still ranks")
            self.assertEqual(control_bound.slate(t, binp), 0)

    def test_audited_gold_wins_over_a_draft_when_both_exist(self):
        """Once authoring lands, the audited set is strictly better evidence."""
        with tempfile.TemporaryDirectory() as t:
            self._vertical(t, {"repo-a": 0})
            draft = os.path.join(t, "scenarios", "repo-a.draft.yaml")
            with open(draft, "w") as fh:
                fh.write("gold:\n  - {id: x, group: g, match: [pkg/x.go]}\n")
            path, kind = control_bound.scenario_for(t, "repo-a")
            self.assertEqual(kind, "audited")
            self.assertTrue(path.endswith("repo-a.yaml"))

    def test_empty_dryrun_tree_is_a_clean_error(self):
        with tempfile.TemporaryDirectory() as t:
            with self.assertRaises(SystemExit):
                control_bound.slate(t, None)


if __name__ == "__main__":
    unittest.main()
