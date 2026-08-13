# Citation hallucinations

Citations the answer printed that did not resolve against the repo checked out at the benchmarked commit. **Hallucinated** = line number beyond end-of-file (a made-up number). **Unresolved** = file not in the repo, or symbol not within ±5 lines of the cited line.

Reported for transparency; not folded into the headline score.

## baseline

### baseline/consul  - 269/275 grounded

**Unresolved**
- `local.state.go:164` - file not found at local.state.go
- `agent/grpc-external/services/peerstream/subscription_manager_test.go:long` - `long` not found anywhere in agent/grpc-external/services/peerstream/subscription_manager_test.go
- `_backend_test.go:backends` - file not found at _backend_test.go
- `_test.go:ServerDataSourceDeps` - file not found at _test.go
- `acl_server_test.go:ACLResolver` - file not found at acl_server_test.go
- `_test.go:server` - file not found at _test.go

### baseline/dolt  - 238/239 grounded

**Unresolved**
- `schemas_diff_table.go:49` - file not found at schemas_diff_table.go

## sense

### sense/consul  - 106/111 grounded

**Unresolved**
- `.../peerstream/server.go:36` - file not found at .../peerstream/server.go
- `.../peerstream/subscription_manager.go:51` - file not found at .../peerstream/subscription_manager.go
- `.../peerstream/subscription_view.go:28` - file not found at .../peerstream/subscription_view.go
- `.../serverdiscovery/server.go:22` - file not found at .../serverdiscovery/server.go
- `.../v1compat/controller.go:81` - file not found at .../v1compat/controller.go
