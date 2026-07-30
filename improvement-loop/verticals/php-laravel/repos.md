# PHP / Laravel Vertical - Repo Selection (Step 0)

The repo-selection deliverable (manifesto §1 + §7): the one
manual judgment gate the bootstrap does NOT automate. Method:
[`../../docs/scenarios/sourcing-runbook.md`](../../docs/scenarios/sourcing-runbook.md).

> **The SET is 4 repos, firm** (manifesto §7.0): `1 framework + 1 big + 2 medium`
> (or `2 big + 2 medium` when the framework is too small/memorized). Each slot carries a
> same-type backup; a swap is the LAST resort.

## The firm 4-repo set

Composed and admitted by `bash bench/drivers/loop2-hunt.sh php-laravel --write` - no human confirm
(the admission sign-off retired 2026-07-29). 326 anchors gated across 14 pooled repos, 13 ADMIT.

| Slot | Repo | Anchor | Gate cell | Backup |
|---|---|---|---|---|
| framework | `filament` | `HasSchemas` | ADMIT, affected 1100, precision 0.46 | `october` `Blueprint` |
| big | `snipe-it` | `Presentable` | ADMIT, affected 336, precision 0.957 | **none - see below** |
| medium 1 | `akaunting` | `Jobs` | ADMIT, affected 729, precision 0.154 - **win signature** | `lychee` `InternalLycheeException` |
| medium 2 | `invoiceninja` | `ActivityRepository` | ADMIT, affected 606, precision 0.067 - **win signature** | `statamic` `ContainsQueryableValues` |

Two slots carry the full win signature (no usable cover + precision ≤ 0.3 + total_affected ≥ 500).
Anchors are traits, contracts and repository classes - never Eloquent models, which all die on K7.

SHAs pinned in `PINNED_COMMITS.json`; `repos.txt` lists the four keys; the slot map + backups are
`slate.json`, verified by `bench/lib/slate_check.py php-laravel` (SLATE STANDS: §7.0 composition,
pins, both arms at identical SHAs, indexes all present).

**This slate superseded two earlier ones the same day**, and the reasons are the instrument's, not the
pool's: a hand-composed set (four GRAY cells, zero win signatures) died when the driver gated the full
ranking instead of anchors picked by eye, and a driver-composed set (bagisto framework / lychee medium)
died when bar 5 stopped accepting model refusals as ignorance. Both corrections are recorded in the
`stopper/loop2-compose-slot-rule` and `stopper/loop2-bar5-noise` ledger entries. What changed each time
was WHICH admitted anchor filled a slot - never whether the slate composes, and never a bar's threshold.

**The big slot has no backup, and this is a measured pool fact, not a shortfall of hunting.** 28
PHP/Laravel repos were sized from their own indexes; exactly two clear the 4000-prod-file big floor -
snipe-it (7789) and filament (4942) - and filament holds the framework slot. Next three: akaunting
(3855), invoiceninja (3680), lychee (2775). Seven further candidates were cloned and measured too
small: monica (1133), winter (919), pterodactyl (882), freescout (787), backpack/crud (626),
orchid/platform (349), cachet (35). The floor was NOT moved to make akaunting fit - that is the "never
lower the bar to fill a slot" rule. If snipe-it fails in Loop 3, the doc's order applies: re-author the
scenario or fix Sense first; a swap has nowhere to go.

**Bar 5 is the one probabilistic step, and its limits are known.** It calls a model, so it is sampled
max-of-3 (memorization is a capability: one refusal is a false negative). Calibrated on this stack:
laravel `Model` 0.79-0.83 recall over 122-of-154 members and filament `HasTable` 0.63 over 20-of-46
both reject; every admitted anchor scores 0.0, or is inapplicable for declaring fewer than 5 members.
Two floors exist because both failure modes were measured here: a truth set under 5 members makes the
recall fraction a coin flip (snipe-it `Presentable` has ONE member, `present`, and the model using that
ordinary word scored 1.0), and a fraction without an absolute hit count credits ecosystem vocabulary as
recitation (akaunting `Jobs`: 0.4 recall from `dispatch` + `dispatchSync` alone). Residual, stated
plainly: borderline anchors can still flip between runs - bagisto `Repository` sampled
[0.0, 0.545, 0.0] - which can change which admitted anchor fills a slot. The deterministic bars do not
flip: 326 of 326 measurement cells were byte-identical across consecutive cold runs.

**Reproduction, measured after the fixes.** Two consecutive cold runs (`.gate-cells` wiped, identical
settings) agreed on 321 of 322 cells, a 0.3% flip rate on one non-slate anchor
(`filament` `HasActions`), and produced the SAME SLATE - all four slots and all three backups
identical. Before the fixes the same test disagreed on 3 of 326 cells and moved the big slot's anchor.

The outcome of a repo is the loop's to determine, not this file's: no repo is labelled a win,
a pillar, or win-eligible before a real test has run (`decision-errors.md` - the WIN-VIABLE
label that eight repos carried into Loop 3 before dying there).

## Admission-gate measurements (2026-07-29, `bench/lib/admission_gate.py`)

Pool: the 19 PHP/Laravel clones already under `$SENSE_CLONES`. Ten were re-indexed first
(`bench/lib/ensure-index.sh`) - 17 of 19 pooled indexes carried a pre-2026-07-17 scan fingerprint,
so every number below post-dates the last PHP resolver fix. `sense` 1.13.2, schema v5.
Hubs enumerated from `sense_edges` (non-test, non-vendor, class/interface/trait, ranked by distinct
dependent files); `User` and framework `Controller` bases skipped as hub-explosion (manifesto §6).

**Verdicts are per CONTRACT, not per repo** - 52 anchors across 10 repos. 8 GRAY (no kill fires,
win signature incomplete), 42 BALLAST-ONLY, 2 MEASUREMENT-INVALID, 1 bar-4 FAIL. GRAY cells first,
then by volume.

| Repo | Anchor | dep files | total_affected | token precision | verdict | kills |
|---|---|---|---|---|---|---|
| `filament` | `HasSchemas` | 98 | 1100 | 0.460 | GRAY | - |
| `filament` | `InteractsWithSchemas` | 55 | 1023 | 0.686 | GRAY | - |
| `statamic` | `Augmentable` | 47 | 707 | 0.474 | GRAY | - |
| `october` | `Theme` | 43 | 425 | 0.177 | GRAY | - |
| `snipe-it` | `Presentable` | 66 | 336 | 0.957 | GRAY | - |
| `snipe-it` | `Searchable` | 42 | 297 | 0.786 | GRAY | - |
| `filament` | `HasTable` | 23 | 279 | 0.195 | GRAY | - |
| `october` | `Blueprint` | 48 | 254 | 0.136 | GRAY | - |
| `statamic` | `Event` | 44 | 1176 | 0.140 | BALLAST-ONLY | K7 |
| `filament` | `Field` | 52 | 969 | 0.090 | BALLAST-ONLY | K7 |
| `statamic` | `Tags` | 38 | 787 | 0.248 | BALLAST-ONLY | K7 |
| `statamic` | `Fieldtype` | 16 | 782 | 0.058 | BALLAST-ONLY | K4+K7 |
| `filament` | `Action` | 49 | 690 | 0.096 | BALLAST-ONLY | K7 |
| `snipe-it` | `Setting` | 39 | 601 | 0.055 | BALLAST-ONLY | K7 |
| `snipe-it` | `Helper` | 43 | 579 | 0.177 | BALLAST-ONLY | K7 |
| `akaunting` | `Job` | 87 | 544 | 0.416 | BALLAST-ONLY | K1+K7 |
| `snipe-it` | `Actionlog` | 51 | 509 | 0.500 | BALLAST-ONLY | K1+K7 |
| `pixelfed` | `AccountService` | 37 | 506 | 0.363 | BALLAST-ONLY | K1+K7 |
| `snipe-it` | `Asset` | 54 | 497 | 0.029 | BALLAST-ONLY | K7 |
| `bookstack` | `Entity` | 46 | 403 | 0.187 | BALLAST-ONLY | K7 |
| `akaunting` | `Event` | 100 | 390 | 0.252 | BALLAST-ONLY | K1+K7 |
| `snipe-it` | `Company` | 56 | 364 | 0.096 | BALLAST-ONLY | K7 |
| `snipe-it` | `SnipeModel` | 41 | 296 | 0.750 | BALLAST-ONLY | K7 |
| `koel` | `Song` | 49 | 293 | 0.112 | BALLAST-ONLY | K7 |
| `statamic` | `CpController` | 27 | 247 | 0.293 | BALLAST-ONLY | K2+K4+K7 |
| `snipe-it` | `License` | 36 | 244 | 0.035 | BALLAST-ONLY | K2+K7 |
| `statamic` | `Value` | 26 | 241 | 0.050 | BALLAST-ONLY | K2 |
| `pixelfed` | `Profile` | 66 | 239 | 0.102 | BALLAST-ONLY | K2+K7 |
| `bookstack` | `Page` | 34 | 224 | 0.056 | BALLAST-ONLY | K2 |
| `akaunting` | `Transaction` | 51 | 208 | 0.128 | BALLAST-ONLY | K2 |
| `october` | `WidgetBase` | 30 | 202 | 0.367 | BALLAST-ONLY | K1+K2 |
| `firefly-iii` | `TransactionJournal` | 46 | 195 | 0.275 | BALLAST-ONLY | K2+K7 |
| `pixelfed` | `Status` | 67 | 192 | 0.168 | BALLAST-ONLY | K2+K7 |
| `october` | `FormWidgetBase` | 22 | 175 | 0.739 | BALLAST-ONLY | K2 |
| `statamic` | `AbstractNode` | 86 | 175 | 0.837 | BALLAST-ONLY | K1+K2+K7 |
| `pixelfed` | `Helpers` | 45 | 171 | 0.840 | BALLAST-ONLY | K1+K2+K7 |
| `october` | `PluginManager` | 49 | 169 | 0.919 | BALLAST-ONLY | K2 |
| `firefly-iii` | `AccountRepositoryInterface` | 58 | 161 | 0.518 | BALLAST-ONLY | K1+K2+K5+K7 |
| `akaunting` | `Modules` | 41 | 142 | 0.396 | BALLAST-ONLY | K1+K2+K7 |
| `filament` | `Component` | 64 | 113 | 0.079 | BALLAST-ONLY | K2 |
| `filament` | `HasEmbeddedView` | 91 | 96 | 0.935 | BALLAST-ONLY | K2 |
| `pixelfed` | `Media` | 31 | 91 | 0.095 | BALLAST-ONLY | K2+K7 |
| `akaunting` | `Document` | 57 | 74 | 0.189 | BALLAST-ONLY | K2+K7 |
| `bookstack` | `Loggable` | 35 | 70 | 0.480 | BALLAST-ONLY | K2 |
| `filament` | `Schema` | 23 | 68 | 0.044 | BALLAST-ONLY | K2 |
| `october` | `VueComponentBase` | 45 | 64 | 0.976 | BALLAST-ONLY | K1+K2 |
| `statamic` | `ProvidesCommitMessage` | 52 | 62 | 0.963 | BALLAST-ONLY | K1+K2+K7 |
| `october` | `SettingsManager` | 24 | 35 | 0.679 | BALLAST-ONLY | K2 |
| `filament` | `Table` | 3 | 7 | 0.007 | BALLAST-ONLY | K2+K4+K5 |
| `statamic` | `GraphQL` | 0 | 0 | 0.000 | MEASUREMENT-INVALID | - |
| `statamic` | `Site` | 0 | 0 | 0.000 | MEASUREMENT-INVALID | - |
| `statamic` | `Statamic` | - | - | - | bar-4 FAIL | - |

Controls run the same day on the same binary: `sentry` `Group` = WIN-VIABLE (affected 1462,
precision 0.036); `dolt` `DoltDB` = BALLAST-ONLY on K1 (token cover 0.925 at precision 0.517, the
GO-NAMING LAW class it is named for).

**What the numbers say, bounded to what was measured** (52 anchors, 10 repos, one stack, one day):

- **Every Eloquent-model anchor dies on K7.** The killing pattern is a namespace-PREFIX
  (`use App\Models\`, `use App\`) at cover ≈ 1.0. In the flat-namespace apps it is not
  anchor-specific: for pixelfed `Status`, `use App\` means "any file importing anything from the
  app" (522 hits, 67 deps, precision 0.13). K7 kills on cover alone by design - its premise is that
  file-level cited credit lets a baseline dump the whole superset and still score full recall.
- **The survivors are contracts and traits, not models.** All 8 GRAY cells are interfaces
  (`HasSchemas`, `Augmentable`, `HasTable`), Concerns (`InteractsWithSchemas`) or model traits
  (`Presentable`, `Searchable`) - dependents satisfy them by `implements`/`use` inside the class
  body, so no import prefix enumerates the set.
- **No cell carries the full win signature** (no usable cover + precision ≤ 0.3 + affected ≥ 500).
  The two halves split cleanly: `october` `Blueprint` (precision 0.136, affected 254) and
  `filament` `HasTable` (0.195, 279) have the precision; `filament` `HasSchemas` (1100) and
  `InteractsWithSchemas` (1023) have the volume at precision 0.46-0.69.
- **K7's escape hatch cannot open in PHP.** It reads `retained_via_interfaces` from blast, a field
  absent from PHP output entirely (present for Go - dolt `DoltDB`, 53 rows). So
  `retained_prod_files` is 0 by construction for every PHP anchor.
- **NOT concluded:** that a win-signature anchor does not exist in PHP/Laravel. The hunt ranked
  candidates one way (distinct dependent files) and never reached event/listener contracts,
  container bindings, or middleware pipelines.

**Product finding filed from this loop** (blast duty, `docs/loops/02-repo-admission.md`): Laravel
facades return an empty blast with a "complete" verdict. See `findings/facade-blast-empty.md`.

## Freeze plan (at clone time)

`PINNED_COMMITS.json` (this folder): for each repo `git ls-remote <url> HEAD` -> pin the
SHA, then `bash bench/drivers/provision-repos.sh` clones both arms and strips any
anti-LLM banner from BOTH (fairness, manifesto §3).
