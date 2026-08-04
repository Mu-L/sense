#!/usr/bin/env python3
"""scenario_root_migrate.py - move existing cells under their scenario-version root.

    scenario_root_migrate.py <results-root> [...] [--apply]

WHY. Runs are now filed under `<root>/<version>/<arm>/<repo>/run-N` so that two questions can
never share a cell (the rationale is in bench-paths.sh). Trees written before that live at
`<root>/<arm>/<repo>/run-N`. Left alone they do not merely look untidy: the NEXT run for an
existing cell lands in the versioned root while its predecessors stay in the old one, and
every reader then sees a cell with one run in it instead of three. Migrating is what keeps
`rails` at n=3 rather than splitting it into n=2 plus n=1.

WHAT IT MOVES. Only directories matching `<root>/<arm>/<repo>/run-N` that hold a
`run_meta.json` with a `scenario_version`. Reports and variance pages beside them are left
alone, and a root that is already versioned is skipped rather than nested one level deeper.

RUNS WITH NO VERSION. A run whose `run_meta.json` is missing or carries no version cannot be
attributed to a question. It is moved to `<root>/unversioned/` and NAMED in the output rather
than deleted or silently left in the old layout - one such run exists today, a minibench
baseline killed before it wrote its metadata.

DRY BY DEFAULT. Prints the moves and changes nothing unless `--apply` is given.
"""
import argparse
import json
import os
import shutil
import sys

HEX16 = 16


def _is_version_dir(name):
    return len(name) == HEX16 and all(c in "0123456789abcdef" for c in name)


def run_version(run_dir):
    """The scenario version a run belongs to, or None when it cannot be attributed."""
    try:
        with open(os.path.join(run_dir, "run_meta.json")) as fh:
            ver = json.load(fh).get("scenario_version")
    except (OSError, ValueError):
        return None
    if not ver:
        return None
    return str(ver).split(":", 1)[-1]


def plan(root):
    """[(src, dest, version)] for every run dir that should move under `root`."""
    moves = []
    for arm in sorted(os.listdir(root)) if os.path.isdir(root) else []:
        arm_dir = os.path.join(root, arm)
        # Already-migrated roots, and the phase roots that are themselves results roots,
        # are not arms - descending into them would nest one question inside another.
        if not os.path.isdir(arm_dir) or _is_version_dir(arm) or arm in (
                "validation", "minibench", "unversioned", "variance", "loop", "dryrun"):
            continue
        for repo in sorted(os.listdir(arm_dir)):
            repo_dir = os.path.join(arm_dir, repo)
            if not os.path.isdir(repo_dir):
                continue
            for run in sorted(os.listdir(repo_dir)):
                run_dir = os.path.join(repo_dir, run)
                if not run.startswith("run-") or not os.path.isdir(run_dir):
                    continue
                ver = run_version(run_dir) or "unversioned"
                moves.append((run_dir, os.path.join(root, ver, arm, repo, run), ver))
    return moves


def apply_moves(moves):
    for src, dest, _ver in moves:
        if os.path.exists(dest):
            raise SystemExit(f"refusing to overwrite an existing {dest}")
        os.makedirs(os.path.dirname(dest), exist_ok=True)
        shutil.move(src, dest)
    # Leave the vacated <arm>/<repo> shells behind only if something else is in them.
    for src, _dest, _ver in moves:
        for d in (os.path.dirname(src), os.path.dirname(os.path.dirname(src))):
            if os.path.isdir(d) and not os.listdir(d):
                os.rmdir(d)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("roots", nargs="+")
    ap.add_argument("--apply", action="store_true", help="perform the moves (default: dry run)")
    args = ap.parse_args()

    total, unversioned = 0, []
    for root in args.roots:
        moves = plan(root)
        if not moves:
            print(f"# {os.path.relpath(root)}: nothing to migrate")
            continue
        by_ver = {}
        for src, dest, ver in moves:
            by_ver.setdefault(ver, []).append((src, dest))
        print(f"# {os.path.relpath(root)}")
        for ver, items in sorted(by_ver.items()):
            print(f"  {ver}  {len(items)} run(s)")
            for src, dest in items:
                print(f"    {os.path.relpath(src, root)}  ->  {os.path.relpath(dest, root)}")
            if ver == "unversioned":
                unversioned += [s for s, _ in items]
        if args.apply:
            apply_moves(moves)
        total += len(moves)

    print(f"\n{total} run(s) {'moved' if args.apply else 'would move'}")
    if unversioned:
        print(f"{len(unversioned)} run(s) could not be attributed to a question and "
              f"{'went' if args.apply else 'would go'} to unversioned/:")
        for s in unversioned:
            print(f"  {s}")
    if not args.apply:
        print("\ndry run - nothing was changed. Re-run with --apply.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
