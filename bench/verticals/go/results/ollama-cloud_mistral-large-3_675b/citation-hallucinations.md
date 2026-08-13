# Citation hallucinations

Citations the answer printed that did not resolve against the repo checked out at the benchmarked commit. **Hallucinated** = line number beyond end-of-file (a made-up number). **Unresolved** = file not in the repo, or symbol not within ±5 lines of the cited line.

Reported for transparency; not folded into the headline score.

## baseline

### baseline/dolt  - 42/53 grounded

**Unresolved**
- `dsess/global_state.go:20` - file not found at dsess/global_state.go
- `env/repo_state_reader.go:10` - file not found at env/repo_state_reader.go
- `env/repo_state_writer.go:10` - file not found at env/repo_state_writer.go
- `dtables/dolt_table.go:20` - file not found at dtables/dolt_table.go
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/sqle/dsess/global_state.go:20` - file not found at /Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/sqle/dsess/global_state.go
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/table/table.go:25` - file not found at /Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/table/table.go
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/table/table.go:100` - file not found at /Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/table/table.go
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/store/tree/node_store.go:20` - file not found at /Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/store/tree/node_store.go
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/env/repo_state_reader.go:10` - file not found at /Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/env/repo_state_reader.go
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/env/repo_state_writer.go:10` - file not found at /Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/dolt/go/libraries/doltcore/env/repo_state_writer.go
- `go-mysql-server/sql/table.go:10` - file not found at go-mysql-server/sql/table.go

### baseline/nomad  - 74/82 grounded

**Hallucinated**
- `scheduler/scheduler.go:50` - line 50 out of range (file only 44 lines)
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/nomad/scheduler/scheduler.go:50` - line 50 out of range (file only 44 lines)
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/nomad/nomad/client_alloc_endpoint.go:1273` - line 1273 out of range (file only 638 lines)
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/nomad/nomad/operator_endpoint.go:1289` - line 1289 out of range (file only 866 lines)
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/nomad/nomad/drainer/drainer.go:1199` - line 1199 out of range (file only 441 lines)
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/nomad/nomad/volumewatcher/volumes_watcher.go:1179` - line 1179 out of range (file only 210 lines)

**Unresolved**
- `scheduler/structs.go:20` - file not found at scheduler/structs.go
- `/Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/nomad/scheduler/structs.go:20` - file not found at /Users/luc/Developer/luuuc/oss/sense-benchmark/baseline/nomad/scheduler/structs.go

## sense

### sense/dolt  - 31/34 grounded

**Hallucinated**
- `go/libraries/doltcore/sqle/cluster/remotesrv.go:86` - line 86 out of range (file only 49 lines)

**Unresolved**
- `go/libraries/doltdb/dolt_db.go:25` - file not found at go/libraries/doltdb/dolt_db.go
- `go/libraries/doltcore/sqle/engine.go:45` - file not found at go/libraries/doltcore/sqle/engine.go

### sense/nomad  - 70/71 grounded

**Unresolved**
- `client/allocrunner/alloc_watcher.go:34` - file not found at client/allocrunner/alloc_watcher.go
