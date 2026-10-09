# Source and dependency evidence corrections

This change corrects three analyzer evidence defects against Columbo base `05dd0479ad968edc399804dabd92f4c335dfcd9f`. It retains the existing analysis policy and thresholds.

## Physical source positions

Go line directives can remap logical file positions. Receipts read physical source text, so using a remapped line as a physical line index can point outside the file. The regression fixture reproduced a baseline panic at logical line 901 in a six-line source file.

The file line accessor now requests unadjusted physical positions, and line-metric collection uses that accessor. `TestLineDirectivesKeepMetricReceiptsPhysical` checks that the two counted source lines are lines 4 and 5 and belong to the physical source file.

This is a source-position correction. It does not add cgo loading or analysis support.

## Private type declaration ownership

`types.Info.Defs` is package-wide. Iterating its whole map for each file incorrectly allowed the first visited file to own definitions from other files. That changed the participating-file set, cross-file access count, and treatment of excluded declarations.

The definition scan now walks identifiers in the current file's AST and resolves only those identifiers through the shared map. Existing private-type, alias, and production filters remain in place.

`TestPrivateTypeDefinitionOwnership` covers forward/reverse file order, unrelated files before/after the declaration file, the actual declaration location, the true three-file footprint, and exclusion of same-file accesses. `TestPrivateTypeDefinitionProductionScope` verifies that excluded and test-file declarations cannot be reassigned to another production file. The ownership and scope regressions fail on the baseline and pass with the correction.

## Generic instantiation dependencies

Visiting both an instantiated type expression and its generic syntax head could add a phantom uninstantiated dependency. The correction excludes the head only when:

1. AST ancestry proves that it is the head of an index or index-list expression, including intervening parentheses.
2. The parent expression is a checked type instantiation.
3. The checked instance and head have the same named-type or alias origin.

Alias identity is compared before unaliasing. The correction preserves actual type arguments, distinct specializations, ordinary named dependencies, generic function behavior, and existing receiver exclusions. It does not remove dependencies by name or underlying-type similarity.

The generic regression suites cover 17 inventory, threshold, and alias scenarios. Compound pointer, slice, and map aliases are included alongside a non-generic alias control. The existing dependency expectation for `atomic.Pointer[Buffer]` drops only the phantom generic origin. Seventeen additional direct tests cover syntax ownership and type-origin identity, including mismatched origins, non-instantiated types, and builtin rejection.

The five-real-dependencies control and seven inventory variants fail on the original baseline; compound-alias tests also reproduced the incomplete initial correction. The final fixtures pass. Helper separation preserves those expectations while keeping the baseline self-check case set unchanged.

## Verification

Local validation used Go 1.25.1:

| Check | Result |
| --- | --- |
| Full Go suite, cgo disabled | Pass; analyzer package 35.052 seconds |
| Full Go suite, cgo enabled | Pass; analyzer package 32.771 seconds |
| Vet and CLI build | Pass |
| Final focused generic and syntax/origin tests | Pass |
| Database manifest, reachable-package inventory, and dedicated lint | Pass; zero lint issues |
| sqlc 1.31.1 regeneration | Byte-identical generated output |
| Database guard fixtures | 65 pass |
| Published SQLite queries | 15 files, 39 statements, 11 case filters pass |
| Action runner / Checks API tests | 5 / 22 pass |
| Patch whitespace check | Pass |
| Apply cumulative patch to pristine base | All nine changed code/test files match |

The database lint tool was pinned to golangci-lint 2.5.0 built with Go 1.25.1. The all-smells fixture retains its expected findings exit status.

Strict self-check still reports **13 unsuppressed FAIL, 9 WARN, and 1 suppressed case**. Baseline and corrected case identities, smells, verdicts, suppressions, line ranges, rationale, and diagnosis match. Analysis warnings affected by private-type ownership are separate from this case-set comparison. Existing failures remain visible. This is not a clean self-check result.

These are local results for the prepared correction. The pull request's exact published commit must receive its own CI results.

## Controlled source comparisons

The source trees were pinned and remained byte-identical throughout the comparisons:

| Repository | Source revision | Manifest files | Scan mode |
| --- | --- | ---: | --- |
| derailed/k9s | `2319c919ba7d70c14504d8f20d6657070d27130c` | 1,050 | Ordinary, no history |
| minio/mc | `77f82e18b5401a65958f1619df6ebb994634bd88` | 456 | Ordinary, no history |
| pocketbase/pocketbase | `b1da83e5165f938453fbb21e748bca317b08239d` | 949 | Staged, no history |

These are manifest file counts, not counts of analyzed production files. Scans ran sequentially with Go 1.25.1, cgo enabled, two Go processors, and a 2 GiB Go memory target. That target is not an RSS cap; the final K9s scan peaked at approximately 4.03 GiB RSS.

- **K9s:** the generic correction clears the excessive-dependency cases for `Browser.Aliases` and `Xray.Aliases`, leaving five real dependencies in each. `TableData.Delete` falls from nine to eight dependencies and still fails. `Header.FilterColIndices` changes from four to three. The private-type correction changes only warning rows relative to the generic-only corrected snapshot; the other 42 tables match exactly.
- **MinIO mc:** the generic-only correction leaves all 43 tables exactly unchanged. Adding the private-type correction changes only warning rows; the other 42 tables match exactly. The immutable reference source was used for this comparison.
- **PocketBase:** the private-type correction locates `collectionValidator` and `optionsValidator` at their actual declarations and removes the false `join` dispersion warning. The physical-line correction leaves that snapshot unchanged. Adding the generic correction removes nine explicitly identified phantom generic origins, clears 14 excessive-dependency cases, recomputes dependency counts and overlaps, and prunes two now-unreferenced declarations and associated records. No case is added.

Comparisons with declaration pruning replace surrogate IDs with stable identities and omit dense presentation ordinals. Every one of the 43 normalized table row multisets matches the explicitly allowed correction. This is distinct from the exact comparisons that retain IDs and ordinals.

The final helper refactor preserves every row of all 43 tables, including IDs and explicit ordinals, against the combined pre-refactor correction on all three repositories. SQLite integrity checks pass and foreign-key violations are absent.

## Interpretation limits

These comparisons verify bounded evidence corrections on the named source snapshots. They do not establish holdout performance, architectural completeness, or correctness of a new refactoring phase. Passing the suite with cgo enabled does not establish broader cgo analysis coverage.

A separate lifecycle-localization screen did not earn promotion of an added collector. With two fixed replicates per arm and repository, both evidence arms localized 2/2 K9s tasks; both were 0/2 on the required combined MinIO boundary, with the MinIO outcomes scope-inconclusive. Complementary partial results were not pooled into a success. That screen establishes neither added-collector benefit nor global sufficiency of the original evidence. No architecture-improvement or efficiency claim follows from those counts.

This change adds no experimental collector or public command and changes no stage membership, stage ordering, thresholds, suppressions, exclusions, or module dependencies. It makes no claim about the separate full-refactoring evaluation.
