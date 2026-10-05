# Developing Columbo

Build and test with Go 1.25.1 and `CGO_ENABLED=0`. From a checkout:

```sh
CGO_ENABLED=0 go build -o columbo ./cmd/columbo
```

## Checks

```bash
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...

# Check every documented query against a completed snapshot (uses Python 3 SQLite)
./scripts/test-sqlite-queries.sh report.sqlite
```

CI also builds Columbo from the checked-out revision and runs `./columbo ./...` in a separate **Columbo self-check** job. This includes the application's tests and enforces the default FAIL severities and thresholds. Findings fail the job; the log contains the compact summary and the `columbo-self-check-snapshot` artifact captures `columbo-*.sqlite` with the full evidence and refactoring leads even on failure. A green acceptance-test job does not imply a green self-check; existing findings must be addressed for the full workflow to pass.

GitHub Actions automatically receives file-and-line annotations from the same saved findings. The self-check keeps its existing `./columbo ./...` command and read-only permissions. The intentionally failing query fixture disables annotations to avoid surfacing expected test findings as PR diagnostics. Use `--github-annotations=false` to disable annotations elsewhere, or `--github-annotations` to exercise them locally. Regression tests cover stored severities and comparisons, suppressions, policy notes, escaping, and explicit/default CLI selection.

Acceptance tests pin Go 1.25.1, Linux/amd64, and a fixed build environment. The suite includes all seven default FAIL smells and checked-in logical SQLite-row/compact-summary goldens. CI also runs every published SQL query read-only against the all-seven fixture snapshot. Refresh goldens deliberately with `UPDATE_GOLDEN=1 go test ./internal/columbo`.

Snapshot database operations use pinned sqlc-generated bindings behind a guarded private lifecycle. See the [sqlc migration guide](sqlc-migration.md) and [database access boundary](database-access-boundary.md) for regeneration, dedicated lint and regression commands.

CE-001 is a provisional dogfooding policy. Ordinary builds and tests support it; `go run ./cmd/releasecheck` deliberately fails until the policy is resolved before public v1 release.

Cgo inputs are rejected with exit 2 when the loader cannot establish original physical-source/type correspondence; generated compiler wrappers never substitute for source receipts.

Dependency scoring counts named types and packages not already represented by a counted type. Universal `error` and empty-interface (`any`) plumbing remains in source receipts without consuming the collaborator budget. Cosmetic Extraction uses the same scored sets.

## Reference

- [Configuration, analysis rules, and CLI contract](../SPEC.md)
- [SQLite schema, report handling, and query guide](sqlite-schema.md)
- [Accepted SQLite implementation specification](sqlite-implementation-spec.md)
- [SQLite cutover validation](sqlite-validation.md)

Generated `.sqlite` files are ignored by this checkout. Retain or upload them explicitly as CI artifacts. Read snapshots without modifying them; rerun Columbo for new results. If you choose a path with `--output`, it must not already exist. See the query guide for publication failures and report recovery.
