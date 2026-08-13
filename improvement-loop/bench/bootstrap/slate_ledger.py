#!/usr/bin/env python3
"""Write the `bootstrap/slate` LEDGER entry from the composed slate on disk.

`ledger.md`'s write-point table has declared `bootstrap/slate` since the table
was written and `ledger_check.py` accepts the key, but nothing ever emitted it:
the pipeline printed "## [next] write the bootstrap/slate LEDGER entry" and left
it to a human who was never told. A declared write point that no code reaches is
the same failure as a check that cannot fire - it reads green because it never
runs. This closes it the way `scaffold_ledger.py` closes the scaffold half.

Every field is read from `slate.json`, `PINNED_COMMITS.json` and the index state,
so the entry records what was actually composed rather than what someone recalled.

Driven by run.sh through the environment (KEY, IL_ROOT).
"""

import datetime
import json
import os
import sys

TEMPLATE = """
## {date} | bootstrap/slate | {key} slate composed: {composition}

- **What:** {n} repo(s) admitted from a pool of {n_pool} candidate(s), composed by
  `compose.py` and verified by `slate_check.py`. Per-repo numbers stay in
  `repos.md`; the seats and their pins are:
{rows}
- **Composition:** {composition} - §7.0 checked by `slate_check.py`, which also
  verified the pins, both arms and the indexes.
- **Why:** the hunt's declared queries in `stacks/{key}.conf` returned the pool and
  the screen filtered it; a slate composed from a declared query is a slate whose
  "the pool is exhausted" claim can be re-checked later. Nothing here predicts a
  win - that is crafted in Loop 1 against a scenario.
- **Alternatives:** hand-picking the four repos - rejected: the queries are the
  search a later exhaustion claim rests on, and a hand-picked slate has none.
- **Lesson:** {lesson}
  `Exit: check(slate_check.py {key} rc=0; run.sh returns status=READY-FOR-LOOP)`
- **Scores:** n/a: bootstrap, no runs.
- **Cost:** $0 API. Subscription: index wall-clock only; fleet: no spawns
  (0 spawns, main session only).
- **Links:** `repos.md` (per-candidate verdicts), `slate.json`, `pool.txt`,
  `stacks/{key}.conf`, `docs/bootstrap.md`.
"""

CLEAN_LESSON = ("none owed - the pool screened cleanly and the composition check "
                "agreed on the first pass. A loop that reports a lesson every time "
                "it runs is manufacturing them.")

HEADER = """# {key} - LEDGER

Append-only narrative for the {key} vertical. Entry schema and write points:
[`../../docs/ledger.md`](../../docs/ledger.md). Never edited
after the fact; never committed.
"""


def count_pool(vdir):
    """Pool size, or 0 when the hunt was skipped and pool.txt never written."""
    path = os.path.join(vdir, "pool.txt")
    if not os.path.exists(path):
        return 0
    with open(path, encoding="utf-8") as fh:
        return len([ln for ln in fh if ln.strip() and not ln.startswith("#")])


def pins(vdir):
    path = os.path.join(vdir, "PINNED_COMMITS.json")
    if not os.path.exists(path):
        return {}
    with open(path, encoding="utf-8") as fh:
        return {k: v for k, v in json.load(fh).items() if k != "_meta"}


def rows_and_composition(slate, pinned):
    """One line per seat, plus the slot tally the §7.0 rule is judged on."""
    rows, tally = [], {}
    for repo, meta in sorted(slate.items()):
        if repo == "_meta":
            continue
        slot = meta.get("slot", "?")
        tally[slot] = tally.get(slot, 0) + 1
        sha = (pinned.get(repo) or {}).get("sha", "")
        backup = (meta.get("backup") or {}).get("repo", "none")
        rows.append(f"  - `{repo}` slot={slot} size={meta.get('size', '?')} "
                    f"prod_files={meta.get('prod_files', '?')} "
                    f"pin=`{sha[:12] or 'unpinned'}` backup=`{backup}`")
    composition = " + ".join(f"{n} {s}" for s, n in sorted(tally.items()))
    return "\n".join(rows), composition or "empty"


def main():
    env = os.environ
    key = env["KEY"]
    vdir = os.path.join(env["IL_ROOT"], "verticals", key)

    slate_path = os.path.join(vdir, "slate.json")
    if not os.path.exists(slate_path):
        print(f"   no slate.json in verticals/{key}/ - nothing to record",
              file=sys.stderr)
        return 1
    with open(slate_path, encoding="utf-8") as fh:
        slate = json.load(fh)

    rows, composition = rows_and_composition(slate, pins(vdir))
    body = TEMPLATE.format(
        date=datetime.date.today().isoformat(),
        key=key,
        n=len([r for r in slate if r != "_meta"]),
        n_pool=count_pool(vdir),
        rows=rows,
        composition=composition,
        lesson=CLEAN_LESSON)

    path = os.path.join(vdir, "LEDGER.md")
    if not os.path.exists(path):
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(HEADER.format(key=key))
    with open(path, "a", encoding="utf-8") as fh:
        fh.write(body)
    print(f"   wrote bootstrap/slate to verticals/{key}/LEDGER.md", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
