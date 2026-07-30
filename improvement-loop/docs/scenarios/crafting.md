# Scenario crafting - the heart of the bench

> **Scope:** this is the durable PHILOSOPHY (the principles that don't change). The CURRENT, executable
> method - the validated baseline-hell recipe, per-repo-type playbook, calibration, and workflow - lives in
> **`sourcing-runbook.md` (this folder)** (this folder).
> Read those to act; read this to understand why.
>
> **↑ CONSOLIDATED into the program-level bench bible: [`../manifesto.md`](../manifesto.md)**
> - the authoritative rulebook across ALL verticals (prime directives, win-axis catalog, no-separate shapes,
> the per-repo loop, judging the judge, calibration, cost, Definition of Done). When it disagrees with this
> file or an older prompt, the manifesto wins.

**The one truth, learned the hard way:** the scenario IS the result. A scenario that does not reflect a
**real maintainer task** lets a capable model (especially a frontier one) brute-force the answer with
grep/ls/read, and the baseline ties or wins. That is a measurement artifact, not reality - Sense helps on
real work, every day, every repo size. The scenario's only job is to make that real value measurable. **If
the baseline ties, the scenario is wrong - fix the scenario, never conclude "Sense can't win this repo."**
(This was disproven twice: see `repos.md` - discourse went −0.10 → +0.60, forem tie → +0.56, purely by
changing the scenario's contract.)

## What Sense is actually better at - design every scenario toward these

The axes where Sense lifts you past plain grep, and which a real scenario must exercise:
- **Blast radius** - the COMPLETE transitive change-impact set across the codebase, in one call.
- **Graph** - precise callers / callees / inheritance, not noisy text matches.
- **Deep structural analysis** - relationships grep cannot cleanly resolve: callers reached through
  associations, concerns, polymorphism, wrappers, dynamic dispatch, cross-file inheritance.
- **Semantic search** - find by meaning, not by string.

## Why the baseline-hell shape wins (the validated mechanism)

A frontier baseline greps/reads/`ls`'s its way to any answer that is named by one literal token, colocated in
one directory, or composed inside one god-file. It CANNOT cheaply assemble the **complete, transitive impact
set of a central model whose dependents are heterogeneous (each touches it a different way), non-obvious (the
file doesn't announce the dependency), and scattered across many directories under a noisy token.** That set is
what `sense_blast <CentralModel>` returns in one call and a hand audit reconstructs incompletely under session
load. Make the task demand exactly that set.

## The rules (every scenario, every repo)

1. **A real CHANGE-IMPACT task.** "You are about to rework the <central model> contract; audit every dependent
   before touching it." Not "add an X by copying the others" (enumerable → grep/ls turf).
2. **Repo size does not exempt - and FAME is a second, separate exemption you must defeat.** Small repos are
   HARDER (a frontier baseline can read the whole thing): go deeper, target deps reached via dynamic dispatch /
   2-hop chains, accept a tighter but ≥+0.50 margin (lobsters is the worked case). FAMOUS,
   heavily-trained-on codebases (frameworks, popular libraries - Rails is the worked case) are hard for a
   DIFFERENT reason: **the baseline has MEMORIZED the public API and recites it from weights without opening a
   file**, so completeness TIES on every documented/famous dependent no matter how scattered. Two consequences:
   (a) the contract must live in a BIG, UNREADABLE subsystem (Rails won on `ActiveRecord::Relation` in the
   401-file activerecord/lib; the SAME recipe TIED +0.05 on `ActiveStorage::Blob` because its gem is only 82
   files the baseline reads whole - it is the GEM/subsystem size, not the repo size, that must exceed readable);
   (b) the discriminator gold must be the OBSCURE, NON-memorized internals (Rails: the query-compilation plumbing
   - where_clause/predicate_builder/from_clause/join_dependency/preloader), with the famous public-API deps moved
   to a non-discriminator `context` group so `pergroup.py` isolates the real gap. You FIND which deps are
   memorized only by reading the per-dep tally across the ×3 transcripts (baseline cites 3/3 = memorized →
   demote to `context`; baseline misses 3/3 = the discriminator). Full account: project memory
   `project_rails_framework_memorization_win`.
3. **Heterogeneous, scattered, non-obvious dependents** - the discriminator gold. Center on what a hand audit
   misses: deps in disjoint dirs, reached via association/concern/derive/delegate, under a token too noisy to
   grep cleanly. Never on enumerable facts (an `include` list, an `ls`-able dir, a naming convention).
4. **Neutral prompts AND description.** State the task only. Never the answer's shape, locations, symbol names,
   counts, or which tool to use. Verify with `python3 bench/lib/scenario.py verticals/ruby-rails/scenarios/<repo>.yaml --prompt` and
   read what the agent sees.
5. **Verified, default-retrievable gold.** File-path-matched relational targets (so both mention and cited
   score), built from `sense_blast <Model>` at the agent's DEFAULT settings (NOT `--min-confidence 0.3`, or the
   agent can't retrieve its own gold), each carrying a `relation:` field for the reference-aware judge.
6. **Force exhaustive per-dependent citation.** The audit step must demand file:line for EVERY dependent
   ("do not collapse a set into 'various services'") - that transcription discipline is what makes sense
   consistent across runs.
7. **Measure the discriminator.** Headline = objective per-group cited-recall (`pergroup.py`) on the scattered
   `dependents` group, ≥+0.50 vs baseline (the strict minimum; widen it). Report efficiency and hallucination
   too. Rigor: `--runs 3`, the headline arm, the pinned judge (`arms.txt`), both arms the identical prompt.

## The litmus (apply before locking any scenario)

> Would a thorough grep + ls + read by a frontier model produce the same answer at similar cost?

If yes, the scenario is wrong: pick a more central model, a more heterogeneous/scattered/non-obvious dependent
set, a noisier token, until the map has to pay for itself. A scenario a strong model can grep its way through
measures the case where Sense is least needed and lies about the daily reality.

## Note on the instrument

The headline path is **single-prompt over the 7-step scenario** (`bench/drivers/runs-variance.sh`) - the trustworthy
path, and it WINS on the frontier model when the task is baseline-hell. The earlier belief that single-prompt
structurally can't show Sense's value (so a multi-turn "session bench" was required) is superseded; multi-turn
is deferred (the runner still degrades to grep on per-turn MCP re-init). Do not block on it.
