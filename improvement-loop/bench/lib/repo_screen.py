#!/usr/bin/env python3
"""Loop 2's four repo-level screens (docs/loops/02-repo-admission.md).

    repo_screen.py <clone_dir> --key <repo-key> [--url URL] [--stack composer.json:a|b]
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

SRC_EXT = (".php", ".py", ".rb", ".go", ".ts", ".js", ".java", ".rs", ".kt")
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


def prod_source_files(clone):
    """Source files that are not vendored, generated or tests - the size gauge."""
    n = 0
    for root, dirs, files in os.walk(clone):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS and not d.startswith(".")]
        for f in files:
            if not f.endswith(SRC_EXT):
                continue
            rel = os.path.relpath(os.path.join(root, f), clone)
            if TEST_FILE.search(rel):
                continue
            n += 1
    return n


def size_class(n):
    return "big" if n >= SIZE_BIG_FLOOR else "medium" if n >= SIZE_MEDIUM_FLOOR else "small"


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


def screen_size(clone):
    n = prod_source_files(clone)
    cls = size_class(n)
    return {"ok": cls != "small", "prod_files": n, "size": cls,
            "why": f"{n} prod source files = {cls} (floor {SIZE_MEDIUM_FLOOR})"}


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


def screen(clone, key, url, stack, use_api=True):
    meta = None
    slug = gh_repo(url)
    if use_api and slug:
        meta = gh_api(slug)
    out = {"repo": key, "url": url, "github": slug,
           "in_vertical": screen_in_vertical(clone, stack),
           "maintained": screen_maintained(meta),
           "size": screen_size(clone),
           "used": screen_used(meta),
           "banner": scan_banner(clone)}
    out["size_class"] = out["size"]["size"]
    verdicts = [out[k]["ok"] for k in ("in_vertical", "maintained", "size", "used")]
    if False in verdicts:
        out["verdict"] = "REJECT"
    elif None in verdicts:
        out["verdict"] = "UNRUN"      # never admits - an unrun screen is not a passed one
    else:
        out["verdict"] = "ADMIT"
    return out


def render(r):
    L = [f"### {r['repo']} - {r['verdict']} ({r['size_class']})"]
    for k in ("in_vertical", "maintained", "size", "used"):
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
    args = ap.parse_args()

    if not os.path.isdir(args.clone):
        sys.exit(f"repo_screen: no such clone: {args.clone}")
    r = screen(args.clone, args.key, args.url, args.stack, use_api=not args.no_api)
    print(render(r))
    print(f"SCREEN: {r['verdict']}")
    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(r, fh, indent=1, sort_keys=True)
    return 0 if r["verdict"] == "ADMIT" else 1


if __name__ == "__main__":
    sys.exit(main())
