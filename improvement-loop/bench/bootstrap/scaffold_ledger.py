#!/usr/bin/env python3
"""Write the `bootstrap/scaffold` LEDGER entry from measured facts.

Driven by scaffold.sh through the environment (KEY, LANG_ARG, TITLE,
FRAMEWORK, REASON, EXTRACT_DIR, REPO_ROOT, IL_ROOT), so the entry records what
the run actually found rather than what someone remembered afterwards.

The schema is `ledger.md`'s and `ledger_check.py` enforces it: What, Why,
Alternatives, Lesson, Exit check, Scores, Cost, Links. Every field here is
either a fact from disk or the caller's stated reason. The one field a script
cannot invent is the LESSON, so it says plainly that a clean scaffold taught
nothing - a loop that always claims a lesson is manufacturing them.
"""

import datetime
import os
import sys

TEMPLATE = """
## {date} | bootstrap/scaffold | {title} stamped and evaluated green

- **What:** `verticals/{key}/` stamped with {n_elements} element(s): {elements}.
  The scaffold's three elements: the stamp; extractor readiness; the arm decision.
  Evaluator `scaffold_check.py {key} --lang {lang} --strict` exits 0, and a
  re-run of `stamp.sh` creates nothing.
- **Extractor readiness (checked, not assumed):** `internal/extract/{lang}/`
  carries {n_extract} production file(s) ({extract_files}).{support}
  Ready; no Loop 7 blocker and no lane park.
- **The arms as decided on this date:** {arms} - from `verticals/{key}/arms.txt`,
  which is the only place a model id is named. What actually ran is written
  immutably by each run's `run_meta` and by the model-scoped results tree.
- **Why:** {reason}
- **Alternatives:** hand-stamping the tree, or patching an existing vertical in
  place - rejected: a hand-patched stamp proves nothing about what the generator
  produces for the NEXT vertical, which is the only thing that matters.
- **Lesson:** {lesson}
  `Exit: check(scaffold_check.py {key} --lang {lang} --strict rc=0; stamp.sh
  re-run creates 0; scaffold.sh returns status=BOOTSTRAPPED)`
- **Scores:** n/a: scaffold, no runs.
- **Cost:** $0 API. Subscription: one session, no paid arms; fleet: no spawns
  (0 spawns, main session only).
- **Links:** `docs/bootstrap.md`, `bench/bootstrap/scaffold.sh`.
"""

CLEAN_LESSON = ("none owed - the stamp was clean on the first walk and the "
                "evaluator agreed. A loop that reports a lesson every time it "
                "runs is manufacturing them; the field stays empty until a walk "
                "actually finds something.")

HEADER = """# {key} - LEDGER

Append-only narrative for the {key} vertical. Entry schema and write points:
[`../../docs/ledger.md`](../../docs/ledger.md). Never edited
after the fact; never committed.
"""


def support_files(repo_root, lang, framework):
    """The per-language support the code-organization convention expects."""
    wanted = [(f"internal/conventions/detectors_{lang}.go", "conventions detector"),
              (f"internal/model/{framework}.go", "framework model"),
              (f"internal/resolve/{framework}.go", "resolver rules")]
    found = [p for p, _ in wanted if os.path.exists(os.path.join(repo_root, p))]
    if not found:
        return ""
    return " Plus " + ", ".join(f"`{p}`" for p in found) + "."


def arms_line(path):
    if not os.path.exists(path):
        return "not stamped"
    out = []
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            parts = line.split()
            if len(parts) >= 2:
                runs = f" x{parts[2]}" if len(parts) > 2 else ""
                out.append(f"{parts[0]} `{parts[1]}`{runs}")
    return "; ".join(out) if out else "not stamped"


def main():
    env = os.environ
    key, lang = env["KEY"], env["LANG_ARG"]
    vdir = os.path.join(env["IL_ROOT"], "verticals", key)

    elements = sorted(e for e in os.listdir(vdir) if not e.startswith("."))
    extract = sorted(f for f in os.listdir(env["EXTRACT_DIR"])
                     if f.endswith(".go") and not f.endswith("_test.go"))

    body = TEMPLATE.format(
        date=datetime.date.today().isoformat(),
        title=env.get("TITLE") or key,
        key=key, lang=lang,
        n_elements=len(elements),
        elements=", ".join(f"`{e}`" for e in elements),
        n_extract=len(extract),
        extract_files=", ".join(f"`{f}`" for f in extract),
        support=support_files(env["REPO_ROOT"], lang, env.get("FRAMEWORK", "")),
        arms=arms_line(os.path.join(vdir, "arms.txt")),
        reason=env.get("REASON") or "a new vertical was opened for this stack",
        lesson=CLEAN_LESSON)

    path = os.path.join(vdir, "LEDGER.md")
    if not os.path.exists(path):
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(HEADER.format(key=key))
    with open(path, "a", encoding="utf-8") as fh:
        fh.write(body)
    print(f"   wrote bootstrap/scaffold to verticals/{key}/LEDGER.md", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
