#!/usr/bin/env python3
"""scenario_archive.py - keep the scored bytes of every scenario version, forever.

    scenario_archive.py add  <scenario.yaml> [rubric.yaml] [--store DIR]
    scenario_archive.py get  <sha256:...>                  [--store DIR]
    scenario_archive.py list                               [--store DIR]

WHY THIS EXISTS. `run_meta.json` records a `scenario_version` and a `scenario_file`, but the
file it names is the LIVE path, which every authoring cycle overwrites. So a number on disk
knows the identity of the question that produced it and cannot reach its gold. Recovering
which question produced the twelve scored mastodon runs took a brute-force search over ten
`.bak` files crossed with two rubrics - and the answer was a backup plus the COMMITTED rubric,
not the pair sitting in the working tree. That is a lookup, and this makes it one.

WHAT IT STORES. Exactly the bytes `scenario_version.py` hashes: the scenario yaml and its
rubric sibling, under `<store>/<16-hex>/`. Content-addressed, so `add` on an unchanged pair is
a no-op and the same version can never hold two different golds. Nothing is ever overwritten
or deleted; a version that changes its bytes is a different version by construction.

WHAT IT DOES NOT DO. It does not read results, score anything, or decide anything. It is a
side-effect-free lookup in the `get`/`list` direction, and an append-only copy in `add`.
"""
import argparse
import os
import shutil
import sys

LIB = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, LIB)
from scenario_version import version as compute_version  # noqa: E402


def default_store(scenario):
    """Beside the scenarios themselves, so a vertical carries its own history."""
    return os.path.join(os.path.dirname(os.path.abspath(scenario)), ".versions")


def rubric_for(scenario):
    """The sibling scenario_version.py hashes. A `.bak` has no sibling by that rule, so
    the caller must pass one explicitly - which is precisely how ten backups came to hash
    to versions that matched nothing."""
    return scenario[:-5] + ".rubric.yaml" if scenario.endswith(".yaml") else ""


def add(scenario, rubric=None, store=None):
    """Copy a scenario+rubric pair into the store. Returns (version, was_new)."""
    if rubric is None:
        rubric = rubric_for(scenario)
    ver = compute_version(scenario, rubric)
    store = store or default_store(scenario)
    dest = os.path.join(store, ver.split(":", 1)[1])
    if os.path.isdir(dest):
        return ver, False
    os.makedirs(dest, exist_ok=True)
    shutil.copyfile(scenario, os.path.join(dest, "scenario.yaml"))
    if rubric and os.path.exists(rubric):
        shutil.copyfile(rubric, os.path.join(dest, "rubric.yaml"))
    return ver, True


def get(ver, store):
    """The archived scenario for a version, or None. Accepts the bare hex or the sha256: form."""
    dest = os.path.join(store, ver.split(":", 1)[-1], "scenario.yaml")
    return dest if os.path.isfile(dest) else None


def versions(store):
    if not os.path.isdir(store):
        return []
    return sorted(d for d in os.listdir(store)
                  if os.path.isfile(os.path.join(store, d, "scenario.yaml")))


def gold_paths(scenario_yaml):
    """{row id: first file-like match pattern} for an archived scenario.

    The file pattern is the row's durable identity: a re-gold renames `d:feed-manager` to
    something else and mints a new scenario version, but the row still points at
    `app/lib/feed_manager.rb`, and THAT is what difficulty should accumulate against.
    Rows with no file-like pattern (pure symbol targets) have no durable key and are
    returned under their id, so they still appear rather than vanishing.

    Which patterns count as files is gold.py's rule, imported rather than restated:
    `annual_reports_presenter.rb` has no slash but is still a file, and a second copy of
    that test is how the two silently stop agreeing.
    """
    import yaml

    from gold import _is_file_like
    with open(scenario_yaml) as fh:
        doc = yaml.safe_load(fh) or {}
    out = {}
    for item in doc.get("gold", []) or []:
        if isinstance(item, str):
            item = {"id": item, "match": [item]}
        rid = item.get("id") or (item.get("match") or ["?"])[0]
        files = [str(p) for p in (item.get("match") or []) if _is_file_like(str(p))]
        out[rid] = files[0] if files else rid
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    sub = ap.add_subparsers(dest="cmd", required=True)
    a = sub.add_parser("add"); a.add_argument("scenario"); a.add_argument("rubric", nargs="?")
    a.add_argument("--store")
    g = sub.add_parser("get"); g.add_argument("version"); g.add_argument("--store", required=True)
    ls = sub.add_parser("list"); ls.add_argument("--store", required=True)
    args = ap.parse_args()

    if args.cmd == "add":
        ver, new = add(args.scenario, args.rubric, args.store)
        print(f"{ver}  {'archived' if new else 'already archived'}")
        return 0
    if args.cmd == "get":
        path = get(args.version, args.store)
        if not path:
            print(f"no archived scenario for {args.version} in {args.store}", file=sys.stderr)
            return 1
        print(path)
        return 0
    for v in versions(args.store):
        print(f"sha256:{v}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
