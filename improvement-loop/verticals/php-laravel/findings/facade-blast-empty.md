# Laravel facades: blast returns an empty set and calls it complete

**Found:** 2026-07-29, php-laravel Loop 2 admission sweep. `sense` 1.13.2, schema v5, index rebuilt
the same day.

## What happens

`sense blast Site --file src/Facades/Site.php` on the statamic clone returns:

```
symbol          Statamic\Facades\Site
risk            low
total_affected  0
completeness    {"verdict": "complete", "resolved": 0,
                 "advice": "Complete resolvable dependent set - act on it, do not re-grep.
                            Dynamic-dispatch residual, if any, is in index_caveat."}
```

No `index_caveat` key is present in the response.

The same symbol has **137 inbound edges in the index**, all of kind `imports`, from 137 distinct
files, and **112 files under `src/` make static calls on it** (`Site::get(...)`, `Site::current()`,
`Site::all()`). `Statamic\Facades\Collection` measures the same shape (139 imports, 0 affected), as
does filament's `Filament` facade (affected 1).

## Why it happens

A Laravel facade class declares only `getFacadeAccessor()`; `Site::current()` is routed by
`__callStatic` to a container-bound implementation. There is no `current` method symbol on the
facade for a call edge to bind to, so the resolver records the `use` import and nothing else.

## What is NOT the bug (checked, killed)

Static calls are **not** broken in PHP generally. `Statamic\Support\Str` is a plain class whose
static methods exist as symbols: its methods carry 165 inbound `calls` edges and
`sense blast Str --file src/Support/Str.php` returns total_affected 1143 / 60 direct callers. The
repo holds 54,980 `calls` edges overall. The gap is specific to facades - a class whose methods are
not declared on it.

## The reportable defect

Not "facades don't resolve" - that is a dynamic-dispatch limit and the response format has a place
to say so. The defect is that blast asserts `verdict: complete` with `risk: low` and advises
"act on it, do not re-grep" on a **zero-result** answer for a symbol 137 files import. An agent that
trusts the contract concludes the facade has no dependents. `index_caveat` exists in the contract for
exactly this residual and is absent here.

## Why it matters for this vertical

Facades are the most idiomatic Laravel construct there is. Any php-laravel scenario whose anchor is
facade-reached is silently unmeasurable, and the admission gate reads these cells as
MEASUREMENT-INVALID (statamic `Site`, `GraphQL`) - correctly, but only because the gate has a
zero-edges guard, not because blast disclosed anything.

## Repro

```
cd $SENSE_CLONES/statamic
sense blast Site --file src/Facades/Site.php --json --min-confidence 0.3
sqlite3 .sense/index.db "select kind,count(*) from sense_edges where target_id=5789 group by kind;"
grep -rl 'Site::' src | wc -l
```
