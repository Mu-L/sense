#!/usr/bin/env bash
# stamp.sh - stamp the directory structure for a new stack vertical
# (docs/bootstrap.md). It scaffolds the MECHANICAL skeleton; it deliberately
# does NOT choose repos, pin commits, or author scenarios - those are the human
# judgment gates: it never chooses repos, pins commits, or authors scenarios.
#
# It stamps ONE home: improvement-loop/verticals/<key>/ - the whole vertical in one
# folder, and ONLY what is genuinely about this stack: repos.txt + scenarios/ + results/
# (results stay private), arms.txt (the tools/LLMs, copied from docs/arms.default.txt),
# a README tracker, a repos.md slate, and an EMPTY findings/ -
# the packs are written by Loop 6 from real results, never stamped from a template:
# a stamped pack would carry another campaign's numbers into a vertical with no runs. LEDGER.md is written by the loop at its first transition and stays
# private. The stack-agnostic method docs (scenario crafting, findings workflow, the
# findings skeleton) live ONCE in docs/ and are never copied per vertical.
#
# Idempotent + non-destructive: every existing file is SKIPPED, never overwritten, so it
# is safe to re-run and safe against a partially-stamped vertical.
#
# Usage:
#   bash bench/bootstrap/stamp.sh <key> [--title "Display Name"]
#   bash bench/bootstrap/stamp.sh laravel --title "PHP / Laravel"
#   bash bench/bootstrap/stamp.sh laravel --no-doc        # bench dirs only
set -uo pipefail
cd "$(dirname "$0")/../.."                      # improvement-loop root
ROOT="$(pwd)"

KEY=""; TITLE=""; NODOC=0
while [ $# -gt 0 ]; do
  case "$1" in
    --title)   TITLE="$2"; shift 2 ;;
    --no-doc)  NODOC=1; shift ;;
    -h|--help) sed -n '2,24p' "$0"; exit 0 ;;
    -*) echo "unknown flag: $1" >&2; exit 64 ;;
    *)  KEY="$1"; shift ;;
  esac
done
[ -z "$KEY" ] && { echo "usage: stamp.sh <key> [--title T] [--no-doc]" >&2; exit 64; }
[ -z "$TITLE" ] && TITLE="$KEY"

made=0; skipped=0
mk()   { if [ -e "$1" ]; then echo "  [skip] ${1#$ROOT/}"; skipped=$((skipped+1)); else mkdir -p "$1"; echo "  [mkdir] ${1#$ROOT/}"; made=$((made+1)); fi; }
cpv()  { # src dst  - copy only if dst missing
  if [ -e "$2" ]; then echo "  [skip] ${2#$ROOT/}"; skipped=$((skipped+1)); return; fi
  [ -e "$1" ] || { echo "  [warn] template missing: ${1#$ROOT/}"; return; }
  cp "$1" "$2"; echo "  [copy] ${2#$ROOT/}  <- ${1#$ROOT/}"; made=$((made+1));
}
writef() { # path heredoc-content (stdin)  - write only if missing
  if [ -e "$1" ]; then echo "  [skip] ${1#$ROOT/}"; skipped=$((skipped+1)); cat >/dev/null; return; fi
  mkdir -p "$(dirname "$1")"; cat > "$1"; echo "  [write] ${1#$ROOT/}"; made=$((made+1));
}

echo "== stamping vertical '$KEY' (title: $TITLE) =="

VDIR="$ROOT/verticals/$KEY"
echo "-- $VDIR --"

# ---- 1. membership + data ---------------------------------------------------
mk "$VDIR/scenarios"
writef "$VDIR/repos.txt" <<EOF
# $TITLE vertical - the repo list. One repo key per line.
EOF

# the arms: copied from the editable default so a model id is named in ONE place
cpv "$ROOT/docs/arms.default.txt" "$VDIR/arms.txt"

# EACH STACK ANSWERS DIFFERENTLY, AND THE PLAN CANNOT SAY HOW. 01-author.md reads this
# file before choosing a kind of question. Stamped EMPTY on purpose: a form earns its way
# onto the page by being measured in THIS stack, and a page seeded with another vertical's
# forms would be the digest problem again - a summary that reads as the complete set.
# Measured 2026-08-12: php-laravel spent 36 attempts across 3 repos with the plain arm's
# floor binding in every one, because "every place that calls or holds X" is one regex in
# PHP, where the same form banks +0.775 in Ruby. Same plan, same laws, opposite outcome.
writef "$VDIR/answer-forms.md" <<EOF
# $TITLE - answer forms

What the ANSWER to a scored step is allowed to be, in THIS stack. Read by
\`plans/cycle-1-craft-the-scenario/01-author.md\` before it chooses a kind of question.

**Empty is the correct state for a new vertical.** Every line added here carries the
measurement and the n that earned it; a form with no number under it does not belong on
this page. Nothing here gates a draft - it ORDERS what to try first, and it records what
has already been paid for so a later cycle does not buy it twice.

## Forms measured to WIN here

_nothing measured yet._

## Forms measured to FAIL here

_nothing measured yet._ A form recorded here needs the run that killed it, with its
numbers. A draft may still use it, but the yaml header owes the sentence saying what is
different this time.

## Mechanisms killed by a run - do not re-propose

_nothing measured yet._

## Stack facts that bound what can be asked

_nothing measured yet._ Examples of what belongs here once measured: whether a retention
ring exists at all (\`ring_sweep.py\`), which framework edges the resolver actually joins,
and what a single search returns for the idioms this stack writes.
EOF

writef "$VDIR/PINNED_COMMITS.json" <<EOF
{
  "_meta": {
    "vertical": "$KEY",
    "note": "Repo pins for this vertical. Top-level keys are repo keys; _meta is metadata, not a repo. Fill after the repo-selection gate: {\"url\": ..., \"sha\": ...} per repo."
  }
}
EOF

# ---- 2. docs ------------------------------------------------------------------
if [ "$NODOC" = 1 ]; then
  echo "-- skipping docs scaffold (--no-doc) --"
else
  DOCDIR="$VDIR"
  mk "$DOCDIR/findings"
  # tracker + repo-selection deliverable: small stubs pointing at the authorities
  writef "$DOCDIR/README.md" <<EOF
# $TITLE Vertical - Tracker

Vertical scaffolded by \`bench/bootstrap/stamp.sh\`.

> **Authorities** (this folder never overrides them):
> [\`../../docs/manifesto.md\`](../../docs/manifesto.md) (rules),
> [\`../../docs/vertical-program.md\`](../../docs/vertical-program.md) (sequence),
> [\`../../docs/bootstrap.md\`](../../docs/bootstrap.md) (the bootstrap).

## Status

| Step | Artifact | State |
|---|---|---|
| 0 - Choose repos (4, firm) | [\`repos.md\`](repos.md) | ⬜ |
| 1 - Stamp dirs | this folder (\`verticals/$KEY/\`) | ✅ |
| 2 - Pin commits | \`PINNED_COMMITS.json\` (this folder) | ⬜ |
| 3 - Build indexes | \`bench/lib/ensure-index.sh <repo>\` | ⬜ |
| 4 - Per-repo loop | \`bench/drivers/vertical-loop.sh <repo>\` | ⬜ |

The per-repo mechanical loop is driven by \`vertical-loop.sh\`; it stops at the two
human gates (scenario authoring, tie diagnosis).
EOF
  writef "$DOCDIR/repos.md" <<EOF
# $TITLE Vertical - Repo Selection (Step 0)

The repo-selection deliverable (manifesto §1 + §7): the one
manual judgment gate the bootstrap does NOT automate. Method:
[\`../../docs/scenarios/sourcing-runbook.md\`](../../docs/scenarios/sourcing-runbook.md).

> **The SET is 4 repos, firm** (manifesto §7.0): \`1 framework + 1 big + 2 medium\`
> (or \`2 big + 2 medium\` when the framework is too small/memorized). Each slot carries a
> same-type backup; a swap is the LAST resort.

## The firm 4-repo set

| Slot | Repo | Central target |
|---|---|---|
| framework | | |
| big | | |
| medium 1 | | |
| medium 2 | | |

The outcome of a repo is the loop's to determine, not this file's: no repo is labelled a win,
a pillar, or win-eligible before a real test has run (\`decision-errors.md\` - the WIN-VIABLE
label that eight repos carried into Loop 3 before dying there).

## Freeze plan (at clone time)

\`PINNED_COMMITS.json\` (this folder): for each repo \`git ls-remote <url> HEAD\` -> pin the
SHA, then \`bash bench/bootstrap/provision.sh\` clones both arms and strips any
anti-LLM banner from BOTH (fairness, manifesto §3).
EOF
fi

echo ""
echo "== done: $made created, $skipped already present =="
echo "Next:"
echo "  2. choose the 4 repos + contracts -> verticals/$KEY/repos.md, fill verticals/$KEY/repos.txt"
echo "  3. pin commits in verticals/$KEY/PINNED_COMMITS.json, then bash bench/bootstrap/provision.sh"
echo "  4. per repo:  bash bench/drivers/vertical-loop.sh <repo>   (VERTICAL=$KEY)"
