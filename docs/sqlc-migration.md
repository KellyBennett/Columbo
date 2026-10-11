# sqlc snapshot bindings

`internal/snapshotdb/schema.sql` is the canonical schema source, embedded by the lifecycle owner and read by sqlc. See the [snapshot guide](sqlite-schema.md) for current schema versions and evidence tables.

## Ordinary operations and lifecycle exceptions

Ordinary data operations live in `internal/snapshotdb/queries.sql` and are
bound/scanned by sqlc. They include writer inserts and typed relationships,
evidence/inventory reads, summaries and warnings, and report-identity reads.
The checked-in query source is authoritative for the operation inventory.

The SQL source is handwritten; generated Go is never edited manually. Explicit
INSERT column lists preserve the original values and insertion order. Inserts
that previously used `LastInsertId` use sqlc's `:execlastid` annotation, retaining
that exact mechanism rather than changing to `RETURNING`.

Only aggregate counts/schema identity are cast to INTEGER for concrete generated
result types. Metric values and limits are never cast, rounded, or coerced.
The `STRICT ANY` columns remain generated `interface{}` values, preserving
`int64`, `float64`, and SQL NULL. Nullable text/relationship columns use pointers.
Existing supported-number and finite-number validation remains in the writer.

`internal/snapshotdb/lifecycle.go` owns connection setup, fixed durability and
identity PRAGMAs, schema bootstrap, integrity/foreign-key checks, and SQLite
catalog/schema verification. Those operations are fixed lifecycle exceptions.
It owns private database and transaction handles; callers get only sqlc's named
`Querier` operations. No raw handle, DBTX, generic SQL-string method or raw-handle
callback escapes. The SQLite driver is still `modernc.org/sqlite v1.39.1` and the
application continues to build with `CGO_ENABLED=0`.

The filesystem publication code keeps sibling temporary construction, closed
connections before fsync, atomic no-clobber hard-link publication, directory
sync, and transient-file cleanup. Completed snapshots are reopened immutable
and read-only; each generated `:many` query consumes/closes its cursor before
another operation uses the single connection. Summary/exit status still comes
from stored verdicts and suppression flags.

## Regression contracts

The regression suite checks generated numeric round-tripping, rejection of
writes through the read-only reader, missing-report completion rejection and
sidecar/symlink rejection. Logical-row/summary goldens, malformed-value and
cross-case constraints, disk/interruption faults and race/no-clobber checks
remain independent obligations. Raw SQL test oracles use only the exact
exceptions in the [database access boundary](database-access-boundary.md).

## Changing a query

1. Edit `queries.sql` or the canonical schema under the existing schema contract
2. Run the pinned `.tools/bin/sqlc generate` from the repository root
3. Run `./scripts/check-database-boundary.sh` and the negative guard fixtures
4. Run tests, vet, build, all published queries and the unchanged Columbo self-check

The pinned generator is a development tool, not a runtime dependency. No sqlc
Cloud account, credentials, upload, `push`, or cloud-backed `verify` is used.
