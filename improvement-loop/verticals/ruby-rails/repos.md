# Ruby on Rails Vertical - Repo Selection (Step 0)

The repo-selection deliverable (manifesto §1 + §7): the one
manual judgment gate the bootstrap does NOT automate. Method:
[`../../docs/scenarios/sourcing-runbook.md`](../../docs/scenarios/sourcing-runbook.md).

> **The SET is 4 repos, firm** (manifesto §7.0): `1 framework + 1 big + 2 medium`
> (or `2 big + 2 medium` when the framework is too small/memorized). Each slot carries a
> same-type backup; a swap is the LAST resort.

## The firm 4-repo set

| Slot | Repo | Central target |
|---|---|---|
| framework | | |
| big | | |
| medium 1 | | |
| medium 2 | | |

The outcome of a repo is the loop's to determine, not this file's: no repo is labelled a win,
a pillar, or win-eligible before a real test has run (`decision-errors.md` - the WIN-VIABLE
label that eight repos carried into Loop 3 before dying there).

## Freeze plan (at clone time)

`PINNED_COMMITS.json` (this folder): for each repo `git ls-remote <url> HEAD` -> pin the
SHA, then `bash bench/drivers/provision-repos.sh` clones both arms and strips any
anti-LLM banner from BOTH (fairness, manifesto §3).
