#!/usr/bin/env python3
"""The arithmetic bound: kill a cell the bar makes impossible, before it is paid for.

    control_bound.py <scenario.yaml> <probe_answer.md> [<probe_answer2.md> ...]

Exit 0 = the cell can still clear the bar (spend allowed).
Exit 1 = KILL: no group can reach the bar. The cell is arithmetically dead. Do not spend.

## THE BOUND

    delta = mean(sense) - mean(control),  and  cited_recall <= 1.0
      =>   delta >= BAR   REQUIRES   mean(control) <= 1.0 - BAR

With the campaign bar at +0.50 (pergroup.py's default), a group whose CONTROL scores above
0.50 cannot produce a +0.50 delta even if sense scores a perfect 1.00. This is arithmetic,
not a heuristic, not a threshold fitted to history: it holds for every repo, language,
framework and question shape, because no term in it mentions any of those.

## WHY THIS EXISTS (measured over all 384 frozen scored runs)

Ten of twenty paid cells on the bench model were arithmetically dead BEFORE they ran:
healthchecks 1.00, litellm 1.00 (ceiling EXACTLY 0.000), sentry 0.90, wagtail 0.81,
redmine 0.77, lobsters 0.77, ruby_llm 0.76, netbox 0.67, dolt 0.67, raix 0.60. Each was
killable by one comparison. We paid for all of them. The whole dolt campaign (five cells,
two Loop 7 attempts) ran against a control at 0.67 whose ceiling was +0.33.

## WHY 0.50 AND NOT TIGHTER (the false-negative check; do NOT "improve" this)

Measured against the 5 real wins in the corpus:

    control > 0.50 -> kill   10 cells killed,  0 WINS killed   <- this rule
    control > 0.40 -> kill   13 cells killed,  1 WIN  killed   (saleor)
    control > 0.25 -> kill   14 cells killed,  1 WIN  killed   (saleor)
    control > 0.20 -> kill   17 cells killed,  2 WINS killed   (saleor, discourse)

saleor proves the bound is TIGHT, not merely safe: control 0.50, sense 1.00, delta EXACTLY
+0.500 = the bar. A control of 0.50 is winnable; anything above it is not. Every threshold
below 0.50 is an empirical guess that costs real wins. This one costs none, provably.

## GOLD FIDELITY: "ROUGH GOLD IS SAFE" IS FALSE. DO NOT SCREEN WITH UNAUDITED GOLD.

Measured and REFUTED on gitea 2026-07-16, and it is the trap this gate is most exposed to.

The reasoning was: over-inclusive gold LOWERS the control's recall, biasing toward PASS, and a
false pass costs ~$4 while a false kill costs a win - so rough gold errs cheap. The arithmetic is
right and the conclusion is wrong. **Bias-toward-PASS is not "safe", it is TOOTHLESS**: a gold that
does not answer the ask always passes, so the screen cannot kill anything and tells you nothing.
The real cost of a false pass is not $4, it is authoring a whole cell on a false premise.

The gitea run: gold built from `sense blast Repository --min-confidence 0.3` (42 files), ask =
"the deletion-cascade rework: what must you touch". Control scored 0.238/0.167 -> mean 0.202 ->
PASS, ceiling +0.798. It looked like a floor. It was a GOLD MISMATCH:

  - 32 of 42 gold rows were MISSED, and the misses are routers that render commits, branches,
    feeds and org homes: `routers/web/repo/commit.go` (0 mentions of "delete"),
    `routers/web/feed/repo.go` (0), `routers/common/compare.go` (0). They sit in Repository's
    BLAST RADIUS (affected if the TYPE changes) and are irrelevant to a TEARDOWN rework. The gold
    answered a different question than the ask.
  - Worse, it punished CORRECT answers: the gold demands `models/perm/access/repo_permission.go`
    for the Access row, while both probes cited `services/repository/delete.go:148`
    (`&access_model.Access{RepoID: repo.ID}`) - the same fact, at the file where the cascade
    actually happens.

So: **blast-radius gold != edit-impact gold**, and the hand-audit is NOT skippable. The bound's
arithmetic is only as meaningful as the gold it consumes. Before trusting ANY verdict from this
gate, run the per-unit check: list the rows the control missed and ask whether the ask ever pointed
at them. A surprising PASS is a gold bug until proven otherwise.

## PER-GROUP, AND WHY

pergroup.py (the WIN arbiter) flags a win on ANY gold group, not just `dependents` -
saleor's banked win is on its `context` group. So the bound is evaluated per group and the
cell is killed ONLY when EVERY group is dead. Applying it to one group would re-create the
exact false negative the 0.50 threshold was chosen to avoid.

## SAMPLING (the RUNS=2 law applies here too)

The control's score is a random variable, and it is not a small one: dolt's control swung
8/18 -> 16/18 (0.444 -> 0.889) across two UNCONSTRAINED runs of the same arm and prompt.
The verdict uses the MEAN (because pergroup's delta does), so the mean must be estimated
from >= 2 probe runs. A single probe carries an OPEN flag: the standing RUNS=2 law, applied
to the gate that decides the spend.

The probe MUST be run at the cell's REAL wall. This is not pedantry: dolt's control scores
~0.00 at a 300s wall and 0.67 at 720s - same repo, same gold, same question. The wall IS the
control's score, so a probe at the wrong wall measures a different cell.

Scoring uses gold.score_gold_recall - the SAME instrument the bench scores with. A gate that
scored differently from the arbiter would be a second scorer, and two scorers is a bug
factory (see stopper/gold-basename-false-credit).

## SLATE MODE (--slate <vertical-dir>): the ranked paid queue

Loop 3 runs depth-first: one repo reaches a verdict before the next opens, so SOMETHING has
to decide which repo goes first. That decision is this gate's numbers, sorted - the repo whose
control is weakest has the most headroom, so it is the highest-probability win and it goes
first. Slate mode lives here, in the gate, rather than in a script of its own: the ranking is
a sort over numbers this file already computes, and a separate ranker would be a second
opinion about which cells are alive (the same "two scorers is a bug factory" rule).

It also enforces the expiry the ranking depends on. A bound verdict is true OF a Sense build:
Loop 7 ships fixes between verticals, and a fix can turn a correctly-killed cell live. So a
probe carries the build it ran against (sense_build.py), and slate mode refuses to rank on a
probe whose build is not the one in hand - STALE (superseded build) and UNSTAMPED (no build
recorded at all) are reported separately, because they are different failures. Silently
reusing an expired probe is the one way this gate can lie without anyone noticing.
"""

import os
import statistics
import sys

import yaml

from gold import score_gold_recall
from sense_build import binary_key, default_bin, freshness

BAR = 0.50            # pergroup.py's default campaign bar
MAX_RECALL = 1.0      # cited_recall is a ratio
BOUND = MAX_RECALL - BAR   # = 0.50; the highest control that leaves the bar reachable


def score_probe(answer_text, gold):
    """Per-group cited_recall for one control probe answer."""
    gr = score_gold_recall(answer_text, gold)
    return {g: d.get("cited_recall") for g, d in (gr.get("groups") or {}).items()}


def evaluate(scenario_path, probe_paths):
    with open(scenario_path) as fh:
        gold = (yaml.safe_load(fh) or {}).get("gold") or []
    if not gold:
        raise SystemExit(f"control_bound: no gold in {scenario_path}")

    per_run = []
    for p in probe_paths:
        with open(p, errors="ignore") as fh:
            per_run.append(score_probe(fh.read(), gold))

    groups = sorted({g for r in per_run for g in r})
    means = {}
    for g in groups:
        vals = [r[g] for r in per_run if r.get(g) is not None]
        if vals:
            means[g] = statistics.mean(vals)
    return per_run, means


def scenario_for(vertical_dir, repo):
    """<vertical>/scenarios/<repo>.yaml, or None. A probe dir with no scenario is not rankable."""
    p = os.path.join(vertical_dir, "scenarios", f"{repo}.yaml")
    return p if os.path.exists(p) else None


def probe_dirs(vertical_dir):
    """(repo, dir) for every repo with a probe folder, in name order."""
    root = os.path.join(vertical_dir, "results", "dryrun")
    if not os.path.isdir(root):
        return []
    return [(r, os.path.join(root, r)) for r in sorted(os.listdir(root))
            if os.path.isdir(os.path.join(root, r))]


def repo_probes(probe_dir):
    return sorted(os.path.join(probe_dir, f) for f in os.listdir(probe_dir)
                  if f.endswith(".md"))


def assess_repo(vertical_dir, repo, probe_dir, current_key):
    """One row of the slate: the bound verdict plus the freshness of the probes behind it."""
    row = {"repo": repo, "probes": repo_probes(probe_dir), "means": {}, "note": None}
    if not row["probes"]:
        row["note"] = "no probe .md files"
        return row

    row["freshness"] = [freshness(p, current_key) for p in row["probes"]]
    scenario = scenario_for(vertical_dir, repo)
    if not scenario:
        row["note"] = f"no scenarios/{repo}.yaml"
        return row

    try:
        _, row["means"] = evaluate(scenario, row["probes"])
    except SystemExit as exc:            # no gold in the scenario yet
        row["note"] = str(exc)
        return row

    if row["means"]:
        row["best_group"], row["best_control"] = min(row["means"].items(), key=lambda kv: kv[1])
        row["ceiling"] = MAX_RECALL - row["best_control"]
        row["alive"] = row["best_control"] <= BOUND
    return row


def rankable(row):
    """A row ranks only if it is alive AND every probe behind it ran on the build in hand."""
    return bool(row.get("alive")) and set(row.get("freshness") or []) == {"FRESH"}


def print_slate(rows, current_key):
    print(f"## slate rank - bar +{BAR:.2f}, build in hand {current_key}")
    print("   weakest control first: most headroom, so the highest-probability win goes first.")
    print()
    print(f"   {'repo':<20} {'probes':>6}  {'weakest group':<16} {'control':>7} "
          f"{'ceiling':>8}   state")
    for row in rows:
        if row.get("note"):
            print(f"   {row['repo']:<20} {'-':>6}  {'-':<16} {'-':>7} {'-':>8}   "
                  f"UNRANKABLE ({row['note']})")
            continue
        states = set(row["freshness"])
        if states == {"FRESH"}:
            state = "ranked" if row["alive"] else "DEAD (bound kill)"
        else:
            state = " + ".join(sorted(states - {"FRESH"}))
        print(f"   {row['repo']:<20} {len(row['probes']):>6}  {row['best_group']:<16} "
              f"{row['best_control']:>7.3f} {row['ceiling']:>+8.3f}   {state}")
        if len(row["probes"]) < 2 and states == {"FRESH"}:
            print(f"   {'':<20} {'':>6}  [OPEN: n=1, the RUNS=2 law is unmet]")


def print_queue(queue):
    print()
    if not queue:
        print("   NO RANKABLE CELL. Nothing here is both bound-legal and measured on the")
        print("   build in hand. Re-probe the STALE rows (a fix may have made a dead cell")
        print("   live), stamp the UNSTAMPED ones (sense_build.py --stamp), or route the")
        print("   DEAD rows to gold re-target / re-shape / swap.")
        return 1
    print("   PAID QUEUE (depth-first: finish one before opening the next)")
    for i, row in enumerate(queue, 1):
        print(f"     {i}. {row['repo']:<20} control {row['best_control']:.3f} "
              f"on '{row['best_group']}', ceiling {row['ceiling']:+.3f}")
    return 0


def slate(vertical_dir, bin_path=None):
    current_key = binary_key(bin_path or default_bin())
    dirs = probe_dirs(vertical_dir)
    if not dirs:
        raise SystemExit(
            f"control_bound --slate: no probe dirs under {vertical_dir}/results/dryrun")
    rows = [assess_repo(vertical_dir, repo, d, current_key) for repo, d in dirs]
    print_slate(rows, current_key)
    queue = sorted((r for r in rows if rankable(r)), key=lambda r: r["best_control"])
    return print_queue(queue)


def main(argv):
    if len(argv) >= 2 and argv[1] == "--slate":
        if len(argv) < 3:
            raise SystemExit("usage: control_bound.py --slate <vertical-dir> [--bin PATH]")
        bin_path = argv[4] if len(argv) > 4 and argv[3] == "--bin" else None
        return slate(argv[2], bin_path)

    if len(argv) < 3:
        raise SystemExit(__doc__.strip().splitlines()[2].strip())
    scenario, probes = argv[1], argv[2:]
    per_run, means = evaluate(scenario, probes)

    n = len(probes)
    print(f"## control bound - bar +{BAR:.2f} requires control <= {BOUND:.2f}")
    print(f"   probe runs: {n}" + ("   [OPEN: n=1, the RUNS=2 law is unmet; "
                                   "the mean is an estimate of one sample]" if n < 2 else ""))
    print()
    print(f"   {'group':<16} {'per-run control':<28} {'mean':>6}   {'ceiling':>8}   verdict")
    alive = []
    for g, m in sorted(means.items()):
        runs = ", ".join(f"{r[g]:.3f}" for r in per_run if r.get(g) is not None)
        ceiling = MAX_RECALL - m
        ok = m <= BOUND
        if ok:
            alive.append(g)
        print(f"   {g:<16} {runs:<28} {m:>6.3f}   {ceiling:>+8.3f}   "
              f"{'reachable' if ok else 'DEAD (ceiling < bar)'}")
    print()

    if alive:
        print(f"   PASS - {len(alive)}/{len(means)} group(s) can still reach +{BAR:.2f}: "
              f"{', '.join(alive)}")
        if n < 2:
            print("   NOTE: n=1. The control's spread is real (dolt: 0.444 -> 0.889 on "
                  "identical prompts). Run a second probe before trusting a near-bound number.")
        return 0

    worst = min(means.values())
    print(f"   KILL - every group's control exceeds {BOUND:.2f}; the best reachable delta is "
          f"{MAX_RECALL - worst:+.3f}, below the +{BAR:.2f} bar.")
    print("   This cell cannot clear the bar even if sense scores a perfect 1.00. Do not spend.")
    print("   Levers: re-target the gold, re-shape the ask, or swap the repo (the swap gate).")
    print("   NOT a lever: the wall. Tightening it lowers the control AND truncates sense "
          "(the dolt campaign measured this: no wall value exists where the audit completes "
          "but both control routes are starved).")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
