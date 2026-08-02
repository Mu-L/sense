# Vertical Bench Manifesto - how we benchmark Sense for a stack vertical

> **What this is.** The authoritative, durable rulebook for benchmarking Sense against a baseline coding agent
> across a stack's real repos, distilled from the Rails vertical (the first one: 7 wins, 3 bench-driven product
> fixes, dozens of corrected mistakes). It governs every vertical and seeds the automation program
> (`vertical-program.md`). When this file and an older prompt/doc disagree, **this file wins** - it is the
> consolidated truth. Worked detail lives in `scenarios/crafting.md`, `findings/workflow.md`, and each
> vertical's own `repos.md`, plus the project memories; this is the spine that points to them.
>
> **How to use it.** Read §1–§3 once to load the mental model, then run §8 (the loop) per repo with §5/§6/§7
> as the lookup tables and §9–§12 as the discipline. Never ship a repo result that violates §2 or §13.

---

## 1. The North Star - what a finished vertical looks like

A vertical is DONE when, on the community's own repos, **Sense brings the baseline agent high value on every
shipped repo**, proven and published:

- **Margin:** Sense − baseline ≥ **+0.50** cited-recall on the discriminator group is the ABSOLUTE FLOOR; the
  **favored target is +0.80**. Bigger is always better - a small margin erodes as the next frontier model
  raises the baseline; only a large margin is durable.
- **Cheaper, faster, more accurate:** the win is not only recall. Tailor the scenario/tooling so the Sense arm
  also trends toward fewer billed tokens, shorter wall time, and higher accuracy at held or lower cost.
- **Human-readable, shareable scenarios:** every scenario is a real maintainer task a reader of that community
  would recognize and respect. It will be published. Never contrive, never leak the answer, never optimize the
  prose away from a genuine task to chase a number.
- **Reproducible:** pinned repo commits, the `bench/` harness published, the numbers traceable to
  `verticals/<name>/results/<repo>/` snapshots, and **the places the baseline beats or ties Sense reported honestly.** A bench
  that only flatters the tool measures nothing.

The output per repo is the benched scorecard + an article that MATCHES it. The output per vertical is the set of
wins + the published scorecard where that community reads.

---

## 2. Prime Directives (non-negotiable - violating one invalidates the result)

1. **YOU ARE THE BASELINE - and Sense is YOUR tool, not a rival.** In this bench you are Claude Code on the
   frontier model (the headline arm); the two arms are YOU *without* the tool (baseline) and YOU *with* it (baseline +
   Sense). So the question is never "where does Sense beat me?" - it is "where does the tool make me better than I
   am without it?" (a hammer for the carpenter). That lift is genuinely hard to see from the inside, because you
   read/recall/trace your way to most answers even WITHOUT Sense, which hides the help and breeds **false
   confidence**. Your own blind spots are where the tool helps most. Never thumb the scale for the tool-less arm.
   (`feedback_i_am_the_baseline_never_settle`, `feedback_sense_is_the_agents_tool_not_a_rival`.)
2. **A tie is a design failure / a MISSING AXIS - never a verdict on the repo, never a stopping point.** Rails
   was "likely no-win" until the memorization axis flipped it to +0.56. Keep adapting until Sense brings high
   value; do not write "foreclosed." (`feedback_never_conclude_tie_from_proxy`, `feedback_all_scenarios_must_win_baseline_hell`.)
3. **Truth = the TRANSCRIPTS + objective recall, NOT the LLM judge.** A judge "tie" is usually the judge failing
   (it is blind to omission and rates a 35%-recall answer "exhaustive"). When the judge contradicts the
   transcripts, fix the judge. (`feedback_judge_blind_to_omission`.)
4. **Never conclude from a PROXY.** grep-reachability ≠ baseline-finds-it; a single run = noise; metadata ≠
   evidence. Read both transcripts side by side, per gold target, before any conclusion.
   (`feedback_never_conclude_tie_from_proxy`, `feedback_deep_transcript_analysis`.)
5. **The SCENARIO is the lever and the result.** Make the task baseline-HELL for grep/list/find/read. Don't
   fake it by deleting the grep path - engineer a task whose answer no grep/ls/read can assemble.
6. **Bench on the headline arm ×2, judge pinned; confirmation arms per `verticals/<key>/arms.txt` - that file is the ONLY place a model id is named.** A weaker-model win is not a
   win (it inflates Sense). A single run is noise - **×2 is the settled standard** (repeated ×2 batches land
   within noise). The non-Opus cross-model arms (GPT/Codex, Kimi, Devstral/Ollama) run **×1** as directional
   confirmation the win holds on cheaper models (a single run carries an OPEN flag).
   (`feedback_bench_model_opus_4_8`, `feedback_runs2_settled_standard`.)
7. **Honesty guardrails (publishable):** arms differ ONLY by toolset; identical prompt; gold is the must-find set
   not a stacked deck; the prompt leaks no nouns/paths/counts/tool-names; report where the baseline wins; never
   fabricate maintainer handles or numbers.

---

## 3. The mental model - what Sense is better at, and WHY baseline-hell wins

**The axes where Sense lifts you past grep** (design every scenario toward one):
- **Blast radius** - the complete transitive change-impact set, in one call.
- **Graph** - exact callers/callees/inheritance, not noisy text matches.
- **Deep structural relationships** grep can't cleanly resolve - reached through associations, concerns,
  polymorphism, wrappers, dynamic dispatch, cross-file inheritance.
- **Semantic search** - find by meaning, not string.

**The mechanism (why a baseline-hell scenario wins, stated as a law):** a frontier baseline greps/reads/`ls`'s
its way to any answer that is **named by one literal token, colocated in one directory, composed inside one
god-file, or memorized from training.** It CANNOT cheaply assemble the **complete, transitive dependent set of a
central abstraction whose dependents are HETEROGENEOUS (each touches it a different way), NON-OBVIOUS (the file
doesn't announce the dependency), SCATTERED across many directories, and either under a NOISY token (grep
over-collects to a haystack) or reached through a NON-LITERAL edge (grep misses them) - in a tree too large to
read whole.** That set is exactly what `sense_blast <Abstraction>` returns in one call and a hand audit
reconstructs incompletely under session load. **Make the task demand exactly that set.**

---

## 4. The validated recipe (the one shape, proven on 7 repos)

1. **Contract = the repo's CENTRAL abstraction** under a noisy token or reached non-literally - a model
   (Inbox/Article/Upload/Status/Order/MergeRequest) or, for a framework, a hub class (`ActiveRecord::Relation`).
   NEVER a single greppable method, NEVER a colocated family in one dir.
2. **Discriminator gold = ~16 (or ≥6 all-hard) SALIENT dependents** that are heterogeneous, non-obvious,
   scattered across disjoint dirs, retrievable by the agent's DEFAULT `sense_blast` (min_confidence 0.7 - curate
   from `direct_callers`, NOT `--min-confidence 0.3`, or Sense can't retrieve its own gold).
3. **Each gold target carries a `relation:` field** (the true edge, never shown to the agent) so the
   reference-aware judge can grade covered + related per item.
4. **Frame as a CHANGE-IMPACT / teardown-rework audit**, 7 neutral steps. The step that ENUMERATES the
   dependents is a **DEDICATED, LOW-CONSTRAINT step** - its ONLY task is the bare list ("list EVERY dependent
   individually with its `file:line`, one per line … do not collapse into 'various services'"), with **nothing
   else in the same instruction**; all analysis (how each depends, relationship kind, on-delete, risk ranking,
   uncertainty) lives in SEPARATE steps. This is because Opus 4.6+ drops constraints from the final answer when
   ~5-6 are stacked at once ("acknowledges in reasoning, absent from the answer"), which silently truncates the
   retrieved set and under-measures Sense MORE than baseline (Sense returns more, so it has more to drop). A bare
   enumeration step lets the agent transcribe the full set Sense returned - that is what makes Sense consistent
   across runs. (`project_agent_drops_retrieved_deps_bench_pollution`.)
5. **Non-discriminator anchor groups** (`contract`, `context`, `surface`) hold the obvious/memorized pieces both
   arms get; `pergroup.py` measures the `dependents` group delta - the headline.

---

## 5. The WIN-AXIS catalog - the searchable space of "where Sense lifts you past what you reach alone"

When hunting a win, walk this list. Each is a way the tool-less approach hits a wall that Sense's structure clears for you.

| Axis | The baseline fails because… | Sense wins because… | Worked / candidate |
|---|---|---|---|
| **Scale completeness** | the tree is too large to read whole; grep over-collects | one blast returns the resolved set | ✅ 6 large apps |
| **Memorization gap** (famous repos) | it RECITES the public API from weights and ties on famous deps, but has NOT memorized the obscure internals | blast returns the engine-room internals that aren't in training | ✅ Rails `Relation` (`project_rails_framework_memorization_win`) |
| **Cross-module / cross-gem scatter** | it audits one subsystem and never chases dependents in other modules | blast crosses module boundaries in one call | ✅ Solidus, GitLab, Rails |
| **Non-literal edges** | grep for the class misses association/concern/polymorphic/`===`/short-name reaches | the graph resolves them regardless of how they're written | ✅ Chatwoot, Discourse |
| **Async / task-queue dispatch** (celery `.delay`/`.apply_async`; Sidekiq/ActiveJob) | tracing a blast radius across an async boundary needs a DIFFERENT grep at every hop (`foo.delay`, not `foo(`) and task→task chains dead-end; a hand-audit drops branches under load | one blast walks the whole async fan-out; Sense resolves dispatch→task natively | ✅ resolver shipped `internal/extract/python/celery.go` (`project_python_celery_dispatch_edge_gap`); reach-at-parity lever - strongest when the async chain is deep + scattered + task→task (a single *named* `.delay` hop still greps trivially) |
| **Load-degraded correctness** | late in a long session, under context pressure, it trusts priors / skips verification | one cheap grounded call stays correct | ⬜ candidate (untested) |
| **Budget-constrained tasks** | a tight tool-call/time budget forces it to shortcut to its prior | Sense makes the correct answer cheap, so diligence isn't needed | ⬜ candidate |
| **Project-INVENTED conventions / local law** | it recognizes framework idioms but has NO prior for a bespoke in-house pattern, and its *generated code* regresses to the global mean | a usage-distribution query quantifies what it can't shortcut; the local-law output (rule + tier + exceptions + scope) makes the dialect enforceable | ⬜ candidate - detectors SHIPPED (Rails denoise pack, PRs #142/#143/#154/#161); the gap is the NORMATIVE output. Bench = a **write-task** gate, designed in `01-production-spec.md` (private tree) §8. DECIDED 2026-07-06: benched at the local-law build gate + an end-of-program cross-stack pass, NOT per-vertical; every vertical harvests for $0 into `02-bench-harvest.md` (private tree) |
| **Whole-repo invariants / conjunctions** | "does every X also do Y" is error-prone to verify by reading even on a small repo | Sense can assert it definitively | ⬜ candidate |

The ✅ axes are proven; the ⬜ axes are the **open hunt** on the not-yet-won repos (small/medium + gems). Treat
them as targets, not footnotes. The catalog is not closed - invent new axes (§2.1: think outside the box).

---

## 6. The NO-SEPARATE shapes - what ties, and how to transform it

A candidate seam that has any of these will TIE; don't ship it - pick a different seam in the same repo, or
transform the task. (None of these means the repo is unwinnable.)

- **Colocated / `ls`-able** - the fan-out lives in one dir → baseline lists it. (queue_adapters, conditions/.)
  → Move to a central model whose dependents scatter across disjoint dirs.
- **Literal-precise token (grep-clean)** - the contract name appears in few files, all naming it → baseline
  greps cleanly. (ActiveStorage::Blob, 40 files.) → Pick a noisy-token contract, OR a contract in a gem too
  large to read where the name is referenced by short name.
- **Named-intermediate / 2-hop-greppable** - every node in the chain is a named class/method → baseline
  two-hop-greps. (Mastodon trigger→FollowService→serialize_payload.) → Pick a grep-hostile hop (implicit
  self / duck-typed / `===` / short-name).
- **Type-named accessor family - the GO-NAMING LAW** (dolt `DoltDB` dry-run kill, 2026-07-13) - in Go the
  acquisition idioms INHERIT the type name (`dEnv.DoltDB(ctx)`, `session.GetDoltDB`, `provider.DoltDatabases()`),
  so "every holder of X" is ONE substring grep even in files that never name the type; the frontier baseline
  swept 7 idiom patterns + receiver checks and assembled a SUPERSET of the gold (18/18, 479 sites) in 8 min.
  Bar-2/bar-3 admission metrics measure TYPE-TOKEN noise and are blind to this - the accessor family is the
  real cover. Python attribute access (`variant.channel_listings`) hides what Go naming betrays. → Anchor on
  contracts acquired through NON-type-named paths (pebble `versionSet` via the `d.mu.versions` FIELD - the
  saleor shape, rg-verified), and NEVER make flat holder-enumeration the headline: flat lists are grep's home
  turf; the headline must demand ANSWER-ASSEMBLY (relations, conjunctions, whole-structure risk semantics) or
  the transitive-carrier closure the sweep itself cannot compose.
- **Rare-member erased interface - the MEMBER-NAME LAW** (pebble `Batch` dry-run kill, 2026-07-14; the
  GO-NAMING law's sibling on the INTERFACE side) - a small erased interface with globally-rare member names
  (`readyForFlush`, `computePossibleOverlaps`, `inuseBytes`) is one field-name grep (`.flushable`) + one
  interface read + one member-name grep from FULL closure, even when the operating files carry ZERO type
  token: the frontier baseline hit 1.0 on the token-present context group and 0.71 on the token-free
  discriminators (missing only 2-hop riders excise/checkpoint) - no ≥+0.50 margin. Type-token counting
  (bar 2, the groundwork audit) is blind to this: it certifies grep-darkness of the TYPE while the
  contract's MEMBERS are the real cover. → Erased-interface anchors only survive when the members are
  NOISY names (`apply`, `Get`, `Next` - hundreds of hits) or the dispatch is genuinely unnameable
  (function values, generics) - and that residue is usually Sense-blind too (index_caveat class), i.e.
  product-lane, not gold. Probe duty: the adversary probe's member-name family kills BEFORE gold curation.
- **Typed-field composition closure - the TYPED-FIELD BFS LAW** (dolt carrier-audit probe kill, 2026-07-14;
  generalizes GO-NAMING beyond names) - in Go every struct field is TYPE-ANNOTATED, so the composes closure
  itself is text-derivable: the frontier baseline parsed every `type X struct` block into a field TSV with one
  awk pass and ran a bash BFS over declared field types, converging on the FULL 86-struct / 5-hop transitive
  carrier closure of `doltdb.DoltDB` in minutes, then made credible lifetime/goroutine judgments from targeted
  reads. Flat "who transitively holds X via fields" can NEVER be the discriminator on Go, and neither can
  judgments the BFS table already localizes. → The closure hop that BREAKS field-type BFS is the
  INTERFACE-typed field: the runtime carrier behind `kv StatsKv` / `db SqlDatabase` is invisible to text
  tools unless the satisfier is assertion-declared or member-name-greppable (the probe disclaimed exactly
  this, verbatim). The surviving anchor shape = the interface-LAUNDERED carrier set (composes × truthful
  satisfied_by JOIN - both surfaces post-heal), junk-screened per JUNK-FABRICATED-SHAPE (single-common-member
  arity-matching interfaces like `Next(ctx)` still fabricate satisfaction edges - screen them out first).
- **Wide laundering interface / no retained graph - the RETENTION-SHAPE LAW** (gitea retention pass,
  2026-07-15; the TYPED-FIELD BFS law's converse - it says WHERE the surviving interface-laundered
  anchor shape actually exists). The retention axis (`blast` → `retained_via_interfaces`, the pebble
  win) needs BOTH: (a) a repo that RETAINS long-lived objects in struct fields, and (b) NARROW
  laundering interfaces, so may-retain ≈ does-retain. Pebble has both - `rangeDelIterSetter`,
  `switchableWriter` have 1-2 satisfiers each, so every row is a real holder and the closure is
  hand-auditable. **Request-scoped web repos have NEITHER and the axis is measured dead:** state flows
  as ARGUMENTS (`ctx`, receivers), not fields, so there is no retained graph to audit - gin
  (`gin.Context`/`Engine`/`RouterGroup`) and miniflux (`model.Entry`/`Feed`/`storage.Storage`) return
  **retained=0 on every anchor probed**, and gitea returns 0 on `issues.Comment`/`webhook.Webhook`/
  `storage.ObjectStorage`. **Where such a repo DOES launder, the interface is a wide service seam and
  may-retain SATURATES into noise:** gitea `db.Paginator` (59 implementors) makes every
  `*SearchOptions` "may-retain a `user.User`"; gms is alive-but-saturated (59-146 rows via `plan.Node`
  / `sql.Session` / `sql.Expression`) - technically true under may-hold semantics, useless as an audit
  answer, and the baseline dismisses it in one sentence. → Screen the axis at PHASE 0, $0, before any
  probe or gold: run `blast(anchor)` and read `retained_via_interfaces`. **0 rows = no retained graph
  (dead, move on). Tens-to-hundreds of rows through a hub interface = saturation (dead - no
  discriminator).** The win band is a SMALL closure through NARROW interfaces (pebble: 14 rows in one
  call, chaining to 18 real holders). Corollary for slate design: retention is a SYSTEMS/STORAGE-repo
  axis (long-lived object graphs), never a framework axis. Aggravator: on a repo whose package basename
  shadows a stdlib package, the closure can be near-100% FABRICATED and look meaty - always audit the
  carrier edges before believing a promising count (the fabricated-ring finding; gitea `context.Base` = 30/31 fake @ 1.0).
  **⛔ SCOPE GUARD (the owner, 2026-07-15 - this law was mis-applied within the hour of being born, so the
  guard ships WITH it): this law bounds an AXIS, never a REPO.** "Retention is dead here" licenses
  exactly ONE inference - *don't spend a retention pass on this repo* - and NEVER "this repo has no
  path to a Sense win." The first draft collapsed a four-repo program order off one $0 screen; the owner
  rejected it. A repo dies only when its axes are exhausted AND its product levers are named and
  shipped - and gitea's own "dead" kill line turned out to be a PRODUCT GAP (the declared-receiver gap:
  calls stranded in the 0.3 band, 56% of its call graph) hiding behind a repo verdict, found only by
  digging PAST the kill into an edge kind the screen never touched. **A kill is MY failure to find the
  axis, never the repo's failure to have one** (TRY-HARDER law; [[project_small_medium_foreclosed_all_axes]]
  - NOT-FOUND-YET, never "foreclosed"). Screens are cheap and their reach is small: bank the $0 saving,
  never the verdict.
- **Memorized public API** - the baseline recites it from training. (Rails famous deps.) → Make the
  discriminator the obscure NON-memorized internals; demote memorized deps to a `context` group.
  (`feedback_memorization_confound_famous_repos`.)
- **Small-readable completeness** - the whole repo/subsystem is small enough to read, so completeness doesn't
  separate. (Lobsters, 82-file gems.) → It is the SUBSYSTEM size, not the repo size, that must exceed readable;
  if nothing is unreadable, switch from a coverage task to a correctness-not-coverage axis (§5 ⬜ rows).
- **Resolver gap** - the edge is real but Sense doesn't resolve it (pub-sub event bus, config-string registry,
  truly-dynamic dispatch). → Either pick a different seam, or build the resolver as a product fix that unlocks
  it (a byproduct, not a blocker - §12). REALIZED: Python **celery `.delay`/`.apply_async`** dispatch was
  such a gap (async boundary severed every task edge); shipped as `internal/extract/python/celery.go`
  (mirrors Ruby `enqueue.go`) - now a usable seam, not a blocker (`project_python_celery_dispatch_edge_gap`).
- **Window-resolvable judgment (batchable) - THE BATCHING LAW** (django `QuerySet.filter` dry-run,
  2026-07-04) - a judgment-per-hit bet (satisficing-drop OR precision-axis) where every hit can be adjudicated
  from ±5 lines of context. The baseline BATCHES it: one full grep, then bash for-loops sed-printing the
  context windows - **~40 per-hit judgments per single tool call** - and clears ANY hit volume (181 hits →
  101/101 recall, enumeration precision 1.00, including verifying the decoys before excluding them). Hit
  COUNT is irrelevant to a batching baseline; the litmus is the judgment's **CONTEXT RADIUS**. → The per-hit
  judgment must require WHOLE-STRUCTURE reading (sentry's "does this dataclass EMBED the model" salience call
  over whole files), not a small window. Corollary for sequencing product work: **"capability gate lifted" ≠
  "win unlocked"** - before betting a scenario on a resolver improvement, check whether the baseline's
  judgment on that seam is window-resolvable (it was on django, so PR #181's lift changed nothing).
  (`feedback_batching_law_scenario_design`.)

---

## 7. Repo-type playbook (the same mechanics, different beasts)

### 7.0 The bench SET - 4 repos, fixed composition, per-arm runs (the SET-level rule)

Rails was the flagship: 13 repos, deliberately over-invested because it was the proving ground and the author's
home stack. Python/Django shipped the interim SIX. **Every vertical from php-laravel onward ships exactly FOUR
repos** (ruling 2026-07-18: the 6-set carried two redundant ballast slots - a lean 4 covers the same win pillars
with half the labor). The point shifts from depth-on-one to BREADTH across stacks - "Sense wins across Rails,
Django, Go, TS" is a stronger, more durable claim than thirteen-deep on one - and **4 repos → 5 articles (1 per
repo + global) → one week**. The throughput target the set is sized to hit.

**Composition - EXACTLY 4, never more.** The framework slot and a second big app are MUTUALLY EXCLUSIVE (they
trade places); never both:
- **Framework-bearing stack:** `1 framework + 1 big app + 2 medium = 4`
- **Small/memorized framework** (Flask/Gin/Express-class, demoted - see below): `2 big apps + 2 medium = 4`

Medium=2 is FIXED (RULING 2026-07-20: the small honesty-ballast slot is REMOVED and replaced by a
second medium). Rationale, measured in the Go lane: a small repo cannot produce a win - its whole
corpus is readable inside the wall and its dependents are textually enumerable (gin 7.9k prod LOC /
58 files; miniflux control 1.00 with ceiling +0.000; memos/hugo/nats/navidrome rings all under floor).
A slot that can only tie spends authoring labor to buy an asterisk, and a 4-repo set has no room for
it. Honesty is now carried by publishing every measured cell including losses, not by reserving a seat
for one. The former ballast text, kept for the WHY it encoded:  it may tie or lose, and that
IS the published "where the baseline ties Sense, and why" row (§13), proving the set isn't cherry-picked to only
winnable repos. The ONLY variable slot is framework-vs-second-big. The set is always 4; never write "1–2 big".

**Two independent win pillars, not one.** Don't let the framework repo be the SOLE win. Require ≥1 big app to
ALSO carry a strong reach-at-parity win independent of the framework, so a single scenario misfire can't sink the
vertical.

**The discriminator picks its own repo.** The headline win lives where the subsystem is big + unreadable +
non-memorized (§3, §5 memorization gap). For a big framework (Rails/Django) that's the framework repo's
engine-room; for a small framework it's a big APP in that ecosystem, and the framework repo demotes to an honest
medium/small cell. Pick the discriminator by big+unreadable+not-memorized, THEN see which repo it lands on.

**Swap is the LAST resort, never the first move (Prime Directive 2: a tie is a design failure, not a verdict on
the repo).** Before swapping a repo that won't separate, exhaust the two fixes that keep it:
1. **Fixable in the SCENARIO?** Run the full §8 iterate loop - re-author, $0 gold-retarget, or pick a different
   seam in the SAME repo (the §6 no-separate transforms). Most "misfires" are a wrong contract/gold, not a dead
   repo (Rails' Relation flipped +0.19→+0.56 this way, no swap).
2. **Fixable in SENSE itself?** If the seam is real but Sense doesn't resolve it (a resolver gap - §6, §12), the
   fix may be a product byproduct that unlocks the repo (Rails shipped 3 such fixes). Weigh it against the
   pre-bench window before swapping.

Only when BOTH are exhausted and the repo still won't separate do you swap 1–2 repos mid-campaign for a same-type
backup. **Swap risk is accepted, but swapping too fast is the failure mode** - it discards winnable repos and
reads as cherry-picking. Mitigation is choosing wisely up front (sourcing with a misfire in mind), exhausting the
fixes, THEN swapping. This is why repo selection is the load-bearing human gate (§8.2, `bootstrap.md`).

**Per-arm runs across the published matrix.** Every LLM runs on every one of the 4 repos (honest cross-model
scorecard - no asymmetric coverage). Run counts differ by arm: **headline ×2** (the §8 hardening loop
runs here) + **non-Opus ×1** (GPT/Codex, Kimi, Devstral/Ollama - directional confirmation, OPEN-flagged). See
Prime Directive 6.

**Size mix is quota-FORCED, decide it up front.** The most-constrained cheap sub burns ~⅓ of its weekly window on
a single big-repo baseline run, so even at ×1 the cheap arms fit only ~2 big repos/week - the composition (≤2 big,
incl. the framework if it's big) is sized to that ceiling. But the real ceiling is **labor**: every repo needs a
hand-authored baseline-hell scenario + gold retarget (§8), and at solo-plus-AI pace that authoring effort, not
quota, is what caps the set at 6. Don't pad with cheap-to-RUN repos that are expensive to AUTHOR.
(`project_next_vertical_bench_set_design`.)

### 7.1 Per-repo-type mechanics

- **Medium/large apps** - the easy win. Central model has 40–66 non-obvious direct callers across 15–26 dirs.
  Curate ~16, bench, done.
- **Huge repos (gitlab)** - same, but the blast is enormous and the index build is the cost. Pick a
  central-but-curatable model one tier below the top (not User/Project which blast to thousands). The noise is
  your friend (baseline drowns). Gate the bench cost.
- **Old/flat repos (redmine)** - literal naming + flat structure → weak non-obvious fan-out. Try a different
  central model with more heterogeneous reach, broaden the contract to a pair, or target a cross-cutting
  concern. Expect 2+ cycles. Currently UNFOUND - an open hunt, not a no-win.
- **Small gems** - completeness alone is CAPPED (no 16 hard-to-reach deps). Levers: discriminator dominated by
  deps in lib/extras/jobs/mailboxes reached via dynamic dispatch / 2-hop chains; a smaller all-hard
  discriminator (≥6); or a correctness-not-coverage axis. Verify from the transcript that the baseline actually
  MISSES the gold. Currently UNFOUND - keep hunting (§5 ⬜).
- **The framework itself** - NOT one app; pick a BIG bounded subsystem and a hub class in it (Rails won on
  `ActiveRecord::Relation` in the 401-file activerecord/lib). The baseline has MEMORIZED the public API, so the
  discriminator MUST be the obscure non-memorized internals. (§5 memorization gap.)

---

## 8. The operating loop (run this per repo)

> **Automated by `bash bench/drivers/vertical-loop.sh <repo>`** (a phase machine that runs the mechanical steps
> and PAUSES for the two judgment steps - scenario authoring + tie diagnosis, which the AI agent DRAFTS and the
> human REVIEWS adversarially - plus the one blocking human cost-confirm before the paid sweep). The steps below
> are what it runs; see `bootstrap.md` for the driver + the per-repo scenario anatomy.

1. **Ensure index fresh.** `bash bench/lib/ensure-index.sh <repo>` (rebuilds only if the scan engine changed).
   Scan-layer fixes need a full rescan to verify; query-layer fixes don't.
2. **Read the BASELINE transcript first.** Extract its grep/ls/read moves. If it wins by `grep <one-method>` or
   `ls <one-dir>`, you picked the wrong contract. Design to break every move it made. (Fresh scenario with no
   transcript yet? GENERATE one cheaply via the step-5b baseline dry-run - never conclude from hand-grep.)
3. **Scout the central abstraction.** `sense blast <Symbol> --json` at DEFAULT settings; count non-obvious
   direct callers and how many disjoint dirs/modules they span. Want a large, heterogeneous, scattered set. If
   the symbol is ambiguous (reopened class, JS/Ruby name collision), use `--file` to disambiguate (the MCP tool
   returns a hint that steers the agent to retry with it).
3b. **ADVERSARY PROBE - $0, BEFORE any gold curation (the 2026-07-13 dolt lesson: run the adversary FIRST,
   design on its blind spots, don't grade your draft against your own greps).** Launch one frontier
   subagent in the BASELINE clone, grep/read only, sense tools forbidden, with just the headline
   enumeration task (~100k subagent tokens, ~8 min - subscription, not paid bench). Read TWO things as
   DESIGN INPUT: (a) its METHOD - every pattern family it composed is a dead shape (dolt: 7 idiom greps +
   receiver checks killed the whole handle-audit ticket); (b) its HONESTY DISCLAIMER - what it says it could
   NOT enumerate ("I did not enumerate transitive holders…") is the discriminator axis, verbatim. Only shapes
   the probe itself disclaims proceed to gold curation. This does not replace the step-5b runner gate - 5b
   stays the final GO/NO-GO under real 7-step session load; 3b kills dead designs before you spend a
   curation session on them.
4. **Curate the gold** from the default blast (heterogeneous, non-obvious, scattered), each with a `relation:`.
   Verify every match path is UNIQUE (`git ls-files | grep -c`). Add light non-discriminator anchor groups.
5. **Author the neutral 7-step scenario + matching rubric** (copy an exemplar - chatwoot/discourse). Force
   per-item `file:line` in the audit step. Back up the old scenario as `<repo>.<oldname>.*.bak`. Validate:
   step-name parity, gold groups, prompt leak-free (`python3 bench/lib/scenario.py … --prompt`).
5b. **AGENT DRY-RUN GATE - the GO/NO-GO before you pay (replaces hand-grep with real agent behaviour).** Steps
   0/3/4 prove what Sense RETURNS and what a *perfect grepper* COULD reconstruct - NOT what the real baseline
   agent ACTUALLY assembles under 7-step session load. Your own `git grep`/`structural_surplus` is a
   perfect-grepper proxy: it OVER-estimates the baseline and manufactures FALSE TIES (grep-CAPABILITY ≠
   grep-BEHAVIOUR; chatwoot's `inbox` was greppable everywhere yet baseline assembled 1/3 and WON). Only an agent
   settles it. Run the tool-less arm FIRST (cheapest): `VERTICAL=<stack> bash bench/drivers/bench-sense-local.sh
   --tool baseline --repo <repo> --runs 1 --no-build`, then read the JUDGE-FREE objective recall (`python3
   bench/lib/scorer.py <baseline_run_dir> verticals/<stack>/scenarios/<repo>.yaml bench`) + its transcript.
   Baseline assembles the FULL `dependents` set → TIE PREDICTED, do NOT pay for the pair; reframe (§6) and
   re-gate. Baseline DROPS deps → run the sense arm (`--tool sense`), confirm it ADOPTS (mcp_count>0) and CITES
   the dropped deps. **Proceed to step 6 ONLY on a real arm-asymmetry** (baseline drops what sense keeps), never
   on a grep-capability gap. The two single arms ARE run-1 of the pair, so this costs nothing extra when it
   passes - it just KILLS tie scenarios after one ~$0.70 arm instead of a full judged pair.
6. **Bench:** `RUNS=2 bash bench/drivers/runs-variance.sh <repo>` (MODELS defaults to the headline arm) (both arms, identical prompt).
   ×2 is the settled standard. Once the scenario is hardened here, the non-Opus confirmation arms run ×1 across
   the same 4 repos (§7.0 bench-SET, Prime Directive 6).
7. **Truth-check:** `python3 bench/lib/pergroup.py <repo>` - is the `dependents` group ≥ +0.50? Check adoption
   (Sense fired the tools, mcp_count > 0) and hallucination (citation_grounding) in scored.json. Objective
   cited-recall is the headline, never the LLM judge.
8. **If < target: diagnose by arm, then iterate (often $0).**
   - **Per-dep diagnostic:** tally, across runs, which gold dep ids the baseline CITED vs MISSED, and which
     Sense did. This is the single most important diagnostic - it tells you which deps separate.
   - **Gold-retarget ($0, no re-bench):** changing WHICH files are in the gold leaves the ×2 transcripts valid.
     Re-score with `bench/lib/scorer.py <run_dir> verticals/<name>/scenarios/<repo>.yaml bench` per run dir, read the per-dep
     tally, move baseline-gets-3/3 deps to a `context` group and promote baseline-misses-3/3 deps to the
     discriminator. (This is how Rails went from +0.19 to +0.56 with no extra bench.)
   - **Re-author + re-bench** only when the PROMPT/steps must change.
   - **Widen past the floor toward +0.80** - don't stop at exactly +0.50.

---

## 9. Judging the judge & the metrics

- **Objective per-group cited-recall (`pergroup.py`) on the discriminator group is THE HEADLINE.** Everything
  else is secondary.
- **The reference-blind LLM judge LIES** (blind to omission; rates incomplete answers "exhaustive"). Never use
  it as a headline. Make it reference-aware: feed it the must-find set + each target's `relation:` and grade
  covered/related per item (`relationship_audit.py`). (`feedback_judge_blind_to_omission`, `project_2903_f1_metric_dilutes_headline`.)
- **F1 / overall recall DILUTES the headline** - a big non-discriminator group both arms ace drags the overall
  delta down (Rails overall +0.19 hid a +0.56 discriminator). Report the discriminator group as the headline;
  use `context`/`contract` groups to park the deps both arms get.
- **Step-averaged `completeness` ALSO dilutes - demote it to a GATE, headline UNBOUNDED REACH.** `completeness`
  (mean per-step combined_score, bounded 0–1) compresses a +10-dependents reach gap into a +0.27 score gap
  (discourse 0.96 vs 0.69; gitlab 0.95 vs 0.69 - while dependents CITED is 15.7 vs 5.7 and 12.3 vs 2.0). The one
  discriminating step ("audit EVERY dependent") is 1/7 of the average, and the metric saturates near 1.0 so it
  cannot express a 6× reach. **Sense's value is REACH AT TOKEN PARITY, not cost** - at equal billed tokens the
  agent reaches 2.5–6.9× more dependent coverage and 7–11 dependents the baseline cites 0/3 (SENSE-ONLY reach),
  with zero baseline-only. Headline the unbounded signals - **dependents-cited COUNT + SENSE-ONLY-reach +
  coverage-per-billed-token**; use completeness only as a shape-gate (did it do the task?).
  (`feedback_completeness_dilutes_lead_reach`, `project_sense_reach_at_parity`.)
- **Always report:** adoption (did Sense fire its tools, or grep instead?), hallucination/citation_grounding
  (are cites grounded in real files? is the baseline's low recall legit, not an error/refusal?), and efficiency
  (tokens/wall/tool-calls) - these are part of "high value," and they keep the win honest.
- **STANDING RULE - no baseline-raising scorer change without an arm-asymmetry test (council 2026-07-01).** Any
  change to `gold.py`/the matcher that could RAISE the baseline arm's recall (chain-tolerant credit, line-range
  aliases, symbol/via-chain resolution, looser path-matching) is REJECTED by default - it credits the sibling/hub
  nodes the baseline reaches by reading, so it is net baseline-favorable and compresses the discriminator. It
  ships only if a real-transcript test first proves the credit is ARM-ASYMMETRIC (helps Sense, not baseline).
  Corollary: the agent OMITTING a Sense-returned dep is not a scorer bug to paper over - a SYMMETRIC drop cancels
  in the delta (accept it); an ASYMMETRIC (Sense-only) drop is a realized-value limit fixed UPSTREAM (Sense output
  prominence / scenario discrimination), never in the matcher. (`project_agent_drops_retrieved_deps_bench_pollution`.)
- **The metrics above ARE the "help the AI" gate.** Their consolidated, operational form - the single definition
  every Sense change must pass - lives in [`help-the-ai.md`](help-the-ai.md). It
  derives from this section; when the two disagree, **this manifesto wins** and that doc is corrected.

---

## 10. Calibration gotchas (each cost real time at least once)

- **ADAPT TO WEAK SUBSCRIPTIONS, NOT WEAK MODELS - the cheap-arm law.** The cheap/footnote arm fails on the
  *delivery channel*, not on model capability. The Sense arm carries ~2× the token weight of baseline (Sense
  JSON is the heavy request AND the model still reads files on top - Sense is *additive*, §3), so on a
  token-metered small subscription it is the Sense stream that crosses the throttle and gets **cut mid-answer**.
  The truncated answer still clears a lenient char gate, so it scores as a real run → a **false LOSS** - the
  mirror of the judge's false-tie blind spot (`feedback_judge_blind_to_omission`). raix/smoke proved the cheap
  *model* is fine; an uncontended re-run of the same scenario scored 0.83 where the throttled run scored 0.15.
  So: **a "loss" under a metered sub is a measurement-load artifact until you've ruled out truncation** - never a
  Sense-quality or model-capability finding. Controls: space the runs, run the heavier (Sense) arm into a FRESH
  window (not baseline-first into a spent one), tighten the gate to discard step-incomplete answers, and re-run
  any throttled repo uncontended ×2 before believing the number. **The canonical proof: the Ollama-cloud Qwen
  arm - we tried nearly everything on it, then moved to Kimi cloud, which helped not because Kimi is a smarter
  model but because the subscription is a little stronger (less throttle-prone). The fix to a weak-arm result was
  a stronger SUBSCRIPTION, not a stronger MODEL.** So when an arm keeps hitting the channel wall, switch subs
  before chasing a different model. The headline Opus-4.8 arm (Prime Directive 6)
  never throttles; this rule governs the cheap arm only. The disease behind the symptom is a real Sense fix -
  right-size structural output so it survives a narrow channel (§11, `project_small_plans_bench_throttle_saga`).
- **Gold must be retrievable in the SHOWN budgeted blast at BOTH min_confidence 0.3 AND 0.7 - the eviction
  runs BOTH ways.** The known direction: 0.7 drops low-confidence riders (`project_blast_minconf_contract_bug`;
  agents pass the documented 0.7 in the wild). The 2026-07-04 direction: **0.3 admits more competitors to the
  60-caller cap and can EVICT 0.7-confidence production gold that survives at 0.7** (django: deletion.py +
  generic/detail.py, both resolved 0.7, both gone from the shown list at 0.3). Verify over MCP stdio (the
  agent-facing output), not the CLI, at both settings, before pinning any gold.
- **Gold credit is FILE-level (gold.py matches path:ANY-line), three consequences:** (a) two gold items must
  never share a file - they free-credit each other; (b) the CONTRACT file can hold no dependent golds (every
  answer cites it); (c) a gold file's effective hardness = its EASIEST call site (one name-hint site in the
  file makes the whole item easy - grade hardness per FILE, not per site).
  (`feedback_batching_law_scenario_design`.)
- **Ambiguous central symbols need `--file`** - reopened classes / cross-language name collisions. The MCP blast
  hint steers the agent; make sure your scout used the same disambiguation.
- **Don't prime the baseline toward thoroughness** in a probe ("quantify, sample several") - it invalidates the
  test by making the baseline do work a natural task wouldn't. Ask the NATURAL question.
- **Single-run numbers are NOISE - ×2 minimum (the settled standard).** The cheap decisive preview is baseline
  ×1 + read the transcript before paying for ×2 both-arms. (Non-Opus confirmation arms then run ×1 - §7.0.)
- **Forcing adoption BACKFIRES** - tightened steering can raise Sense calls but make it slower/heavier. The
  agent's default light/targeted use is usually optimal; fix the SCENARIO, not the steering.
- **The agent OMITS retrieved deps from its answer - but do NOT "fix" it in the scorer (council 2026-07-01).**
  Opus 4.6+ "acknowledges in reasoning but omits from the final answer," so the agent drops deps Sense returned.
  A dedicated bare-enumeration step (§4.4) was TESTED (saleor+sentry) and is EFFICIENCY-only, NOT a recall fix -
  the drop is agent CONFIDENCE on deep/obscure items, not constraint-count. Crucially: a drop BOTH arms share
  (deep dep) CANCELS in the cited-recall DELTA (the headline) - accept it, do not chase it. Only an ASYMMETRIC
  drop (a Sense-ONLY dep the agent retrieved but dropped) costs margin, and its fix is UPSTREAM - Sense output
  prominence/ranking (`project_right_info_product_thesis`) or scenario discrimination - never the matcher.
  **REJECTED (council): chain-tolerant / line-range / via-chain scorer credit.** It credits sibling HUB nodes the
  baseline reaches by reading, so it is net BASELINE-favorable and compresses the discriminator. **Standing rule:
  no `gold.py` change that can raise the baseline ships without a real-transcript arm-asymmetry test.** The one
  survivor is a GOLD-AUTHORING guard (§8): resolve each cited `file:line` to its owning SYMBOL via CLI Sense (the
  index), compare arms at the symbol level, and CUT any dep both arms reach; never pin a thin delegator the agent
  won't cite (it is invisible *because* it is a thin wrapper - both arms cite the logic node instead). Transcripts
  alone MISLEAD (same symbol cited at different lines reads as two different deps).
  (`project_agent_drops_retrieved_deps_bench_pollution`.)
- **Index freshness:** `sense scan -rebuild -embed` is expensive and only needed when the SCAN ENGINE changed;
  use `ensure-index.sh`. A stale-resolver index silently breaks a bench (it cost a re-bench once).

---

## 11. Cost discipline & the $0 levers

A bench is ~$10–18 (headline ×2 both arms + reference-aware judge). Budget 1–2 per repo. Control it by:
- **Scout before authoring** (read the baseline transcript + the default-blast deps) so the gold is right first.
- **Preview gold tweaks for $0** by re-scoring existing transcripts (`scorer.py`) before paying for a re-bench.
- **Pipeline authoring while one repo benches** (~20–25 min) - author the next; bench sequentially to keep the
  numbers clean.
- **Gold-only changes need NO re-bench;** only a PROMPT/step change does.
- **Right-size Sense output so the cheap arm survives the channel.** The same heavy `sense_blast`/`sense_graph`
  dump that throttles a metered sub (§10 cheap-arm law) is the real fix: summarize-and-sample (count + by-area +
  top-N actionable with file:line) instead of dumping the full set nobody can cite. Locus `mcpio.ApplyBlastBudget`.
  This cuts cost/latency/throttle-risk AND delivers the "understand without reading dozens of files" value across
  ALL providers, not just the frontier arm. (`project_small_plans_bench_throttle_saga`.)

---

## 12. Product fixes are byproducts, not the goal

When the transcripts expose a real Sense gap (Rails shipped 3 this way: the `--file` disambiguator, the
deterministic high-fan-out blast cap, the acts_as resolver), shipping the fix is welcome - but it is a
byproduct, never a precondition for the +50pt scenario. Before shipping any fix for user value (not a bench
win): **(a) RE-ASSESS the gap is still factual in current `main`** (reproduce it concretely; the code moves);
**(b) after the fix, TEST against repos of ALL sizes, one run each, for side effects** - the resolver change
must not regress blast/graph/dead on unrelated symbols. Coverage floor applies to any new/modified file.

### 12.1 The bench is also an ACCURACY instrument - a SECOND gain, NOT a consolation for a tie (2026-07-02)
**Read Prime Directive 2 first: a tie is a MISSING AXIS you have not found yet, never a verdict, never a
stopping point.** This section does NOT license "it tied, so audit accuracy and move on" - that is the exact
premature-closure bias the directives forbid (`feedback_never_settle_reframe_tie_as_my_failure`). The
win-hunt (§8: reframe the contract, memorization axis, a deeper/scattered/non-obvious seam, a
correctness-not-coverage axis) stays OPEN. What §12.1 adds is *orthogonal*: **while you keep hunting the win,
the same repo's OUTPUT can be audited for accuracy** - an additive product gain, not a graceful exit.
LiteLLM (where we had **not yet found the winning angle** - a failure of looking, not a property of the repo)
surfaced three real accuracy gaps whose fixes shipped as PR #177 (honest call confidence, de-noised hubs,
`_next/` build-artifact filtering). The accuracy loop, two standing mechanics
([[feedback_bench_as_accuracy_instrument]] + [[feedback_spike_method_prove_or_revert]]):
- **Audit Sense's ACTUAL output**, don't just re-author the scenario: drive `sense_blast`/`graph`/`summary` over
  `sense mcp` stdio + the raw index (`sqlite3 .sense/index.db`) to find inaccuracies (nonsense hubs, junk symbols,
  wrong confidence).
- **Fix each via the SPIKE METHOD:** build minimally → verify value + no side-effects on ONE real repo (rescan if
  scan-layer) → iterate ≤3 → **REVERT if it doesn't pay** (this killed a Prisma extractor at iteration 2). One
  branch/commit per fix so the batch is decomposable and per-fix revertible.
- **VERIFY-FIRST beats assume** (half the session's diagnoses were wrong until tested): before paying, run **$0
  structural checks** - does the budgeted blast still retrieve the gold? did hubs clean up? does `sense dead` not
  regress? Only then a **no-regression cited-recall bench** (headline ×2). The $0 check proved a saleor dependents
  dip was variance (gold deps reached via reference edges the fix never touched), not a regression.
- Consolidate the co-developed fixes into one build, reindex the bench repos on it, bench together, ship one PR.

---

## 13. Honesty & publication guardrails (the trust layer)

- Arms differ ONLY by toolset; identical prompt; gold is the must-find set, not a stacked deck.
- The prompt leaks NO answer shape: no paths, symbols, counts, or tool names. Verify with `--prompt`.
- The scenario reads as a real, human-shareable maintainer task. It will be published.
- **Report where the baseline wins or ties.** Publish the harness and pin repo commits. Reproducibility is the
  trust; a bench that only flatters Sense is worthless.
- Verify maintainer handles/links LIVE before publishing; never fabricate. Tag maintainers only on a fair run.
- **Publication handoff boundary (do NOT cross it).** The numbered base articles in each vertical's
  `findings/` (e.g. `verticals/<key>/findings/`) are the SOURCE. The downstream writing project that turns
  them into published posts is **`/Users/luc/Documents/Writing/social-writing`** (e.g. `medium/drafts/rails-and-ai`),
  run by a SEPARATE session with its own writing rules. A bench/vertical session **NEVER writes to
  `social-writing`** - it produces and organizes the base articles + `media/` + the findings board.md` index,
  and hands those off. Keep the two worlds separate.

---

## 14. Definition of Done

**Per repo:** discriminator group ≥ +0.50 (favored +0.80) on the headline arm ×2, Sense adopted its tools, no
hallucinated cites, baseline floor legit (transcript-verified), efficiency reported, scenario human-readable and
leak-free, article matches the benched numbers, snapshot under `verticals/<name>/results/<repo>/`.

**Per vertical:** **exactly 4 repos** in the §7.0 composition, every shipped repo won (with ≥2 independent win
pillars), every LLM run on every repo (Opus ×2 + non-Opus ×1), the scorecard published where the community reads,
the harness public, commits pinned, baseline-wins/ties reported. 4 repos → 5 articles (1 per repo + global) →
one week. A tie that ships is a violated Definition of Done - keep hunting the axis (§2.2, §5).

---

## 15. Pointers (where the worked detail lives)

- **Scenario rules + litmus:** `scenarios/crafting.md` (this folder).
- **Article build + maintenance workflow:** `findings/workflow.md` (this folder).
- **Per-repo verdicts + tracker:** `verticals/<key>/repos.md` (stamped empty; a filled slate from a past
  campaign is deliberately NOT kept as an example - see LEDGER `stopper/scaffold-stamped-fabricated-results`).
- **Per-repo scenario authoring procedure:** `scenarios/sourcing-runbook.md` (this folder).
- **Program (which verticals, in what order):** `vertical-program.md`.
- **Methodology memories (the hard-won principles, each with its scar):**
  `feedback_i_am_the_baseline_never_settle` · `feedback_never_conclude_tie_from_proxy` ·
  `feedback_judge_blind_to_omission` · `feedback_all_scenarios_must_win_baseline_hell` ·
  `feedback_bench_model_opus_4_8` · `feedback_deep_transcript_analysis` ·
  `feedback_memorization_confound_famous_repos` · `project_2903_f1_metric_dilutes_headline` ·
  `project_rails_framework_memorization_win` · `project_small_medium_foreclosed_all_axes` (stance corrected) ·
  `feedback_adapt_to_weak_subscriptions` · `project_small_plans_bench_throttle_saga`.
