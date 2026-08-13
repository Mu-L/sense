#!/usr/bin/env python3
"""The scaffold evaluator - mechanical validation of a stamped vertical.

    scaffold_check.py <key> --lang <extract-dir> [--prev <key>] [--stale tok1,tok2]

Four file-level checks (the scaffold's un-fakeable list,
docs/bootstrap.md); nothing here can
be gamed by prose:

  structure   verticals/<key>/ has every stamped element: the tracker, the slate,
              findings/, scenarios/, repos.txt                          [FAIL]
  stale-refs  previous-stack tokens (--stale) in the vertical's own stamped docs,
              excluding results/ and the loop-written LEDGER/STATUS. At STAMP time
              a hit = not-yet-retargeted content and should be ~zero (--strict makes
              hits fail); on a finished vertical residual mentions are legitimate
              comparisons, so hits print as a WARN worklist                [WARN]
  extractor   internal/extract/<lang>/ exists and is non-trivial

Exit 0 = bootstrapped; 1 = findings printed as a worklist. Known-answer control:
run it against a stamped vertical (structure/extractor PASS) and against an
unstamped key (structure FAILs).
"""

import argparse
import os
import re
import sys

# The loop's own root (docs/, bench/) and the Sense product repo it benches are
# two different trees since the improvement-loop became self-contained.
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
PRODUCT_REPO = os.path.dirname(REPO)

# Only what is genuinely per-vertical. The stack-agnostic method docs live once in
# docs/ and are never copied here, so they are not stamp elements.
DOC_ELEMENTS = ["README.md", "repos.md", "findings"]
BENCH_ELEMENTS = ["scenarios", "repos.txt", "PINNED_COMMITS.json", "arms.txt",
                  "answer-forms.md"]


def find_doc_dir(key):
    d = os.path.join(REPO, "verticals", key)
    return d if os.path.isdir(d) else None


def check_structure(key, findings):
    doc = find_doc_dir(key)
    if not doc:
        findings.append(f"structure: no verticals/{key}/ directory")
        return
    for el in DOC_ELEMENTS:
        if not os.path.exists(os.path.join(doc, el)):
            findings.append(f"structure: missing {os.path.relpath(os.path.join(doc, el), REPO)}")
    for el in BENCH_ELEMENTS:
        if not os.path.exists(os.path.join(doc, el)):
            findings.append(f"structure: missing verticals/{key}/{el}")


# Loop-written files are not stamp output; a previous stack named in the LEDGER is
# history, not a retarget miss.
LOOP_WRITTEN = {"LEDGER.md", "STATUS.md"}


def check_stale_refs(key, stale, warns):
    doc = find_doc_dir(key)
    if not doc:
        return  # structure check already reports it
    pats = [(t, re.compile(r"\b" + re.escape(t) + r"\b", re.IGNORECASE))
            for t in stale if t]
    for root, _dirs, files in os.walk(doc):
        if "results" in os.path.relpath(root, doc).split(os.sep):
            continue
        for fn in sorted(files):
            if not fn.endswith(".md") or fn in LOOP_WRITTEN:
                continue
            path = os.path.join(root, fn)
            rel = os.path.relpath(path, doc)
            text = open(path, encoding="utf-8", errors="replace").read()
            for tok, rx in pats:
                n = len(rx.findall(text))
                if n:
                    warns.append(f"stale-refs: {rel}: {n}× '{tok}'")


# A path-shaped token in a stamped file is a promise: follow it and something is there.
# Placeholders (<key>, NN-slug), URLs, the private results tree, and the deliberately
# external .doc/ area are not promises and are skipped.
_PLACEHOLDER = set("<>*{}$")
_REF_EXT = (".md", ".py", ".sh", ".json", ".yaml", ".yml", ".go", ".txt")


def _ref_targets(text):
    """Path-shaped tokens: markdown link targets + backticked paths. Backticks matter as
    much as links - a citation in prose is the same promise, and a link-only checker is
    blind to it (46 stale backticked paths hid behind a green link sweep, 2026-07-29)."""
    for m in re.finditer(r"\[[^\]]*\]\(([^)\s]+)\)", text):
        yield m.group(1).split("#")[0]
    for m in re.finditer(r"`([^`\s]+)`", text):
        yield m.group(1)


def _skip_ref(ref):
    if not ref or ref.startswith(("http", "mailto:", ".doc/", "#")):
        return True
    if any(c in ref for c in _PLACEHOLDER) or "NN-" in ref:
        return True
    if "/" not in ref:
        return True          # a bare filename in prose names a file, it does not
                             # promise a location; only a PATH is a promise
    if not ref.endswith(_REF_EXT) and not ref.endswith("/"):
        return True          # e.g. `--flag`, `some:thing`
    return ref.split("/")[0] == "results" or "/results/" in ref


def check_dangling_refs(key, findings):
    """Every path a stamped file cites must resolve - from the file, from the loop
    root, or from the product repo. Catches a scaffold that points at a script or a
    doc living somewhere else (or nowhere): the failure mode a file-existence check
    is blind to, because the citing file itself is perfectly well-formed."""
    doc = find_doc_dir(key)
    if not doc:
        return  # structure check already reports it
    for root, _dirs, files in os.walk(doc):
        if "results" in os.path.relpath(root, doc).split(os.sep):
            continue
        for fn in sorted(files):
            # LEDGER.md is append-only history: an entry may cite a path that was
            # true when it was written. STATUS.md is regenerated, so a dangling ref
            # there is live and must fail.
            if not fn.endswith((".md", ".json")) or fn == "LEDGER.md":
                continue
            path = os.path.join(root, fn)
            text = open(path, encoding="utf-8", errors="replace").read()
            seen = set()
            for ref in _ref_targets(text):
                if _skip_ref(ref) or ref in seen:
                    continue
                seen.add(ref)
                # `bench/...` means THIS loop's bench. The product repo has a bench/
                # too (the legacy vertical loop + the global bench), and resolving
                # against it would bless exactly the leak this check exists to catch.
                bases = (root, REPO) if ref.startswith("bench/") else (root, REPO, PRODUCT_REPO)
                if not any(os.path.exists(os.path.join(b, ref)) for b in bases):
                    rel = os.path.relpath(path, doc)
                    findings.append(f"dangling-ref: {rel} cites `{ref}` - resolves nowhere")


def check_extractor(lang, findings):
    d = os.path.join(PRODUCT_REPO, "internal", "extract", lang)
    if not os.path.isdir(d):
        findings.append(f"extractor: internal/extract/{lang}/ does not exist")
        return
    go_files = [f for f in os.listdir(d) if f.endswith(".go") and not f.endswith("_test.go")]
    if not go_files:
        findings.append(f"extractor: internal/extract/{lang}/ has no production files")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("key", help="vertical key, e.g. go, python-django")
    ap.add_argument("--lang", required=True, help="internal/extract/<lang> dir name")
    ap.add_argument("--prev", default=None, help="previous vertical key (informational)")
    ap.add_argument("--stale", default="", help="comma-separated stale stack tokens to grep the stamped docs for")
    ap.add_argument("--strict", action="store_true",
                    help="stamp-time mode: stale-ref hits fail (the stamp should be retargeted)")
    args = ap.parse_args()

    findings, warns = [], []
    check_structure(args.key, findings)
    check_dangling_refs(args.key, findings)
    check_stale_refs(args.key, args.stale.split(","), warns)
    check_extractor(args.lang, findings)

    print(f"# bootstrap check - {args.key} (lang={args.lang})")
    for f in findings:
        print("FAIL:", f)
    for w in warns:
        print("WARN:" if not args.strict else "FAIL:", w)
    bad = len(findings) + (len(warns) if args.strict else 0)
    if bad:
        print(f"NOT BOOTSTRAPPED - {bad} finding(s)"
              + (f", {len(warns)} stale-ref warn(s) to review"
                 if warns and not args.strict else ""))
        return 1
    if warns:
        print(f"BOOTSTRAPPED with {len(warns)} stale-ref warn(s) - review before the next stamp")
        return 0
    print("BOOTSTRAPPED - structure, dangling-refs, stale-refs, extractor all clean")
    return 0


if __name__ == "__main__":
    sys.exit(main())
