# php-laravel - answer forms

What the ANSWER to a scored step is allowed to be, in THIS stack. Read by `01-author.md`
before choosing a kind of question. Every line here is a measurement with its n; a form with
no number under it does not belong on this page.

Nothing here gates anything. It ORDERS what to try first, and it names what has already been
paid for so a cycle does not buy it twice.

## The measured state, 2026-08-12

36 authoring attempts, 3 repos, all parked. The binding constraint was the BASELINE's floor in
every one of them - never Sense's reach, which hit 1.000 repeatedly. Baseline `dependents`
cited recall below 0.50 in 10 of 34 measured attempts, and all 10 were on coolify.

| repo | attempts | baseline range | ever below 0.50 |
|---|---|---|---|
| laravel-framework | 6 | 0.667 - 1.000 | no |
| bagisto | 6 | 0.571 - 0.944 | no |
| coolify | 22 | 0.000 - 0.944 | 10 of 22 |

## Form that has NOT worked: the call-site list

**Do not ask for "every place that calls / holds / is injected with X".** In PHP each kind of
relationship has its own syntax, so naming the kind hands the plain arm a regex for it:

- `Container::getInstance()` - 45 files, 22 scored, closed by ONE grep piped through awk.
  Baseline 22 of 22.
- `getTypeInstance()->isSaleable()` - `grep -rl 'getTypeInstance()'` returns 68 files and
  covers the whole accessor route. Baseline 13 of 16 from a single command.
- `protected Connection $conn` / `Filesystem $files` - a typed property or a constructor
  signature is two regexes, and both print `path:line` directly.

Five independent mini-bench reads said this in their own words. Attempt 6 on
laravel-framework tried to beat it with a 45-file pool against a 22-row answer and still lost
22 of 22. This form is closed on this stack until something new is measured.

## Form under test: the value-flow row - HYPOTHESIS, n=15, NOT established

A row where an IDENTIFIER was lifted off the object and parked somewhere the object never
goes - a screen's saved state, a join-table column, a cache key, a field in an outbound
payload - so the value's name at the destination is arbitrary (`$this->serverId`,
`'server_name' =>`, `'storage-check:'.$id`) and no single token enumerates the set.

Within coolify, one anchor, valid runs only: gold sets that are >=50% value-flow rows sit at
baseline **0.482** (n=13); gold sets below that sit at **0.823** (n=2). The gap holds at every
split from 0.30 to 0.90 and inverts at 0.95.

**Why this is not yet a finding:** Spearman is only **-0.300** at n=15, the low group is two
cells, and inside the high group the baseline still spans 0.231 to 0.929 - the marker
separates the extremes and does not order within them. It may not gate a draft. The designed
experiment that would settle it: the SAME anchor and the SAME session shape, one gold set of
each form, both benched.

## What is NOT the mechanism here - each killed by a run, do not re-propose

- **Token-darkness.** A dependent whose file never writes the anchor name is not the lever.
  The rails winners are 0 of 20, 0 of 12 and 2 of 13 dark and they banked +0.775, +0.708,
  +0.577. laravel-framework's 22-row set is 0 of 22 dark and its baseline scored 1.000.
- **Precision / grep-hostility / scatter.** Spearman of `seam_hunt` precision against baseline
  recall runs BACKWARDS (-0.483 pooled, n=9): the three rails winners are the most grep-clean
  anchors measured anywhere (0.42, 0.82, 0.97). And within the single anchor `Server` the
  baseline spans 0.00 to 0.93, so no anchor-level number can explain what the question does.
- **Repo-shipped agent guidance.** coolify ships 18.8KB of `AGENTS.md` plus 11 skills;
  laravel-framework ships none and has the strongest baseline; mastodon ships none and has the
  weakest. No relationship.
- **An unscored step leaking the scored question.** Tested with a real pair on coolify: task 3
  stripped of the classification the scored task wants, everything else byte-identical.
  Predicted delta >= +0.50, measured **+0.154**. The baseline moved 0.308 -> 0.231 (inside
  noise) and Sense fell 0.692 -> 0.385.

## Session length, coolify, same 13 gold rows

| shape | sense wall | sense | baseline | delta |
|---|---|---|---|---|
| 2 tasks | 289s of 480 | 0.692 | 0.154 | +0.538 |
| 7 tasks | 466s, 432s | 0.692, 0.615 | 0.308, 0.308 | +0.385, +0.308 |
| 7 tasks, leak removed | 477s (cap hit once at 481s) | 0.385 | 0.231 | +0.154 |

Sense's recall falls as the session lengthens and falls FASTER than the baseline's. The
seven-task session is at or over the 480s ceiling on this repo (the cap was hit on two
separate scenarios). `CANNOT-FINISH-AT-BUDGET IS A RESULT` - shorten what the tasks ask for,
never raise the ceiling.

## What Sense resolves in this stack that a grep cannot join

Verified over MCP on the bagisto index, not read off the source:

- **Facade -> concrete class.** `sense_graph` on `Webkul\Checkout\Cart` returns 61 callers
  whose call sites write `Cart::setCart(...)`; the method form returns 43. Only 2 files in the
  repo name `Webkul\Checkout\Cart` at all.
- **Container bindings** (`bind`/`singleton` with a literal key -> `app(X::class)` consumers),
  **Eloquent relations**, **query scopes** (`->active()` -> `scopeActive`), **observers**,
  **route/listener/middleware string dispatch**. All literal-only by design.

Two cautions, both measured. A gold set built on ONE such route is still one grep away
(`getTypeInstance()` covers 68 files). And the payload ships an `index_caveat` claiming
"Static graph MISSES facade static calls" in the very response that resolved 61 of them -
filed as a product finding, not a scenario problem.

## Adoption, per repo - check it before blaming the question

Structural calls (`sense_blast`/`sense_graph`) per sense run, from `sense-io.jsonl`
`tools/call` records: bagisto **33%** (4 of 6 runs never asked Sense anything structural),
coolify 94%, laravel-framework 100%. A run that never called blast or graph did not measure
Sense, and its delta says nothing about the question.
