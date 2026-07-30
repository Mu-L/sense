#!/usr/bin/env python3
"""Compose the §7.0 slate from the gate's ADMIT cells, and write the loop's output.

    compose_slate.py <vertical> --cells DIR [--clones DIR] [--write]

Reads every `*.json` the gate wrote into --cells, keeps the ADMITs, and picks
one repo per slot. Without --write it prints the composition and changes
nothing; with --write it writes repos.txt, PINNED_COMMITS.json and slate.json.

PICKING RULE, in order:
  1. the WIN SIGNATURE (no usable cover + token precision <= 0.3 +
     total_affected >= 500) - what all three banked wins share;
  2. then total_affected, descending.
A repo is used ONCE: its best admitted anchor takes a slot, and the next-best
repo of the same size class becomes that slot's backup. Anything left over is
recorded, not discarded - a rejected repo is not a dead repo.

The slate itself is only valid if `slate_check.py` passes afterwards; this
script composes, that one verifies, and they are deliberately separate so the
composer cannot bless its own output.
"""

import argparse
import glob
import json
import os
import subprocess
import sys

PRECISION_CEILING = 0.30
AFFECTED_FLOOR = 500
SIZE_MEDIUM_FLOOR = 1_000
SIZE_BIG_FLOOR = 4_000
LAYOUTS = ({"framework": 1, "big": 1, "medium": 2}, {"big": 2, "medium": 2})


def size_of(clone):
    db = os.path.join(clone, ".sense", "index.db")
    if not os.path.exists(db):
        return None, None
    import sqlite3
    con = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
    n = con.execute(
        "SELECT count(*) FROM sense_files WHERE path NOT LIKE '%vendor/%' "
        "AND path NOT LIKE '%node_modules/%' AND path NOT LIKE '%test%' "
        "AND path NOT LIKE '%spec%'").fetchone()[0]
    con.close()
    return n, ("big" if n >= SIZE_BIG_FLOOR else
               "medium" if n >= SIZE_MEDIUM_FLOOR else "small")


def load_cells(cells_dir, clones):
    out = []
    for p in sorted(glob.glob(os.path.join(cells_dir, "*.json"))):
        try:
            d = json.load(open(p))
        except (json.JSONDecodeError, OSError):
            continue
        if d.get("admission") != "ADMIT" or "bar2" not in d:
            continue
        repo = os.path.basename(os.path.abspath(d.get("clone", "")))
        b2, b3 = d["bar2"], d.get("bar3", {})
        n, size = size_of(os.path.join(clones, repo))
        prec = b2.get("precision")
        out.append({
            "repo": repo, "symbol": d["symbol"], "file": d.get("file_hint"),
            "affected": b2.get("total_affected", 0), "precision": prec,
            "prod_files": n, "size": size,
            "win_signature": bool(not b3.get("covered")
                                  and prec is not None and prec <= PRECISION_CEILING
                                  and b2.get("total_affected", 0) >= AFFECTED_FLOOR)})
    out.sort(key=lambda c: (not c["win_signature"], -c["affected"]))
    return out


def pick(cells, framework_repos):
    """One repo per slot, best cell first, each repo used once."""
    slate, backups, used = {}, {}, set()
    want = dict(LAYOUTS[0])
    for cell in cells:
        if cell["repo"] in used or cell["size"] == "small":
            continue
        # §7.0's pillar rule, exactly: "two independent win pillars - don't let
        # the framework repo be the SOLE win, require >=1 big APP". So the BIG
        # slot must be a non-framework app. A framework is otherwise free to
        # take the framework slot or stand in a medium one (statamic and october
        # are frameworks AND perfectly good mediums); barring them outright just
        # shrinks the pool for no gain.
        if cell["repo"] in framework_repos:
            slot = "framework" if want.get("framework") else "medium"
        elif cell["size"] == "big" and not want.get("big") and want.get("medium"):
            slot = "medium"          # a big app can stand in for a medium slot
        else:
            slot = cell["size"]
        if want.get(slot):
            slate[cell["repo"]] = dict(cell, slot=slot)
            want[slot] -= 1
            used.add(cell["repo"])
    for cell in cells:
        if cell["repo"] in used or cell["size"] == "small":
            continue
        for repo, s in slate.items():
            if repo in backups:
                continue
            is_fw = cell["repo"] in framework_repos
            if s["slot"] == "framework" and not is_fw:
                continue          # only a framework backs the framework slot
            if s["slot"] == "big" and (is_fw or cell["size"] != "big"):
                continue          # the big slot's standby must also be a big APP
            if s["slot"] == "medium" and cell["size"] == "small":
                continue
            backups[repo] = cell
            used.add(cell["repo"])
            break
    return slate, backups


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("vertical")
    ap.add_argument("--cells", required=True)
    ap.add_argument("--clones", default=os.environ.get(
        "SENSE_CLONES", os.path.expanduser(
            "~/Developer/luuuc/oss/sense-benchmark/sense")))
    ap.add_argument("--frameworks", default="",
                    help="comma-separated repo keys that are frameworks (slot hint)")
    ap.add_argument("--write", action="store_true")
    args = ap.parse_args()

    root = os.path.normpath(os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "..", "..",
        "verticals", args.vertical))
    fw = {x for x in args.frameworks.split(",") if x}
    cells = load_cells(args.cells, args.clones)
    slate, backups = pick(cells, fw)

    print(f"### Compose - {args.vertical} ({len(cells)} ADMIT cells)")
    for repo, s in slate.items():
        b = backups.get(repo)
        print(f"- {s['slot']:9} `{repo}` `{s['symbol']}` affected={s['affected']} "
              f"precision={s['precision']} win_signature={s['win_signature']}"
              + (f" | backup `{b['repo']}` `{b['symbol']}`" if b else " | backup MISSING"))
    counts = {}
    for s in slate.values():
        counts[s["slot"]] = counts.get(s["slot"], 0) + 1
    ok = any(counts == l for l in LAYOUTS)
    print(f"- composition: {counts} - {'§7.0 OK' if ok else 'NOT a §7.0 layout'}")
    missing = [r for r in slate if r not in backups]
    if missing:
        print(f"- backups missing for: {', '.join(missing)} "
              "(pool exhausted for that size class - escalate WITH the numbers)")
    if not args.write:
        print("- dry run; pass --write to update repos.txt / PINNED_COMMITS.json / slate.json")
        return 0 if ok else 1

    with open(os.path.join(root, "repos.txt"), "w") as f:
        f.write(f"# {args.vertical} vertical - the repo list. One repo key per line.\n")
        for repo in slate:
            f.write(repo + "\n")

    pins_path = os.path.join(root, "PINNED_COMMITS.json")
    pins = json.load(open(pins_path)) if os.path.exists(pins_path) else {}
    for repo in slate:
        clone = os.path.join(args.clones, repo)
        sha = subprocess.run(["git", "-C", clone, "rev-parse", "HEAD"],
                             capture_output=True, text=True).stdout.strip()
        url = subprocess.run(["git", "-C", clone, "remote", "get-url", "origin"],
                             capture_output=True, text=True).stdout.strip()
        pins[repo] = {"url": url, "sha": sha}
    json.dump(pins, open(pins_path, "w"), indent=2, sort_keys=True)

    out = {"_meta": {"vertical": args.vertical,
                     "note": "Written by compose_slate.py; verified by slate_check.py."}}
    for repo, s in slate.items():
        b = backups.get(repo)
        out[repo] = {"slot": s["slot"], "anchor": s["symbol"], "anchor_file": s["file"],
                     "affected": s["affected"], "precision": s["precision"],
                     "win_signature": s["win_signature"],
                     "backup": ({"repo": b["repo"], "anchor": b["symbol"],
                                 "anchor_file": b["file"], "affected": b["affected"],
                                 "precision": b["precision"]} if b else None)}
    json.dump(out, open(os.path.join(root, "slate.json"), "w"), indent=2)
    print("- WROTE repos.txt, PINNED_COMMITS.json, slate.json - now run slate_check.py")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
