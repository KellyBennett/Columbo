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

## Validation at this revision

Tests, vet, and CLI build pass. The strict self-check reports 153 FAIL cases,
zero warnings, and zero suppressions: 52 long-function, 69 excessive-dependencies, 12 feature-envy, 20 high-cognitive-complexity.
The additional regression scenarios remain included in self-analysis.
