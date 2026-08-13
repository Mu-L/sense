# Citation hallucinations

Citations the answer printed that did not resolve against the repo checked out at the benchmarked commit. **Hallucinated** = line number beyond end-of-file (a made-up number). **Unresolved** = file not in the repo, or symbol not within ±5 lines of the cited line.

Reported for transparency; not folded into the headline score.

## baseline

### baseline/dolt  - 193/195 grounded

**Hallucinated**
- `go/libraries/doltcore/sqle/dtables/conflicts_tables.go:90` - line 90 out of range (file only 40 lines)
- `go/libraries/doltcore/sqle/dtables/conflicts_tables.go:93` - line 93 out of range (file only 40 lines)

## sense

### sense/pebble  - 77/78 grounded

**Unresolved**
- `/db.go:950` - file not found at /db.go
