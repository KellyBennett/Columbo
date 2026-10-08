# SQLite evidence snapshots

Columbo publishes one self-contained, immutable analysis result per invocation. SQLite is the reporting source of truth for both agents and the compact terminal summary; exit 0/1 queries stored outcomes. There is no JSON export, reporting compatibility mode, server, accumulated run history, or migration framework.

```sh
columbo --output report.sqlite ./...
sqlite3 -readonly report.sqlite '.schema'
sqlite3 -readonly -header -column report.sqlite < docs/sqlite/discovery.sql
```

Without `--output`, each invocation chooses a fresh `columbo-<token>.sqlite` filename in the invocation directory. The token is 26 uppercase base32 characters (`A-Z`, `2-7`) from the pinned Go 1.25.1 `crypto/rand.Text`. `--output PATH` selects that exact path, and the compact summary prints the exact database path. Exit 0 and exit 1 both mean a completed snapshot; exit 1 retains unsuppressed FAIL findings. Exit 2 means invocation, analysis, publication, or summary delivery failed. A pre-publication failure preserves earlier snapshots and emits no analysis summary. A summary delivery failure may leave a successfully published new snapshot, so inspect the diagnostic before attributing an existing file to the current run.

The writer builds a temporary sibling file in a transaction, validates integrity/foreign keys, closes it, and publishes with atomic no-clobber creation. Every existing destination is refused unchanged with exit 2, including valid Columbo snapshots, unrelated/corrupt/unsupported files, symlinks and nonregular paths. This also applies when another writer creates the destination immediately before publication. Columbo preserves earlier snapshots and provides no overwrite flag or automatic snapshot cleanup. Completed snapshots need no journal/WAL sidecars. Open them read-only; rerun Columbo for new results rather than editing outcomes or appending runs.

## Identity and discovery

Schema version 8 is recorded in both `report.schema_version` and `PRAGMA user_version`. `PRAGMA application_id` is `0x434c4d42` (ASCII `CLMB`, decimal 1129073986). `report` has exactly one row (`id=1`) containing the schema and Columbo build versions. `summary` is a one-row view with `failed`, `warned`, and `suppressed`, including zeros for an empty analysis. Suppressed cases retain their original WARN/FAIL verdict and count only toward suppressed.

The authoritative DDL is [internal/snapshotdb/schema.sql](../internal/snapshotdb/schema.sql). SQLite introspection discovers tables, columns, declared foreign keys, indexes, and views without Columbo or Go:

```sh
sqlite3 -readonly report.sqlite 'PRAGMA user_version; PRAGMA application_id;'
sqlite3 -readonly report.sqlite 'PRAGMA table_info(clues); PRAGMA foreign_key_list(clues);'
sqlite3 -readonly report.sqlite 'PRAGMA integrity_check; PRAGMA foreign_key_check;'
```

All tables are STRICT. Foreign keys, primary/unique/check constraints, and typed alternatives reject orphaned and cross-case links. Lookup indexes cover foreign-key paths plus case outcomes, declaration symbols, canonical dependency identities, receipt physical ranges/kinds, and expansion sites. Refer to discovery.sql for exact names and definitions.

## Tables and cardinalities

IDs identify logical rows within a snapshot. Numeric IDs are not global identities across runs; use stable case IDs or declaration file/symbol identities when comparing snapshots. Every relationship below has a declared foreign key.

| Group | Rows and key columns |
| --- | --- |
| `files` | One row per emitted module-relative `path`; `id` PK and unique path |
| `declarations` | One evidence-bearing declaration per `(file_id, symbol)`; physical start/end lines and offsets; init#N and _#N identities are preserved |
| `cases` | Stable string `id`, unique `ordinal`, `primary_declaration_id`, smell, stored verdict, integer suppressed flag, primary lines, full why/diagnosis |
| `case_declarations` | Many supporting declarations per case with explicit `role`; key `(case_id, declaration_id, role)` |
| `case_guidance` | Ordered lead/avoid items; key `(case_id, kind, ordinal)` |
| `clues` | Many per case; unique `(case_id, ordinal)`, original kind/subject, value/comparison types and explicit declaration/cluster/member/pair references |
| `clue_values` | Zero or more ordered strings for a list-valued clue; key `(clue_id, ordinal)`; repeated clump types remain repeated |
| `dependencies` | One row per canonical `identity` |
| `declaration_dependencies` | Declaration-to-identity association with integer `scored` flag; key `(declaration_id, dependency_id)` |
| `dependency_receipts` | Source sites supporting those associations, retaining case/source ownership; key `(declaration_id, dependency_id, receipt_id)` |
| `clusters` | Case-scoped qualifying clusters, with original `cluster_key`, owner declaration/file, and ordinal; unique `(case_id, cluster_key)` and `(case_id, ordinal)` |
| `cluster_members` | Lexically ordered members per cluster, explicit case/helper FK, and physical `call_offset`; unique `(cluster_id, ordinal)` |
| `source_receipts` | Ordered per-case physical evidence, original subject, nullable numeric value/nesting, and source declaration FK where known; unique `(case_id, ordinal)` |
| `receipt_expansion_declarations` | Ordered root-to-helper declaration path plus original symbol; key `(receipt_id, ordinal)` |
| `receipt_expansion_sites` | Ordered root-to-copy file/call_offset path and owner declaration where known; key `(receipt_id, ordinal)` |
| `clue_receipts` | Explicit actual-support bridge between a clue and source receipts in the same case; key `(clue_id, receipt_id)` |
| `commits` | Shared full commit hash PK plus integer Unix committer timestamp |
| `case_history` | Ordered history associations per case; key `(case_id, ordinal)`, unique `(case_id, commit_hash)`, retained kind=history |
| `case_history_files` | Exact matching file subset for each case/history association, in lexical order; key `(case_id, history_ordinal, ordinal)` |
| `suppressions` | All valid directives including unused ones; ordered smell/symbol/file/line/justification, applied flag, nullable case FK |
| `warnings` | Ordered code, nullable file FK, physical line and literal message |
| `policy_reviews` | Policy ID PK, literal note and human-review prompt |
| `case_policy_reviews` | Ordered per-case policy associations; key `(case_id, ordinal)` |

Clusters can describe the same physical calls in multiple cases. They remain separate case-scoped rows; cluster/member/pair FKs cannot point into another case. Pair-overlap clues reference both member IDs directly. Read these links instead of parsing the original subjects or cluster keys. A shared case alone never establishes clue support; use `clue_receipts`.

## Nulls, numeric values, and ordering

- `clues.value_type` is `integer`, `real`, or `list`. `numeric_value` preserves the numeric storage class using STRICT ANY. A list has NULL numeric_value and ordered `clue_values` children. A list with no children is an empty list, not an unknown/null value
- Comparisons use `limit_type`, `limit_value`, and `operator`. All three are NULL when no comparison applies. Zero is a value, not absence. Individual helper overlaps have no threshold comparison; their cluster means retain the configured comparison
- Source receipt `value_type`/`value` may both be NULL. `nesting` may be NULL or a nonnegative integer, including zero. Diagnostic nesting is the pinned visitor's value, not reconstructed control-flow depth
- Optional declaration/member/cluster and suppression case references use SQL NULL. A history-unavailable warning has NULL file_id and line 0, representing the original empty-path location sentinel
- Leads, avoid items, policy associations, and both expansion paths use no child rows when empty. Do not turn a LEFT JOIN's NULL child into a fabricated list element
- `ordinal` is the established order, not a derived alphabetical order. Case ordinals preserve canonical case order; clue ordinals preserve canonical clue order; source receipt ordinals preserve canonical source order, and case-history ordinals independently start at zero in history order. Cluster ordinals preserve file/first-call order; member ordinals preserve lexical call order. All ordinals are zero-based

Same inputs yield the same logical rows, IDs, and ordinals, not necessarily byte-identical database files. Internal canonical JSON encodings still define stable case identity and evidence ordering; they are not a report payload or export format. Numeric ratios are stored at six-decimal report precision; stored verdicts were decided using the unrounded exact comparisons. Never infer enforcement by comparing rounded SQL values.

## Evidence fidelity and coverage

Paths are module-relative and physical; line numbers ignore //line remapping. Source offsets are zero-based UTF-8 bytes, start inclusive and end exclusive. Line-bucket receipts retain their complete physical line range, including the terminator when present. Complexity receipts retain the pinned increment token range and diagnostic nesting. No source excerpts or synthetic expanded file positions are required.

Each expansion copy retains both its ordered declaration chain and its ordered call-site path. Original evidence has no expansion children. A nonempty declaration chain has one more entry than its call-site path. Separate copies can share source ranges and declaration chains; they remain separate receipts because the call-site paths distinguish them. Do not deduplicate by file/line or sum a join cross-product of the two paths. Metric-contribution sums reconcile with stored function-lines, expanded-lines, cognitive-complexity, and expanded-complexity clues through actual support links.

Dependency inventory retains both `dependency` and `dependency-inventory` sites. Only `declaration_dependencies.scored=1` consumes the collaborator budget or participates in helper overlap. Universal error/empty-interface plumbing and other retained inventory-only identities remain discoverable without increasing the scored count.

History is file provenance, not symbol tracking or architectural intent. A shared commit row does not imply the same file subset for every case: join `case_history_files` using both case_id and history_ordinal. History is ordered as analyzed (timestamp descending, hash ascending), and each matching file subset is lexical. History disabled/unavailable means no association rows, not a clean history claim; the unavailable warning remains discoverable. History never changes a verdict.

Coverage is emitted evidence and supporting declarations/helpers, not all program declarations, references, dependencies, or repository files. A query for shared dependencies means shared among evidence-bearing declarations in this snapshot. Missing rows never prove the corresponding code or relationship is absent.

## Runnable queries

Run these from a checkout so the relative SQL paths exist. The snapshot itself is portable and needs only SQLite to inspect.

```sh
sqlite3 -readonly -header -column report.sqlite < docs/sqlite/unsuppressed-failures.sql
sqlite3 -readonly -header -column report.sqlite < docs/sqlite/shared-scored-dependencies.sql
sqlite3 -readonly -header -column report.sqlite < docs/sqlite/metric-to-receipt-reconciliation.sql
```

The examples accepting `:case_id` examine all cases when the parameter is NULL/unset. Bind a stable case ID for one case:

```sh
sqlite3 -readonly -header -column report.sqlite <<'SQL'
.parameter init
.parameter set :case_id C-0123456789
.read docs/sqlite/ordered-clusters.sql
SQL
```

| Query | Purpose |
| --- | --- |
| [discovery.sql](sqlite/discovery.sql) | Versions, summary, tables/views, columns, keys, integrity |
| [unsuppressed-failures.sql](sqlite/unsuppressed-failures.sql) | Stored CI failures with primary location and diagnosis |
| [shared-scored-dependencies.sql](sqlite/shared-scored-dependencies.sql) | Shared scored collaborators and each evidence-bearing declaration |
| [dependency-inventory.sql](sqlite/dependency-inventory.sql) | Scored and inventory-only identities with actual source sites |
| [metric-to-receipt-reconciliation.sql](sqlite/metric-to-receipt-reconciliation.sql) | Stored metrics, supporting contributions, sums, and equality |
| [shared-source-locations.sql](sqlite/shared-source-locations.sql) | Exact physical ranges used by multiple cases, retaining receipt/copy identity |
| [expansion-paths.sql](sqlite/expansion-paths.sql) | Both ordered paths without multiplying child rows |
| [ordered-clusters.sql](sqlite/ordered-clusters.sql) | Cluster ownership and lexical member/call order |
| [clue-evidence.sql](sqlite/clue-evidence.sql) | Typed values, list order/multiplicity, direct member/pair links, actual receipt support |
| [suppression-policy-explanations.sql](sqlite/suppression-policy-explanations.sql) | Stored verdict, suppression reason, literal policy notes/prompts, unused directives and warnings |
| [case-history.sql](sqlite/case-history.sql) | Ordered commits and exact case-specific matching file subsets |

`./scripts/test-sqlite-queries.sh report.sqlite` runs every published SQL statement with Python 3's SQLite in read-only/query-only mode, then checks metadata/integrity, summaries, scored dependency counts, contribution sums, expansion lengths, and case-history ownership. CI runs it against the all-seven-smells fixture and retains that database. Go writer/reader tests cover richer edge fixtures, constraints, empty lists, saved snapshots, and failure paths.

Generated `.sqlite` files and transient sidecars are ignored. CI captures `columbo-*.sqlite` in `columbo-self-check-snapshot` even when the unchanged strict `./columbo ./...` command exits 1, and keeps the explicit fresh runner-temp fixture in `columbo-query-fixture-snapshot` for documented-query evidence. Upload only completed `.sqlite` files; do not make a failing enforcement step succeed merely to collect them.

### Duplicate-code evidence

The `duplicate-code` case smell uses the existing relational layout. One case groups all function-anchored matching fragments; `source_receipts.kind = 'duplicate-fragment'` records each physical range. `duplicate-tokens` and `duplicate-fragments` clues link to every fragment through `clue_receipts`. Default WARN is a review lead; summary counts count groups. The compact summary and GitHub finding message list all ranges. Inspect them directly:

```sql
SELECT c.id, f.path, r.start_line, r.end_line, r.start_offset, r.end_offset
FROM cases c JOIN source_receipts r ON r.case_id = c.id
JOIN files f ON f.id = r.file_id
WHERE c.smell = 'duplicate-code' AND r.kind = 'duplicate-fragment'
ORDER BY c.ordinal, f.path, r.start_offset;
```

## Repeated variant evidence

`repeated-variant-decision` groups decisions by canonical typed domain, independent of branch bodies and duplicate-code. `variant-decision-sites`, `repeated-variant-count`, and `variant-support` are numeric clues; `repeated-variant-set` and `variant-set` use ordered `clue_values`. Source receipts use `variant-decision` and `variant-arm`, with explicit `source_declaration_id` ownership. The additive `source_receipts.spelling` column retains arm text (including aliases); its default is an empty string for other receipts. See [the rule specification](repeated-variant-decision.md).

```sql
SELECT c.id,r.subject,f.path,r.start_line,r.start_offset,d.symbol,r.spelling
FROM cases c JOIN source_receipts r ON r.case_id=c.id
JOIN files f ON f.id=r.file_id
JOIN declarations d ON d.id=r.source_declaration_id
WHERE c.smell='repeated-variant-decision'
ORDER BY c.ordinal,f.path,r.start_offset,r.kind;
```

## Schema version 3: common-role evidence

Version 2 adds the six `role_candidate*` relations documented in [Common-Role Inference](common-role-inference.md). They do not participate in verdicts or summary totals. Use [role-evidence.sql](sqlite/role-evidence.sql) to inspect both linked and unlinked candidates. Regenerate version-1 snapshots or use their matching reader; existing files are not migrated in place.

### Cross-case correlations

`correlations` contains diagnosis, categorical confidence, canonical variant
domain, and provisional MPR-001 review metadata. `correlation_cases` and
`correlation_role_candidates` link existing records. `correlation_evidence`
references case-owned source receipts with a compound ownership foreign key.
`correlation_guidance` stores ordered lead/avoid text. These tables never enter
`summary`; suppression status is read from the original member cases.

## Schema version 4: prose comments

`prose-comment` is a permitted smell. File-wide comment cases use physical package-clause declarations, identified by file and `<package import path>.<package-clause>`, with no function dependency inventory. Every prohibited comment has an explicitly linked `prose-comment` source receipt. See [comment policy](prose-comments.md).

## Schema version 5: advisory evidence

Ordinary advisory collectors run during normal analysis. Their evidence lives in the same database and transaction as cases, without independent verdicts. Older snapshots remain readable with ordinary SQLite tools; this build's reader and GitHub adapter require schema version 8, consistent with the existing single-version contract.

| Table | Contents |
| --- | --- |
| `advisory_collectors` | Included production file/declaration counts for each collector, including zero findings |
| `advisory_groups` | Kind, subject, lead and scope limitations |
| `advisory_values` | Constants and identities, projected field, repeated comparison values, written fields |
| `advisory_sites` | Ordered supporting sites linked to existing declarations, plus representation and input expression |
| `advisory_site_values` | Ordered choice-set seed identities/values for each site |
| `advisory_receipts` | Ordered kinds, subjects, full source excerpts and physical line/byte ranges; file identity comes from the site's declaration |

Group/site keys and foreign keys prevent orphaned evidence; receipt ranges are checked. No JSON blob substitutes for these relations. [advisories.sql](sqlite/advisories.sql) shows the joins needed to inspect groups and their sources without the original checkout. The compact CLI summary and GitHub check summary query these rows. Check annotations and pass/fail totals remain case-based.

## Schema version 6: ordered stages

Opt-in staged runs persist ordered `refactoring_stages`, `stage_collectors`
membership and counts, and independently collected `variant-coordination`
advisories. `active_stage_issues` selects only active members. Legacy runs have
no stage rows. See [stage semantics](refactoring-stages.md); ordinary `summary`
continues to count legacy verdicts and does not represent the staged exit code.

## Schema version 8: guarded-update advisories

Staged advisories admit `category-selected-behavior`; its collector membership,
counts, candidate callable surface, category mappings and source receipts use the
existing normalized advisory and stage tables. Both configured stages can now be
cleared. Legacy severity/enforcement is unchanged; no stage rows are stored in
legacy mode. This build follows the existing single-version reader contract and
requires schema 8.

`repeated-guarded-update` is registered in ordinary and staged snapshots. Its
resolved field and structurally normalized check/write expressions use
`advisory_values`; conditions, writes and typed input-role bindings use
`advisory_receipts`. It is deliberately absent from
`stage_collectors`, and does not affect legacy verdicts.

`witnessed-conditional-overwrite` is an additive advisory kind within the existing
schema-8 row contract. It uses `advisory_values` for typed concrete witness inputs
and earlier/later results; `advisory_receipts` stores both guards and assignments
plus the evaluated prefix, with source spelling and ranges. It has no
`stage_collectors` membership and cannot change verdicts or stage counts. See
[bounded witness contract](witnessed-conditional-overwrite.md).
