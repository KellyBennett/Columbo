# Test abstraction audit

Reviewed PRs #10–#15 against the code before #10 (`4e15c5a0c5dd7f1a1c6cc0a62fc1012cf0c56e4c`) and main after #15 (`73b47728e95da13c2e6d86650ee2a2a7f991fa97`). Scope: the helpers, fixtures, evidence queries, and test-only methods introduced during the final test refactors. Earlier production refactors are outside this audit.

The review used the original diffs, callers, data ownership, assertions, and failure behavior. Columbo was run afterward to measure outcomes, not to decide whether the abstractions were justified. The pre-refactor source reproduces 31 FAIL findings; the reviewed main snapshot has zero. Neither snapshot has warnings or suppressions.

## Decisions

| Abstraction | Responsibility | Judgment | Action |
| --- | --- | --- | --- |
| Expansion fixture and evidence | Root, trace, measurement, and provenance queries | Keep | Cohesive setup and meaningful evidence operations; preserve the assertions |
| Test-only policy queries on `Case` | Test expectations added to a production type's test method set | Replace | Use explicit test-owned assertions; remove the vacuous identity query |
| `evaluatedType` | Unwrap the standard type checker's result | Review | Inline alternative passes its test but fails the dependency rule; retain pending calibration |
| Named Go type fixture | Canonicalization ignores argument/result names and resolves byte/rune aliases | Repair | Restore named parameters/results lost in #15 |
| Receipt, cluster, history, and scenario helpers | Physical evidence, deterministic setup, and focused assertions | Keep | The inspected boundaries preserve or strengthen the original checks |

## Inventory

Each row groups an abstraction with its construction/query operations. Callers are test or harness method names. LF = Long Function, ED = Excessive Dependencies, HC = High Cognitive Complexity. Finding associations identify the original failing caller and the responsibility moved out of it; several helpers jointly remove a caller's findings, so no individual helper is credited with an isolated numerical reduction.

| Helper / fixture / method | Callers | Owned data / responsibility | Original finding association | Judgment |
| --- | --- | --- | --- | --- |
| `goldenFile`, `golden` | `TestAllSevenDefaultFail`, `TestHistoryWarningGoldens`, `TestCosmeticSeverityAndPolicy` | Goldie configuration and the corresponding fixture name; deliberate update mode and fatal serialization/update failures | History and policy tests: ED, LF; default-smell test already clear | Keep: library adapter removes repeated fixture knowledge; returned values describe the same fixture |
| `require`, `requiref`, `check`, `checkf` | Existing suite assertions | Existing harness assertion API adapted to Testify; fatal versus nonfatal behavior and message formatting | Existing helpers, not new boundaries; support #10 migration | Keep: existing call sites retain their assertion behavior |
| `scriptColumbo`, `TestCLI`, `flags.txtar` | `testscript.Run` | In-process CLI invocation, exact exit codes, stdout/stderr expectations | No original CLI finding; established test driver | Keep: checks exact exit 0/2 and adds diagnostic/output expectations |
| `dependencyReceiptEvidence`, `receiptEvidence`, `measuredDependencyEvidence`, `designatedTexts`, `uniqueCount` | `TestDependencySiteRanges` | Source bytes paired with physical receipt ranges; text queries and duplicate detection | ED, LF | Keep: bytes and ranges must correspond; nonfatal missing-text checks and fatal duplicate check remain |
| `complexitySite`, `complexitySites` | `TestComplexityReceipts` | Physical operator text, file, value, and nesting projected from receipts | ED, HC, LF | Keep: meaningful evidence tuple; exact aggregate operator assertion plus reconciliation remain |
| `expansionFixture`, `newExpansionFixture`, `lines`, `complexity` | `TestExpansionBoundaries`, `TestExpansionChildCopies`, `TestExpandedLines`, `TestExpandedComplexity`, `TestExpansionCopyEvidence` | Loaded engine/root, measurement initialization, root trace and recursion stack | Boundaries/child copies: ED; other three: ED, LF | Keep: callers request virtual metrics with one consistent root context; metric initialization is explicit |
| `expansionEvidence`, `childPaths`, `hasCopiedContribution`, `copyKeys` | Child-copy, expanded-complexity, and copy-evidence tests | Source receipts and copy-path/contribution queries | Same expansion caller findings | Keep: domain queries retain the physical provenance distinctions being tested |
| `expandedLineCase`, `expandedLineCases`, `source` | `TestExpandedLines` | Body, expected line count, and matching helper return signature | ED, LF | Keep: scenario owns the fixture transformation required by its body |
| `clusterBoundaryCase`, `clusterBoundaryCases`, `source`, `checkClusterBoundary` | `TestClusterBoundaries` | Boundary scenario and expected finding; positive cases reconcile both expanded metrics | LF | Keep: coherent scenario execution; missing finding and metric checks retained |
| `orderedClusterSource` | `TestClusterJSONOrder` | Fixture containing two ordered helper clusters | ED | Keep: hides source-generation mechanics, not test expectations |
| `clusterOwnershipEvidence`, `clusterOwnership`, `requireOwnedCluster` | `TestReachableClusterOwnership` | Cluster owners and input keys whose prefixes must match the owner | ED, HC | Keep: these values share an ownership invariant; adds a nonempty input-evidence assertion |
| `reportDocument` | `TestClusterSchema`, `requireReportSchema` | Serialize and decode the report's actual JSON schema | Cluster schema: LF | Keep: repeated schema inspection owns both failure checks |
| `clumpSource`, `clumpParameters` | `TestLargeClosedClump`, `clumpSource` | Corresponding named-type declarations and parameter references shared by three functions | ED, LF | Keep: builder mutation and returned parameters refer to the same generated types |
| `gitFixture`, `fixtureGitEnv`, `committedFixture`, `run` | `TestHistoryProvenance`, fixture setup | Disposable repository, directory, deterministic author/committer environment | ED, LF | Keep: repository owns command context; absent Git still skips, command failure now fails instead of silently skipping |
| `historyReceipts`, `requireHistory` | `TestHistoryProvenance` | Filter history evidence, require its presence, check hash/time/files | ED, LF | Keep: preserves original comparisons and the nonempty guard |
| `measuredDependencySets` | `requirePublicDependencies`, `requirePrivateDependencyExemptions` | All declarations' measured collaborator sets | Dependency identities/exemptions: LF | Keep: homogeneous result, reused across public/private scenarios |
| `requirePublicDependencies`, `requirePrivateDependencyExemptions` | `TestDependencyIdentitiesAndExemptions` | Public identities and same-file/cross-file private-type exemption scenarios | LF | Keep: separate named subtests preserve the original checks and add a nonempty measured-set guard |
| `evaluatedType` | `TestCanonicalTypeMatchesGoFormatting` | No persistent state; unwrap `types.Eval(...).Type` and its error | ED | Review: thin single-use adapter; inline trial described below |
| `Case.hasSuppressedWarningPolicy`, `requireCosmeticPolicy` | `TestCosmeticSeverityAndPolicy` | Compound expected verdict/suppression/review assertion | ED, LF | Replace test-only `Case` method with separate Testify assertions in the existing assertion helper; improves failure diagnostics |
| `Case.identityWithoutPolicyReviews` | `requireCosmeticPolicy` | Recompute identity after assigning an empty review slice | ED, LF | Remove: assignment does not affect the called `identity` function; replace with actual annotation invariance regression |
| `requireDisabledCosmeticSuppression` | `TestCosmeticSeverityAndPolicy` | Off-severity scenario with zero cases and one unused-suppression warning | ED, LF | Keep: meaningful second policy scenario; original assertions retained |
| `requireReportSchema`, `requireIdentityDelimiterBoundaries` | `TestIdentitiesAndSchema` | Report field count/non-null arrays; escaped delimiter distinction | LF | Keep: separate schema and identity contracts; moving source/severity identity check remains in caller |
| `requireOverlapComparison` | `TestOverlapComparisonEvidence` | Mean versus individual overlap comparison metadata | HC | Keep: rule-specific assertion retains both branches; caller still selects parameter-overlap clues |
| `thresholdCase`, `thresholdCases`, `checkThresholdBoundary` | `TestThresholdBoundaries` | Fixture/metric/limit association; pass at T and fail at T+1 | LF | Keep: cohesive scenario owns both configurations and expected outcomes |

Single use alone did not determine these judgments. No newly introduced helper in this scope takes the caller's entire collaborator set. Evidence tuples and fixture return values were checked for relationships between their fields rather than judged by their size.

## Changes and alternatives

The policy assertion now reports verdict, suppression, and review-list failures individually. The two test-only methods on `Case` are removed. Their definitions were in `_test.go`, so they never shipped in the production binary; the objection is misplaced test expectations and weak evidence, not shipped API expansion.

The old identity query existed before #15 as inline code. Extracting it did not create its weakness: clearing a copy's `PolicyReviews` then calling `identity(smell, file, symbol, key)` cannot demonstrate that the annotation path preserves enforcement. The new `TestPolicyReviewPreservesCase` uses a real analyzed FAIL case, snapshots its complete canonical representation before annotation, removes reviews, calls the production `addPolicyReview`, and compares the complete result. An immutable string snapshot catches in-place mutation of shared slices as well as replacement of fields. This tests annotation invariance; existing identity tests and goldens continue to cover construction and serialization.

The policy test follows the suite's existing `testHarness` discovery adapter. Testify receives the harness directly, which already implements its testing interface. This avoids naming the embedded `testing.T` collaborator at the assertion call and preserves Testify's diff diagnostics; it adds no new forwarding facade.

For `evaluatedType`, the local alternative calls `types.Eval` directly in the test and passes `value.Type` to `canonicalType`. It passes `TestCanonicalTypeMatchesGoFormatting`, and removing the adapter makes the control flow easier to see. Columbo reports ED **6 > 5** for that test, counting `go/token.FileSet`, `go/token.Pos`, `go/types.Package`, `go/types.Signature`, `go/types.Type`, and `go/types.TypeAndValue`. The thin adapter therefore helps the metric more than it establishes ownership. It remains unchanged in this PR, with this explicit unresolved calibration judgment. No replacement wrapper, suppression, threshold change, or rule change is introduced. A decision to change the rule requires human agreement.

The original canonicalization fixture explicitly constructed named parameters and results. #15 switched to an unnamed expression, dropping name-erasure coverage. The expression now uses distinct parameter/result names, `func(input []byte) (output rune)`, while still expecting `func([]uint8) int32`. The existing structural-signature-name acceptance test also remains in place.

## Verification

Go 1.25.1, Linux/amd64, `GOTOOLCHAIN=local`, `GOTELEMETRY=off`, `GOFLAGS=-buildvcs=false`. The test suite fixes `GOWORK=off` and `CGO_ENABLED=0`. All checked-in goldens are unchanged.

Temporary production mutations were applied individually, run against the named test, and restored. Detection required an assertion failure, not a build failure. These are targeted probes, not exhaustive mutation coverage.

| Mutation | Detecting test | Result |
| --- | --- | --- |
| Omit policy review | `TestCosmeticSeverityAndPolicy` | Detected |
| Emit wrong review | `TestCosmeticSeverityAndPolicy` | Detected |
| Annotation changes identity | `TestPolicyReviewPreservesCase` | Detected |
| Annotation changes FAIL to WARN | `TestPolicyReviewPreservesCase` | Detected after adding an explicit expected FAIL check; the first probe exposed an oracle weakness |
| Annotation deletes receipts | `TestPolicyReviewPreservesCase` | Detected |
| Annotation corrupts an existing clue through a shared slice | `TestPolicyReviewPreservesCase` | Detected |
| Omit expansion copy sites | `TestExpandedComplexity` | Detected |
| Reset copied complexity nesting to zero | `TestExpandedComplexity` | Detected |
| Omit parent-input ownership clue | `TestReachableClusterOwnership` | Detected |

Final checks: `go test ./...`, `go vet ./...`, CLI build, and `columbo --no-history --format=json ./...`. Self-check: **0 FAIL, 0 WARN, 0 suppressed**. CI rules, thresholds, exclusions, severities, and CE-001 are unchanged. CI verification is recorded in the PR rather than predicted here.
