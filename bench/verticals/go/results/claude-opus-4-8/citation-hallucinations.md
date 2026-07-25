# Citation hallucinations

Citations the answer printed that did not resolve against the repo checked out at the benchmarked commit. **Hallucinated** = line number beyond end-of-file (a made-up number). **Unresolved** = file not in the repo, or symbol not within ±5 lines of the cited line.

Reported for transparency; not folded into the headline score.

## baseline

_No ungrounded citations._

## sense

### sense/dolt  - 180/217 grounded

**Unresolved**
- `.../column_diff_table.go:42` - file not found at .../column_diff_table.go
- `.../commit_ancestors_table.go:32` - file not found at .../commit_ancestors_table.go
- `.../commit_diff_table.go:40` - file not found at .../commit_diff_table.go
- `.../commits_table.go:35` - file not found at .../commits_table.go
- `.../diff_iter.go:394` - file not found at .../diff_iter.go
- `.../diff_table.go:58` - file not found at .../diff_table.go
- `.../log_table.go:38` - file not found at .../log_table.go
- `.../remotes_table.go:41` - file not found at .../remotes_table.go
- `.../schema_conflicts_table.go:35` - file not found at .../schema_conflicts_table.go
- `.../stashes_table.go:34` - file not found at .../stashes_table.go
- `.../status_ignored_table.go:35` - file not found at .../status_ignored_table.go
- `.../status_table.go:35` - file not found at .../status_table.go
- `.../table_of_tables_in_conflict.go:30` - file not found at .../table_of_tables_in_conflict.go
- `.../tags_table.go:34` - file not found at .../tags_table.go
- `.../unscoped_diff_table.go:47` - file not found at .../unscoped_diff_table.go
- `.../backups_table.go:30` - file not found at .../backups_table.go
- `.../branch_activity_table.go:33` - file not found at .../branch_activity_table.go
- `.../branches_table.go:53` - file not found at .../branches_table.go
- `.../conflicts_tables_prolly.go:85` - file not found at .../conflicts_tables_prolly.go
- `.../commits_table.go:141` - file not found at .../commits_table.go
- `.../log_table.go:408` - file not found at .../log_table.go
- `.../diff_table.go:722` - file not found at .../diff_table.go
- `.../conflicts_tables_prolly.go:620` - file not found at .../conflicts_tables_prolly.go
- `.../conflicts_tables_root_objects.go:42` - file not found at .../conflicts_tables_root_objects.go
- `.../constraint_violations_prolly.go:127` - file not found at .../constraint_violations_prolly.go
- `.../docs_table.go:39` - file not found at .../docs_table.go
- `.../tests_table.go:37` - file not found at .../tests_table.go
- `.../user_space_system_table.go:40` - file not found at .../user_space_system_table.go
- `.../dtablefunctions/dolt_log.go:571` - file not found at .../dtablefunctions/dolt_log.go
- `.../statspro/controller.go:68` - file not found at .../statspro/controller.go
- `.../statspro/stats_kv.go:211` - file not found at .../statspro/stats_kv.go
- `.../merge/violations_fk.go:197` - file not found at .../merge/violations_fk.go
- `.../clusterdb/cluster_status_table.go:60` - file not found at .../clusterdb/cluster_status_table.go
- `.../clusterdb/database.go:36` - file not found at .../clusterdb/database.go
- `.../commitgraph/commitgraph.go:89` - file not found at .../commitgraph/commitgraph.go
- `.../sqle/cluster/commithook.go:36` - file not found at .../sqle/cluster/commithook.go
- `.../sqle/cluster/dialprovider.go:35` - file not found at .../sqle/cluster/dialprovider.go

### sense/pebble  - 52/55 grounded

**Unresolved**
- `.../interleaving_iter.go:226` - file not found at .../interleaving_iter.go
- `.../keyspan/iter.go:83` - file not found at .../keyspan/iter.go
- `.../interleaving_iter.go:103` - file not found at .../interleaving_iter.go
