# Columbo 🕵️

*Just one more thing… about that function.*

Columbo is a strict code-smell detective for Go. It follows suspicious code to possible architectural trouble, shows the evidence, and leaves leads for a better design. Long functions, tangled decisions, too many dependencies, and suspiciously tidy helper extractions all deserve a closer look.

Run it locally or let it patrol CI. Configured violations fail the build, giving you—or your coding agent—a case to work on.

## Why Columbo?

Named after the persistent TV detective, Columbo keeps following the clues and asking awkward questions. Splitting a sprawling function into `stepOne`, `stepTwo`, and `stepThree`? Just one more thing: did the design actually improve?

## How to use it

Build with Go 1.25.1 and `CGO_ENABLED=0`, then run from the Go module you want to investigate:

```bash
CGO_ENABLED=0 go install github.com/KellyBennett/Columbo/cmd/columbo@latest
# Or, from a checkout:
CGO_ENABLED=0 go build -o columbo ./cmd/columbo
```

```bash
# Investigate all packages
columbo

# Focus on one part of the project
columbo ./internal/orders/...

# Choose the SQLite evidence snapshot destination
columbo --output report.sqlite ./...

# Discover the stored evidence and outcome totals
sqlite3 -readonly report.sqlite '.schema'
sqlite3 -readonly -header -column report.sqlite 'SELECT * FROM summary;'
```

Every analysis run publishes one self-contained SQLite snapshot, defaulting to `columbo.sqlite` in the invocation directory. A compact SQL-backed terminal summary includes stored verdicts, triggering comparisons, WARN/suppressed cases, policy-review prompts, warnings, totals, and the database path. Full evidence, diagnosis, refactoring leads, and shortcuts to avoid stay in the database. There is no `--format` flag or JSON export; `--output -` is invalid.

Use `.columbo.yml` to adjust thresholds and choose which smells warn, fail, or stay off. Follow the stored leads, improve the design, and run it again. See the [schema reference and query guide](docs/sqlite-schema.md) for shared dependencies, contribution reconciliation, ordered clusters, expansion paths, suppressions, and case-specific history. The database contains emitted evidence and supporting declarations/helpers, not a complete program graph; missing rows do not establish that code is absent.

In CI, `0` means completed without unsuppressed FAIL cases, `1` means completed with a case to solve, and `2` means analysis or delivery failed. Counts and exit 0/1 come from stored verdicts, not rounded metric comparisons. Publication is atomic: earlier valid Columbo snapshots survive pre-publication failures. Existing unrelated, invalid, unsupported, symlink, or nonregular destinations are refused unchanged. A terminal-summary write failure exits 2 even if the new snapshot was already published.

Curious about the complete contract? See [SPEC.md](SPEC.md) and the accepted [SQLite implementation specification](docs/sqlite-implementation-spec.md).

## Capturing reports

Keep or upload the `.sqlite` file after either exit 0 or exit 1; both are completed analyses. Do not replace a failing Columbo command with a successful query when enforcing CI. On exit 2, check the diagnostic: an existing snapshot may belong to the prior invocation, or publication may have succeeded before summary delivery failed. No journal/WAL sidecars are needed for a completed snapshot.

Generated `.sqlite` files are ignored by this checkout. Choose a destination outside the checkout or retain/upload the file explicitly as a CI artifact. Open snapshots read-only and rerun Columbo to produce a new snapshot; do not edit stored outcomes or append runs to one database.

## Development

```bash
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...

# Check every documented query against a completed snapshot (uses Python 3 SQLite)
./scripts/test-sqlite-queries.sh report.sqlite
```

CI also builds Columbo from the checked-out revision and runs `./columbo ./...` in a separate **Columbo self-check** job. This includes the application's tests and enforces the default FAIL severities and thresholds. Findings fail the job; the log contains the compact summary and the `columbo-self-check-snapshot` artifact retains the full evidence and refactoring leads even on failure. A green acceptance-test job does not imply a green self-check; existing findings must be addressed for the full workflow to pass.

Acceptance tests pin Go 1.25.1, Linux/amd64, and a fixed build environment. The suite includes all seven default FAIL smells and checked-in logical SQLite-row/compact-summary goldens. CI also runs every published SQL query read-only against the all-seven fixture snapshot. Refresh goldens deliberately with `UPDATE_GOLDEN=1 go test ./internal/columbo`.

CE-001 is a provisional dogfooding policy. Ordinary builds and tests support it; `go run ./cmd/releasecheck` deliberately fails until the policy is resolved before public v1 release.

Cgo inputs are rejected with exit 2 when the loader cannot establish original physical-source/type correspondence; generated compiler wrappers never substitute for source receipts.

Dependency scoring counts named types and packages not already represented by a counted type. Universal `error` and empty-interface (`any`) plumbing remains in source receipts without consuming the collaborator budget. Cosmetic Extraction uses the same scored sets.
