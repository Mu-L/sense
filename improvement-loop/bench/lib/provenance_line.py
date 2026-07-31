#!/usr/bin/env python3
"""provenance_line.py -- project a run's run_meta.json into a LEDGER Provenance line.

Schema v2's Provenance field (ledger.md; ledger_check.py rule 7) is REQUIRED on
Loop 3 verdict entries (loop3/<repo>/probe|run-<n>|swap|close): sense version, sense
BUILD key, pinned repo SHA, scenario file + date. This generates that field from the
run's own run_meta.json, so it is a PROJECTION of the on-disk record, not hand-authored
-- and "has anything changed since this verdict" is answerable by diffing run_meta.

The build key (added 2026-07-30) is the half that actually answers it. Version + ref +
dirty-flag cannot: two DIFFERENT dirty trees on one commit produce identical provenance,
and that is precisely a Loop 7 spike-rebuild-rerun. See sense_build.py.

  python3 provenance_line.py <path/to/run_meta.json>

Prints a ledger-ready line, e.g.:
  - **Provenance:** sense 1.13.3-dev+gf220d58 @f220d58 [build 4d871f3a31e1]
    (release v1.13.2, dirty tree), repo dolt@7e268bf, scenario dolt.yaml
    @sha256:ba855c96c09ff4a3 (2026-07-16)

$0, read-only. Write-only law holds: this EMITS a line for a human to paste; no loop
reads run_meta to decide anything.
"""
import json
import os
import sys


def provenance_line(meta):
    ver = meta.get("tool_version") or "sense (unknown version)"
    ref = meta.get("sense_ref")
    sense = ver + (f" @{ref}" if ref else "")
    # The BUILD key (sha256 of the binary) is what makes "has anything changed since this
    # verdict" answerable. Version + ref + dirty cannot: two different dirty trees on one
    # commit read identically, and that is exactly a Loop 7 spike-rebuild-rerun. Runs
    # stamped before the key existed have none, and print without it rather than lying.
    if meta.get("sense_build_key"):
        sense += f" [build {meta['sense_build_key']}]"
    quals = []
    if meta.get("sense_release"):
        quals.append(f"release {meta['sense_release']}")
    if meta.get("sense_dirty"):
        quals.append("dirty tree")
    if quals:
        sense += f" ({', '.join(quals)})"

    repo = f"{meta.get('repo', '?')}@{meta.get('repo_commit') or '?'}"

    scen_file = meta.get("scenario_file")
    scen = os.path.basename(scen_file) if scen_file else (meta.get("scenario") or "?")
    if meta.get("scenario_version"):
        scen += f" @{meta['scenario_version']}"

    date = (meta.get("timestamp") or "").split("T")[0] or "?"

    return f"- **Provenance:** {sense}, repo {repo}, scenario {scen} ({date})"


def main(argv):
    if len(argv) != 2:
        sys.exit("usage: provenance_line.py <path/to/run_meta.json>")
    with open(argv[1]) as f:
        meta = json.load(f)
    print(provenance_line(meta))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
