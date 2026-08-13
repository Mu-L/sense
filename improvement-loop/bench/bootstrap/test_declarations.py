#!/usr/bin/env python3
"""The two declarations a stack profile can make, and the size gauge they lean on.

Known-answer control for both: one row known true, one known false, per rule. The
precedent these tests exist to stop repeating is measured, not hypothetical - the
C# hunt admitted three repos on their TypeScript file counts and rejected
bitwarden/server, 5,282 .cs files, as too small to bench.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import hunt  # noqa: E402
import screen  # noqa: E402


def tree(**counts):
    """A clone with `counts` files per extension, e.g. tree(cs=5, ts=2)."""
    d = tempfile.mkdtemp()
    for ext, n in counts.items():
        for i in range(n):
            open(os.path.join(d, f"f{i}.{ext}"), "w").close()
    return d


def conf(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".conf", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


class SizeIsCountedInTheVerticalsLanguage(unittest.TestCase):
    def test_csharp_counts_cs_and_not_typescript(self):
        clone = tree(cs=1200, ts=5)
        self.assertEqual(screen.prod_source_files(clone, screen.lang_exts("csharp")), 1200)

    def test_a_typescript_frontend_does_not_make_a_csharp_repo_big(self):
        clone = tree(cs=10, ts=8000)
        r = screen.screen_size(clone, screen.lang_exts("csharp"), "csharp")
        self.assertEqual(r["prod_files"], 10)
        self.assertEqual(r["size"], "small")

    def test_every_queued_language_has_an_extension(self):
        queued = ("ruby csharp golang java python rust kotlin swift elixir dart "
                  "clojure php tsjs haskell zig").split()
        missing = [q for q in queued if q not in screen.LANG_EXT]
        self.assertEqual(missing, [], f"no extensions declared for {missing}")

    def test_an_unknown_language_falls_back_to_the_union_not_to_zero(self):
        self.assertEqual(screen.lang_exts("cobol"), screen.SRC_EXT)
        self.assertEqual(screen.lang_exts(""), screen.SRC_EXT)


class ListedRepoWaivers(unittest.TestCase):
    BASE = {
        "in_vertical": {"ok": False, "why": "no Directory.Packages.props"},
        "maintained": {"ok": True, "why": "pushed 3 days ago"},
        "size": {"ok": False, "prod_files": 400, "size": "small", "why": "400 files"},
        "used": {"ok": False, "stars": 12, "why": "12 stars (floor 1000)"},
    }

    def listed(self, **over):
        out = {k: dict(v) for k, v in self.BASE.items()}
        out.update({k: dict(v) for k, v in over.items()})
        return screen.apply_listed(out, "csharp")

    def test_the_manifest_marker_is_waived(self):
        self.assertTrue(self.listed()["in_vertical"]["ok"])

    def test_the_stars_floor_is_waived(self):
        self.assertTrue(self.listed()["used"]["ok"])

    def test_the_size_floor_is_waived_but_the_class_still_reads_small(self):
        r = self.listed()
        self.assertTrue(r["size"]["ok"])
        self.assertEqual(r["size"]["size"], "small")

    def test_the_wrong_language_is_the_one_thing_a_listing_cannot_override(self):
        r = self.listed(size={"ok": True, "prod_files": 0, "size": "small", "why": "0"})
        self.assertFalse(r["size"]["ok"])
        self.assertIn("not a repo of this language", r["size"]["why"])

    def test_maintained_is_not_waived(self):
        r = self.listed(maintained={"ok": False, "why": "archived"})
        self.assertFalse(r["maintained"]["ok"])


class ScreenVerdicts(unittest.TestCase):
    def test_a_listed_repo_admits_where_an_unlisted_one_rejects(self):
        clone = tree(cs=1500)
        stack = "Directory.Packages.props:Microsoft.AspNetCore"
        plain = screen.screen(clone, "x", "", stack, use_api=False, lang="csharp")
        self.assertEqual(plain["in_vertical"]["ok"], False)   # no such manifest
        listed = screen.screen(clone, "x", "", stack, use_api=False, lang="csharp",
                               listed=True)
        self.assertTrue(listed["in_vertical"]["ok"])
        # both still UNRUN on the API screens, which never admit - the point here
        # is the in_vertical fact flipping, not the final verdict
        self.assertNotEqual(listed["verdict"], "REJECT")

    def test_triage_lets_a_listed_repo_reach_the_clone_on_low_stars(self):
        r = screen.triage("x", "https://github.com/a/b", "12", "2026-08-01",
                          "manifest:needle", listed=True)
        self.assertEqual(r["verdict"], "CLONE-ME")

    def test_triage_still_rejects_a_listed_repo_that_is_unmaintained(self):
        r = screen.triage("x", "https://github.com/a/b", "9000", "2019-01-01",
                          "manifest:needle", listed=True)
        self.assertEqual(r["verdict"], "REJECT")


class ConfGrammar(unittest.TestCase):
    def test_repo_lines_are_read_and_kept_apart_from_framework_lines(self):
        p = conf("stack: a.props:Needle\n"
                 "hunt: --language c#\n"
                 "framework: dotnet/aspnetcore\n"
                 "repo: bitwarden/server\n"
                 "repo: ServiceStack/ServiceStack\n")
        stack, queries, frameworks, listed = hunt.read_conf(p)
        self.assertEqual(stack, "a.props:Needle")
        self.assertEqual(queries, ["--language c#"])
        self.assertEqual(frameworks, {"dotnet/aspnetcore"})
        self.assertEqual(listed, {"bitwarden/server", "servicestack/servicestack"})

    def test_a_profile_with_no_repo_lines_still_reads(self):
        p = conf("stack: a.props:Needle\nhunt: --language c#\n")
        _, _, frameworks, listed = hunt.read_conf(p)
        self.assertEqual((frameworks, listed), (set(), set()))



class ListedRepoRanking(unittest.TestCase):
    """A listing that cannot win its own slot did nothing."""

    def cells(self, tmp, rows):
        import json
        for repo, stars, size in rows:
            with open(os.path.join(tmp, f"{repo}.json"), "w") as fh:
                json.dump({"repo": repo, "verdict": "ADMIT", "size_class": size,
                           "size": {"prod_files": 5000}, "used": {"stars": stars}}, fh)
        return tmp

    def test_a_listed_repo_outranks_a_more_popular_hunt_find(self):
        import compose
        d = self.cells(tempfile.mkdtemp(),
                       [("roslyn", 20608, "big"), ("bitwarden-server", 19827, "big")])
        order = [c["repo"] for c in compose.load_cells(d, {"bitwarden-server"})]
        self.assertEqual(order[0], "bitwarden-server")

    def test_without_a_listing_stars_still_decide(self):
        import compose
        d = self.cells(tempfile.mkdtemp(),
                       [("roslyn", 20608, "big"), ("bitwarden-server", 19827, "big")])
        order = [c["repo"] for c in compose.load_cells(d)]
        self.assertEqual(order[0], "roslyn")

    def test_listed_repos_keep_star_order_among_themselves(self):
        import compose
        d = self.cells(tempfile.mkdtemp(),
                       [("a", 100, "big"), ("b", 900, "big"), ("c", 50000, "big")])
        order = [c["repo"] for c in compose.load_cells(d, {"a", "b"})]
        self.assertEqual(order, ["b", "a", "c"])


if __name__ == "__main__":
    unittest.main()
