# PHP / Laravel Vertical - Tracker

Vertical scaffolded by `bench/bootstrap/stamp.sh`.

> **Authorities** (this folder never overrides them):
> [`../../docs/manifesto.md`](../../docs/manifesto.md) (rules),
> [`../../docs/vertical-program.md`](../../docs/vertical-program.md) (sequence),
> [`../../docs/bootstrap.md`](../../docs/bootstrap.md) (the bootstrap).

## Status

| Step | Artifact | State |
|---|---|---|
| 0 - Choose repos (4, firm) | [`repos.md`](repos.md) | ⬜ |
| 1 - Stamp dirs | this folder (`verticals/php-laravel/`) | ✅ |
| 2 - Pin commits | `PINNED_COMMITS.json` (this folder) | ⬜ |
| 3 - Build indexes | `bench/lib/ensure-index.sh <repo>` | ⬜ |
| 4 - Per-repo loop | `bench/drivers/vertical-loop.sh <repo>` | ⬜ |

The per-repo mechanical loop is driven by `vertical-loop.sh`; it stops at the two
human gates (scenario authoring, tie diagnosis).
