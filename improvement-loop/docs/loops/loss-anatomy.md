# Loss anatomy - every non-WIN outcome, distilled (cross-vertical, append-only)

> **What this is.** One row per recorded tie / killed slot across all verticals: the transcript-level
> reason the baseline reached parity (or the design-time reason the slot died). the admission screens'
> recalibration input ([`00-bootstrap.md`](00-bootstrap.md)) and the standing
> "what beats us" map (goal file, sensory system 3). Loop 5 appends a row per new tie/loss at harvest.
> Discipline: every row cites transcript evidence (headline-arm baseline transcripts) or the
> design-time measurement doc; no row concludes from a proxy. Backfilled 2026-07-11 from the frozen
> ruby-rails + python-django boards (headline arm = Claude Code · Opus 4.8, RUNS=2). Every row is a
> banked finding, not a live check.
>
> **Mechanism vocabulary** (bar-3's check list): `window-batching` (bulk sibling reads cover the gold),
> `covering-declaration` (one declared textual pattern enumerates it), `memorized-API` (recited from
> weights), `enumerable-surface` (directory/registry listing hands it over), `mechanized-enumeration`
> (baseline writes a throwaway script - ast.walk, closure loop - that computes the full set; depth is
> NOT grep-hostility on statically-declared hierarchies), `satisficing-shape` (prompt/gold format let
> both arms stop at the same floor), `seam-nonexistent` (design-time: no grep-invisible edges to find).
>
> **Sense-ARM failure vocabulary** (added 2026-07-22, go vertical). The list above explains why the
> BASELINE reaches parity. A different axis: on a genuine WIN scenario a WEAK model can lose on the
> SENSE arm for reasons that are not about retrieval at all. Two distinct modes, and they must not be
> merged into "the cheap arm is bad" - they take opposite fixes:
> `tool-not-reached` (the model never CALLS the tool the answer lives in; the payload is perfect but
> unrequested - fix is guidance/routing: procedural workflow rows, a load-bearing next_steps hint that
> fires on reach, not a payload change) vs `transcription-ceiling` (the model calls the tool, receives
> the full gold, and writes down a fraction - the returned-but-not-cited ceiling; a MODEL limit no
> Sense change reaches, and a RESULT not a bug). Separator: read the run's `sense-io.jsonl` for whether
> the tool was called and what its payload held, then compare to what was cited. Cross-cell corollary:
> two losing cells that look like "one anomaly" can be one of each - check the mode per cell before
> proposing a single fix.

## Ledger

### ruby-rails (board: 12W / 1T / 0L on cited_recall; 3 relation-group ties)

**llm.rb** | verdict: TIE (cited 0.48/0.48; ◆ efficiency −19% billed at parity) | mechanism:
**enumerable-surface + satisficing-shape (the 0.48 ceiling)**
The entire 27-item gold is conventionally-named file paths in a parallel provider directory layout: the
baseline's FIRST tool call (`find . -type f -name "*.rb"`) plus one `ls` loop handed over every gold
file; the claimed "grep-invisible reuse by inheritance" is a declared one-liner
(`require_relative "openai" unless defined?(LLM::OpenAI)`) at the top of each thin provider. The shared
0.48 is a citation-FORMAT floor, not retrieval: both arms named the sibling component files but only
line-cited the OpenAI exemplar per family (run-2 baseline mention_recall 0.93 vs cited 0.48; judge
covered_recall 0.889 in all four runs).
- Evidence: run-1 call 1 `find` = 7.4KB listing containing all 24 non-spec gold paths; run-1 call 4
  per-provider `ls` loop = the four "parallel component families" as directory listings; run-1 calls
  6-9 = every thin provider opens with the `require_relative "openai"` declaration; run-2 call 1
  `find lib -type f | sort` = same single-call handover. Strategies identical across runs.
- Admission bar: **bar 2** (≈0 of 27 items grep/ls-invisible) with the reuse edge also caught by
  **bar 3** (covering declaration). The satisficing tail is a SCENARIO-SHAPE lesson besides: gold that
  demands `file:line` on every sibling scores exemplar-citing agents at 0.25/group in BOTH arms.

**ruby_llm** | verdict: WIN cited +0.04, relation-group TIE | mechanism: **covering-declaration**
The gold protocol relation (which providers subclass `Protocols::ChatCompletions`) is declared textually
at a fixed site in every provider file via the `protocol :chat_completions, X` DSL line plus the literal
`class ChatCompletions < Protocols::ChatCompletions` header; one grep or one ~10KB batch-cat of the
enumerable `providers/` dir hands the baseline the entire inheritance fan-out.
- Evidence: run-1 `grep -rn "protocol :" lib/ruby_llm/providers/*.rb` returned the complete
  provider→protocol map (file:line) in one 1.3KB call; run-1 `find lib/ruby_llm/providers -type f`
  enumerated the whole family; run-2 batch-cat of all nine providers (10.2KB) surfaced every subclass
  declaration without even needing the grep.
- Admission bar: **bar 3** (covering pattern). The `protocol :sym, Klass` DSL is the pretix
  `related_name` analog: a declared covering pattern that makes "grep-invisible inheritance"
  grep-visible.

**langchainrb** | verdict: WIN cited +0.17, relation-group TIE | mechanism: **enumerable-surface**
The four-way per-provider correspondence is fully visible in a single `ls -R` (the naming convention IS
the relation), and the entire dispatch is one 28-line `adapter.rb` `is_a?` chain; the baseline read the
whole gold surface in ~30 tiny file reads.
- Evidence: run-1 `ls -R lib/langchain/assistant/ && ls -R lib/langchain/llm/` (1.1KB) exposed all four
  name-paired families at once; run-1 read of `assistant/llm/adapter.rb` (28 lines) = the complete
  dispatch gold group in a sub-1KB read; run-2 reproduced the same one-shot enumeration then batched
  `grep -n "def build_"` across five adapters.
- Admission bar: **bar 2** (grep-invisible count ≈ 0). Every family member is directory-enumerable by
  naming convention; the dispatch is a single covering file. "Nothing but the provider-name convention
  links the files" was the give-away: the convention is a textual key.

Cross-cutting (both gems): baselines finished the relation surface inside ~26-34 tool calls with 1-10KB
results and no truncation. On small gems the whole relevant surface fits in a handful of windows; Sense's
cited-recall wins there came from efficiency/precision, not from unreachable relations.

### python-django (board: 3W / 3T control + sentry ◆; 0L)

**sentry** | verdict: cited 0.85/0.88 (+0.03) recall tie, ◆ efficiency-at-parity (billed −7%, wall
−27%); dependents +0.10 on the on-disk Jul 7 pair. _Provenance note (resolved 2026-07-11, the owner):
report.json's +0.60 traced to earlier sessions whose archives were deleted as temp cleanup; the
standing repo-side cell is the on-disk one; sentry's win-class standing rests on the three
confirmation arms (Kimi +0.24 / Devstral +0.29 / GPT-5.5 +0.35). See
`sentry-provenance-incident-2026-07-11.md`._ | mechanism: **covering-declaration (field-shape)**
The baseline never walked the 269-file import haystack the scenario was built to punish (its two
attempts at the import-grep failed on shell errors and were abandoned). Instead it grepped the
contract-embedding SHAPE itself: an indent-anchored `^    group: Group` class-field pattern that
transcribes most of the gold with exact line numbers in one 15KB call, plus `self.group = ` assignment
greps and `-vE` blacklists of the ~25 GroupXxx satellite names. Zero bulk file reads on a giant repo.
- Evidence: run-1 call 9 (the field-shape grep: 3/5 discriminator deps + 3 context deps, verbatim with
  line numbers); run-1 calls 6-7 (the abandoned import-walk); run-2 call 10 (same idea unanchored +
  truncated at head -60 → dropped dep:rule-history: truncation-fragility is the baseline's real
  weakness). Misses were DISJOINT, not shared: baseline missed the no-field dependent
  (ctx:rest-details); sense missed famous context surfaces (ctx:serializer, ctx:slack-builder) while
  relaying blast's contract-embedding deps - symmetric context drops cancel on overall cited, only the
  dependents cut exposes the gap.
- Admission bar: **bar 3**, with a sharpening: admission analysis asserted "no ≤3-pattern cover" against
  the IMPORT cover, but the DECLARED FIELD SHAPE (`group: Group`) is a one-pattern near-cover. Bar 3
  must test declaration shapes (typed fields, accessors, DSLs), not just import/name covers. The
  winning lever was gold curation: demote the deps the baseline always finds via the cover to context,
  keep as dependents the ones that drop under truncation/noise (sentry.yaml lines 248-267).

**wagtail** | verdict: TIE (control, 0.88/0.88) | mechanism: **enumerable-surface**
A single listable package whose dependent set lives in nameable directories (`admin/views/pages/`,
`admin/views/reports/`, `contrib/*`): `ls` plus sibling for-loop greps transcribe the gold; even the
"import-dark" lazy-string FK fell to one relational-field grep (`ForeignKey|ParentalKey` × `page` →
`sites.py:108 root_page`). Run-1's three misses were admin surfaces it enumerated but never opened
(satisficing, not reachability); run-2's batch loops closed them at 13/13.
- Evidence: run-1 calls 15-16 (pure `ls` geography mapping); run-1 call 13 (the FK grep); run-2 calls
  22-23 (sibling for-loops over `views/pages/*` and `views/reports/*`, exactly where the misses lived);
  run-2 calls 14-15 (4KB full prod file list, then walked).
- Admission bar: **bar 2, and it DID predict this**: the medium-slot dig measured `Page.objects` covering
  29/37 prod matches and the only 3 grep-dark dependents absent from blast∪graph at both confidences -
  no recall discriminator exists by measurement. Banked deliberately as the honest §13 control; the
  designed, expected published row.

**healthchecks** | verdict: TIE (control, 1.00/1.00) | mechanism: **enumerable-surface**
The `hc/integrations/<name>/transport.py` file-layout convention makes the whole subclass set enumerable
by one find/glob plus one layout-shaped `^class` grep, which returns every implementation WITH its true
base, neutralizing the Slackalike-intermediate trap the scenario was built on (the grep is keyed on
layout, not base name, so "class line never contains Transport" never engaged).
- Evidence: run-1 call 2 `find hc -name "transports.py" -o -name "transport*.py"` = 29 files, 18/18
  gold dependents before any reasoning; run-1 call 4 `grep -rn "^class " hc/integrations/*/transport.py`
  = 18/18 with bases verbatim (`Discord(Slackalike)`, `Mattermost(Slackalike)`); run-2 closed the same
  closure twice independently.
- Admission bar: none - honest ballast by design (§7.0 small slot), hypothesis CONFIRMED (refined: the
  per-integration file convention, not a single module, is the enumerator). An enumerable-surface check
  would have flagged it pre-bench had ballast not been deliberate.

**litellm** | verdict: TIE (control, 1.00/1.00; the PR #178 directed-inheritance honest-tie negative
control) | mechanism: **mechanized-enumeration** (run-2) / batched closure greps (run-1)
No registry handed over the gold: hop-1 grep found 0/16 dependents (the textual wall worked as
designed). The frontier baseline closed the transitive closure itself: run-1 with ONE batched for-loop
grepping each named intermediate base (16/16 in a single call, then 3 more recursion levels without
dropping a branch); run-2 by WRITING AN AST SCRIPT (`ast.walk` over `litellm/`) that computed the
reachable-from-BaseConfig closure programmatically (187 classes, 16/16 gold, name/file:line/bases).
- Evidence: run-1 call 4 (hop-1 wall: 0/16), call 5 (batched loop: 16/16 including the databricks
  diamond); run-2 calls 6-8 (`/tmp/audit2.py`, "all reached: 187"); run-2 call 5 (independent wall
  confirmation, ~100 files, none of the 16).
- Admission bar: none - honest ballast tie by design, hypothesis REFUTED in mechanism (no registry) but
  CONFIRMED in substance: every hop is one named-identifier grep away, and statically-parseable class
  declarations make ANY depth $0-enumerable to an agent willing to script. Transitive DEPTH alone is
  never grep-hostility; this is now a bar-3 law.

### go (board 2026-07-21: 16 cross-agent cells + 4 headline seats; 1L, 3T - all four on the two ollama-cloud arms)

> Harvested at Loop 5 on 2026-07-21, after the validity reclassification and the artifact exclusion
> (`bench/lib/run_validity.py`). Every row below is a `run-1` x1 confirmation cell and carries the
> OPEN flag; the headline arm (claude-opus-4-8) has NO loss and no tie on this board.

**ollama-cloud/glm-5.2 · consul** | verdict: **LOSS** (cited 0.62 baseline -> 0.38 sense, -0.23) |
mechanism: **subset-collapse under substitution, NOT an index gap**
The sense arm returned a strict SUBSET of its own baseline: baseline cited 8/13, sense cited 5/13, and
all 5 are among the baseline's 8 (`reporting-mgr`, `local-state`, `raft-forwarding` dropped). Tool
counts say what happened: baseline 33 greps + **45 reads**, sense 32 greps + **18 reads** + 5 Sense
calls. It swapped 27 file reads for 5 structural calls and then stopped, at the same answer length
(55,208 vs 55,458 chars). Killer run: the missing items are NOT unreachable - claude-opus-4-8,
gpt-5.5 and kimi-k3 all cite 13/13 on this same scenario against the same index. The index had them;
this model stopped asking.

**ollama-cloud/glm-5.2 · nomad** | verdict: TIE (0.47/0.47) | mechanism: **same score, different items**
Not a parity of content: the sense arm cites `auth-encrypter` (an interface-narrowed holder) that the
baseline misses, and drops `volume-watcher` that the baseline gets. Both arms miss the SAME seven hard
items (`allocrunner`, `csi-hook`, `alloc-watcher`, `arconfig`, `client-config`, `widmgr-signer`,
`plan-builder`), so the discriminator group is unreached by both and the tie is a shared ceiling.

**ollama-cloud/mistral-large-3:675b · dolt** | verdict: TIE (0.25/0.25) | mechanism: **shared ceiling
at full adoption**
Sense arm: adoption 0.90, 8 Sense calls, **zero greps and zero reads** - it queried structure and wrote
the answer without opening a file. Same score as a baseline that made 2 greps + 4 reads, and again a
different 3 of 12 (`datas-roottracker` in, `dbfactory-file` out). mention_recall == cited_recall (0.25)
on both arms, so this is NOT a citation-discipline floor: the nine missed items never appear at all.

**ollama-cloud/mistral-large-3:675b · pebble** | verdict: TIE (0.14/0.14) | mechanism: **shared ceiling
at full adoption**
Sense arm: adoption **1.00**, 3 Sense calls, zero greps, zero reads, and a 34,956-char answer (baseline
20,691) that cites the same single item. All six `dep:` discriminator items missed by both arms,
mention == cited on both.

**HYPOTHESIS RAISED AND KILLED THE SAME SESSION (recorded so it is not re-raised).** From these four
cells I proposed "Sense wins as a TARGETING layer for follow-up reads; arms that query Sense and then
stop score worst." Run across all 38 sense-arm runs on the board, the aggregate REVERSES it: cells at
cited >= 0.90 average **6.6** Sense calls and **18.5** follow-up grep+read, while cells below 0.50
average **8.6** Sense calls and **26.6** follow-ups. The sub-bar arms called Sense MORE and read MORE.
Counter-examples inside the same board: gpt-5.5/dolt scores 1.00 on 9 Sense calls with ZERO follow-up,
and glm-5.2/pebble scores 1.00 on 3 Sense calls with 109 follow-ups. Tool mix and adoption score do
not predict recall.

**What the confound check leaves standing.** Cross-tabbing sub-bar cells against the model: of 38
sense-arm runs (artifact excluded), **33 sit at >= 0.92 and all five below it belong to the two
ollama-cloud arms** - claude-opus-4-8 20/20 at 1.00, kimi-k3 4/4 at 1.00, claude-fable-5 2/2 at 1.00,
gpt-5.5 min 0.92. The separator is the MODEL, not the tool mix. It is not a flat capability ceiling
either: both ollama arms reach 1.00 on one repo each (glm on dolt and pebble, mistral on nomad), so
the sub-bar cells are within-model variance on the harder seats, not an inability to use the index.
Loop-2 recalibration input: a weak-model arm can LOSE a cell by substituting structural calls for
reading (the glm/consul subset-collapse), which the "sense never hurts" prior did not predict.

**RESOLVED 2026-07-22 - the two ollama LOSS/near-tie cells reclassified into the two Sense-arm failure
modes above, and one was FIXED in the product (PR #230, v1.13.1).** The earlier rows read these as
model-variance ceilings; a deeper look split them:
- **glm/consul was `tool-not-reached`, not a ceiling.** The model was not calling sense_blast (5 Sense
  calls, none blast, in the run that scored 0.38), and when it did the completeness verdict disowned the
  block holding 100% of the gold (the retention ring, `retained_via_interfaces` - a real accounting
  defect, D1/D2). Fixing the ACCOUNTING alone did not move it (d-spike: 7/13, 11/13); adding the
  GUIDANCE half (a retention-audit workflow row + next_steps made load-bearing so the blast hint fires
  on reach = shown + low-confidence-hidden) converted it to **13/13 twice** on the release-validation
  binary. This is the first recorded case of a weak-model Sense LOSS closed by a PRODUCT fix rather than
  a scenario recut.
- **mistral/dolt was `transcription-ceiling`, confirmed and left as a RESULT.** It calls blast, receives
  12/12 gold in the payload, and cites 2-5. The same fix that converted consul did NOT move it (5/12,
  2/12), which is the proof that these are two different failure modes and not one anomaly. No Sense
  change reaches a model that is handed the answer and does not transcribe it.
- **Method note:** the separator was reading each run's `sense-io.jsonl` for tool-called +
  payload-held vs cited. A payload probe alone proved the accounting defect but NOT its causal role; the
  falsifier run (fix + re-bench) is what told the two modes apart. See `decision-errors.md` 2026-07-22.

### Killed / swapped slots (design-time, $0 - no paid transcripts; measurement docs are the evidence)

**haystack** (deepset-ai/haystack, swapped 2026-07-06) | mechanism: **seam-nonexistent**
Of 1,256 cross-file prod→prod edges at ≥0.7 confidence, **0** are grep-invisible: every dependency name
is literal in the dependent's own text; every citable set reduces to one pattern derivable from the
contract name (transcription-distance 0). The hub-embed shape was also dead (2 Document fields, 0
ChatMessage).
- Source: the python-django vertical's `repos.md` (internal tree, `repos.md` (private tree) §948-963) (the 1,256 sweep).
- Admission bar: **bar 2** - this case IS bar 2's calibration anchor (must-FAIL).

**pretix** (pretix/pretix, promoted then killed after four dead framings) | mechanism:
**covering-declaration (LAW)**
The contract-mapping step hands the baseline the accessor pattern (`related_name="teams"` declared as
text one step before enumeration), making the transcription-distance condition structurally
unsatisfiable; in a 981-file app every covering pattern (contract token, declared accessor, FK list) is
derivable, and baseline audits finish in ~290s.
- Source: `repos.md` §1158-1173 (Team related-manager kill; four dead framings).
- Admission bar: **bar 3** - this case IS bar 3's calibration anchor (must-FAIL).

**django/django** (framework slot, abandoned after SIX dry-run-killed angles, ~$7.68, never a paired
spend) | mechanism: **memorized-API + window-batching**
The frontier baseline batch-clears every judgment shape (receiver windows, config-enumerable discovery,
dir-read confirmation, per-point verification), and the public QuerySet API is recited from weights. The
central-symbol recipe was exhausted; the slot was recomposed per §7.0 (django/django OUT, NetBox
promoted).
- Source: `repos.md` §545-554 + §17-20; six-kill record = the methodology doc; never re-run the six.
- Admission bar: **bar 5** (memorization screen) + bar 2 on the batchable shapes.

**Adjacent design-time kills** (same sweep family, recorded for the pool): DSPy 0/470 grep-invisible
edges (`repos.md` line 211/1011); LangChain scoped out (slim live core, max memorization).

## Standing observations (recalibration input for admission)

1. **The small-gem/ballast lane is consistent:** every small-slot tie so far is enumerable-surface or
   covering-declaration. §7.0 ballast behaves as designed; bar 2/3 numbers should still be RECORDED at
   admission so the tie is interpretable (wagtail's control win shows ballast can surprise).
2. **Covering declarations are the #1 killer of win-pillar candidates** (pretix, ruby_llm's DSL): bar 3
   must scan for DSL/accessor/registry declarations that textually enumerate the gold, not just for the
   gold names themselves.
3. **Design-time kills are cheaper than crafting-time kills which are cheaper than benched ties:**
   haystack cost a sweep, pretix cost four framings, the gems cost paid runs. The gate exists to move
   every future kill to the leftmost column.
4. **Mechanized enumeration is the frontier baseline's ceiling move (litellm run-2):** any hierarchy
   whose edges are DECLARED in statically-parseable text is $0-enumerable via a throwaway ast.walk
   script, regardless of depth. Bar 3 must therefore ask "is the edge declared anywhere in text?" and
   not "how many hops away is it?". Go-vertical consequence: declared hierarchies (struct embedding,
   explicit interface assertions) will not discriminate; implicit interface SATISFACTION (never declared,
   `satisfy.go`'s inference) is the seam class an AST script cannot close cheaply - the fan-question
   axis bet, now backed by transcript evidence.
5. **Citation-format satisficing produces identical sub-floor ceilings in BOTH arms (llm.rb 0.48):**
   when gold demands file:line per sibling and agents exemplar-cite one per family, the tie is
   scenario-shape, not tool value. Gold curation must either accept exemplar citation or the prompt must
   demand per-sibling lines explicitly; otherwise the discriminator measures citation habits.
6. **Bar 3 must test DECLARATION SHAPES, not just import/name covers (sentry):** a typed class field
   (`group: Group`), a declared accessor (`related_name`), a DSL line (`protocol :x, K`), or a file-layout
   convention (healthchecks) each one-pattern-covers gold that the import graph says is scattered. The
   covering-pattern scan enumerates candidate declaration shapes from the contract type's usage, then
   asks whether one indent-anchored grep transcribes the gold.
7. **The baseline's real weaknesses, visible even in ties: truncation-fragility and salience-drop under
   noise** (sentry run-2's `head -60` dropped rule-history; run-1's shell errors killed the import walk).
   Gold that survives the covering pattern but sits past a truncation horizon or below satellite-name
   noise is where Sense's margin lives on giant repos - the chatwoot/sentry shape, now stated as a law.
8. **Sense-arm misuse shows up inside ties too (sentry):** the sense agent missed famous context
   surfaces (serializer, slack-builder) both runs while faithfully relaying blast's deps - the
   agent-drops-context sibling of the known drops-retrieved-deps pollution. A misuse-audit angle
   (context-vs-relay balance), not a scorer fix.
