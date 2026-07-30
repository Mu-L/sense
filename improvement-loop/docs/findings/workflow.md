# Findings workflow - how a vertical's fact packs are built and maintained

The reference for **creating and maintaining** the per-repo fact packs in `verticals/<key>/findings/`.
This loop does not write articles: it produces the findings, and a separate writing project renders
them into prose. Two motions drive it - building the packs, and keeping them accurate as results move.

## The golden rule

**The findings set is a projection of the bench data. Every number traces to a snapshot; nothing is authored.**
If a figure is not in a snapshot, it does not go in a pack. Prose framing is the downstream writer's job
(`social-writing`); these files are the *fact pack* the writer renders. Create and maintain BOTH start from
data, never from memory or from another article.

## Anatomy of the set (what exists)

Location: `improvement-loop/verticals/<key>/findings/` - written by Loop 6 from real results, never stamped
- **`00-scorecard.md`** - the aggregate data pack (the board, per-repo one-number index, the pattern,
  the boundaries, the thesis answer). The synthesis essay's source of truth.
- **`NN-slug.md`** (`01`…`13`) - one pack per repo, in publication order, each holding **Blocks A–J**:
  A repo + stats · B scenario · C agents/scoring/judging · D outcome + scores (recall + efficiency/effectiveness
  + reach/reliability) + across-agents strip · E what the agent did well on its own · F where Sense shined ·
  G where Sense does not help · H how we got there (the journey) · I close · J the behavioral read (transcript
  analysis: how each arm navigated).
- **`_skeleton.md`** - the shared blocks (problem / Sense / method), the campaign facts (agents, scoreboard,
  axis definitions, lessons, journey pool), and the Block A–J spec. Read it first.
- **`../README.md`** - the **board / manifest** (status legend, the publication table, what's left). The
  human-readable source of truth for "what exists and where it goes."
- **`maintainer-checklist.md`** - handles/links to verify LIVE before publish. `media/` - figures.

### The agent matrix (the core mental model)

An **agent** = (harness · model). Which agents a campaign benches is declared in `verticals/<key>/arms.txt`; the headline arm owns the per-repo narrative. (the **headline
agent** - owns the per-repo prose and the board), **Codex · GPT-5.5**, **OpenCode · Kimi K2.7 (Kimi for Coding)**. An **arm** = baseline | sense, *within* one agent. The comparison is ALWAYS baseline-vs-sense inside
one agent; **recall is comparable across agents, cost is not.** The headline agent owns Blocks A–J; the other
agents appear only in the Block D "across agents" strip + one Block H paragraph (pending until their runs land).

## Data sources (what → where)

| you need | source |
|---|---|
| the canonical board (cited_recall, B-score, related, grounded, verdicts) | regenerate from disk with `python3 bench/lib/scoreboard.py`; cross-model matrix in the product repo's `bench/verticals/ruby-rails/results/report.md` (the top-level `SENSE-SCORING-REPORT.md` was retired) |
| objective per-run recall (overall + dependents group) | `verticals/ruby-rails/results/<agent>/<arm>/<repo>/run-*/scored.json` → `gold_recall` |
| per-repo + group deltas | `RESULTS_DIR=verticals/ruby-rails/results/<agent> python3 bench/lib/pergroup.py <repo>` |
| repo + index stats (Block A) | `sense status` in the pinned clone: `/Users/luc/Developer/luuuc/oss/sense-benchmark/sense/<repo>` |
| contract blast fan-out (Block A) | `sense blast <Contract> [--file <path>] --json` in the clone (use `--file` for ambiguous names) |
| Sense adoption (Block C) | `scored.json` → `mcp_count` / `grep_count` / `read_count` (gems nest these under `metrics.`) |
| efficiency / reach / reliability (Block D, within-agent) | `python3 bench/lib/efficiency.py <repo>` - one reproducible block: billed tokens / wall-median / cost / tool_calls / nav + sense-only reach + per-run cited counts + baseline never-cited miss-list (raw fields: `scored.json` → `metrics.*` + `gold_recall.details`) |
| sense-only reach (campaign board column) | `scoreboard.py` `sense-only` column |
| the narrative (E / F / H) | the two `transcript.json` files (what baseline grepped/missed, what blast resolved) |
| the behavioral read (Block J) | the two `transcript.json` files - tool-call timeline per arm (depth / reach / turnarounds / re-derivation / fabrication moment); see prompt 08 Step 3b |
| per-repo journey + verdict history | `repos.md` (the investigation tracker) |

## The frontmatter contract (what `check-findings.sh` enforces)

A pack with a `headline:` block `{repo, deps_delta, overall_from, overall_to}` + a `data:` model-root is
**validated**: `check-findings.sh` recomputes those three numbers from live `scored.json` (tol 0.01) and prints
FRESH / OUTDATED. Packs **without** a `headline:` block (`00`, and any OPEN/contested pack like `09-redmine`,
and small-gem packs scored on cited not deps) are intentionally skipped. Every pack also carries informational
`axes:` (canonical Sonnet numbers), `stats:` (from `sense status`), and `agents:` (the agent matrix + status).
**Never hand-edit a `headline:` number** - it must match live data or the check goes red.

## The blast radius of any result change (keep these in sync)

When a repo's result changes (re-bench, re-judge, re-aim, re-scan), these all may need updating:
1. the repo's `NN-slug.md` pack (Blocks C/D/H + frontmatter axes/stats),
2. `00-scorecard.md` (the board + the per-repo index row),
3. `README.md` (this folder) (the board table + verdict),
6. `repos.md` (the verdict tracker + reconciliation note),
8. the canonical board (**regenerate it** - `bench/lib/scoreboard.py` from disk + the cross-model
   the product repo's `bench/verticals/ruby-rails/results/report.md`; the retired top-level `SENSE-SCORING-REPORT.md` is the step that,
   when skipped, created the redmine discrepancy).

## Invariants (must always hold)

- `bash bench/drivers/check-findings.sh` → all headline-block packs **FRESH** + the structural audit at **0 FAIL**.
- Every repo has exactly one pack; the README board rows == the article files; `00`'s index points to real files.
- Each pack has Blocks A–J (gems may be terser but keep the block set); frontmatter parses.
- Claims discipline (from `_skeleton.md`): measured-not-claimed, AI-is-the-consumer, no universal token savings,
  **no em-dashes in body prose**, cost never compared across agents, the boundary reported beside the win.
- **Results vs journey are separated:** Blocks C/D carry only current axes; dropped metrics + the path + product
  fixes live in Block H. Never mix retired metrics into a scores table.
- **deps-delta** (discriminator group) is the sharpest per-repo number; **overall cited_recall** is the
  campaign-consistent headline. Both are real; don't conflate them.

## Known drift patterns (what the audit watches for)

- **Canonical-vs-on-disk mismatch** (the redmine case): the canonical board (now `scoreboard.py` / `report.md`,
  formerly the retired `SENSE-SCORING-REPORT.md`) lists an old verdict while the on-disk `scored.json` reflects a
  re-aim/re-bench the board never regenerated. Fix: regenerate the board from disk (`python3 bench/lib/scoreboard.py`).
- **Count drift:** board rows / `00` index / repo count disagree (e.g. duplicate rows in `repos.md`, "12
  codebases" when there are 13).
- **Orphan references:** a doc points to a removed file or folder (`appendix/`, `09-essay.md`).
- **Unhardened headline:** a number quoted from a single run or ×2; mark the pack OPEN (no `headline:` block)
  until ×3.

## Accuracy model

"Accurate" = (1) every number traces to its cited snapshot and is current, (2) every block follows the spec,
(3) every cross-doc reference resolves, (4) no invented facts/handles, (5) the claims discipline holds. This is
enforced by one command, **`bash bench/drivers/check-findings.sh`**, which runs both layers and fails on any FAIL:
- **headline freshness** (`bench/lib/check_findings_stats.py`) - recomputes each teardown's headline numbers from
  live `scored.json`; prints FRESH / OUTDATED.
- **structural + referential audit** (`bench/lib/findings_audit.py`) - coverage (every benched repo has exactly
  one pack), Block A–J structure, required frontmatter keys, broken local links, README board sync, and an
  em-dash WARN on body prose. FAIL findings fail the gate; em-dashes are a WARN (house-style debt, not a block).
  Guarded by `bench/lib/test_findings_audit.py` (incl. a regression test that the live set has 0 FAIL).
