#!/usr/bin/env python3
"""rubric_check.py - run the judge's own rubric contract at authoring time, for $0.

`judge.load_rubric` already validates everything that matters: `audience` and `steps`
present, one rubric step per scenario step, step names matching VERBATIM, every
required criterion declared, and extended-rubric weights summing to 1.0.

The problem was never the contract, it was WHEN it ran. It ran at judge time, which
is after both arms have been spent: a rubric authored in an invented shape
(`{scenario, judge}` instead of `{audience, steps}`) took a full validation pair with
it before anything said so. Nothing else checked the rubric at all - the gold gets a
hand audit and a verify gate, the rubric got nothing.

So this is the same check, moved to where it costs nothing:

    python3 rubric_check.py <scenario.yaml> [<rubric.yaml>]

Rubric path defaults to the scenario path with `.yaml` -> `.rubric.yaml`.
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import judge
import scenario as scenario_mod


def default_rubric_path(scenario_path):
    base = scenario_path[: -len(".yaml")] if scenario_path.endswith(".yaml") else scenario_path
    return base + ".rubric.yaml"


def check(scenario_path, rubric_path=None):
    """Return (ok, message). Never raises for a bad rubric - that is the answer."""
    rubric_path = rubric_path or default_rubric_path(scenario_path)
    try:
        parsed = scenario_mod.parse(scenario_path)
    except (OSError, ValueError) as exc:
        return False, "scenario does not parse: %s" % exc
    try:
        judge.load_rubric(rubric_path, parsed.get("steps", []))
    except SystemExit as exc:
        return False, str(exc)
    return True, "rubric matches the scenario's %d steps" % len(parsed.get("steps", []))


def main(argv):
    if not argv or argv[0] in ("-h", "--help"):
        print("usage: rubric_check.py <scenario.yaml> [<rubric.yaml>]")
        return 0 if argv else 64
    ok, message = check(argv[0], argv[1] if len(argv) > 1 else None)
    print("## rubric check - %s" % os.path.basename(argv[0]))
    print("   %s - %s" % ("PASS" if ok else "FAIL", message))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
