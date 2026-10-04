# SQLite cutover validation

Initial cutover validation ran on 2026-10-04 with Go 1.25.1, Linux/amd64, `CGO_ENABLED=0`, `GOTOOLCHAIN=local`, `GOTELEMETRY=off`, `GOWORK=off`, and `GOFLAGS=-buildvcs=false`. The fresh-snapshot amendment and its additional verification are recorded separately below.

## Correctness and enforcement

- `go test ./... -count=1`, `go vet ./...`, and the CLI build passed
- The unchanged `./columbo ./...` self-check passed with 0 FAIL, 0 WARN, and 0 suppressed cases
- All 11 published SQL files (22 statements) ran read-only on empty and populated snapshots, including all eight case filters
- Independent reconstruction through SQL matched all eight pre-cutover cases and all 120 source receipts, including original subjects, values/comparisons, physical ranges, cluster/member order, both expansion paths, guidance, and literal policy reviews
- Tests cover all seven smells, canonical identity/order, inventory-only dependencies, list multiplicity/empty lists, case-specific history subsets, WARN/suppressed CE-001, saved-database rendering without analysis, and summary-write failures
- Storage tests cover real SQLite disk-full errors, interruption inside a transaction, sync/publication failures, prior-snapshot preservation, invalid/unrelated/unsupported files, symlinks/sidecars, orphan/cross-case links, and concurrent no-clobber creation
- Independent review found no unresolved semantic must-fix

The provisional CE-001 public-release gate remains unchanged. This is a reporting cutover, not a policy resolution or public release.

## 2026-10-04 fresh-snapshot amendment

The accepted contract now creates `columbo-<token>.sqlite` by default and refuses every existing explicit destination. Earlier snapshots remain unchanged. CI still enforces the unchanged strict `./columbo ./...` command and captures `columbo-*.sqlite`; the query fixture retains its explicit fresh runner-temp path.

Schema validation and discovery exclude only SQLite's literal `sqlite_` prefix using `NOT GLOB 'sqlite_*'`. The previous `LIKE 'sqlite_%'` predicate treated the underscore as a wildcard and could hide an ordinary user table such as `sqliteX`. `TestSQLiteUserTableWithSQLitePrefixPreserved` requires that table to remain discoverable, makes the altered database invalid as a Columbo snapshot, and verifies its data is preserved on refused publication.

Publication regressions cover a file created immediately before publication and a competing valid snapshot replaced by unrelated data before publication (`TestSQLiteAbsentPublicationNeverClobbersNewFile` and `TestSQLitePublicationNeverClobbersConcurrentReplacement`). Atomic no-clobber creation must refuse both destinations and preserve the competing bytes. CLI regressions check successive default runs, the prescribed token shape, exact summary paths, unchanged earlier files, and refusal of an existing explicit snapshot.

Clue support now attaches to named local clues before append. Ordinary parameter metrics collect parameter-field receipts first; Feature Envy builds own, foreign and ratio clues with their established support; Cosmetic Extraction supports each local metric before applying its comparison. Data Clump uses named types, size and occurrences support collectors until the evidence is complete. No support relationship depends on a clue's slice position. Focused tests passed under the pinned environment, including `TestSnapshotClueSupportRoles`, `TestFeatureEnvyRatioReceiptSupport`, contribution reconciliation and the unchanged all-seven logical-row/compact-summary goldens.

Final local amendment checks passed under that same pinned environment: `go test ./... -count=1`, `go vet ./...`, CLI build, unchanged `./columbo ./...` (0 FAIL, 0 WARN, 0 suppressed), all 11 published SQL files / 22 statements against a fresh all-seven fixture, and `git diff --check`. Independent static re-review found all three Squint findings addressed and no new actionable gap. Malformed-report/value and storage-failure tests use fresh targets so they still exercise construction and validation; directory-sync and summary-delivery failures retain the published database.

## Bounded retrieval comparison

The previous JSON-plus-jq baseline is commit `58d2fa3205fc92737449c901685e6338fd0b8b5b`. Its analyzer results match the implementation base for these inputs. The baseline is development-only; the current CLI ships no JSON export or compatibility mode.

The matched fixture starts from `internal/columbo/testdata/all/all.go` and its module. Add `// columbo:ignore cosmetic-extraction -- accepted during policy comparison` immediately before `func Parent`, set `long-parameter-list` severity to `warn`, and disable history. Both snapshots contain 5 unsuppressed FAIL cases, 2 WARN cases, and 1 suppressed cosmetic FAIL carrying CE-001.

Fresh trials used the same inherited runtime model and xhigh reasoning. The runtime did not expose a model identifier. Agents could inspect only the assigned report and its format's documentation/recipes; implementation source, analyzer reruns, the other report format, and earlier trial reports were unavailable to them.

Fixed questions:

1. Shared scored dependencies and their declaration users, with coverage limitations
2. Parent's expanded line/complexity metrics reconciled to physical locations and ordered copy annotations
3. Parent's ordered clusters and helper call offsets
4. Distinct copy paths, including whether equal source ranges/declaration chains have differing site paths
5. Every WARN/suppressed case and literal cosmetic policy ID/review prompt

The completed paired trials had a 60,000-byte returned command-output budget and a 12-invocation cap. The accounting below includes orchestration wrappers, underlying execution calls, and progress calls. Bytes measure returned command stdout, not total model tokens; wrapper metadata is excluded. Efficiency is informational and is not a cutover gate.

| Attempt | Execution commands | Total invocations | Returned stdout bytes | Query errors / retries | Outcome |
|---|---:|---:|---:|---:|---|
| SQLite, 60 KB | 4 | 9 | 46,098 | 0 / 0 | Completed all questions within budget |
| JSON/jq, 60 KB | 6 | 13 | 43,683 | 1 / 1 | Completed answers, but exceeded the invocation cap by one; protocol deviation |
| JSON/jq replacement, 60 KB | 3 | 6 | 67,160 | Not fully assessed | Stopped on context-budget exhaustion; one displayed output was truncated; not a completed trial |

An earlier SQLite discovery attempt used a 25,000-byte cap and returned 28,930 bytes before task-specific queries. It stopped without assessing correctness. These failed/deviating attempts are retained rather than omitted from the comparison.

The completed SQLite answers correctly identify `package:fmt` users `LongFunction`, `stepOne`, and `stepTwo`; expanded lines 12 and complexity 0; helper calls at offsets 901/916; two distinct copy paths; and both WARN cases plus the suppressed CE-001 case. The JSON report establishes two observed `package:fmt` users from emitted dependency clues/receipts; it does not expose the complete collaborator inventory for every declaration receipt. Neither fixture contains the equal-range/equal-chain/different-site collision edge case; dedicated saved-evidence regressions cover that preservation requirement.

A final matched trial uses the same questions, model, and total caps, with an identical 8,000-byte per-command output guard in both conditions. Its result will be recorded in the pull request whether it completes or stops at the budget. No further retries are required to produce a cleaner efficiency result. Exact evidence parity, meaningful regression tests, and independent review provide the correctness gate.
