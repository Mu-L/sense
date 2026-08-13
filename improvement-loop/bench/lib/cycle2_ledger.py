#!/usr/bin/env python3
"""Write the `cycle2/<repo>/board` LEDGER entry from the numbers the board stands on.

Driven by cycle2-board.sh at publish, from numbers.json only, so the entry records
what the run actually measured rather than what someone remembered afterwards. Same
shape as scaffold_ledger.py, which writes the bootstrap entry the same way.

WHY THE PAGE IS NOT ENOUGH. The published board answers "what did we measure". The
ledger answers "why did we run it, what else was considered, and what did it teach",
and it is where a future session goes to ask whether a cell is still live. A tracked
markdown page cannot carry the second half without becoming a different document.

THE LESSON FIELD IS THE ONE A SCRIPT CANNOT INVENT. It says so plainly when the run
taught nothing, rather than manufacturing one: a loop that always claims a lesson is
manufacturing them. Where the numbers DO carry a lesson (an arm that never routed, a
column that did not replicate), it is stated as the fact it is.

Usage:
    cycle2_ledger.py <numbers.json> [--date YYYY-MM-DD]
"""
import argparse
import json
import sys
import textwrap


def _pct(column):
    o = column.get("overall") or {}
    return o.get("baseline_mean"), o.get("sense_mean"), o.get("delta")


def _scores_line(data):
    bits = []
    for col in data["columns"]:
        if not col.get("measured"):
            bits.append(f"{col['model']} not run")
            continue
        base, sense, delta = _pct(col)
        reach = len(col.get("sense_only_reach") or [])
        bits.append(f"{col['model']} {base:.2f} -> {sense:.2f} ({delta:+.2f}, "
                    f"{reach} reached only with Sense)")
    return "; ".join(bits)


def _lesson(data):
    """Stated from the numbers, or stated as absent. Never manufactured."""
    rep = data["replication"]
    routed_now = rep.get("routed") or []
    if not routed_now:
        return ("none yet. No confirmation arm has reached a Sense resolver on this "
                "question, so the board records the headline column and nothing more.")
    never = rep.get("never_routed") or []
    search_only = rep.get("search_only") or []
    routed, repl = rep.get("routed") or [], rep.get("replicated") or []
    if never or search_only:
        who = ", ".join(never + search_only)
        return (f"**a model that does not call Sense is a routing failure of ours, not a "
                f"result about the product.** {who} had the server and did not use it, so "
                f"the column is reported and left out of the replication count.")
    if not repl:
        return ("**the win did not travel.** Every arm reached a resolver and none cleared "
                "the floor its own baseline set, which is a finding about the question or "
                "the payload, not about the models.")
    if routed and len(repl) == len(routed):
        return ("none. The question held on every arm that ran it, which is the expected "
                "outcome and teaches nothing on its own.")
    return (f"**the win travels unevenly.** {len(repl)} of {len(routed)} arms cleared the "
            f"floor on the same question, so a single-model result is not yet a claim "
            f"about Sense in general.")


def entry(data, date):
    rep = data["replication"]
    repo, version = data["repo"], data.get("scenario_version", "")
    routed, repl = rep.get("routed") or [], rep.get("replicated") or []
    measured = [c for c in data["columns"] if c.get("measured")]
    confirmations = len(data["columns"]) - 1
    benched = sum(1 for c in data["columns"][1:] if c.get("measured"))
    arms_word = "arm" if confirmations == 1 else "arms"
    roots = sorted({f"results/{c['model'].replace('/', '_').replace(':', '_')}/"
                    f"{version.split(':')[-1]}/" for c in measured})

    lines = [
        f"## {date} | cycle2/{repo}/board | the question put to every arm",
        "",
        f"- **Provenance:** {data.get('sense_version', 'unknown build')}, scenario "
        f"`{version}`, headline `{data.get('headline', '')}` carried over from its banked "
        f"cell and NOT re-run. Confirmation arms benched for this board.",
        f"- **What:** {repo}'s proved question, put to {confirmations} "
        f"confirmation {arms_word} at 2 runs each, and published as "
        f"`reports/{repo}-{version.split(':')[-1]}.md`.",
        f"- **Why:** a win on one model is a win on one model. Cycle 2 exists to say "
        f"whether the same question, gold and repo hold when only the model changes.",
        f"- **Alternatives:** none at this stage. The question is settled before cycle 2 "
        f"starts and may not be re-authored; the only choice is which cell goes next.",
        f"- **Lesson:** {_lesson(data)}",
        f"  `Exit: check(board_check.py rc=0 against numbers.json; "
        f"{len(repl)} of {len(routed)} routed arms over the {rep.get('threshold')} floor)`",
        f"- **Scores:** {_scores_line(data)}.",
        f"- **Cost:** subscription across {benched} benched confirmation "
        f"{'arm' if benched == 1 else 'arms'}, 2 runs per arm per tool; the headline "
        f"column cost nothing, it was read. Fleet: 2 spawns (validity, read).",
        f"- **Links:** `verticals/{data.get('vertical', '')}/reports/{repo}-"
        f"{version.split(':')[-1]}.md`, " + ", ".join(f"`{r}`" for r in roots) + ".",
        "",
    ]
    out = []
    for line in lines:
        if line.startswith("- **") and len(line) > 95:
            # No hyphen or long-word breaks: a wrapped `results/claude-opus-5/...`
            # splits the code span and the path stops being copy-pasteable.
            out.extend(textwrap.wrap(line, width=95, subsequent_indent="  ",
                                     break_on_hyphens=False, break_long_words=False))
        else:
            out.append(line)
    return "\n".join(out)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("numbers")
    ap.add_argument("--date", required=True, help="UTC date, passed in by the driver")
    args = ap.parse_args(argv[1:])
    sys.stdout.write(entry(json.load(open(args.numbers)), args.date))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
