#!/usr/bin/env python3
"""ledger_check.py - $0 advisory check for a vertical's LEDGER.md (ledger.md).

Runs at the publish sign-off alongside the closing review; blocks nothing mid-flight.
Verifies shape and coverage, NOT immutability (.doc has no git history):

  1. every entry heading is `## YYYY-MM-DD | key | title`
  2. every entry carries the required fields (What/Why/Alternatives/Lesson/Scores/Cost/Links)
  3. no duplicate entry keys (keys are what make checklist-loop re-runs idempotent)
  4. every results cell (results/<model>/<arm>/<repo>, including _invalid-* groups)
     is referenced by at least one entry; a reference to an ancestor directory
     covers its subtree (seed entries do this deliberately)
  5. codename scan (heuristic): the first line mentioning a G-<n>/F-<...> codename
     inside an entry must expand it on that same line (a parenthetical or a
     comma expansion following the codename)
  6. lesson exit law (schema v2): a Lesson that is not "none" must carry an exit
     tag: Exit: check(...) | rule(...) | fixture(...) | parked(...)
  7. provenance (schema v2): the per-repo verdict entries (loop2/<repo>/run-<n>,
     loop3/<repo>/swap|close) must carry a **Provenance:** field (sense version, pin
     SHA, scenario + date). Loop 7 ships fixes between verticals, so a cell's verdict
     is always true OF a build; without the build recorded, a dead cell gets silently
     re-proposed or a stale kill gets cited as current.
  8. cost currency (schema v3): a Cost field must record the subscription side, not
     dollars alone. "$0 API" is true and irrelevant when the weekly reset is the
     binding constraint; the fleet (spawn counts, sessions) is what spends it.
  9. key contract (by ruling, forward-only): entries dated on or after
     KEY_CONTRACT_FROM must use a key from ledger.md's write-point table.
     Earlier entries are grandfathered by ruling and never rewritten. Born from
     rule 7 sitting inert: nothing had ever used the verdict key shape, so
     Provenance never fired on a real entry and looked green the whole time.

 10. stopper law (by ruling): a working-tree/staged change to a MEASUREMENT
     INSTRUMENT (the chain whose output becomes a number that decides something:
     gold/scorer/judge/screen/grounding/pergroup/efficiency) requires a
     `stopper/<slug>` entry dated today, carrying the re-score blast radius
     ("N of M runs") from `bench/lib/rescore_diff.py`. Born 2026-07-15: a scorer
     false-credit was FOUND, correctly analysed, then MITIGATED instead of fixed and
     filed to a review pile; it took 36h and a direct human question to surface, and a
     win was declared on inflated numbers in between. You cannot quietly change the
     scorer.
     DORMANCY CLOSED 2026-07-30: `git diff` never lists an UNTRACKED file, so while
     improvement-loop/ was untracked this rule fired for nothing and read green over all
     seven instruments. An instrument that exists on disk but is untracked now reports as
     UNVERIFIABLE - a finding of its own, because "did it change" has no answer without a
     baseline, and a check that cannot fire is worse than a short list.

Usage: python3 ledger_check.py <vertical-key> <vertical-doc-dir>
       e.g.: python3 ledger_check.py laravel 05-laravel-vertical
Exit 0 = clean, 1 = findings printed.
"""

import re
import sys
from pathlib import Path

HEADING = re.compile(r"^## (\d{4}-\d{2}-\d{2}) \| (\S+) \| (.+)$")
FIELDS = ["What", "Why", "Alternatives", "Lesson", "Scores", "Cost", "Links"]
CODENAME = re.compile(r"\b(G-\d+|F-[A-Z0-9]+(?:-[A-Za-z0-9]+)*)\b")
EXIT_TAG = re.compile(r"Exit: (check|rule|fixture|parked)\(")
VERDICT_KEY = re.compile(r"^(loop2/[^/]+/run-\d+|loop3/[^/]+/(swap|close))$")
# The subscription side of Cost = the FLEET, as a spawn count. Deliberately strict: an
# earlier draft accepted any mention of "session", which passed every dollars-only entry
# on the phrase "one session segment" (that is time, not fleet). The spawn count is the
# one subscription figure that is free from disk, so it is the one the check stands on.
FLEET = re.compile(r"(\d+\s+spawns?|no\s+spawns)", re.IGNORECASE)

# Rule 10. The measurement chain: any instrument whose output becomes a number that
# decides something. screen is IN - it decides what gets benched at all. Keyed by
# blast radius, not by folder: a bad
# gold ROW in one scenario is scoped (the hand-audit catches it); a bad MATCHER is systemic
# and retroactive across every run ever scored.
MEASUREMENT_INSTRUMENTS = (
    "improvement-loop/bench/lib/gold.py",
    "improvement-loop/bench/lib/scorer.py",
    "improvement-loop/bench/lib/judge.py",
    "improvement-loop/bench/bootstrap/screen.py",
    "improvement-loop/bench/lib/grounding.py",
    "improvement-loop/bench/lib/pergroup.py",
    "improvement-loop/bench/lib/efficiency.py",
)
BLAST_RADIUS = re.compile(r"\b\d+\s+of\s+\d+\b|\b\d+\s*/\s*\d+\s+runs?\b", re.IGNORECASE)

# The write-point table of ledger.md, as patterns. Keys are the ledger's index: they
# make checklist-loop re-runs idempotent, and they are what rule 7 keys off.
KEY_CONTRACT = [
    re.compile(p)
    for p in (
        r"^seed/\d{4}-\d{2}-\d{2}$",
        r"^ruling/[^/]+$",
        r"^stopper/[^/]+$",
        r"^bootstrap/scaffold$",
        r"^bootstrap/slate$",
        r"^loop1/[^/]+/(scenario|event-b)$",
        r"^loop2/[^/]+/(event-c|run-\d+)$",
        r"^loop3/[^/]+/(swap|close)$",
        r"^loop4/[^/]+/(done|parked)$",
        r"^loop5/[^/]+$",
        r"^loop6/event-e$",
        r"^loop7/[^/]+/(open|close|.+-shipped|.+-reverted)$",
        # Added 2026-08-05: three write points the ledger had been using since July
        # without the table ever naming them, plus cycle 2's published board. Rule 9
        # was flagging real entries as illegal, which makes the rule the error.
        r"^bench/[^/]+$",
        r"^finding/[^/]+$",
        r"^correction/[^/]+$",
        r"^cycle2/[^/]+/board$",
    )
]
# Forward-only by ruling: the entries recorded before KEY_CONTRACT_FROM keep
# their drifted keys and are never rewritten. A date, not a grandfather list, so the
# exemption cannot quietly grow.
KEY_CONTRACT_FROM = "2026-07-16"


def parse_entries(lines):
    """Return (entries, findings): entries as (key, heading_lineno, body_lines, date).

    The date is carried because rule 10 requires a stopper entry dated TODAY, and it was
    previously discarded here - so the check accepted any stopper entry ever written and a
    two-week-old one satisfied a fresh instrument change.
    """
    entries, findings, current = [], [], None
    for n, line in enumerate(lines, 1):
        if line.startswith("## "):
            m = HEADING.match(line)
            if m:
                current = (m.group(2), n, [], m.group(1))
                entries.append(current)
            else:
                findings.append(f"line {n}: malformed entry heading (want '## YYYY-MM-DD | key | title'): {line.strip()}")
                current = None
        elif current is not None:
            current[2].append(line)
    return entries, findings


def check_fields(entries):
    findings = []
    for key, n, body, _ in entries:
        text = "\n".join(body)
        for field in FIELDS:
            if f"**{field}:**" not in text:
                findings.append(f"entry '{key}' (line {n}): missing field **{field}:**")
    return findings


def check_duplicate_keys(entries):
    seen, findings = {}, []
    for key, n, _, _ in entries:
        if key in seen:
            findings.append(f"entry '{key}' (line {n}): duplicate key (first at line {seen[key]})")
        else:
            seen[key] = n
    return findings


def results_cells(results_dir):
    """Cells = parents of run-* dirs at results/<model>/<arm>/<repo>/run-N."""
    cells = set()
    if not results_dir.is_dir():
        return cells
    for run in results_dir.glob("*/*/*/run-*"):
        if run.is_dir():
            cells.add(run.parent.relative_to(results_dir).as_posix())
    return cells


def check_coverage(cells, ledger_text):
    findings = []
    for cell in sorted(cells):
        parts = cell.split("/")
        ancestors = ["/".join(parts[: i + 1]) for i in range(len(parts))]
        if not any(a in ledger_text for a in ancestors):
            findings.append(f"results cell not referenced by any entry: {cell}")
    return findings


def field_text(body, field):
    """The text of one bold field: its line plus continuations up to the next field."""
    out, on = [], False
    for line in body:
        if f"**{field}:**" in line:
            on = True
            out.append(line.split(f"**{field}:**", 1)[1])
            continue
        if on:
            if re.match(r"^- \*\*[A-Z]", line) or line.startswith("## "):
                break
            out.append(line)
    return "\n".join(out).strip()


def check_lesson_exit(entries):
    findings = []
    for key, n, body, _ in entries:
        lesson = field_text(body, "Lesson")
        if not lesson or re.match(r"^none\b", lesson, re.IGNORECASE):
            continue
        if not EXIT_TAG.search(lesson):
            findings.append(
                f"entry '{key}' (line {n}): Lesson has no exit tag "
                f"(Exit: check|rule|fixture|parked(...)); a lesson that exits as prose alone repeats"
            )
    return findings


def check_provenance(entries):
    findings = []
    for key, n, body, _ in entries:
        if not VERDICT_KEY.match(key):
            continue
        text = "\n".join(body)
        if "**Provenance:**" not in text:
            findings.append(
                f"entry '{key}' (line {n}): per-repo verdict entry missing **Provenance:** "
                f"(sense version, pin SHA, scenario + date; the stale-verdict mechanization)"
            )
    return findings


def check_cost_currency(entries):
    """Schema v3: a Cost field must carry the fleet as a spawn count, not dollars alone."""
    findings = []
    for key, n, body, _ in entries:
        cost = field_text(body, "Cost")
        if not cost or FLEET.search(cost):
            continue
        findings.append(
            f"entry '{key}' (line {n}): Cost carries no fleet count (N spawns / no spawns); "
            f"dollars alone is not a cost record when the weekly reset binds (spawn_cost.py measures it)"
        )
    return findings


def check_key_contract(lines):
    """Rule 9: entries dated on/after KEY_CONTRACT_FROM use a write-point-table key."""
    findings = []
    for n, line in enumerate(lines, 1):
        m = HEADING.match(line)
        if not m:
            continue
        date, key = m.group(1), m.group(2)
        if date < KEY_CONTRACT_FROM:
            continue
        if not any(p.match(key) for p in KEY_CONTRACT):
            findings.append(
                f"entry '{key}' (line {n}): key is not in the write-point table (ledger.md); "
                f"the keys are the ledger's index and rule 7 keys off them"
            )
    return findings


def check_codenames(entries):
    findings = []
    for key, n, body, _ in entries:
        seen = set()
        for offset, line in enumerate(body):
            for m in CODENAME.finditer(line):
                name = m.group(1)
                if name in seen:
                    continue
                seen.add(name)
                rest = line[m.end():m.end() + 80]
                if "(" not in rest and "," not in rest:
                    findings.append(
                        f"entry '{key}' (line {n + offset + 1}): first mention of {name} "
                        f"does not expand on its line (codename law)"
                    )
    return findings


def _git(repo_root, *args):
    import subprocess
    r = subprocess.run(["git", "-C", str(repo_root)] + list(args),
                       capture_output=True, text=True)
    return {ln.strip() for ln in r.stdout.splitlines() if ln.strip()} if r.returncode == 0 else set()


def _touched_instruments(repo_root):
    """Measurement-chain files changed in the working tree or index vs HEAD."""
    out = _git(repo_root, "diff", "--name-only") | _git(repo_root, "diff", "--cached", "--name-only")
    return sorted(f for f in out if f in MEASUREMENT_INSTRUMENTS)


def _unverifiable_instruments(repo_root):
    """Instruments that exist on disk but are UNTRACKED, so rule 10 is blind to them.

    The dormancy hole this closes: `git diff` never lists an untracked file, so while the
    tree was untracked rule 10 fired for NOTHING - every instrument read clean, including
    the scorer whose quiet change the rule was written for. An untracked instrument has no
    baseline in git, so "did it change" is not answerable; that is UNVERIFIABLE, and it is
    reported as its own finding rather than folded into the stopper requirement. Calling it
    "changed" would demand a re-score blast radius against a HEAD that does not exist;
    calling it clean is the bug.
    """
    root = Path(repo_root)
    on_disk = [f for f in MEASUREMENT_INSTRUMENTS if (root / f).is_file()]
    if not on_disk:
        return []
    tracked = _git(repo_root, "ls-files", "--", *on_disk)
    return sorted(f for f in on_disk if f not in tracked)


def _today():
    """Injectable for tests via LEDGER_CHECK_TODAY; otherwise the real date."""
    import datetime
    import os as _os
    return _os.environ.get("LEDGER_CHECK_TODAY") or datetime.date.today().isoformat()


def check_stopper(entries, repo_root):
    """Rule 10: a measurement-instrument change needs a stopper entry carrying its blast
    radius. The stop is affordable BECAUSE the re-score is $0 - see rescore_diff.py."""
    findings = []
    blind = _unverifiable_instruments(repo_root)
    if blind:
        findings.append(
            f"rule 10 (stopper law) is BLIND to {len(blind)} instrument(s): "
            f"{', '.join(blind)} exist on disk but are UNTRACKED, and `git diff` never lists "
            f"an untracked file - so a quiet change to them cannot be detected at all. Commit "
            f"the measurement chain (or the whole tree) so rule 10 can fire; until then this "
            f"check cannot tell you the scorer is unchanged, only that it did not look changed."
        )
    touched = _touched_instruments(repo_root)
    if not touched:
        return findings
    # DATED TODAY, as rule 10 says. The date was previously not read at all, so ANY
    # stopper entry in the file's history satisfied a fresh instrument change - including
    # one written weeks earlier for an unrelated defect. Found by triggering it: an
    # admission-gate change passed the check on the strength of a stopper logged hours
    # earlier for the scorer.
    today = _today()
    todays = [(k, n, b) for k, n, b, d in entries if k.startswith("stopper/") and d == today]
    if not todays:
        findings.append(
            f"rule 10 (stopper law): {', '.join(touched)} changed, but no `stopper/<slug>` "
            f"entry dated {today} exists. A measurement instrument cannot change quietly: "
            f"STOP, re-score with bench/lib/rescore_diff.py, log the blast radius, let the "
            f"human rule.")
        return findings
    if not any(BLAST_RADIUS.search("\n".join(b)) for _, _, b in todays):
        findings.append("rule 10 (stopper law): a stopper entry exists but carries no re-score "
                        "blast radius (\"N of M runs\"); run bench/lib/rescore_diff.py and record it.")
    return findings


def main(argv):
    if len(argv) < 2:
        print("usage: ledger_check.py <vertical-key>")
        return 2
    key = argv[1]
    il_root = Path(__file__).resolve().parents[2]
    product_root = il_root.parent   # rule 10 reads git state of the Sense repo
    ledger_path = il_root / "verticals" / key / "LEDGER.md"
    results_dir = il_root / "verticals" / key / "results"

    if not ledger_path.is_file():
        print(f"ledger_check: no ledger at {ledger_path}")
        return 1

    lines = ledger_path.read_text(encoding="utf-8").splitlines()
    entries, findings = parse_entries(lines)
    findings += check_fields(entries)
    findings += check_duplicate_keys(entries)
    findings += check_coverage(results_cells(results_dir), "\n".join(lines))
    findings += check_codenames(entries)
    findings += check_lesson_exit(entries)
    findings += check_provenance(entries)
    findings += check_cost_currency(entries)
    findings += check_key_contract(lines)
    findings += check_stopper(entries, product_root)

    if findings:
        print(f"ledger_check: {len(findings)} finding(s) in {ledger_path}")
        for f in findings:
            print(f"  - {f}")
        return 1
    print(f"ledger_check: OK - {len(entries)} entries, {len(results_cells(results_dir))} results cells covered")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
