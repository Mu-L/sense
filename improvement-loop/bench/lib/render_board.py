#!/usr/bin/env python3
"""Render a cycle 2 board: the same numbers JSON in, the same bytes out.

NO AGENT IN THE RENDER PATH. The why line behind every column is a templated
sentence chosen by the dominant cell of the mechanism table, not prose an agent
writes. Free prose about four vendors' models is exactly where an invented
figure would do the most damage, and a template cannot invent one. Style, tone
and section order live in the cycle 2 plan; changing the voice is editing the
plan, never re-running a bench.

THE SUBJECT OF EVERY LINE IS SENSE, NOT THE MODEL. Each column is one model's
sense arm against that model's OWN baseline. Absolute scores are never set side
by side, because the arms run different harnesses at different budgets and such
a table would read as a model leaderboard, which is not the claim and not a
fight worth having. What goes across models is one count: how many replicated.

A COLUMN THAT NEVER CALLED SENSE SAYS SO IN ITS FIRST LINE. Reporting a
never-routed arm as "Sense barely helped" states the opposite of what happened:
Sense was never asked. That is our routing failure and the page names it as one.

Usage:
    render_board.py <numbers.json> [-o <board.md>]
"""
import argparse
import json
import sys

REACH, IGNORED, FOUND_ANYWAY, MISSED = "reach", "ignored", "found-anyway", "missed"

# One sentence per dominant cell. Three of the four are findings about Sense.
WHY = {
    REACH: ("Sense returned {reach} of {gold} rows with a line and the answer cited "
            "them. This is the product working."),
    IGNORED: ("Sense returned {ignored} rows the answer never cited. The payload "
              "arrived and went unused, which is a format or routing problem on our "
              "side, not a coverage gap."),
    FOUND_ANYWAY: ("The answer cited {found_anyway} rows that no Sense call returned. "
                   "It found them by reading and grepping, so Sense was not what got "
                   "it there: a coverage gap that happened to cost no points."),
    MISSED: ("{missed} rows were neither returned by Sense nor cited. This is a "
             "coverage gap that cost the answer directly."),
}

ROUTING_NOTE = {
    "never-routed": ("This arm never called Sense. Its delta measures configuration, "
                     "not the product, and it is excluded from the replication count."),
    "search-only": ("This arm called sense_search but never reached a resolver "
                    "(sense_blast or sense_graph), so the dependency question was "
                    "never actually asked. Excluded from the replication count."),
    "harness-failure": ("The MCP server never came up for this arm. Not a "
                        "measurement; the run is discarded rather than scored."),
}


# A non-dominant cell this large is still the headline finding for that column.
# chatwoot is the precedent: dominant `reach` at 21 of 38 rows, while 12 rows were
# cited that Sense never returned. Reporting only the dominant cell printed "this
# is the product working" over a 32% coverage gap.
SECONDARY_SHARE = 0.15
# Worst news first, so a column that has something wrong with it says so early.
CELL_ORDER = (MISSED, IGNORED, FOUND_ANYWAY, REACH)


def _fmt(x, places=4):
    return f"{x:.{places}f}"


def _n(x):
    """Whole means print whole: 21.0 rows reads like a measurement error."""
    return f"{x:g}"


def _counts_across_runs(mech):
    """Summed cells over a column's measured runs, for the templated why line."""
    total = {REACH: 0, IGNORED: 0, FOUND_ANYWAY: 0, MISSED: 0}
    runs = [r for r in mech.get("runs", []) if r.get("routing") != "harness-failure"]
    for run in runs:
        for cell, n in (run.get("counts") or {}).items():
            total[cell] = total.get(cell, 0) + n
    n = max(len(runs), 1)
    return {k: round(v / n, 1) for k, v in total.items()}, len(runs)


def _why_line(column, gold_rows):
    """The one sentence, chosen mechanically by the dominant cell."""
    states = column.get("routing") or []
    for state in ("never-routed", "search-only", "harness-failure"):
        if states == [state]:
            return ROUTING_NOTE[state]
    mech = column.get("mechanism") or {}
    if mech.get("verdict_split"):
        return ("The two runs disagree on what happened: they land in different cells "
                "of the returned-by-Sense against cited table. A third run rules.")
    dominant = mech.get("dominant")
    if dominant not in WHY:
        return "No mechanism data for this column."
    avg, _ = _counts_across_runs(mech)
    fields = {"gold": gold_rows, "reach": _n(avg[REACH]), "ignored": _n(avg[IGNORED]),
              "found_anyway": _n(avg[FOUND_ANYWAY]), "missed": _n(avg[MISSED])}
    # The dominant cell, then any other cell big enough to be the real story.
    said = [WHY[dominant].format(**fields)]
    floor = max(gold_rows * SECONDARY_SHARE, 1)
    for cell in CELL_ORDER:
        if cell != dominant and avg[cell] >= floor:
            said.append(WHY[cell].format(**fields))
    return " ".join(said)


def _column_block(column, gold_rows):
    out = [f"### {column['model']}", ""]
    if not column.get("measured"):
        out += [f"Not measured: {column.get('reason', 'no runs on disk')}.", ""]
        return out

    o = column["overall"]
    runs = column["runs"]
    src = "reused from the headline bench" if column["source"] == "banked" else "benched here"
    out.append(f"- **pair** baseline {_fmt(o['baseline_mean'])} to sense "
               f"{_fmt(o['sense_mean'])}, delta **{o['delta']:+.4f}** "
               f"({runs['baseline']} baseline runs, {runs['sense']} sense runs, {src})")
    out.append(f"- **best group** {column['best_group_delta']:+.4f}")

    mech = column.get("mechanism") or {}
    avg, measured = _counts_across_runs(mech)
    if measured:
        out.append(f"- **rows** reach {_n(avg[REACH])}, ignored {_n(avg[IGNORED])}, "
                   f"found anyway {_n(avg[FOUND_ANYWAY])}, missed {_n(avg[MISSED])} "
                   f"(mean over {measured} run{'s' if measured != 1 else ''} "
                   f"of {gold_rows} gold rows)")
    out.append(f"- **routing** {', '.join(column.get('routing') or ['unknown'])}")
    tok = column.get("billed_tokens") or {}
    if tok.get("baseline") and tok.get("sense"):
        bmean = sum(tok["baseline"]) / len(tok["baseline"])
        smean = sum(tok["sense"]) / len(tok["sense"])
        out.append(f"- **billed tokens** baseline {bmean:,.0f}, sense {smean:,.0f} "
                   f"(raw means; the parity call belongs to cost_parity.py)")
    if column.get("recorded_at"):
        out.append(f"- **run** {column['recorded_at']}")
    out += ["", _why_line(column, gold_rows), ""]

    flipped = mech.get("rows_disagreeing") or []
    if flipped:
        out += [f"Rows that flipped between this arm's runs ({len(flipped)}): "
                + ", ".join(f"`{r}`" for r in flipped)
                + ". Sense returned them in one run and not the other, which is a "
                  "determinism finding in its own right.", ""]
    return out


def _replication_block(rep, n_arms):
    out = ["## Does the win replicate", ""]
    routed, repl = rep["routed"], rep["replicated"]
    if not routed:
        out.append(f"No confirmation arm reached a Sense resolver, so this board "
                   f"cannot say whether the win replicates. {n_arms} arms were "
                   f"declared.")
    else:
        out.append(f"Of {n_arms} confirmation arms, {len(routed)} actually reached a "
                   f"Sense resolver, and **{len(repl)} of {len(routed)}** cleared the "
                   f"{rep['threshold']} floor on their own baseline.")
    out.append("")
    if repl:
        out.append("Replicated: " + ", ".join(f"`{m}`" for m in repl))
    for key, label in (("never_routed", "Never called Sense"),
                       ("search_only", "Reached no resolver"),
                       ("not_measured", "Not measured")):
        if rep.get(key):
            out.append(f"{label}: " + ", ".join(f"`{m}`" for m in rep[key]))
    out += ["", "These are counts, not a ranking. Each arm is measured against its own "
            "baseline; the absolute scores are not comparable across models, which run "
            "different harnesses at different budgets.", ""]
    return out


def render(data):
    version = data.get("scenario_version", "")
    out = [f"# {data['repo']}: how Sense did, across {len(data['columns']) - 1} models", ""]
    out += ["| | |", "|---|---|",
            f"| vertical | {data.get('vertical', '')} |",
            f"| question | `{version}` |",
            f"| sense build | {data.get('sense_version', '')} |",
            f"| gold rows | {data.get('gold_rows', 0)} |",
            f"| headline arm | `{data.get('headline', '')}` |", ""]
    out += ["This page audits **Sense**, not the models. Every number is one model's "
            "Sense arm against that same model's baseline on the same question, so a "
            "column says what Sense did for that model and nothing about which model "
            "is better.", ""]

    # The reading sits ABOVE the detail: a human opens this page for the conclusion,
    # and the agent that writes it has already read everything below.
    out += ["## Reading", "", "<!-- reading -->", ""]
    out += ["## Per model", ""]
    for column in data["columns"]:
        out += _column_block(column, data.get("gold_rows", 0))

    out += _replication_block(data["replication"], len(data["columns"]) - 1)
    return "\n".join(out).rstrip() + "\n"


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("numbers", help="the JSON written by board.py assemble")
    ap.add_argument("-o", "--out", default=None)
    args = ap.parse_args(argv[1:])

    data = json.load(open(args.numbers))
    text = render(data)
    if args.out:
        with open(args.out, "w") as fh:
            fh.write(text)
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
