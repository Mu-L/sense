#!/usr/bin/env python3
"""park_report.py - price the park decision. Reporting only: spends nothing, routes nothing.

THE DEFECT THIS CLOSES. At AUTH_CYCLE_MAX the loop says "6 attempts without a payable
question - re-run to spend another 6, or swap". Both branches are unpriced, so the human
decides blind, and the one repo that ever produced a payable question in php-laravel
(coolify) only got there because a human re-authorised it three times by hand. That
intervention IS the loop's success condition and it was being asked for with no numbers
attached.

WHAT IT PRINTS, and nothing else:
  - this repo's attempts, its best delta, and whether anything cleared the window
  - the hit rate observed across this VERTICAL (all repos), with its n
  - the power those attempts bought, and how many more reach ~80%
  - what those attempts cost, from the measured pair walls on disk
  - a refinement check: are consecutive attempts edits of each other, and is the baseline
    trending down at all

WHAT IT MUST NOT DO. It never parks, never advances, never changes a cap and never writes.
Measured 2026-08-12: a "flat slope -> park" rule would have read FLAT at attempt 6 on coolify
and killed it three attempts before its only PASS. The cap is not the thing to tighten; the
decision at the cap is the thing to price.

  python3 park_report.py <vertical_dir> <repo> [window_baseline_max] [window_delta_min]
"""

import glob
import json
import math
import os
import re
import statistics
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import run_validity  # noqa: E402

WIN_BASE = 0.50
WIN_DELTA = 0.50


def attempts_for(vdir, repo):
    """Every measured attempt for one repo, in loop order. Rounds archive as cycles.<n>."""
    out = []
    pats = [os.path.join(vdir, "results", "dryrun", repo, "cycles.*.jsonl"),
            os.path.join(vdir, "results", "loop", repo, "cycles.jsonl")]
    for pat in pats:
        for f in sorted(glob.glob(pat)):
            m = re.search(r"cycles\.(\d+)\.jsonl$", f)
            rnd = int(m.group(1)) if m else 99
            for line in open(f, encoding="utf-8", errors="ignore"):
                line = line.strip()
                if not line:
                    continue
                try:
                    d = json.loads(line)
                except json.JSONDecodeError:
                    continue
                b, s = d.get("baseline_dependents"), d.get("sense_dependents")
                if b is None or s is None:
                    continue
                out.append({"round": rnd, "cycle": d.get("cycle") or 0, "base": b, "sense": s,
                            "delta": s - b, "ask": d.get("ask") or "", "repo": repo})
    out.sort(key=lambda r: (r["round"], r["cycle"]))
    return out


def measured_cells(vdir, repo):
    """Attempts as MEASURED, from the results tree: valid runs only, chronological.

    cycles.jsonl records what each attempt scored, but carries no validity field, so a
    watchdogged or void arm reads there as a real 0.0. Counting those inflates the hit rate
    (a baseline voided at 0.0 against a sense arm at 1.0 looks like a +1.00 hit) and it is
    the same error that once turned a dead half-pair into a phantom +0.528 cell. Hits, the
    best delta and the baseline series therefore come from HERE, never from cycles.jsonl.
    """
    cells = []
    root = os.path.join(vdir, "results")
    for meta in glob.glob(os.path.join(root, "*", "minibench", "*", "baseline", repo,
                                       "run-*", "run_meta.json")):
        cell_dir = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(meta))))
        cells.append(cell_dir)
    out = []
    for cell in sorted(set(cells)):
        arms, ts = {}, None
        for arm in ("baseline", "sense"):
            vals = []
            for run in sorted(glob.glob(os.path.join(cell, arm, repo, "run-*"))):
                try:
                    m = json.load(open(os.path.join(run, "run_meta.json"), encoding="utf-8"))
                except (OSError, json.JSONDecodeError):
                    continue
                # run_validity owns validity. The stored `valid` field is stale for every
                # run written before a classifier change - 17 such baseline runs on disk
                # reclassify as VALID and a reader keying on the raw field discards them.
                sc = None
                sp = os.path.join(run, "scored.json")
                if os.path.exists(sp):
                    try:
                        sc = json.load(open(sp, encoding="utf-8"))
                    except (OSError, json.JSONDecodeError):
                        sc = None
                if not run_validity.classify_run(m, sc, run_dir=run).get("valid"):
                    continue
                ts = min(x for x in (ts, m.get("timestamp")) if x) if ts else m.get("timestamp")
                try:
                    s = json.load(open(os.path.join(run, "scored.json"), encoding="utf-8"))
                except (OSError, json.JSONDecodeError):
                    continue
                v = ((s.get("gold_recall") or {}).get("groups") or {}).get("dependents", {}) \
                    .get("cited_recall")
                if v is not None:
                    vals.append(v)
            arms[arm] = vals
        if not arms["baseline"] or not arms["sense"] or not ts:
            continue
        b = sum(arms["baseline"]) / len(arms["baseline"])
        s = sum(arms["sense"]) / len(arms["sense"])
        out.append({"ts": ts, "cell": os.path.basename(cell), "base": b, "sense": s,
                    "delta": s - b, "nb": len(arms["baseline"]), "ns": len(arms["sense"])})
    out.sort(key=lambda r: r["ts"])
    return out


def repos_in(vdir):
    seen = set()
    for p in glob.glob(os.path.join(vdir, "results", "dryrun", "*")):
        if os.path.isdir(p):
            seen.add(os.path.basename(p))
    for p in glob.glob(os.path.join(vdir, "results", "loop", "*")):
        if os.path.isdir(p):
            seen.add(os.path.basename(p))
    return sorted(seen)


def is_hit(a):
    return a["base"] <= WIN_BASE and a["delta"] >= WIN_DELTA


def pair_minutes(vdir):
    """Median cost of one two-arm mini-bench pair, from the walls actually on disk."""
    walls = {}
    for f in glob.glob(os.path.join(vdir, "results", "*", "minibench", "*", "*", "*", "run-*",
                                    "run_meta.json")):
        try:
            d = json.load(open(f, encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        w = d.get("wall_time_seconds")
        if not w:
            continue
        walls.setdefault(d.get("tool"), []).append(w)
    if "sense" not in walls or "baseline" not in walls:
        return None
    return (statistics.median(walls["sense"]) + statistics.median(walls["baseline"])) / 60.0


def slope(xs):
    n = len(xs)
    if n < 3:
        return None
    mx = (n - 1) / 2
    my = sum(xs) / n
    den = sum((i - mx) ** 2 for i in range(n))
    return sum((i - mx) * (x - my) for i, x in enumerate(xs)) / den if den else None


def jaccard_report(rows):
    """Are consecutive attempts edits of each other, or independent draws?"""
    def toks(s):
        return set(w for w in re.findall(r"[a-z']+", s.lower()) if len(w) > 3)
    T = [toks(r["ask"]) for r in rows if r["ask"]]
    if len(T) < 4:
        return None
    J = lambda a, b: len(a & b) / len(a | b) if (a | b) else 0.0
    con = [J(T[i - 1], T[i]) for i in range(1, len(T))]
    dis = [J(T[i], T[j]) for i in range(len(T)) for j in range(i + 4, len(T))]
    if not dis:
        return None
    return statistics.mean(con), statistics.mean(dis)


def main(argv):
    if len(argv) < 3:
        print(__doc__.strip().splitlines()[-1], file=sys.stderr)
        return 1
    vdir, repo = argv[1], argv[2]
    mine = measured_cells(vdir, repo)
    allr = [a for r in repos_in(vdir) for a in measured_cells(vdir, r)]
    asks = attempts_for(vdir, repo)
    if not mine:
        print("park_report: no VALID measured pair for this repo - nothing to price.")
        return 0

    n_v, hits_v = len(allr), sum(1 for a in allr if is_hit(a))
    best = max(mine, key=lambda a: a["delta"])
    cleared = [a for a in mine if is_hit(a)]
    mins = pair_minutes(vdir)

    print(f"PARKED - {repo}, {len(mine)} attempts with a VALID pair, "
          f"best delta {best['delta']:+.3f} (cell {best['cell'][:8]}, "
          f"n={best['nb']}b/{best['ns']}s), {len(cleared)} cleared the window.")
    if len(asks) > len(mine):
        print(f"  ({len(asks)} attempts were run; {len(asks) - len(mine)} had a void or "
              f"watchdogged arm and are NOT counted below)")
    print()
    print(f"  hit rate, this vertical : {hits_v} payable question(s) in {n_v} attempts", end="")
    if n_v:
        print(f"  ({hits_v / n_v:.0%})")
    else:
        print()

    if hits_v == 0:
        ub = 3.0 / n_v if n_v else 1.0
        print(f"  power                   : no hit observed anywhere in this vertical, so the")
        print(f"                            rate is unknown. Rule of three: the 95% upper bound")
        print(f"                            is {ub:.1%} per attempt, and everything below is")
        print(f"                            computed AT that upper bound - it is the optimistic")
        print(f"                            end, not an estimate.")
        p = ub
    else:
        p = hits_v / n_v

    if 0 < p < 1:
        have = 1 - (1 - p) ** len(mine)
        print(f"  attempts so far bought  : {have:.0%} chance of having drawn a payable question")
        print(f"                            parking now accepts a {1 - have:.0%} chance one exists undrawn")
        need = math.ceil(math.log(1 - 0.80) / math.log(1 - p)) - len(mine)
        if need > 0:
            cost = f", ~{need * mins / 60:.1f} h" if mins else ""
            print(f"  to reach ~80%           : ~{need} more attempts ({need * 2} sessions{cost})")
        else:
            print(f"  to reach ~80%           : already there")
    if mins:
        print(f"  measured cost per pair  : ~{mins:.0f} min (both arms, median wall on disk)")

    print()
    bases = [a["base"] for a in mine]
    sl = slope(bases)
    print(f"  baseline by attempt     : {[round(b, 2) for b in bases]}")
    if sl is not None:
        verdict = "trending DOWN - refining is working" if sl < -0.005 else \
                  "FLAT - refining is not moving the baseline"
        print(f"  slope per attempt       : {sl:+.4f}   {verdict}")
    jr = jaccard_report(asks) if len(asks) >= 10 else None
    if jr:
        con, dis = jr
        kind = "EDITS of each other" if con > dis * 1.3 else "near-independent draws"
        print(f"  question overlap        : consecutive {con:.2f} vs distant {dis:.2f} - {kind}")
    print()
    print("  This is a REPORT. Nothing here parked this repo and nothing here may advance it.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
