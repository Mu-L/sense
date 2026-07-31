#!/usr/bin/env python3
"""Run a stack's DECLARED hunt queries and write verticals/<key>/pool.txt.

    hunt_pool.py <key> [--conf stacks/<key>.conf] [--out verticals/<key>/pool.txt]
                       [--limit 60] [--force]

The queries live in `stacks/<key>.conf`, not in this file and not in a session's
head. That is the point: "the pool is exhausted" is only a checkable claim if
the search that produced it can be re-run by someone else.

What this does NOT do is decide anything. It proposes candidates; every one is
then verified by repo_screen.py against the repo's own manifest, its API facts
and its file count. A repo that does not exist, or is not this stack, dies
there. So the hunt is allowed to be greedy and wrong.

The one field it cannot derive is the framework ROLE, which the conf declares
(measured: composer.json `type` is unset for laravel/framework, filament,
statamic and flarum, and `project` for october and winter, so the manifest does
not separate a framework from an application).

Refuses to overwrite an existing pool.txt without --force: a pool that has been
curated by hand is not the hunt's to discard.
"""

import argparse
import json
import os
import shlex
import subprocess
import sys

FIELDS = "fullName,stargazersCount,pushedAt,isArchived"


def read_conf(path):
    stack, queries, frameworks = "", [], set()
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            head, _, rest = line.partition(":")
            head, rest = head.strip(), rest.strip()
            if head == "stack":
                stack = rest
            elif head == "hunt":
                queries.append(rest)
            elif head == "framework":
                frameworks.add(rest.lower())
    return stack, queries, frameworks


def run_query(query, limit):
    """One `gh search repos` call. A hunt line is raw gh ARGV, not one string:
    `gh search repos "language:php size:>40000"` collapses to a single search
    TERM and returns "none of the search qualifiers apply", so pure-qualifier
    searches - which is where the large applications are - must be passed as
    flags. Measured: 3 of 4 query-string lines failed that way, silently
    narrowing the pool to whatever the one surviving keyword query found.

    A failed query is reported, never silent."""
    p = subprocess.run(
        ["gh", "search", "repos"] + shlex.split(query)
        + ["--limit", str(limit), "--json", FIELDS],
        capture_output=True, text=True, timeout=120)
    if p.returncode != 0:
        print(f"   QUERY FAILED: {query}\n      {p.stderr.strip()[:200]}", file=sys.stderr)
        return None
    try:
        return json.loads(p.stdout)
    except json.JSONDecodeError:
        print(f"   QUERY UNPARSEABLE: {query}", file=sys.stderr)
        return None


def key_for(full_name):
    """Repo key = the repo half of owner/name, lowercased. `cms` and `framework`
    and `panel` are meaningless on their own, so those keep the owner."""
    owner, _, name = full_name.partition("/")
    name = name.lower()
    if name in ("cms", "framework", "panel", "core", "app", "server", "api", "laravel"):
        return f"{owner.lower()}-{name}"
    return name


def hunt(conf_path, limit):
    stack, queries, frameworks = read_conf(conf_path)
    if not stack:
        sys.exit(f"hunt_pool: {conf_path} has no `stack:` marker")
    if not queries:
        sys.exit(f"hunt_pool: {conf_path} declares no `hunt:` queries")

    found, per_query, failed = {}, [], 0
    for q in queries:
        rows = run_query(q, limit)
        if rows is None:
            failed += 1
            per_query.append((q, None))
            continue
        new = 0
        for r in rows:
            if r.get("isArchived"):
                continue          # the maintained screen would kill it anyway
            fn = r["fullName"]
            if fn not in found:
                new += 1
            found.setdefault(fn, r)
        per_query.append((q, (len(rows), new)))
        print(f"   {len(rows):3d} hits, {new:3d} new   {q}", file=sys.stderr)
    return stack, found, frameworks, per_query, failed


def render(key, stack, found, frameworks, per_query):
    L = [f"# {key} candidate pool - one `repo-key|git-url|framework?|stars|pushed`",
         "# per line. stars and pushed come from the search that found the repo, so",
         "# the maintained and used screens cost ZERO API calls: a 156-candidate pool",
         "# is thinned to the handful worth cloning for free. A hand-added line may",
         "# omit them, and then those two screens fall back to one API call for it.",
         "#",
         "# WRITTEN BY hunt_pool.py from the declared queries in "
         f"stacks/{key}.conf.",
         "# Re-running the hunt reproduces it; widening it means editing that conf,",
         "# or adding a line here by hand, which the hunt will not overwrite without",
         "# --force.",
         "#",
         "# The third field marks a FRAMEWORK-role repo (something others build ON):",
         "# eligible for the framework slot, never for the big slot, because the",
         "# pillar rule keeps the framework from being the campaign's sole win.",
         "#",
         f"# stack: {stack}",
         "#",
         "# Queries run, and what each contributed:"]
    for q, res in per_query:
        L.append(f"#   {'FAILED  ' if res is None else '%3d hits, %3d new' % res}  {q}")
    L += ["", f"# {len(found)} candidates"]

    rows = sorted(found.values(), key=lambda r: -r["stargazersCount"])
    for r in rows:
        fn = r["fullName"]
        role = "framework" if fn.lower() in frameworks else ""
        L.append(f"{key_for(fn)}|https://github.com/{fn}.git|{role}"
                 f"|{r['stargazersCount']}|{r['pushedAt'][:10]}")
    return "\n".join(L) + "\n"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("key")
    ap.add_argument("--conf", default=None)
    ap.add_argument("--out", default=None)
    ap.add_argument("--limit", type=int, default=60)
    ap.add_argument("--force", action="store_true")
    args = ap.parse_args()

    root = os.path.normpath(os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    conf = args.conf or os.path.join(root, "stacks", f"{args.key}.conf")
    out = args.out or os.path.join(root, "verticals", args.key, "pool.txt")

    if not os.path.exists(conf):
        sys.exit(f"hunt_pool: no stack profile at {conf}")
    if os.path.exists(out) and not args.force:
        print(f"   pool.txt exists, keeping it (pass --force to re-hunt)", file=sys.stderr)
        return 0

    stack, found, frameworks, per_query, failed = hunt(conf, args.limit)
    if not found:
        print("hunt_pool: every query returned nothing - check `gh auth status`",
              file=sys.stderr)
        return 1
    with open(out, "w", encoding="utf-8") as fh:
        fh.write(render(args.key, stack, found, frameworks, per_query))
    print(f"   wrote {len(found)} candidates to {os.path.relpath(out, root)}"
          + (f" ({failed} query/queries FAILED)" if failed else ""), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
