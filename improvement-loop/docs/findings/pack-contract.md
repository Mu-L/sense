# Findings pack contract - the Blocks A–J a per-repo pack must carry

**What this is.** The reusable shape of a findings pack, and nothing else. Campaign FACTS (boards,
scores, agent matrices) belong in the vertical that measured them - `verticals/<key>/findings/` - never
here. This file is structure; a template that carries values fabricates on every reuse (LEDGER
`stopper/scaffold-stamped-fabricated-results`).

**A pack is a fact pack, not a draft.** It gives the downstream writer the content and the data; the
writer owns format, structure, length, style, tone, headline and CTA. Do not write prose here. Write the
source of truth the prose is built from.

**Handoff boundary - do NOT cross it.** The writing project that renders packs into published posts is a
SEPARATE session with its own rules. A bench session NEVER writes into it: it keeps the packs organised
and hands them off (manifesto §13).

**Blocks A–J are enforced in code** by `bench/lib/findings_audit.py` (`REQUIRED_BLOCKS`), so a pack that
drops one fails the gate. Their meaning lives below.

## How to use this pack (writer-facing)

- **Numbers come ONLY from the cited snapshot.** Each per-repo file's front-matter `data:` is the **headline
  agent's** bench root (what `check-findings.sh` validates); the `agents:` block lists every agent's root +
  status (done / pending). Canonical scoreboard: `bench/lib/scoreboard.py` (regen from disk) + the cross-model matrix in the product repo's `bench/verticals/ruby-rails/results/report.md`.
  Per-run detail: `scored.json`. Repo/index stats: `sense status` on the pinned clone. Narrative color:
  `transcript.json`. If a figure is not in a snapshot, it does not go in the post; pending cells stay "pending."
- **Narrative color comes ONLY from the transcripts** (the headline agent's, unless a cross-agent point is being
  made). What the baseline grepped, what it read, what it missed, what the map resolved in one call.
- **Claims discipline (these are facts, not style - do not soften or drop them):**
  1. *Measured, not claimed.* Every number traces to the snapshot it came from.
  2. *The consumer is the AI agent, not the human.* Frame value as "the map the agent gets," never "zero
     exploration."
  3. *No universal token savings.* Sense's reliable win is correctness/completeness/precision; token cost is
     task-dependent - say so where it is a wash. **Cost is never compared across agents** (harnesses meter
     tokens differently); efficiency is a within-agent claim only.
  4. *No em-dashes* (house style). Disclose Sense plainly, but after the data has earned trust.
  5. *Report the honest boundary next to the win.* Every piece carries Block E (what the agent did well on its
     own) and Block G (where Sense does not help). The wins are credible only because the ties ship beside them.

---

## Block A - the repo

Make the reader fall in love with the project before any tooling talk. **Verified facts only - never invent a
URL or a handle** (`maintainer-checklist.md`).
- **What it is and since when.** Founding year, the problem it solved, where it sits today.
- **What it brings to the community.** Scale, adoption, what it unlocked for the stack's builders.
- **The people.** Creator(s) and core maintainers, one line each, links to their own pages (LinkedIn / X /
  GitHub) verified live.
- **The repo at a glance (stats - from `sense status` on the pinned clone).** A small table the writer can render
  as a sidebar/figure. The index is **identical across all agents** (one repo, one index; only the consuming
  agent differs), so this is per-repo, not per-agent. Pin the commit. Report: **pinned commit**; **tracked
  files** and **files in the stack's language**; **files indexed + coverage %**; **symbols**; **graph edges**; **embeddings (count
  + model)**; **language breakdown** (files / symbols per language); **profile** (e.g. `medium`, primary ruby);
  **index size on disk**; and the **contract's blast fan-out** (`sense blast <Model>`: direct / indirect /
  total affected). These numbers make "the map" concrete and size the problem.

## Block B - the scenario (the task)

The reader-explainable setup, from the scenario `description`. **Identical across all agents.**
- **The contract** (the central model under audit) and **why touching it is dangerous** (what a missed
  dependent ships).
- **The must-find set**: the size and character of the gold (e.g. "11 scattered dependents inside a 17-item
  audit"), and what makes it hard (indirect/association/concern edges, scattered dirs, a noisy token).

## Block C - agents, scoring & judging (what ran here) - PURE, current axes only

Compact, per-repo; the full method is in `Shared 3` + "The metrics, defined". Keep this clean: no retired
metrics, no journey - those go in Block H.
- **Agents × runs that exist for THIS repo.** A small status line per agent: the headline agent (done, ×N) ·
  Codex · GPT-5.5 (done ×N / pending) · OpenCode · Kimi K2.7 (done ×N / pending). Name the **headline agent**.
- **Sense adoption per agent** (the win is conditional on it): how many structural calls the sense arm fired
  (`graph`/`blast`/`search`) vs the baseline arm's 0. If an agent did not adopt Sense, say so - its tie is an
  adoption fact, not a Sense result.
- **Judge:** Sonnet 4.6, reference-aware (same for every agent). **Axes scored:** cited_recall (headline) ·
  deps-delta (discriminator) · B-score · related · grounded_precision. **Reported beside them in Block D
  (within-agent):** sense-only reach (count) · reach-at-parity · reliability/floor · efficiency (tokens / time /
  cost / turnarounds / navigation mode) - all at held-or-improved recall, never blended into the headline, never
  cross-agent.

## Block D - the outcome (the scores)

- **The one-number finding** (headline agent). The headline for this repo (deps-delta is the sharpest per-repo
  lead; overall `cited_recall` is the campaign-consistent one). State the verdict (WIN / TIE / boundary). **Lead
  the verdict with the sense-only reach count** where it is large (e.g. "11 of the 11 scattered dependents the
  baseline never reached") - the count lands harder than the fraction.
- **The headline-agent scores table** (the headline agent). cited recall b→s (Δ) · deps-delta · **sense-only
  deps (count)** · B-score · related · grounded precision · contradictions · verdict. Add the tool-use line
  (structural calls vs grep+read).
- **The reach & reliability line** (the headline agent, within-agent). Two facts the fractions hide:
  (1) **sense-only reach** - the N must-find dependents the baseline never cited in *any* run but Sense did (the
  silent breaks avoided); (2) **reliability** - each arm's per-run cited counts and spread, framed as the map
  *raising the floor* to a dependable answer (e.g. "baseline 8 then 4 → sense 14, 17"), never as "zero variance."
- **The efficiency & effectiveness sub-table** (the headline agent, **within-agent only - never cross-agent**).
  This is the "what the map costs and what it buys" table. Pull from `scored.json` → `metrics.*`, averaged across
  the arm's runs. Report only at *held or improved* recall (a saving counts only if the answer is at least as
  complete). Columns:

  | axis | baseline | sense | Δ | read |
  |---|---|---|---|---|
  | billed tokens | … | … | … | cheaper / wash / pricier (task-dependent) |
  | session time (s, median) | … | … | … | turnaround proxy; noisy, big gaps only |
  | cost ($) | … | … | … | tracks tokens |
  | turnarounds (tool calls) | … | … | … | fewer = more direct path |
  | navigation (grep / mcp / read) | 8/0/8 | 4/5/12 | - | grep-chain → structural calls |
  | grounding (anti-fab) | … | … | … | accuracy: contradictions removed |

  Then the **reach-at-parity line** (the frame that joins this table to recall): "at held-or-lower token cost
  (Δ tokens), the map reached N× the gold targets" (chatwoot: 3.3× the gold, `cited_recall` 0.97 vs 0.29, for
  −18% tokens). Where Sense costs *more* tokens here, do not bury it - say so and lead with the reach the extra
  tokens bought.
  Then **one within-agent honesty sentence**: state plainly whether Sense was cheaper, a wash, or pricier on tokens
  here, and that the reliable buy is completeness/accuracy/fewer-turnarounds, not a guaranteed token saving.
- **The "across agents" strip** (cited-recall b→s per agent; pending where not yet run):

  | agent | baseline | sense | Δ cited | verdict |
  |---|---|---|---|---|
  | the headline agent | … | … | … | … |
  | Codex · GPT-5.5 | pending | pending | - | pending |
  | OpenCode · Kimi K2.7 | pending | pending | - | pending |

  Recall only (never cost across agents). Empty cells are scheduled, not optional.

## Block E - what the agent did well on its own (the baseline, honestly)

Headline-agent narrative (from its baseline transcript). The smaller, honest note: which dependents the
baseline *did* reach, where a strong model's reading/grep is genuinely enough, where the gap narrows. This is
where the baseline is good - and where it is getting better. Honesty here is what earns the win.

## Block F - where Sense shined (what the agent did better with the map)

Headline-agent narrative (from its two transcripts). The win autopsy: **name the specific dependents the baseline
kept missing** (the `gold_recall.missed_cite` ids it never reached in any run, e.g. chatwoot's
`cache-keys-concern`, `dep:stale-notif-job`, `dep:slack-unfurl`) and *why* (the indirect/association/concern
edges with no literal token), what the map
resolved in one `sense_blast`/`sense_graph` call, and the anti-fabrication catch if this repo has one (⚑ - the
baseline asserted a wrong role, Sense stated the verified one). The named miss-list is what makes the win
concrete: "these eleven, by name, are what a grep-driven refactor would have silently broken."

## Block G - where Sense does not help (short bullets)

The limits, stated plainly: the token/time wash (do not sell a saving; within the headline agent only, never
cross-agent), the axes that tie here (hallucination/grounded-precision at the ceiling), and the boundary
conditions (small / readable / colocated / memorized) or any residual resolver gap.

## Block H - how we got there (behind the bench)

The journey *tailored to this repo*, drawn from the campaign "How we got here" pool, kept strictly out of the
results blocks. Include only what actually happened for THIS repo:
- **False starts / rollbacks / re-aims:** the scenario this repo started from and why it was changed.
- **A product enhancement that came from benching this repo,** if any (mastodon `file:` disambiguator; gitlab
  determinism cap; solidus resolver). Say plainly if this repo needed none (a clean win).
- **A scoring / judging lesson this repo taught,** if any (chatwoot is where the blind judge called a
  44%-complete audit "exhaustive," which drove retiring the blind composite).
- **The cross-agent paragraph** (this is the *only* place beyond the Block D strip the other agents get prose):
  did the win hold on Codex · GPT-5.5 and OpenCode · Kimi K2.7 where run; did each agent *adopt* Sense; and the
  open-model angle (does the map help the weaker/open model more?) where partial data exists. Mark pending cells.
- **Data-integrity notes:** re-benches, replays, variance the writer should know about.

## Block I - what the agent can safely do after the map (close)

What the agent could safely do *after* the Sense run that it could not after the baseline run - the concrete
refactor it can now make without shipping a silent break. Link the report page and the repo.

## Block J - the behavioral read (the transcript analysis)

The systematic side-by-side of *how the two arms navigated*, derived from the two `transcript.json` session
logs (the analysis pass that produces it writes a `behavioral`
artifact the writer renders). This is distinct from Blocks E/F: E/F are the narrative of *what each arm found*;
**J is the process** - the sequence of moves, the dead-ends, the turnarounds. **Storage slot, not publication
slot:** the writer usually folds it into D/E/F rather than printing it as a standalone block. Keep every claim
traceable to a transcript move (a real tool call in the log), never an impression. Cover:
- **Depth - how far each arm pushed.** Where the baseline *stopped* (cited 3 of 13, moved on) vs how far the
  sense arm got (audited the full fan-out). The recurring finding: with the map the agent **goes deeper and
  further** before it commits to an answer.
- **Reach - one call vs a grep chain.** The scattered dependents the baseline tried to assemble by grep and
  never fully reached, vs the single `sense_blast` / `sense_graph` call that returned the whole set. Name the
  specific dead-end greps (the indirect/association/concern edges with no literal token to grep for).
- **Turnarounds - iterations to an answer.** Tool-call round-trips per arm (ties to Block D `tool_calls`): the
  baseline thrashes (grep → read → grep again → sample → guess); the sense arm takes a more decisive path. Fewer
  turnarounds = fewer real-world iterations a practitioner would have lived through.
- **Re-derivation - rebuilding structure from scratch.** Whether the baseline spent the session reconstructing
  relationships it cannot see (the missing-map tax), and how the index removes that work.
- **Accuracy in motion - the fabrication moment.** If this repo has an anti-fab ⚑, point to the exact transcript
  turn where the baseline asserted a wrong relation (the confident-but-wrong claim) vs where the sense arm
  verified the real role. The behavioral root of the grounding number.
- **Real-world translation (the one-line takeaway).** What the process difference means off the bench: deeper +
  fewer turnarounds + grounded = a more accurate session and fewer round-trips before a maintainer can act. State
  it plainly, anchored to this repo's transcript, not as a slogan.
