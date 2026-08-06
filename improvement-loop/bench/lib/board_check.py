#!/usr/bin/env python3
"""The gate before a board is published: no figure that is not in the numbers.

The whole page is generated except one section, and that section is written by
an agent. This is what makes "no invented number" a check rather than a hope.
It fails CLOSED: anything it cannot account for is a failure, because the cost
of a wrong figure on a public page about four vendors' models is much higher
than the cost of a rewrite.

Three checks, all mechanical:

  1. THE READING EXISTS. The `<!-- reading -->` marker is gone and something
     stands in its place. A board published with the placeholder still in it is
     the most embarrassing possible outcome and the cheapest to prevent.

  2. EVERY NUMBER IS ACCOUNTED FOR. Each numeric token in the Reading section
     must match a figure derivable from the numbers JSON: a numeric leaf, a list
     length, or one of the roundings the page itself uses. Prose numbers that are
     not measurements ("in one sentence", "three models") are why the agent also
     declares what it used, and both sides have to agree.

  3. NOTHING PRIVATE LEAKED. The results tree is gitignored for a reason: it
     holds full transcripts of somebody else's source. A path into it, or a run
     directory, or a raw log name has no business on a published page.

Usage:
    board_check.py <board.md> <numbers.json> [--verdict <read.verdict.json>]
"""
import argparse
import json
import re
import sys
from pathlib import Path

NUMBER_RE = re.compile(r"(?<![\w/:.-])[+-]?\d[\d,]*(?:\.\d+)?%?(?![\w.-])")
# Paths and artifacts that only exist inside the private results tree.
LEAK_RE = re.compile(r"(results/|/run-\d|sense-io\.jsonl|transcript\.json|scored\.json"
                     r"|judged\.json|claude\.log)")
READING_MARKER = "<!-- reading -->"


def reading_section(text):
    """The one section an agent wrote, between its heading and the next one."""
    match = re.search(r"^## Reading\s*$(.*?)^## ", text, re.M | re.S)
    return match.group(1) if match else ""


def _numbers_in(text):
    """Numeric tokens, normalised: no commas, no percent, no leading plus."""
    out = set()
    for raw in NUMBER_RE.findall(text):
        out.add(raw.replace(",", "").replace("%", "").lstrip("+"))
    return out


def _leaves(node, acc):
    """Every numeric leaf and every list length in the numbers JSON."""
    if isinstance(node, dict):
        for value in node.values():
            _leaves(value, acc)
    elif isinstance(node, list):
        acc.add(float(len(node)))
        for value in node:
            _leaves(value, acc)
    elif isinstance(node, bool):
        pass
    elif isinstance(node, (int, float)):
        acc.add(float(node))


def allowed_figures(data):
    """Every string form a measured figure may honestly take on the page."""
    values = set()
    _leaves(data, values)
    out = set()
    for v in values:
        # .0f as well as %g: %g flips to scientific notation past six significant
        # digits, so a million-token total came out as "1.74833e+06" and no honest
        # figure on the page could ever match it.
        # .1f as well, and it is not a rounding: a token total is a mean over two
        # runs, so the leaf itself carries one decimal, and quoting it verbatim was
        # refused as invented while every rounding of it passed.
        out.update({f"{v:g}", f"{v:.0f}", f"{v:.1f}", f"{v:.2f}", f"{v:.4f}",
                    f"{abs(v):g}", f"{abs(v):.0f}", f"{abs(v):.1f}",
                    f"{abs(v):.2f}", f"{abs(v):.4f}"})
        if abs(v) <= 1:
            # The page states shares as percentages in prose.
            out.update({f"{v * 100:g}", f"{v * 100:.0f}", f"{v * 100:.1f}"})
        if abs(v) >= 60:
            # And durations in minutes, the way the bullets do.
            out.add(f"{v / 60:.1f}")
    return {s.lstrip("+") for s in out}


def _without_model_names(text):
    """A model's own name is not a figure: "Claude Opus 5" is not a claim of 5.

    The regex already skips a version glued into an id (`gpt-5.6-sol`), but a
    label carries its number as a separate word and read as an undeclared 5.
    """
    copy = json.load(open(Path(__file__).with_name("board_copy.json")))
    for key, entry in copy.get("models", {}).items():
        for name in (entry.get("label", ""), key):
            if name:
                text = text.replace(name, "")
    return text


def check(board_text, data, declared=None):
    """Every failure, named. Empty list means the page may be published."""
    problems = []

    if READING_MARKER in board_text:
        problems.append("the reading placeholder is still in the page")
    section = reading_section(board_text)
    if not section.strip():
        problems.append("the Reading section is empty")

    prose = _without_model_names(section)
    allowed = allowed_figures(data)
    if declared is not None:
        undeclared = sorted(_numbers_in(prose) - {str(d) for d in declared})
        if undeclared:
            problems.append("figures used but not declared in the verdict: "
                            + ", ".join(undeclared))
    for figure in sorted(_numbers_in(prose)):
        if figure not in allowed:
            problems.append(f"figure {figure!r} is not derivable from the numbers JSON")

    for match in sorted(set(LEAK_RE.findall(board_text))):
        problems.append(f"private artifact named on a public page: {match!r}")

    return problems


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("board")
    ap.add_argument("numbers")
    ap.add_argument("--verdict", default=None,
                    help="read.verdict.json, whose `figures` must match the prose")
    args = ap.parse_args(argv[1:])

    declared, missing = None, []
    if args.verdict:
        try:
            declared = json.load(open(args.verdict)).get("figures")
        except (OSError, ValueError) as err:
            # A missing or unparseable verdict is a refusal, not a stack trace: this
            # runs at the moment someone is being told why a page cannot ship.
            missing.append(f"no usable report verdict at {args.verdict}: {err}")
        if declared is not None:
            declared = [str(f).replace(",", "").replace("%", "").lstrip("+")
                        for f in declared]

    problems = missing + check(open(args.board).read(),
                               json.load(open(args.numbers)), declared)
    if problems:
        print("board_check: REFUSED", file=sys.stderr)
        for p in problems:
            print(f"  - {p}", file=sys.stderr)
        return 1
    print("board_check: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
