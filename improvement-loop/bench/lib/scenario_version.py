#!/usr/bin/env python3
"""scenario_version.py - the identity of a scenario, as one short hash.

sha256 of the bytes that get SCORED: the scenario yaml plus its rubric sibling. Scoped
that way so an unrelated repo edit does not move it, and so every reshape that reuses a
filename gets a distinct id.

It already rode in every `run_meta.json`; what it did not do was gate anything. A
validation run was matched by model and repo alone, so a re-authored shape inherited the
previous shape's pair and the loop reported a validation of gold that had never been run.
One recipe, used by the runner that stamps it and the driver that matches on it - two
copies of a hash recipe is how the two silently stop agreeing.

  scenario_version.py <scenario.yaml> [rubric.yaml]

Prints `sha256:<16 hex>`. Exits 1 if the scenario file does not exist; a missing rubric is
not an error (it is hashed only when present, matching the stamp).
"""
import hashlib
import os
import sys


def version(scenario, rubric=None):
    if rubric is None:
        rubric = scenario[:-5] + ".rubric.yaml" if scenario.endswith(".yaml") else ""
    h = hashlib.sha256()
    for p in (scenario, rubric):
        if p and os.path.exists(p):
            with open(p, "rb") as fh:
                h.update(fh.read())
    return "sha256:" + h.hexdigest()[:16]


def main(argv):
    if not argv:
        print("usage: scenario_version.py <scenario.yaml> [rubric.yaml]", file=sys.stderr)
        return 64
    if not os.path.exists(argv[0]):
        print("scenario_version: no scenario at %s" % argv[0], file=sys.stderr)
        return 1
    print(version(argv[0], argv[1] if len(argv) > 1 else None))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
