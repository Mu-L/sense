---
slug: adoption-is-earned-per-harness
role: "cross-cutting fact pack - Sense rides MCP into any agent, but each harness has to be wired to call it"
status: seeded (Rails)
data: the product repo's bench/verticals/ruby-rails/results/{claude-opus-4-8,gpt-5.5,kimi-for-coding_k2p7}/ + setup memories
byline: "Luc B. Perussault-Diallo"
rigor: Claude Code hardened; Codex + OpenCode breadth (one harness anecdote each so far)
---
# Adoption is earned per harness

**The finding.** Sense rides MCP, so the *same local index* serves any agent. But the agent only benefits if it
**calls the tools**, and that adoption is **earned per harness**, not given by the protocol. Wiring the harness
correctly is the difference between a sense arm that wins and one that silently equals its baseline.

## The three harnesses (Rails)

- **Claude Code (Opus 4.8) - native adoption.** Calls Sense out of the box once indexed; the headline arm.
- **Codex (GPT-5.5) - adoption had to be wired.** MCP `serverInstructions` alone steered it **0 times**. Adding
  an `AGENTS.md` (via `sense setup --tools codex-cli`) plus `-c` registration took it to **13 MCP calls** and an
  ideal layered pattern (most of the answer assembled structurally via Sense). The lesson: a load-bearing
  `AGENTS.md`, not just MCP metadata.
- **OpenCode (Kimi / Qwen) - works, with a slow cold start.** The early "hang" was a premature kill; OpenCode's
  MCP just has a slow cold start. Once warm it adopts and benches cleanly.

## The rule

State **adoption before delta**, every time: a sense arm is only a sense arm if `mcp_count > 0`. If a harness
does not call the tools, its sense arm ≈ its baseline arm and the comparison is about wiring, not about the map.
The product surface that earns adoption is `sense setup` (it must emit the right per-harness config:
`AGENTS.md` / `.codex/config.toml` for Codex, MCP registration for the others).

## Data points by vertical

| vertical | when | the harness data point | agrees / refines / reverses |
|---|---|---|---|
| Ruby/Rails | 2026-06 | Claude Code native; Codex needed `AGENTS.md` (0 → 13 calls); OpenCode slow cold start, then clean. `sense setup` auto-detects 4 clients (Claude Code, Cursor, Codex CLI, OpenCode). | seed |
| Python/Django | TBD | does any harness need different wiring for a different stack's project conventions? | … |
| Go | 2026-07 | **Provenance is per-harness, and three of four harnesses recorded none.** `opencode-run.sh` wrote no `sense_*` fields at all and `codex-run.sh` wrote `sense_release: null`, so no cross-agent sense arm could be release-verified and `select_final` rejected all 16 as "not-release". Root cause was not a missing feature: `git describe --tags --exact-match` returns EMPTY as soon as HEAD moves past the tag - even by a bench-only commit. All four runners now fall back to the nearest reachable tag and stamp `sense_release_exact`. Second harness-shaped defect: the scorer takes wall time from the Claude transcript's `duration_ms`, which codex and opencode never emit, so every non-Claude arm published `0 -> 0` seconds while `run_meta` held the real numbers (gpt-5.5 263s -> 318s; kimi 2150s -> 742s). **The harness-independent source of truth turned out to be the MCP wire capture itself:** `sense-io.jsonl` records `serverInfo.version` on every handshake, which verified all 16 runs at 1.13.0 - and correctly showed the headline arm carrying BOTH 1.12.4 and 1.13.0 across its pre-replay and replay runs. Capture the wire and provenance survives any harness. | **refines** |

## Per-vertical detail

### Ruby/Rails (seed) - what earned (and blocked) adoption

- **Codex was the clearest lesson.** The `codex-run.sh` clone initially had no `AGENTS.md`/`.codex/config.toml`
  because `sense setup` never ran, so GPT-5.5 made **0** Sense calls (MCP `serverInstructions` alone did not
  steer it). Adding `sense setup --tools codex-cli` → `AGENTS.md` → 7–13 MCP calls and ~88% of the answer
  assembled structurally via Sense. `AGENTS.md` is load-bearing; the MCP metadata is not enough by itself.
- **OpenCode's "hang" was a false alarm.** The early apparent hang was a premature kill; OpenCode's MCP just has
  a slow cold start. Once warm it adopts and benches cleanly (the move to opencode for the open-model arm was
  the right call).
- **A self-inflicted adoption bug worth remembering:** `bench-sense-local.sh` once leaked the global MCP servers
  (Gmail/Calendar/Drive) into the sense arm because it lacked `--strict-mcp-config`, which produced deferred
  tools and weak adoption. Rule that came out of it: when adoption looks weak, check the init `mcp_servers`
  list first. Always run the sense arm with `--strict-mcp-config`.
- **`sense setup` auto-detects four clients** (Claude Code, Cursor, Codex CLI, OpenCode); Windsurf/Cline are
  MCP-compatible but manual. The product surface that earns adoption is this setup command emitting the right
  per-harness config.

## Open questions (carry forward)

- GPT-5.5's full Rails matrix (the fresh bench starting 2026-06-25) is the real test of whether the `AGENTS.md`
  wiring holds adoption across all 13 repos or just the smoke test.
- Cursor and the manual-config clients (Windsurf, Cline) are MCP-compatible but unbenched. Do they adopt like
  Claude Code or like pre-`AGENTS.md` Codex?
