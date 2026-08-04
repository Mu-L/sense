#!/usr/bin/env python3
"""inverse_frequency.py - rank gold rows by INVERSE CITATION FREQUENCY.

The rarely-cited rows sort to the top; those are the ones worth building gold from.

    inverse_frequency.py <repo> <results-root> [<results-root> ...] [--min-runs 2]

WHY THIS EXISTS. Each authoring cycle reads ONE credit table and rewrites the question
against it, so difficulty is never accumulated: a row that eleven runs have never cited
looks exactly like a row that one run happened to miss. This counts citations per row
across every run on disk and ranks by the rate, so the next scenario can be built from
the bottom of the list instead of from the last failure.

WHY COUNTING AND NOT JUDGMENT. Reading the repo and deciding what "should" be hard runs
the same search the agents ran, with the same tools, and finds the same things - the
result would be gold selected for what one more grep can reach. Citation RATE needs no
ground truth: it is the arms' own behaviour, aggregated. What it cannot see is a
dependency no agent has ever named; that residue stays invisible here and is not claimed.

READING THE OUTPUT.
  * `0/N` on BOTH arms - hard or unreachable, and those are different things. Check the
    row against the blast payload: present but uncited is a hard row worth building on;
    absent from the payload is gold the tool cannot serve and should be retired.
  * `0/N` baseline, `N/N` sense - the discriminator you already have. More of these is
    the goal.
  * `N/N` on both - free. It costs nothing to answer and dilutes the group.
"""
import argparse
import collections
import glob
import json
import os
import sys


def runs_for(repo, roots):
    """Every scored run for this repo, as (arm, path, scenario_version, model).

    The version rides along because a repo name is NOT a scenario: one results root can
    hold runs from several questions authored against the same repo, and their gold sets
    have different rows. Blending them would count a row as "never cited" simply because
    the other scenario never had it.
    """
    out = []
    for root in roots:
        for meta in glob.glob(os.path.join(root, "**", "run_meta.json"), recursive=True):
            d = os.path.dirname(meta)
            if os.path.basename(os.path.dirname(os.path.dirname(d))) != repo and repo not in d.split(os.sep):
                continue
            scored = os.path.join(d, "scored.json")
            if not os.path.isfile(scored):
                continue
            try:
                m = json.load(open(meta))
            except (OSError, ValueError):
                continue
            out.append((m.get("tool") or "?", scored,
                        m.get("scenario_version") or "(unversioned)",
                        m.get("model") or "(unknown model)"))
    return out


def tally(runs, keys=None):
    """{row_key: {"group":g, arm: [cited, total]}} across every run.

    `keys` maps scenario_version -> {row id -> durable key}. When given, rows are counted
    under their durable key (the gold's file path) instead of their id, which is what lets
    difficulty survive a re-gold: the same file keeps its history when the question that
    names it is rewritten. Without it, behaviour is unchanged - keyed by id, one version.
    """
    rows = {}
    for arm, scored, sv, _model in runs:
        try:
            gr = json.load(open(scored))["gold_recall"]
        except (OSError, ValueError, KeyError):
            continue
        for item in gr.get("details", []):
            key = (keys or {}).get(sv, {}).get(item["id"], item["id"])
            rec = rows.setdefault(key, {"group": item.get("group", "")})
            cited, total = rec.get(arm, [0, 0])
            rec[arm] = [cited + (1 if item.get("cited") else 0), total + 1]
    return rows


def durable_keys(runs, store):
    """{scenario_version: {row id: gold file path}} for every version we can resolve.

    Returns the unresolved versions too. A version whose scenario is not archived is
    REPORTED and dropped, never silently folded in under its ids: two questions whose rows
    happen to share an id are not the same row, and pretending otherwise is the exact bug
    the scenario_version scoping was added to prevent.
    """
    import scenario_archive
    keys, missing = {}, []
    for sv in sorted({sv for _a, _s, sv, _m in runs}):
        path = scenario_archive.get(sv, store)
        if not path:
            missing.append(sv)
            continue
        keys[sv] = scenario_archive.gold_paths(path)
    return keys, missing


def rate(pair):
    cited, total = pair
    return cited / total if total else None


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("repo")
    ap.add_argument("roots", nargs="+")
    ap.add_argument("--min-runs", type=int, default=1,
                    help="skip rows seen in fewer runs than this (default 1)")
    ap.add_argument("--scenario-version", help="rank this version (default: the one with most runs)")
    ap.add_argument("--list-versions", action="store_true", help="list versions found, then stop")
    ap.add_argument("--by-path", metavar="STORE",
                    help="accumulate difficulty ACROSS re-questions: key each row on its "
                         "gold file path, resolved from a scenario_archive store, and merge "
                         "every version instead of ranking one")
    ap.add_argument("--model", help="scope --by-path to one model (default: the most-run one)")
    args = ap.parse_args()

    runs = runs_for(args.repo, args.roots)
    if not runs:
        print(f"no scored runs for {args.repo} under {', '.join(args.roots)}")
        return 1

    # ONE SCENARIO VERSION, ALWAYS - unless --by-path resolves each version's gold, which
    # makes rows comparable ACROSS questions by keying them on the file they point at.
    # Different questions score different gold, so a table blended on ids alone reports
    # rows as never-cited that the other scenario never had; blended on paths it reports
    # what the file has cost every arm that was ever asked about it.
    by_version = collections.Counter(sv for _a, _s, sv, _m in runs)
    if args.by_path and not args.list_versions:
        return rank_by_path(args, runs, by_version)
    if args.list_versions:
        print(f"# scenario versions for {args.repo}")
        for sv, n in by_version.most_common():
            print(f"  {sv}  {n} run(s)")
        return 0
    if args.scenario_version:
        target = args.scenario_version
        if target not in by_version:
            print(f"no runs at scenario_version {target}; have: "
                  + ", ".join(f"{sv} ({n})" for sv, n in by_version.most_common()))
            return 1
    else:
        # Most-represented wins, ties broken by name so the pick is deterministic. It is
        # announced rather than silent: picking without saying so is the same bug.
        target = sorted(by_version.items(), key=lambda kv: (-kv[1], kv[0]))[0][0]
    skipped = [(sv, n) for sv, n in by_version.most_common() if sv != target]
    runs = [r for r in runs if r[2] == target]

    by_arm = collections.Counter(a for a, _s, _sv, _m in runs)
    print(f"# inverse-frequency ranking (rarest cited first) - {args.repo}  ({len(runs)} runs: "
          + ", ".join(f"{a} x{n}" for a, n in sorted(by_arm.items())) + ")")
    print(f"# scenario {target}")
    if skipped:
        print("# NOT included (a different question on this repo, different gold): "
              + ", ".join(f"{sv} ({n} run(s))" for sv, n in skipped))
    print()

    print_ranking(tally(runs), sorted(by_arm), args.min_runs, width=24)
    return 0


def rank(rows, arms, min_runs):
    """(overall rate, key, record, cited, seen) per row, rarest first."""
    ranked = []
    for rid, rec in rows.items():
        seen = sum(rec.get(a, [0, 0])[1] for a in arms)
        if seen < min_runs:
            continue
        cited = sum(rec.get(a, [0, 0])[0] for a in arms)
        ranked.append((cited / seen if seen else 1.0, rid, rec, cited, seen))
    ranked.sort(key=lambda r: (r[0], r[1]))
    return ranked


def print_ranking(rows, arms, min_runs, width=24):
    ranked = rank(rows, arms, min_runs)
    head = f"{'row':{width}s} {'group':11s} {'overall':>9s}  " + "  ".join(f"{a:>12s}" for a in arms)
    print(head)
    print("-" * len(head))
    for _overall, rid, rec, cited, seen in ranked:
        cells = []
        for a in arms:
            c, t = rec.get(a, [0, 0])
            cells.append(f"{(str(c) + '/' + str(t)):>12s}" if t else f"{'-':>12s}")
        print(f"{rid:{width}s} {rec['group']:11s} {cited:>4}/{seen:<4} {' '.join(cells)}")

    dead = [r for r in ranked if r[3] == 0]
    free = [r for r in ranked if r[0] == 1.0]
    print(f"\n  never cited by any arm : {len(dead)}"
          + (f"  ({', '.join(r[1] for r in dead)})" if dead else ""))
    print(f"  cited by every run     : {len(free)}  <- free rows, they dilute the group")
    print("\n  A never-cited row is HARD or UNREACHABLE. Check it against the blast payload"
          "\n  before building on it: absent from the payload means the tool cannot serve it.")
    return ranked


def rank_by_path(args, runs, by_version):
    """Difficulty accumulated ACROSS questions, keyed on each row's gold file path."""
    keys, missing = durable_keys(runs, args.by_path)
    if not keys:
        print(f"no archived scenario for any version under {args.by_path}")
        print("  archive one with: scenario_archive.py add <scenario.yaml> [rubric] --store "
              f"{args.by_path}")
        print("  versions wanted: " + ", ".join(f"{sv} ({n} run(s))" for sv, n in by_version.most_common()))
        return 1
    kept = [r for r in runs if r[2] in keys]

    # ONE MODEL. Merging questions is the point; merging GENERATIONS is a different claim,
    # and laws.md already requires this ranking be scoped to the headline model because
    # mixing them reorders the middle of the table. Left unscoped, the first run of this
    # blended six baseline runs across two models without saying so.
    by_model = collections.Counter(m for _a, _s, _sv, m in kept)
    model = args.model
    if model and model not in by_model:
        print(f"no runs on model {model}; have: "
              + ", ".join(f"{m} ({n})" for m, n in by_model.most_common()))
        return 1
    if not model:
        top = max(by_model.values())
        tied = sorted(m for m, n in by_model.items() if n == top)
        if len(tied) > 1:
            # Alphabetical would have picked claude-opus-4-8 over claude-opus-5 - a default
            # that silently prefers the older generation. Make the caller say which.
            print("several models are equally represented - pass --model to pick one:\n  "
                  + "\n  ".join(f"{m} ({by_model[m]} run(s))" for m in tied))
            return 1
        model = tied[0]
    other = [(m, n) for m, n in by_model.most_common() if m != model]
    kept = [r for r in kept if r[3] == model]

    by_arm = collections.Counter(a for a, _s, _sv, _m in kept)
    print(f"# difficulty by GOLD PATH, merged across questions - {args.repo}  "
          f"({len(kept)} runs: " + ", ".join(f"{a} x{n}" for a, n in sorted(by_arm.items())) + ")")
    print(f"# model {model}")
    if other:
        print("# NOT included (a different model generation reorders the table): "
              + ", ".join(f"{m} ({n} run(s))" for m, n in other))
    print(f"# {len(keys)} scenario version(s) merged: " + ", ".join(sorted(keys)))
    if missing:
        # Never silently folded in under their ids - see durable_keys.
        print("# DROPPED, no archived scenario so their rows cannot be identified: "
              + ", ".join(f"{sv} ({by_version[sv]} run(s))" for sv in missing))
    print()
    print_ranking(tally(kept, keys), sorted(by_arm), args.min_runs, width=52)
    return 0


if __name__ == "__main__":
    sys.exit(main())
