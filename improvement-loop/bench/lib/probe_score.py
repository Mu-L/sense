#!/usr/bin/env python3
"""probe_score.py - how much of the pool did the adversary probe actually pin? Arithmetic.

The `probe` phase asks a Sense-less agent to beat a shape before it is written, then asks
that same agent to grade itself: "ASSEMBLED means you BELIEVE you produced the answer".
The probe never sees the periphery pool it is being graded against, so the verdict is a
claim about its own exhaustiveness - which is the exact belief this whole bench exists to
measure. A cold agent that finds a quarter of the dependents and calls the audit finished
is the finding, not the instrument.

So score it. `shape.md` carries the pool at `path:line`, one row per FILE, and
`adversary-probe.md` carries what the probe pinned under `# Covered`. Intersect them and
the self-grade becomes a number.

THE NUMBER IS RECORDED, NOT ENFORCED. The probe does not estimate the benched arm, so
this script does not kill a shape and no caller should treat it as a gate. Two calibration
points on ruby-rails, both large and opposite: on the parked cell the probe reached 4 of 16
where the arm reached 10; on the banked cell an unleaked probe reached at most 7 of 16
where the arm reached 2.5, and a leaked one reached 15 of 16 over that same arm. Coverage
here is the probe's, and only the validation pair measures the arm's.

The ceiling it prints is `pay_ceiling.py`'s arithmetic applied to the probe: `pergroup.py`
declares WIN at +0.50 on a gold group and recall caps at 1.00, so a baseline holding
fraction C caps the delta at `1.00 - C`. Read it as "what the ceiling would be IF the arm
matched the probe" - a hypothesis the validation run then tests.

  probe_score.py <adversary-probe.md> <shape.md> [floor]

Exit 0 = thin coverage; the shape has room even if the arm matches the probe.
Exit 1 = the probe covered the pool. A signal to curate carefully, NOT a kill.
Exit 64 = neither section could be read; a missing measurement is not a pass.

Matching mirrors the scorer's path-fragment rule: a pool file counts as covered when the
probe printed a `path:line` for it, comparing repo-relative suffixes. A bare basename does
NOT credit the probe - crediting it loosely would kill good shapes, which is the failure
this script exists to stop.
"""
import re
import sys

DEFAULT_FLOOR = 0.50
PATH_LINE = re.compile(r"([A-Za-z0-9_][A-Za-z0-9_./+-]*\.[A-Za-z0-9]+):(\d+)")


def section(text, heading):
    """The body under a `# <heading>` line, up to the next top-level heading."""
    out, taking = [], False
    for line in text.splitlines():
        if re.match(r"^#\s+\S", line):
            taking = line.lstrip("# ").strip().lower().startswith(heading.lower())
            continue
        if taking:
            out.append(line)
    return "\n".join(out)


def paths(text):
    """Every distinct file that carries a `path:line` in this text."""
    return {m.group(1) for m in PATH_LINE.finditer(text)}


def covers(pool_path, probe_path):
    """Does a probe citation land on this pool file? Suffix match, no bare basenames."""
    if pool_path == probe_path:
        return True
    if "/" in probe_path and pool_path.endswith("/" + probe_path):
        return True
    return "/" in pool_path and probe_path.endswith("/" + pool_path)


def score(probe_text, shape_text):
    """Return (pool, covered, coverage, ceiling) or (pool, ..., None, None) when unusable."""
    pool = sorted(paths(section(shape_text, "Periphery pool")))
    hits = paths(section(probe_text, "Covered"))
    if not pool or not hits:
        return pool, [], None, None
    covered = sorted(p for p in pool if any(covers(p, q) for q in hits))
    coverage = len(covered) / len(pool)
    return pool, covered, coverage, 1.0 - coverage


def main(argv):
    if len(argv) < 2:
        print("usage: probe_score.py <adversary-probe.md> <shape.md> [floor]")
        return 64
    floor = float(argv[2]) if len(argv) > 2 else DEFAULT_FLOOR
    try:
        probe_text = open(argv[0]).read()
        shape_text = open(argv[1]).read()
    except OSError as exc:
        print("probe_score: %s" % exc, file=sys.stderr)
        return 64

    pool, covered, coverage, ceiling = score(probe_text, shape_text)
    if coverage is None:
        print("probe_score: could not read a pool from '# Periphery pool' (%d rows) or a "
              "citation from '# Covered'. A missing measurement is not a pass."
              % len(pool), file=sys.stderr)
        return 64

    print("### adversary probe vs the periphery pool\n")
    print("pool files          %d" % len(pool))
    print("pinned by the probe %d" % len(covered))
    print("coverage            %.3f" % coverage)
    print("ceiling             %+.3f   (1.00 - coverage)" % ceiling)
    missed = [p for p in pool if p not in set(covered)]
    if missed:
        print("\nnot pinned by the probe (%d):" % len(missed))
        for p in missed:
            print("  %s" % p)

    if ceiling >= floor:
        print("\nPROBE_SCORE: THIN - the probe pinned %.0f%% of the pool, leaving room for "
              "+%.2f even if the arm matches it." % (coverage * 100, floor))
        return 0
    print("\nPROBE_SCORE: COVERED - the probe pinned %.0f%% of the pool. An arm matching the "
          "probe would cap the delta at %+.3f against a +%.2f floor."
          % (coverage * 100, ceiling, floor))
    print("RECORDED, NOT A KILL. The arm is not the probe: on the banked cell a probe pinned")
    print("15 of 16 gold rows over an arm that pinned 2.5. Curate from the rows above that")
    print("the probe missed FIRST, and let the validation pair measure the arm.")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
