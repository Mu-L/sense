#!/usr/bin/env python3
"""Render a cycle 2 board: the same numbers JSON in, the same bytes out.

THIS IS A PUBLIC PAGE ON THE SENSE REPOSITORY, read online by someone who
arrived knowing the repo or the model but not the bench. It is written in our
voice and it says plainly where Sense helped. It is not an internal defect log,
and it is not a page that leads with our own gaps.

Honest all the same: every number is measured, the gaps are printed beside the
wins rather than left out, and the sentences are TEMPLATED, chosen mechanically
by the size of each cell of the mechanism table. No agent writes a figure here.
Prose about four vendors' models is exactly where an invented number would do
the most damage, and a template cannot invent one. The wording lives in
board_copy.json and in the templates below, so changing the voice is an edit
here, never a re-run of a bench.

WHAT THE PAGE DOES NOT CLAIM, and says so in as many words: it is not a ranking
of the models, and it is not a ranking of the repositories. Each column is one
model's Sense arm against that same model's own baseline on the same question.
Different models run on different harnesses at different budgets, so their
absolute scores are not comparable to each other, and the page never puts them
side by side as if they were.

SENTENCE ORDER IS CELL SIZE, LARGEST FIRST. That is honest in both directions:
when Sense carried the answer, the win leads; when it did not, the gap leads.
The precedent for saying more than one thing is chatwoot, where reporting only
the biggest cell printed "Sense supplied 21 of 38" over 12 rows the model had to
go find for itself.

Usage:
    render_board.py <numbers.json> [-o <board.md>] [--copy <board_copy.json>]
"""
import argparse
import json
import os
import sys

REACH, IGNORED, FOUND_ANYWAY, MISSED = "reach", "ignored", "found-anyway", "missed"

COPY_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "board_copy.json")

# A cell smaller than this share of the gold list is not part of the story.
CELL_SHARE = 0.15
# Ties break toward the honest half, so a page never rounds in our favour.
CELL_ORDER = (MISSED, IGNORED, FOUND_ANYWAY, REACH)

WHY = {
    REACH: ("Sense put {reach} of the {gold} answers in front of it with exact "
            "locations, and the model used them."),
    IGNORED: ("{ignored} more were returned by Sense and did not make it into the "
              "answer. That one is ours: the information arrived in a shape this "
              "model did not carry through."),
    FOUND_ANYWAY: ("{found_anyway} it reached on its own, by opening and searching "
                   "files rather than from a Sense result, so Sense did not shorten "
                   "that part of the work."),
    MISSED: ("{missed} were reached by neither: Sense did not return them and the "
             "answer did not name them."),
}

ROUTING_NOTE = {
    "never-routed": ("This model never called Sense. The tools were installed and the "
                     "server was running; it chose to work the way it always does. "
                     "That is a routing problem on our side, so this column is left "
                     "out of the replication count rather than counted as a result."),
    "search-only": ("This model called Sense to search, but never asked it a "
                    "dependency question, so the part of Sense under test here was "
                    "never exercised. Left out of the replication count."),
    "harness-failure": ("The Sense server did not come up for this arm, so nothing "
                        "was measured. The run is discarded, not scored."),
}


def load_copy(path=None):
    with open(path or COPY_PATH) as fh:
        return json.load(fh)


def _blurb(copy, kind, key):
    return (copy.get(kind, {}) or {}).get(key, {})


def _label(copy, kind, key):
    return _blurb(copy, kind, key).get("label", key)


def _n(x):
    """Whole means print whole: 21.0 rows reads like a measurement error."""
    return f"{x:g}"


def _counts_across_runs(mech):
    """Mean cells over a column's measured runs."""
    total = {REACH: 0, IGNORED: 0, FOUND_ANYWAY: 0, MISSED: 0}
    runs = [r for r in mech.get("runs", []) if r.get("routing") != "harness-failure"]
    for run in runs:
        for cell, n in (run.get("counts") or {}).items():
            total[cell] = total.get(cell, 0) + n
    n = max(len(runs), 1)
    return {k: round(v / n, 1) for k, v in total.items()}, len(runs)


def _reach_line(column, gold_rows):
    """The headline of the programme: what it could NOT otherwise find.

    A delta says the score moved. Sense-only reach says which answers the model
    never reached on its own in any run, and reached with Sense. That is the
    claim the whole bench exists to test, so it leads the block.
    """
    n = len(column.get("sense_only_reach") or [])
    if not n or not _routed(column):
        return ""
    return (f"**{n} of the {gold_rows} answers this model never found on its own**, "
            f"in either run without Sense, it found with Sense.")


def _why_line(column, gold_rows):
    """The sentences, chosen and ordered mechanically. Largest cell first."""
    states = column.get("routing") or []
    for state in ("never-routed", "search-only", "harness-failure"):
        if states == [state]:
            return ROUTING_NOTE[state]
    mech = column.get("mechanism") or {}
    if mech.get("verdict_split"):
        return ("This model's two runs tell different stories, so the board does not "
                "call it either way until a third run rules.")
    avg, measured = _counts_across_runs(mech)
    if not measured:
        return "No Sense traffic was captured for this column."
    floor = max(gold_rows * CELL_SHARE, 1)
    fields = {"gold": gold_rows, "reach": _n(avg[REACH]), "ignored": _n(avg[IGNORED]),
              "found_anyway": _n(avg[FOUND_ANYWAY]), "missed": _n(avg[MISSED])}
    told = [c for c in CELL_ORDER if avg[c] >= floor]
    told.sort(key=lambda c: (-avg[c], CELL_ORDER.index(c)))
    if not told:
        told = [mech.get("dominant") or REACH]
    return " ".join(WHY[c].format(**fields) for c in told if c in WHY)


def _plural(n, one, many):
    """The plural is passed in, not guessed: "search" pluralised to "searchs"."""
    return f"{n:.0f} {one if abs(n - 1) < 0.5 else many}"


def _mins(seconds):
    """Minutes, because a reader thinks in minutes and not in 322.5 seconds."""
    return f"{seconds / 60:.1f} min"


def _session_bullets(column):
    """Time, tokens and money, per arm. A delta with no cost beside it is half a result."""
    ses = column.get("session") or {}
    b, s = ses.get("baseline"), ses.get("sense")
    if not b or not s:
        return []
    out = []
    if b.get("wall_time_seconds") and s.get("wall_time_seconds"):
        out.append(f"- **time** {_mins(b['wall_time_seconds'])} on its own, "
                   f"{_mins(s['wall_time_seconds'])} with Sense")
    # Tokens, never dollars. Four of the five arms are on flat-rate plans and
    # report no price, so a money column would be filled for one model and empty
    # for the rest; token_total_all is built identically on every harness.
    if b.get("token_total_all") and s.get("token_total_all"):
        out.append(f"- **tokens used** {b['token_total_all']:,.0f} on its own, "
                   f"{s['token_total_all']:,.0f} with Sense "
                   f"(everything the session moved, cached context included)")
    if b.get("token_total_billed") and s.get("token_total_billed"):
        out.append(f"- **of which billed** {b['token_total_billed']:,.0f} on its own, "
                   f"{s['token_total_billed']:,.0f} with Sense")
    return out + [
        f"- **how it worked** {_plural(b['grep_count'], 'search', 'searches')} and "
        f"{_plural(b['read_count'], 'file read', 'file reads')} on its own; "
        f"{_plural(s['mcp_count'], 'Sense call', 'Sense calls')}, "
        f"{_plural(s['grep_count'], 'search', 'searches')} and "
        f"{_plural(s['read_count'], 'file read', 'file reads')} with Sense",
    ]


def _glance_row(column, copy):
    label = _label(copy, "models", column["model"])
    if not column.get("measured"):
        return f"| {label} | not run | not run | | | |"
    o = column["overall"]
    states = column.get("routing") or []
    if states and states[0] in ROUTING_NOTE:
        mark = {"never-routed": "never called Sense",
                "search-only": "no dependency question asked",
                "harness-failure": "not measured"}[states[0]]
        return (f"| {label} | {o['baseline_mean']:.2f} | {o['sense_mean']:.2f} | "
                f"{o['delta']:+.2f} | | {mark} |")
    reach = len(column.get("sense_only_reach") or [])
    ses = (column.get("session") or {}).get("sense") or {}
    when = _mins(ses["wall_time_seconds"]) if ses.get("wall_time_seconds") else ""
    return (f"| {label} | {o['baseline_mean']:.2f} | {o['sense_mean']:.2f} | "
            f"**{o['delta']:+.2f}** | **{reach}** | {when} |")


def _routed(column):
    """A column whose number is actually about Sense."""
    states = column.get("routing") or []
    return bool(column.get("measured")) and (not states or states[0] not in ROUTING_NOTE)


def _delta_chart(data, copy):
    """One bar per model: what Sense added to that model's own score.

    Deltas, never absolute scores. A chart of raw scores side by side would read
    as a model ranking however the caption is worded, and that is the one thing
    this page does not claim. Charted beside the table, never instead of it, so
    the numbers survive if a renderer does not draw mermaid.
    """
    cols = [c for c in data["columns"] if _routed(c)]
    if len(cols) < 2:
        return []
    labels = ", ".join(f'"{_label(copy, "models", c["model"])}"' for c in cols)
    values = ", ".join(f"{c['overall']['delta']:.4f}" for c in cols)
    # Headroom above the tallest bar: an axis that ends exactly at the maximum
    # clips it, and a clipped bar on a public page reads as a rendering fault.
    top = min(1.0, max(0.5, max(c["overall"]["delta"] for c in cols) * 1.15))
    return ["```mermaid", "xychart-beta",
            '    title "What Sense added to each model"',
            f"    x-axis [{labels}]",
            f'    y-axis "Extra answers found, share of the whole" 0 --> {top:.2f}',
            f"    bar [{values}]", "```", ""]


def _coverage_chart(column, copy, gold_rows):
    """Which arm reached each answer. The value picture, not the mechanism one.

    An earlier version charted where the answers came from INSIDE the Sense run:
    supplied and used, supplied and unused, found without Sense, reached by
    neither. With no baseline in it that reads as "Sense supplied about half",
    which is both dispiriting and not what happened. The split that tells the
    truth is by ARM, because it is the only one that can show the answers that
    exist solely because Sense was there.
    """
    cov = column.get("coverage") or {}
    if not cov or not cov.get("total"):
        return []
    slices = [("Found only with Sense", cov.get("sense_only", 0)),
              ("Found either way", cov.get("both", 0)),
              ("Found only without Sense", cov.get("baseline_only", 0)),
              ("Found by neither", cov.get("neither", 0))]
    label = _label(copy, "models", column["model"])
    out = ["```mermaid", "pie showData",
           f"    title {label}: which of the {gold_rows} answers each arm reached"]
    out += [f'    "{name}" : {v:g}' for name, v in slices if v]
    return out + ["```", ""]


def _method_chart():
    """How a number on this page is produced, before anyone argues about one."""
    return ["```mermaid", "flowchart LR",
            '    Q["One question<br/>7 steps, same for both"]',
            '    A["The model on its own"]',
            '    B["The model with Sense"]',
            '    G["Answers fixed in advance<br/>hand-audited, file and line"]',
            '    J["Blind grader<br/>never told which arm"]',
            '    S["Share of the answers named"]',
            "    Q --> A", "    Q --> B", "    A --> J", "    B --> J",
            "    G --> J", "    J --> S", "```", ""]


def _glance(data, copy):
    out = ["## At a glance", "",
           "| model | on its own | with Sense | difference | found only with Sense "
           "| time with Sense |",
           "|---|---|---|---|---|---|"]
    out += [_glance_row(c, copy) for c in data["columns"]]
    out += ["", f"Share of the {data.get('gold_rows', 0)} hand-audited answers each "
            f"model cited, working the same question. Read each row across, never down: "
            f"a model is only ever compared to itself.", ""]
    out += _delta_chart(data, copy)
    return out


def _column_block(column, gold_rows, copy):
    info = _blurb(copy, "models", column["model"])
    out = [f"### {info.get('label', column['model'])}", ""]
    if info.get("blurb"):
        out += [info["blurb"], ""]
    if not column.get("measured"):
        out += ["Not run for this board yet.", ""]
        return out

    o, runs = column["overall"], column["runs"]
    src = ("carried over from the run that first proved this question"
           if column["source"] == "banked" else "benched for this board")
    reach = _reach_line(column, gold_rows)
    if reach:
        out += [reach, ""]
    out += [_why_line(column, gold_rows), ""]
    out.append(f"- **on its own** {o['baseline_mean']:.4f}  ->  **with Sense** "
               f"{o['sense_mean']:.4f}   (**{o['delta']:+.4f}**)")
    out.append(f"- **hardest group of answers** {column['best_group_delta']:+.4f}")
    mech = column.get("mechanism") or {}
    avg, measured = _counts_across_runs(mech)
    if measured:
        out.append(f"- **where the answers came from** Sense and used {_n(avg[REACH])}, "
                   f"Sense but unused {_n(avg[IGNORED])}, found without Sense "
                   f"{_n(avg[FOUND_ANYWAY])}, reached by neither {_n(avg[MISSED])}")
    out += _session_bullets(column)
    out.append(f"- **runs** {runs['baseline']} without Sense, {runs['sense']} with, {src}")
    out.append("")
    out += _coverage_chart(column, copy, gold_rows)

    flipped = mech.get("rows_disagreeing") or []
    if flipped:
        out += [f"Across this model's runs, {len(flipped)} answers came back from Sense "
                f"one time and not the other. We track that as a determinism issue and "
                f"it is ours to close.", ""]
    return out


def _question_block(data, repo_label):
    """The scenario and the task, verbatim. A result is unreadable without it."""
    q = data.get("question") or {}
    out = ["## The question", ""]
    out.append(f"A maintainer is about to rework a central class in {repo_label} and "
               f"needs to know what depends on it before touching it. The answer is "
               f"scattered across the codebase, so the work is finding all of it, not "
               f"reasoning about any one piece.")
    out.append("")
    if q.get("contract_symbol"):
        out += [f"The class under rework is `{q['contract_symbol']}`"
                + (f" in `{q['contract_file']}`." if q.get("contract_file") else "."),
                ""]
    if q.get("description"):
        out += ["The models were given no more than this framing, which deliberately "
                "never names the class: finding it is part of the task.", "",
                "> " + q["description"].replace("\n", "\n> "), ""]
    steps = q.get("steps") or []
    if steps:
        out += [f"The session is {len(steps)} steps, sent in order. Verbatim:", "",
                "<details>", "<summary>The full task, exactly as each model received it"
                "</summary>", ""]
        for i, step in enumerate(steps, 1):
            out.append(f"**{i}. {step.get('name', '')}**")
            out.append("")
            out.append("> " + (step.get("prompt", "").replace("\n", "\n> ")))
            out.append("")
        out += ["</details>", ""]
    return out


def _replication(rep, columns, copy):
    out = ["## Does it hold across models", ""]
    routed, repl = rep["routed"], rep["replicated"]
    if not routed:
        out += ["No confirmation model has been run against this question yet.", ""]
        return out
    names = ", ".join(f"**{_label(copy, 'models', m)}**" for m in repl)
    out.append(f"{len(repl)} of the {len(routed)} models that actually queried Sense "
               f"cleared the same bar the question was proved at"
               + (f": {names}." if repl else "."))
    for key, label in (("never_routed", "Never called Sense"),
                       ("search_only", "Never asked a dependency question"),
                       ("not_measured", "Not run yet")):
        if rep.get(key):
            out.append("")
            out.append(f"*{label}:* "
                       + ", ".join(_label(copy, "models", m) for m in rep[key]) + ".")
    out.append("")
    return out


def _limits(data, copy):
    return [
        "## What this is, and what it is not", "",
        "This is a measurement of **Sense**, run on one question against one "
        "repository, with each model answering twice with Sense available and twice "
        "without.",
        "",
        "It is **not a comparison of the models**. Each one runs on its own harness "
        "with its own budget and its own defaults, so their scores are not comparable "
        "to each other and are never presented that way here.",
        "",
        "It is **not a comparison of the repositories** either. Questions are written "
        "per repository and hand-audited per repository; a number from one page does "
        "not rank against a number from another.",
        "",
        "How a number here is produced:",
        "",
        *_method_chart(),
        f"The answers were fixed in advance: {data.get('gold_rows', 0)} locations, "
        "audited by hand before any model ran, and a model is credited only when it "
        "names the file and the line. Grading is done by a separate pinned model that "
        "is never told which arm or which model produced an answer.",
        "",
    ]


def render(data, copy=None):
    copy = copy or load_copy()
    repo_info = _blurb(copy, "repos", data["repo"])
    vert_info = _blurb(copy, "verticals", data.get("vertical", ""))
    repo_label = repo_info.get("label", data["repo"])
    # The board covers every declared column, benched or not: a page that counted
    # only the measured ones would silently shrink when an arm had not run yet.
    n_models = len(data["columns"])
    plural = "model" if n_models == 1 else "models"

    out = [f"# Does Sense help an AI understand {repo_label}?", ""]
    out.append(f"{repo_info.get('blurb', '')} One question about its code, put to "
               f"{n_models} {plural} twice each, with Sense and without.".strip())
    out.append("")
    out += ["## Reading", "", "<!-- reading -->", ""]
    out += _glance(data, copy)

    out += _question_block(data, repo_label)
    out += ["| | |", "|---|---|",
            f"| repository | [{repo_label}]({repo_info.get('url', '')}) |",
            f"| stack | {vert_info.get('label', data.get('vertical', ''))} |",
            f"| answers to find | {data.get('gold_rows', 0)}, hand-audited |",
            f"| Sense build | {data.get('sense_version', '')} |",
            f"| question id | `{data.get('scenario_version', '')}` |", ""]

    out += ["## Model by model", ""]
    for column in data["columns"]:
        out += _column_block(column, data.get("gold_rows", 0), copy)

    out += _replication(data["replication"], data["columns"], copy)
    out += _limits(data, copy)
    return "\n".join(out).rstrip() + "\n"


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("numbers", help="the JSON written by board.py assemble")
    ap.add_argument("-o", "--out", default=None)
    ap.add_argument("--copy", default=None, help="override board_copy.json")
    args = ap.parse_args(argv[1:])

    text = render(json.load(open(args.numbers)), load_copy(args.copy))
    if args.out:
        with open(args.out, "w") as fh:
            fh.write(text)
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
