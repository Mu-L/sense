#!/usr/bin/env python3
"""The four repo-level admission screens (docs/bootstrap.md).

    screen.py <clone_dir> --key <repo-key> [--url URL] [--stack composer.json:a|b]
                   [--json OUT] [--no-api]

Facts about the repo, never guesses about a scenario. Nothing here opens an
index or picks an anchor: what Sense can win on is decided in Loop 3, against a
scenario that exists. The gate this replaced measured seams before any scenario
existed and, backtested, rejected 4 of 4 banked wins.

  in_vertical  the repo DECLARES the stack in its own dependency manifest.
               --stack is `<manifest>:<needle>[|<needle>...]`, matched as ANY,
               at the root or one/two levels down for a monorepo. The pool
               file's say-so does not count, because a pool line is an
               assertion and a manifest is not.
  maintained   not archived, pushed within MAINTAINED_DAYS.
  size         prod source files on the clone. < SIZE_MEDIUM_FLOOR = small =
               reject (the small slot was removed by ruling). Also assigns the
               class: medium >= 1_000, big >= 4_000.
  used         stars >= STARS_FLOOR. A floor on "real code people depend on",
               and the tie-break when a slot is oversubscribed.

Recorded, never rejecting: the anti-LLM banner scan (the lobsters rule - a
banner means strip it from BOTH arms, it is not a verdict on the repo).

`maintained` and `used` need the GitHub API (`gh`). --no-api skips both and
marks them UNRUN, which NEVER admits: an unrun screen is not a passed screen.
"""

import argparse
import glob
import json
import os
import re
import subprocess
import sys
import time

MAINTAINED_DAYS = 365
SIZE_MEDIUM_FLOOR = 1_000
SIZE_BIG_FLOOR = 4_000
STARS_FLOOR = 1_000

# The union, used only when no language is named. Six queued verticals were
# missing from it, C# among them: bitwarden/server carries 5,282 .cs files and
# 24 files this tuple could see, so the size screen classed it small and the
# repo the C# extractor was proven on was rejected as too small to bench.
SRC_EXT = (".php", ".py", ".rb", ".go", ".ts", ".js", ".java", ".rs", ".kt",
           ".cs", ".swift", ".ex", ".exs", ".dart", ".clj", ".cljs", ".cljc",
           ".hs", ".zig", ".scala")

# Size is a question about the vertical's OWN language. Counting every language
# reads a monorepo's frontend as backend bulk: umbraco-cms was admitted as an
# 8,228-file repo on its TypeScript. Keys are the `lang` column of verticals.txt.
LANG_EXT = {
    "php": (".php",), "python": (".py",), "ruby": (".rb",), "golang": (".go",),
    "tsjs": (".ts", ".js"), "java": (".java",), "rust": (".rs",),
    "kotlin": (".kt",), "csharp": (".cs",), "swift": (".swift",),
    "elixir": (".ex", ".exs"), "dart": (".dart",),
    "clojure": (".clj", ".cljs", ".cljc"), "haskell": (".hs",), "zig": (".zig",),
}


def lang_exts(lang):
    """The extensions a vertical's size gauge counts. An unknown language falls
    back to the union rather than to zero: a wrong-but-wide count is visible,
    and a silent zero rejects every repo in the stack."""
    return LANG_EXT.get((lang or "").strip().lower(), SRC_EXT)
SKIP_DIRS = {".git", "vendor", "node_modules", "storage", "public", "bootstrap",
             "dist", "build", "tests", "test", "spec", "specs", "testing",
             "__pycache__", ".sense"}
TEST_FILE = re.compile(r"(^|[/_.])(test|tests|spec|specs)([/_.]|$)", re.I)

BANNER_DOCS = ("README.md", "README.rst", "README", "CONTRIBUTING.md",
               "AGENTS.md", "CLAUDE.md", ".github/CONTRIBUTING.md")
BANNER_RE = re.compile(
    r"(no|not?\s+for|ban(ned)?|forbid\w*|disallow\w*|reject\w*|prohibit\w*)\W{0,20}"
    r"(ai|llm|chatgpt|copilot|generative|machine[- ]generated|ai[- ]generated)"
    r"|(ai|llm|generative)\W{0,20}(content|code|contribution|pull request|pr)s?\W{0,20}"
    r"(are\s+)?(not\s+(welcome|accepted)|banned|forbidden|rejected)",
    re.I)


def prod_source_files(clone, exts=SRC_EXT):
    """Source files that are not vendored, generated or tests - the size gauge."""
    n = 0
    for root, dirs, files in os.walk(clone):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS and not d.startswith(".")]
        for f in files:
            if not f.endswith(exts):
                continue
            rel = os.path.relpath(os.path.join(root, f), clone)
            if TEST_FILE.search(rel):
                continue
            n += 1
    return n


def size_class(n):
    return "big" if n >= SIZE_BIG_FLOOR else "medium" if n >= SIZE_MEDIUM_FLOOR else "small"


DECLARED = {"ok": True, "why": "framework-role, declared in the stack profile"}
LISTED = {"ok": True, "why": "listed by name in the stack profile"}


def screen_in_vertical(clone, stack):
    """The repo declares the stack in its OWN manifest, not in our pool file.

    The needle takes `|`-separated alternatives, matched as ANY, because a stack
    has more than one way to be declared: a Laravel APP requires
    `laravel/framework`, but a Laravel PACKAGE (filament, flarum - the framework
    slot's whole candidate list) requires `illuminate/*` instead. A single needle
    silently rejected every framework-role repo in the pool."""
    if not stack:
        return {"ok": None, "why": "no --stack given (UNRUN)"}
    manifest, _, needles = stack.partition(":")
    alts = [n for n in needles.split("|") if n]
    # Root manifest first, then a bounded look into a monorepo's sub-packages:
    # filament's root composer.json declares nothing and every real requirement
    # sits in packages/*/composer.json.
    paths = ([os.path.join(clone, manifest)]
             + sorted(glob.glob(os.path.join(clone, "*", manifest)))
             + sorted(glob.glob(os.path.join(clone, "*", "*", manifest))))
    paths = [p for p in paths if os.path.exists(p)]
    if not paths:
        return {"ok": False, "manifest": manifest, "needles": alts,
                "why": f"no {manifest} anywhere in the clone"}
    for p in paths:
        with open(p, encoding="utf-8", errors="ignore") as fh:
            text = fh.read()
        hit = next((n for n in alts if n in text), None)
        if hit:
            where = os.path.relpath(p, clone)
            return {"ok": True, "manifest": manifest, "needles": alts,
                    "matched": hit, "where": where,
                    "why": f"declares {hit} in {where}"}
    return {"ok": False, "manifest": manifest, "needles": alts, "matched": None,
            "why": f"declares none of {'|'.join(alts)} in {len(paths)}× {manifest}"}


def gh_repo(url):
    """owner/name from a git URL, or None when it is not a GitHub remote."""
    m = re.search(r"github\.com[:/]+([^/]+)/(.+?)(?:\.git)?/?$", url or "")
    return f"{m.group(1)}/{m.group(2)}" if m else None


def gh_api(slug):
    p = subprocess.run(["gh", "api", f"repos/{slug}"],
                       capture_output=True, text=True, timeout=60)
    if p.returncode != 0:
        return None
    try:
        return json.loads(p.stdout)
    except json.JSONDecodeError:
        return None


def screen_maintained(meta):
    if meta is None:
        return {"ok": None, "why": "GitHub API unavailable (UNRUN)"}
    if meta.get("archived"):
        return {"ok": False, "archived": True, "why": "archived"}
    pushed = meta.get("pushed_at") or ""
    try:
        age_days = int((time.time() - time.mktime(
            time.strptime(pushed, "%Y-%m-%dT%H:%M:%SZ"))) / 86400)
    except ValueError:
        return {"ok": None, "why": f"unparseable pushed_at {pushed!r} (UNRUN)"}
    ok = age_days <= MAINTAINED_DAYS
    return {"ok": ok, "archived": False, "pushed_at": pushed, "age_days": age_days,
            "why": f"last push {age_days}d ago (floor {MAINTAINED_DAYS}d)"}


def screen_used(meta):
    if meta is None:
        return {"ok": None, "why": "GitHub API unavailable (UNRUN)"}
    stars = meta.get("stargazers_count")
    if stars is None:
        return {"ok": None, "why": "no stargazers_count (UNRUN)"}
    return {"ok": stars >= STARS_FLOOR, "stars": stars,
            "why": f"{stars} stars (floor {STARS_FLOOR})"}


def screen_size(clone, exts=SRC_EXT, lang=""):
    n = prod_source_files(clone, exts)
    cls = size_class(n)
    of = f" {lang}" if lang else ""
    return {"ok": cls != "small", "prod_files": n, "size": cls,
            "why": f"{n} prod{of} source files = {cls} (floor {SIZE_MEDIUM_FLOOR})"}


def scan_banner(clone):
    """The lobsters rule: a banner is a STRIP instruction for both arms, never a reject."""
    hits = []
    for doc in BANNER_DOCS:
        path = os.path.join(clone, doc)
        if not os.path.exists(path):
            continue
        with open(path, encoding="utf-8", errors="ignore") as fh:
            for i, line in enumerate(fh, 1):
                if BANNER_RE.search(line):
                    hits.append(f"{doc}:{i}: {line.strip()[:120]}")
    return hits


def apply_listed(out, lang):
    """A repo listed by name in the stack profile is admitted unless it is
    unreachable, unmaintained, or not written in this language.

    The listing is a human saying "bench this one, I know it". So the framework
    marker and the stars floor are waived outright, and size stops rejecting -
    it still measures, because the SLOT a repo can fill is a size question and
    an unslottable repo has to be visible as such rather than absent. What is
    not waived is language: a listed repo with no source in the vertical's
    language cannot be benched by a vertical about that language, and that is
    the one rejection the listing cannot override."""
    out["in_vertical"] = dict(LISTED)
    out["used"] = dict(out["used"], ok=True,
                       why=out["used"].get("why", "") + " - waived, listed")
    if out["size"]["prod_files"] == 0:
        out["size"] = dict(out["size"], ok=False,
                           why=f"no {lang or 'in-language'} source files - listed, "
                               "but this is not a repo of this language")
        return out
    out["size"] = dict(out["size"], ok=True,
                       why=out["size"]["why"] + " - size floor waived, listed")
    return out


def screen(clone, key, url, stack, use_api=True, lang="", listed=False):
    meta = None
    slug = gh_repo(url)
    if use_api and slug:
        meta = gh_api(slug)
    out = {"repo": key, "url": url, "github": slug,
           "in_vertical": screen_in_vertical(clone, stack),
           "maintained": screen_maintained(meta),
           "size": screen_size(clone, lang_exts(lang), lang),
           "used": screen_used(meta),
           "banner": scan_banner(clone)}
    if listed:
        out = apply_listed(out, lang)
    out["size_class"] = out["size"]["size"]
    verdicts = [out[k]["ok"] for k in ("in_vertical", "maintained", "size", "used")]
    if False in verdicts:
        out["verdict"] = "REJECT"
    elif None in verdicts:
        out["verdict"] = "UNRUN"      # never admits - an unrun screen is not a passed one
    else:
        out["verdict"] = "ADMIT"
    return out


def remote_manifest(slug, manifest):
    """The repo's root manifest over the API, no clone. Returns the text, or
    None when the repo has no such file at all."""
    p = subprocess.run(["gh", "api", f"repos/{slug}/contents/{manifest}",
                        "--jq", ".content"], capture_output=True, text=True,
                       timeout=60)
    if p.returncode != 0 or not p.stdout.strip():
        return None
    import base64
    try:
        return base64.b64decode(p.stdout.strip()).decode("utf-8", "ignore")
    except Exception:
        return None


def screen_in_vertical_remote(slug, stack):
    """Cheap in-vertical triage before anything is downloaded.

    Three outcomes, and the third one matters: a MISSING manifest is a real
    reject (not a project of this ecosystem at all), a MATCHING one is a real
    pass, and a present-but-non-matching one is UNDECIDED, never a reject,
    because a monorepo declares nothing at its root - filament's root
    composer.json is empty of requirements and every one lives in
    packages/*/composer.json. Undecided falls through to the clone."""
    manifest, _, needles = stack.partition(":")
    alts = [n for n in needles.split("|") if n]
    text = remote_manifest(slug, manifest)
    if text is None:
        return {"ok": False, "why": f"no {manifest} in the repo root (remote)"}
    hit = next((n for n in alts if n in text), None)
    if hit:
        return {"ok": True, "why": f"declares {hit} in {manifest} (remote)"}
    return {"ok": None, "why": f"root {manifest} declares none of them; "
                               "a monorepo may still, so clone and look"}


def screen_from_facts(key, url, stars, pushed):
    """maintained + used from facts the HUNT already paid for. The search that
    found a repo returns its stars and pushed_at, so re-asking the API 156 times
    is a minute of latency for data already on disk."""
    meta = {"archived": False, "stargazers_count": int(stars),
            "pushed_at": f"{pushed}T00:00:00Z"}
    out = {"repo": key, "url": url, "phase": "api-only",
           "maintained": screen_maintained(meta), "used": screen_used(meta),
           "in_vertical": {"ok": None, "why": "needs a clone (UNRUN)"},
           "size": {"ok": None, "prod_files": 0, "size": "unknown",
                    "why": "needs a clone (UNRUN)"},
           "banner": [], "size_class": "unknown"}
    out["verdict"] = "REJECT" if False in (out["maintained"]["ok"],
                                           out["used"]["ok"]) else "CLONE-ME"
    return out


def triage(key, url, stars, pushed, stack, declared=False, listed=False):
    """Phase 1: everything decidable without a download."""
    r = screen_from_facts(key, url, stars, pushed)
    if listed:
        # Waived here too, or a listed repo dies on stars before it is ever
        # cloned and the clone-stage waiver never gets to run.
        r["used"] = dict(r["used"], ok=True,
                         why=r["used"].get("why", "") + " - waived, listed")
        r["in_vertical"] = dict(LISTED)
        r["verdict"] = "REJECT" if r["maintained"]["ok"] is False else "CLONE-ME"
        return r
    if r["verdict"] == "REJECT" or not stack:
        return r
    if declared:
        # The screen filters the hunt's greedy output; it does not second-guess
        # a short, reviewed list someone wrote by hand. rails/rails declares no
        # rails dependency in its own Gemfile, and laravel/framework is not a
        # laravel APP either - a framework does not depend on itself.
        r["in_vertical"] = dict(DECLARED)
        return r
    slug = gh_repo(url)
    if slug:
        r["in_vertical"] = screen_in_vertical_remote(slug, stack)
        if r["in_vertical"]["ok"] is False:
            r["verdict"] = "REJECT"
    return r


def screen_api_only(key, url):
    """The two screens that read the API and never the disk. A 156-candidate
    pool is 120 clones; most die on maintained-or-used, and those two facts cost
    one API call each. Anything that survives here still faces the full screen."""
    meta = gh_api(gh_repo(url) or "") if gh_repo(url) else None
    out = {"repo": key, "url": url, "github": gh_repo(url), "phase": "api-only",
           "maintained": screen_maintained(meta), "used": screen_used(meta),
           "in_vertical": {"ok": None, "why": "needs a clone (UNRUN)"},
           "size": {"ok": None, "prod_files": 0, "size": "unknown",
                    "why": "needs a clone (UNRUN)"},
           "banner": []}
    out["size_class"] = "unknown"
    verdicts = [out["maintained"]["ok"], out["used"]["ok"]]
    out["verdict"] = "REJECT" if False in verdicts else "CLONE-ME"
    return out


def render(r):
    L = [f"### {r['repo']} - {r['verdict']} ({r['size_class']})"]
    for k in ("in_vertical", "maintained", "size", "used"):
        if r[k]["ok"] is None and r.get("phase") == "api-only":
            continue
        mark = {True: "pass", False: "FAIL", None: "UNRUN"}[r[k]["ok"]]
        L.append(f"- {k:12s} {mark:5s} {r[k]['why']}")
    if r["banner"]:
        L.append(f"- banner       FLAG  {len(r['banner'])} hit(s) - strip from BOTH arms, not a reject")
        L.extend(f"    {h}" for h in r["banner"][:3])
    return "\n".join(L)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("clone")
    ap.add_argument("--key", required=True)
    ap.add_argument("--url", default="")
    ap.add_argument("--stack", default=os.environ.get("STACK_MARKER", ""))
    ap.add_argument("--json", dest="json_out", default=None)
    ap.add_argument("--no-api", action="store_true")
    ap.add_argument("--declared", action="store_true",
                    help="this repo is declared framework-role in the stack "
                         "profile, so the in-vertical screen passes by declaration")
    ap.add_argument("--listed", action="store_true",
                    help="this repo is listed by name in the stack profile, so "
                         "the marker and the stars floor are waived and size "
                         "stops rejecting; language still binds")
    ap.add_argument("--lang", default="",
                    help="the vertical's language (verticals.txt column 2). The "
                         "size gauge counts THIS language, never every language "
                         "in the tree")
    ap.add_argument("--stars", default="")
    ap.add_argument("--pushed", default="", help="YYYY-MM-DD, from the hunt")
    ap.add_argument("--api-only", action="store_true",
                    help="run only the two screens that need no clone "
                         "(maintained, used), so a large pool is thinned for "
                         "free before anything is downloaded")
    args = ap.parse_args()

    if args.api_only:
        if args.stars and args.pushed:
            r = triage(args.key, args.url, args.stars, args.pushed, args.stack,
                       declared=args.declared, listed=args.listed)
        else:
            r = screen_api_only(args.key, args.url)
    else:
        if not os.path.isdir(args.clone):
            sys.exit(f"screen: no such clone: {args.clone}")
        r = screen(args.clone, args.key, args.url, args.stack,
                   use_api=not args.no_api, lang=args.lang, listed=args.listed)
        if args.declared:
            r["in_vertical"] = dict(DECLARED)
            if r["verdict"] == "REJECT" and r["size"]["ok"] and r["maintained"]["ok"] \
                    and r["used"]["ok"]:
                r["verdict"] = "ADMIT"
    print(render(r))
    print(f"SCREEN: {r['verdict']}")
    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(r, fh, indent=1, sort_keys=True)
    return 0 if r["verdict"] in ("ADMIT", "CLONE-ME") else 1


if __name__ == "__main__":
    sys.exit(main())
