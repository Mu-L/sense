# Agent survey - the user-complaint channel

> **Status: built 2026-07-19; zero runs banked yet.** A standing Loop-5 detector, not a loop phase.
> Sense's consumer is the AI (private product definition); this is the one channel where that consumer is
> *asked* - every other detector observes it. It rides inside runs already paid for (~one extra
> subscription turn per sense-arm run) and its entire discipline is one sentence: **an agent
> self-report is a hypothesis; the transcript is the killer; only transcript-verified friction with
> n≥2 independent runs advances.** It never gates a bench and never feeds a verdict.

## What it is

At the end of every `session-run.sh` sense-arm run, one extra `--resume` turn asks the agent five
evidence-citing questions about Sense, then a 0-10 self-score on an anchored band scale. The reply
is machine-parsed, every cited instance is checked against the scored transcript, and the verified
record accumulates for loop-close analysis. Purpose: harvest friction the agent *noticed*
(`transcript_miss.py` harvests gaps it *didn't*), feeding the Loop 7 backlog through the same
verify-then-promote pipe as everything else.

**What it is NOT:** evidence of value (the bench verdict is), a satisfaction metric (LLM scores are
uncalibrated without anchors), or a ship gate (behavioral changes still earn their way in via their
own bench, manifesto law).

## Where everything lives

| Artifact | Path | Written by |
|---|---|---|
| Survey prompt (questions + anchored scale + JSON shape) | `bench/lib/survey_prompt.md` | hand |
| Trigger (sense arm, clean runs, after `run_meta.json`, output NEVER into `transcript.json`) | ALL runners: `bench/drivers/bench-sense-local.sh` (Loop 2 `vertical-loop.sh` + claude/ollama sweep cells), `codex-run.sh` (`codex exec resume` + parse-codex normalize), `opencode-run.sh` (`opencode run -s` + parse-opencode normalize, `hclass=ok` only), `session-run.sh` (session mode) | hand |
| Parse + transcript-verify + append; `--report` aggregates | `bench/lib/survey_verify.py` (tests: `test_survey_verify.py`, 13 green) | hand |
| Raw survey turn (full provenance, never delete) | `results/<model>/sense/<repo>/survey.json` | runner |
| One verified record per run | `results/<model>/surveys.jsonl` (model-scoped, uncommitted mid-campaign) | `survey_verify.py` |
| Friction ledger (hypothesis → finding → filed → shipped/killed) | `../FRICTION.md` | me, at loop close |

**Arm coverage: all arms** (2026-07-19). The codex/opencode parsers already normalize their streams into
the canonical claude stream-json the scorer reads, so `survey_verify.py` needed zero changes - each
runner resumes its own session (`--resume` / `codex exec resume` / `opencode run -s`, session_id grepped
from the normalized `transcript.json`) and pipes the survey raw through its parser into `survey.json`.
Session-id extraction and parsing verified against real gpt-5.5 and devstral transcripts on disk.
Sense-io hygiene per arm: claude resumes on the plain `.mcp.json` (no tee), codex re-registers the plain
(non-tee) MCP server, opencode's survey runs after teardown removed `opencode.json` (no Sense MCP at
all). Residual until first live sweep: `codex exec resume` flag acceptance is untested on a paid session
(failure mode is a logged WARN, never a lost run).

**Loop wiring (who enforces what):** Loop 3 generates (automatic inside `bench-sense-local.sh`; noted in
`02-repo-run.md` Actors), Loop 4 generates on claude cells only (caveat in
`04-matrix-fill.md`), Loop 5 reads at loop close (`05-harvest.md` per-repo tier), Loop 7 consumes filed
FRICTION.md rows and records their exit (`07-product-fix-window.md` Position).

## The questions and the score

Q1 accuracy (which responses located cited code / which were wrong-empty), Q2 fallbacks (where and
why grep/read took over after a Sense call), Q3 hints (did response hint text change the next
query - before/after pair), Q4 value (single best thing), Q5 improve (single change). Structured
instances (`tool` + `query`) so verification is mechanical; "empty list rather than inventing one"
is in the prompt.

The score comes AFTER the questions so the agent's own enumerated evidence is its material, on
anchors that mirror the bench's value axis: 0-2 misled / 3-4 net drag / **5-6 interchangeable with
grep (= tie)** / 7-8 faster but needed fallbacks / **9-10 recovered what grep could not (= reach)**.
That alignment is deliberate: self-score vs judge-verdict disagreement is itself a finding (below).

## Verification (the killer step, $0)

`survey_verify.py` stamps every cited instance against `transcript.json`: Q1 needs the named tool to
have run with that query in its input; Q2 additionally needs a Grep/Glob/Read (or Bash grep/rg/find)
AFTER the cited call; Q3 needs before-query → after-query ordering, plus a separate `hint_found`
flag (quoted hint text present in a real tool result). Failures are KEPT and stamped
`confabulated` - per-model confabulation rate is signal, and a model with a high rate gets its
q5_improve discounted before aggregation, not after.

## Process at loop close

1. Run `python3 bench/lib/survey_verify.py --report` in the same sitting as the `transcript_miss.py`
   mine (Loop 5 per-vertical tier). Same-gap agreement between the two detectors (agent complains
   about a fallback AND the fallback-reads signature shows it) jumps the queue.
2. Update `../FRICTION.md`: one row per friction hypothesis, status forward-only.
3. Promotion thresholds (mechanical, never vibes):
   - **hypothesis → finding:** verified instances from ≥2 independent model×repo runs (n-killer),
     plus the confound cross-tab (model? repo size? vertical?) - a one-model complaint is a
     prompting quirk, not a Sense gap.
   - **finding → filed:** GitHub issue (public ledger, lane-scoped) with evidence lines pasted.
     Hint wording / tool-description / output-trim items are `enhance` candidates; anything
     behavioral ships only through its own bench.
   - **filed → shipped/killed:** normal Loop 7 pipe; the row records the exit so a killed idea
     cannot resurface as "new" next loop.
4. STATUS.md gets one line (the owner's read surface): findings crossed / issues filed / confab rates.
   Rulings only when a finding wants paid spend or a product-surface change (delegation split
   unchanged; Basecamp queue if it is a real decision).

## Standing cross-checks on the numbers

- **Score vs verdict disagreement:** agent 9 on a judged tie (or inverse) → full-transcript dig,
  never metadata. Either confabulation (feeds the rate) or judge blindness (feeds the
  judge-blind-to-omission file). Both outcomes pay.
- **Confab rate per model over time:** the trust-weighting input for aggregation.
- **Anchored-score drift per vertical:** decoration, never evidence - a sortable column, not a KPI.
