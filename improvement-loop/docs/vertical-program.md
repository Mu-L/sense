# Stack-Vertical Program

Turn the Rails vertical into a repeatable motion for other stacks: prove on a community's own repos that a blind agent navigates them worse than grep while Sense closes the gap, then publish the scorecard where that community reads. Use this once Rails ships.

> **THE BENCH BIBLE: [`manifesto.md`](manifesto.md)** - the authoritative rulebook for HOW to bench a vertical (prime directives, the win-axis catalog, the no-separate shapes, the per-repo loop, judging the judge, calibration, cost, honesty, Definition of Done). Read it before benching any vertical; when it disagrees with an older prompt/doc, it wins. Worked detail lives in `scenarios/crafting.md` + `scenarios/sourcing-runbook.md`.

> **Companion docs (cross-vertical):**
> - [`help-the-ai.md`](help-the-ai.md) - the **canonical operational definition** of "help the AI / bring value to the AI": the gate every Sense change must pass (reach at token parity + tool-only reach + grounding/adoption gates; quality beyond reach). Derives from the manifesto §9.
> - [`cross-cutting/`](cross-cutting/) - the **cumulative** articles that span verticals (methodology, providers/cheap-arm law, cross-model, harness adoption). Each vertical appends a row via `cross-cutting/prompts/harvest-after-vertical.md`. Facts/insights only; prose is drafted downstream.

## The real work (per vertical, in order)

It's a recipe plus the `bench/` harness, not a push-button pipeline. The judgment is all in the scenario.

1. **Choose the right repos - EXACTLY 4** (manifesto §7.0). Fixed composition: `1 framework + 1 big + 2 medium`, or `2 big + 2 medium` when the framework is too small/memorized to discriminate (never both a framework slot AND a 2nd big). Each a central abstraction with a heterogeneous, scattered, non-obvious dependent fan-out, in a community with reachable maintainers and newsletters. Rails' 13 was the flagship exception and Django the interim 6; every vertical from php-laravel onward ships 4 repos → 5 articles (1 per repo + global) → one week.
2. **Write real-life scenarios that are HELL to grep / list / read.** A genuine change-impact task whose answer no single grep/ls/read can assemble. The scenario IS the result.
3. **Bench both arms on the headline arm ×2** (`runs-variance.sh`), then **read the transcripts deeply.** Find exactly where the baseline struggles, and where Sense needs a kick to nail it. Once hardened, run the non-Opus confirmation arms (GPT/Kimi/Devstral) ×1 each across the same 4 repos for the honest cross-model scorecard.
4. **Iterate to +50pt.** Re-author the scenario (and fix any Sense gap the transcripts expose) until Sense beats baseline by >=+50pt cited-recall on the discriminator group (`pergroup.py`). +50 is the floor, widen it.
5. **Write the article.** It must match the benched result.

A Sense enhancement that falls out of steps 3-4 is a welcome byproduct (Rails shipped 3 this way), not the goal and not a precondition. The goal is the +50pt scenario.

## Points of attention

- **4 repos per vertical, fixed composition** (manifesto §7.0) - never more. Two independent win pillars (framework + ≥1 big app), discriminator picks its own repo. Swapping a repo is the LAST resort: first exhaust the scenario fix (re-author / gold-retarget / different seam) and a possible Sense fix; only then swap for a same-type backup. 4 repos → 5 articles (1 per repo + global) → one week.
- Headline ×2, judge pinned, confirmation arms per `verticals/<key>/arms.txt` - the one place a model id is named. Every arm runs on every repo: no asymmetric coverage.
- A tie is a scenario-design failure, never a verdict on the repo. Re-author until it wins.
- Small/medium repos hit a wall: completeness alone doesn't separate when a frontier baseline can grep the whole repo. Those need a correctness-not-coverage task.
- Pin commits, publish the harness, publish where the baseline beats Sense too. Reproducibility is the trust.

## Priority - two axes (they diverge)

"Build" = the vertical also ships a new/deeper extractor (a byproduct, not a blocker). Extractor state read from `internal/extract/`.

**Axis A - Effort (easy to hard):** how much resolver work the stack needs.

| Stack | State | Note |
|---|---|---|
| Python / Django | deep + `django.go` | framework edges exist, least new code |
| Go | native | interface dispatch already modeled; explicit lang |
| Rust / Axum | deep (`traits`/`compose`) | only proc-macros are hard |
| TS / NestJS | deep TS | light DI/decorator work |
| C# / ASP.NET Core | `csharp.go` symbols | typed = tractable; build DI/middleware/EF awareness |
| Java / Spring (Kotlin alongside) | `java.go` symbols | typed; build Spring DI/AOP/annotation resolution |
| PHP / Laravel | `php.go` symbols | dynamic + facade/container magic = harder than C#/Java |
| Elixir / Phoenix | none | build language + message-passing/protocol from scratch |
| React + Next | TS exists | scenario-R&D, not resolver work (component trees don't fit the recipe) |
| Zig / Clojure / Haskell | none | hardest resolution; defer |

**Axis B - Impact (growth + popularity):** Python/Django > Java/Spring > TS/JS > C#/.NET > PHP/Laravel > Go > Rust > Elixir > niche.

Drivers: Python = largest AI-adjacent audience + strongest thesis; Java/Spring + C#/.NET = largest enterprise base and giant monoliths where Sense's value peaks (sell those on "navigate the legacy monolith no one fully holds," not the AI-gem thesis); PHP/Laravel = massive, cohesive, underserved; Go/Rust = credibility; Elixir = small but high-trust with the best showcase idiom.

**Sequence (RE-RULED 2026-07-17, the owner - the single-file queue is RETIRED; verticals are parallel lanes):**
The project is no longer one person working one campaign at a time; several people can craft benches
for different verticals at once. Ordering campaigns in a serial queue blocked $0 work for no reason,
so the 2026-07-06 slot sequence below stands as history only. What replaces it is one mechanical
dependency per lane, not an ordering between lanes: **paid bench cells for a stack require that
stack's extraction/resolution to be built first** (`bootstrap_check.py --strict` green for the
vertical). Everything upstream of paid spend - repo pool, pinning, scenario crafting, gold
curation, article skeletons - is $0 and can start in any lane at any time. Where a lane's index is
still symbols-only, admission-gate numbers are provisional and are re-run once the language support
lands. The cross-language receiver/confidence design pass is no longer a between-verticals gate; it
folds into the first dynamic-language resolver build that needs it (PHP being the live case).

*(Historical, superseded: Rails ✅ → Python/Django → Go → Laravel → C#/.NET → Java/Spring →
Rust/Axum, then Elixir, NestJS, React+Next, Zig/Clojure/Haskell deferred.)*


____

IMPORTANT NOTE: When a vertical is finished Sense win all the repos by far (50pt is the absolute minimum, our favored target is +80pt, shorter session time, better accuracy, and less billed tokens), and as been tailored, along the way to work smoothly and bring high value to the vertical. One small note, not to derive, the scenarios must be human readable and understable as they are going to be shared publically.
