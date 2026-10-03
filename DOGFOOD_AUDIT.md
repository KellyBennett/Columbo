# Dogfooding audit

## Dependency policy revision

Cohesive adapters exposed a recurring pattern: package identities duplicated their
named types, while called signatures added universal error/empty-interface plumbing.
The dependency evidence inventory remains complete. The scored collaborator set
omits `type:error`, `interface:interface{}`, and packages already represented by an
included named type owned by that package. Package-only operations, named interfaces,
nonempty anonymous interfaces, and other called-signature types remain counted.
Thresholds, severities, and exclusions are unchanged.

| Function | Previous count | Revised count | Action |
| --- | ---: | ---: | --- |
| `parsePhysicalFile` | 8 | 3 | Remove temporary suppression. |
| `(*typeNormalizer).method` | 6 | 5 | Passes without extraction or suppression. |
| `serializeJSON` | 7 | 3 | Passes without extraction or suppression. |

Scored receipts use `dependency`; additional evidence uses `dependency-inventory`.
Excessive Dependencies reports include the scored identity set. Cosmetic Extraction
uses the same scored sets, preserving both receipt kinds. Regression coverage checks
that shared error/any plumbing does not create helper overlap, while existing
arbitrary-extraction cases continue to fail. A six-package fixture verifies that
unrelated package-only collaborators still exceed the default dependency limit.

This calibration does not resolve all architectural findings or CE-001. The strict
self-check remains enforced; current counts must come from the complete CLI run.

## Measurement and reference-discovery refactoring

The measurement pipeline now separates recursive type policy, physical dependency
sites, line contributions, complexity evidence, and ordinary verdict creation.
Physical source objects own token ranges, parameter receipts, and declaration
identity. Repeated measurement replaces complexity evidence rather than duplicating
it; the dependency scenarios verify that invariant.

Call discovery now owns a reference graph with separate declaration-call and
whole-file-reference traversals, plus helper eligibility classification. This
retains package initializer and first-class reference disqualification, concrete
method/interface checks, and physical identity across normal/test variants.
Interface discovery and dependency collection share structural type-component
traversal while retaining their different policies for named types and interfaces.
Private-type indexing retains references from the entire selected physical-source
universe, including excluded files and tests.

No smell definitions, thresholds, severities, exclusions, or CI settings changed
in this refactoring batch. No suppression was added and no golden was refreshed.

## Cosmetic Extraction refactoring

Virtual expansion now shares a recursive call-path context. The line worker owns
its token mask and copied line evidence; the complexity worker owns its visitor
hook and diagnostic evidence. Call masks retain nested-call token ranges while
removing only replaced call syntax. Child traces remain independent, and recursive
expansion still stops at the active declaration stack.

Helper clusters own exact forwarding and dependency-overlap calculations. A lexical
scanner owns maximal statement sequences, with explicit grammar tables preserving
all original scope exclusions and sequence boundaries. Builtin/type-call and logging
exceptions retain their original identities. Cluster presentation owns its record,
member evidence, and shared helper-deduplication set. Physical source objects own
call sites and identifier/call receipt ranges.

No smell definitions, thresholds, severities, exclusions, or CI settings changed.
No suppression was added and no golden was refreshed. Existing coverage continues
to verify nested-call token retention, recursive cutoffs, child-copy paths,
nesting-sensitive complexity, maximal clusters, exact overlaps, interface helper
eligibility, and arbitrary/meaningful-but-strict extraction cases.

## Validation at this revision

Tests, vet, and CLI build pass. The strict self-check reports 95 FAIL cases,
zero warnings, and zero suppressions: 47 Excessive Dependencies, 36 Long Function,
7 High Cognitive Complexity, and 5 Feature Envy. The previous refactoring revision
had 122 FAIL cases. Tests remain included in self-analysis.

The Cosmetic Extraction implementation and virtual expansion workers now clear
all configured smells. Remaining findings are in source loading/model construction,
the cognitive-complexity visitor, other smell implementations, and larger test
scenarios. CE-001 remains provisional; its release gate is unchanged.
