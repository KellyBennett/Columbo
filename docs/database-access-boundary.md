# Database access boundary

Application code uses the `snapshotdb.Querier` interface emitted by sqlc and its
named parameter/result types. It obtains that interface from a lifecycle-owned
`Snapshot` or `Writer`. Database connections, transactions, driver registration,
and lifecycle-only validation/PRAGMA operations remain private to
`internal/snapshotdb/lifecycle.go`. There is no public arbitrary-SQL method,
raw-handle accessor, or callback receiving `DBTX`, `DB`, or `Tx`.

## Dedicated guard

This is a narrow architecture gate, independent of Columbo's existing strict
self-check and tests. It runs only the official `depguard` and `forbidigo` linters:

- `database/sql`, `database/sql/driver`, and SQLite-driver imports are forbidden
  outside the exact boundary/generated paths and the explicit independent tests
- References to `Exec`, `Query`, `QueryRow`, `Prepare`, their `Context` variants,
  and `Raw` are reserved across receiver types. The rule covers local interfaces,
  passed handles, wrappers, promoted methods, aliases, method values, and method
  expressions; it does not depend on a receiver being literally `*sql.DB`
- `sql.Open`, `OpenDB`, and `Register`, and the generated `DBTX`, concrete
  `Queries`, `New`, and `WithTx` APIs are not application entry points
- Generated-file comment exclusions and standard exclusion presets are disabled
- Inline `nolint` or `permit` directives cannot create another exemption,
  including the slash/space forms normalized by the official linter parser
- Application and test source have no reviewed build variants: build directives and Go
  1.25.1-recognized GOOS/GOARCH filename constraints are rejected before lint
- Official Go source metadata also rejects `IgnoredGoFiles` and `CgoFiles` in
  application/test package directories, including an entirely cgo-only package
  skipped under the supported `CGO_ENABLED=0` build
- Source directory/file symlinks cannot move application code behind an exclusion

The rule intentionally reserves these API references even on handwritten types
that happen to use the same method names. An unused declaration alone is not an
execution path; using that declaration's raw API is rejected by forbidigo. This
is an architectural guard for the reviewed private-handle design, not a claim
that arbitrary reflection or malicious source rewrites can be formally proved
safe by a linter.

The only excluded analyzer-corpus tree is `internal/columbo/testdata`; direct
application imports of that canonical module prefix, `.tools`, `.git`, or vendor
paths are banned. An additional official Go package-inventory check compares
`go list -test -json ./...` with `go list -test -deps -json ./...`. Every reached
package physically inside this repository must also be an explicit lint target.
This includes test dependencies and catches active helpers hidden in testdata,
dot/underscore directories, directory aliases, or nested modules. External module
cache dependencies are not treated as repository application source. A third
bounded metadata call lists the physical non-corpus Go-source directories
explicitly with `-e`, so an entirely inactive package cannot disappear from the
source-file inventory.

The manifest permits exactly five Go files in `internal/snapshotdb`:

- Handwritten: `lifecycle.go`
- sqlc outputs: `db.go`, `models.go`, `queries.sql.go`, `querier.go`

Every other Go file in that package, including an extra test or one claiming to
be generated, fails. A claimed sqlc output must have the exact sqlc header, and
all four files must byte-match regeneration with the pinned generator in a
private copy. The copied outputs are deleted before generation, so redirecting
output or disabling interface generation cannot pass using stale files. The
verification does not modify the working tree and catches
handwritten edits even when their authentic generated header is retained.

## Tooling and commands

The exact pins are in `scripts/database-boundary/tools.env`:

- Go 1.25.1
- golangci-lint v2.5.0, containing depguard v2.2.1 and forbidigo v2.1.0
- sqlc v1.31.1

`./scripts/install-database-tools.sh` installs trusted official tools under
`.tools/bin` without changing the application's `go.mod` or `go.sum`. The linter
is built from its pinned official module using Go's checksum database and its
module checksum is checked. The sqlc release archive is SHA-256 checked against
its official release asset digest. The linter configuration is schema-validated
using the schema included in the checksum-verified pinned linter module, so the
guard itself needs no remote schema fetch.

Run these from the repository root:

```sh
./scripts/install-database-tools.sh
./scripts/check-database-boundary.sh
python3 scripts/database-boundary/test-guards.py
```

Use `GO` or `PATH` to select Go 1.25.1 for installation. The main guard verifies
the linter's version and build Go version. `COLUMBO_TOOLS_BIN` selects an install
directory; `GOLANGCI_LINT`, `GOLANGCI_LINT_SCHEMA`, and `SQLC` can select the same
verified pinned tools explicitly. Set `BOUNDARY_REPORT_DIR` to retain JSON
manifest/linter findings at a chosen location. No finding limits truncate the
migration checklist. Findings include the exact file manifest, reachable-package
inventory, and linter JSON.

The separate `SQL access boundary` CI job runs the dedicated guard and regression
fixtures on every push and pull request, independently of the existing test and
self-check jobs. It retains JSON findings on failure. Existing behavioral tests,
strict Columbo configuration, specifications, and published SQL query checks
remain separate required evidence.

## Independent test exceptions

Only these pre-existing files may use raw SQL. Each is an independent database
oracle or an adversarial fixture; none is a runtime exemption:

| Exact file in `internal/columbo` | Reason |
| --- | --- |
| `acceptance_test.go` | Independent logical-row oracle over every SQLite table |
| `reporting_test.go` | Published-query/report reconciliation and read-only assertions |
| `sqlite_test.go` | Malformed snapshot creation, storage-type oracle, constraint rejection |
| `sqlite_atomic_test.go` | Storage-failure injection and preservation oracle |
| `sqlite_constraints_test.go` | Independent forbidden-update/ownership assertions |
| `policy_test.go` | Independent relational-group and schema-version assertions |

There is no `*_test.go` wildcard, package-wide exemption, legacy-runtime list, or
future generated-file wildcard. The manifest records the same exact exceptions
and reasons. A new exception needs explicit architectural review and a targeted
configuration change.

## Guard-first evidence and regression fixtures

The guard was run before the migration, on the runtime from `f59809e`. Official
linter output failed with five forbidden imports and thirty raw-API-reference
findings covering nineteen distinct call lines in four runtime files:

- `cli.go`: database handle import and typed-handle handoff
- `snapshot_summary.go`: database handle import and seven raw query call lines
- `sqlite.go`: database/driver imports and nine raw lifecycle/validation call lines
- `sqlite_writer.go`: transaction import and three raw call lines

The five planned boundary files were also reported missing. No runtime file was
exempted to make this baseline pass. This failing checklist established the
migration work before schema generation or runtime changes started.

The executable fixtures include a clean typed-Querier control and failures for
direct, alias, dot, and driver imports; passed raw handles; local-interface APIs;
concrete and interface promotion/method values; method expressions; fake generated
comments; generated concrete/transaction APIs; inline suppression attempts; an
extra same-package file; an edited generated output; redirected generator output;
disabled interface generation with stale checked-in output; slash/space inline
suppression; inactive platform/tagged and implicit-cgo source; and active hidden-directory or
nested-module helpers. Hidden-helper and build-variant regressions are compiled
in their relevant variants before the intended guard failure is asserted. A
fixture counts as
success only when its expected guard rejects it, never merely because the
fixture does not compile.
