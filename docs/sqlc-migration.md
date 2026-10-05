# sqlc snapshot bindings

The snapshot schema remains version 1. `internal/snapshotdb/schema.sql` is the
single schema source, embedded by the lifecycle owner and read by sqlc. Its
12,779 bytes are unchanged from the former `sqliteSchema` constant at `f59809e`.
The 24 tables, indexes, constraints, three list-preservation triggers and summary
view remain the public snapshot contract.

## Ordinary operations and lifecycle exceptions

All 33 ordinary data operations live in `internal/snapshotdb/queries.sql` and
are bound/scanned by sqlc:

- 24 writer inserts, including typed relationships and ordered evidence
- One dependency-scoring inventory read
- Seven summary/warning reads
- One report-identity aggregate read

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

## Verification

The access guards were implemented and run before schema generation or runtime
migration. See [Database access boundary](database-access-boundary.md) for the
failing baseline, exact exceptions, pinned tool installation and adversarial
guard tests.

A bounded compatibility spike used the complete verbatim schema and all 33
operations with sqlc v1.31.1, Go 1.25.1 and the existing driver under CGO=0. It
verified database and transaction execution, LastInsertId, generated scanners,
cursor release, nullable values/IDs, trigger/constraint rejection, an integer of
9,007,199,254,740,993 and integral REAL `1.0` without losing their distinct types.

The committed regression suite also checks generated numeric round-tripping,
rejection of a write through the production read-only reader, missing-report
completion rejection, and direct-boundary sidecar/symlink rejection. Existing logical
row/summary goldens, malformed-value and cross-case constraints, disk/interruption
faults, race/no-clobber cases and sidecar checks remain in force. Independent raw
SQL test oracles retain only their six pre-existing exact-path exceptions.

Before publication, the all-seven fixture produced the same 506 typed logical
rows across 25 tables/views as the pre-migration binary. Its terminal summary was
byte-identical after normalizing the output destination. Every published query
was exercised read-only against the migrated snapshot.

## Changing a query

1. Edit `queries.sql` or the canonical schema under the existing schema contract
2. Run the pinned `.tools/bin/sqlc generate` from the repository root
3. Run `./scripts/check-database-boundary.sh` and the negative guard fixtures
4. Run tests, vet, build, all published queries and the unchanged Columbo self-check

The pinned generator is a development tool, not a runtime dependency. No sqlc
Cloud account, credentials, upload, `push`, or cloud-backed `verify` is used.
