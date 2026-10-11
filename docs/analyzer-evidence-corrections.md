# Source and dependency evidence corrections

The analyzer retains physical source positions, declaration ownership and checked generic dependency identities under the existing analysis policy and thresholds.

## Physical source positions

Go line directives can remap logical file positions. Receipts read physical source text, so using a remapped line as a physical line index can point outside the file. The regression fixture covers a logical line 901 in a six-line physical source file.

The file line accessor now requests unadjusted physical positions, and line-metric collection uses that accessor. `TestLineDirectivesKeepMetricReceiptsPhysical` checks that the two counted source lines are lines 4 and 5 and belong to the physical source file.

This is a source-position correction. It does not add cgo loading or analysis support.

## Private type declaration ownership

`types.Info.Defs` is package-wide. Iterating its whole map for each file incorrectly allowed the first visited file to own definitions from other files. That changed the participating-file set, cross-file access count, and treatment of excluded declarations.

The definition scan now walks identifiers in the current file's AST and resolves only those identifiers through the shared map. Existing private-type, alias, and production filters remain in place.

`TestPrivateTypeDefinitionOwnership` covers forward/reverse file order, unrelated files before/after the declaration file, the actual declaration location, the true three-file footprint, and exclusion of same-file accesses. `TestPrivateTypeDefinitionProductionScope` verifies that excluded and test-file declarations cannot be reassigned to another production file.

## Generic instantiation dependencies

Visiting both an instantiated type expression and its generic syntax head could add a phantom uninstantiated dependency. The correction excludes the head only when:

1. AST ancestry proves that it is the head of an index or index-list expression, including intervening parentheses.
2. The parent expression is a checked type instantiation.
3. The checked instance and head have the same named-type or alias origin.

Alias identity is compared before unaliasing. The correction preserves actual type arguments, distinct specializations, ordinary named dependencies, generic function behavior, and existing receiver exclusions. It does not remove dependencies by name or underlying-type similarity.

The generic regression suites cover 17 inventory, threshold, and alias scenarios. Compound pointer, slice, and map aliases are included alongside a non-generic alias control. The existing dependency expectation for `atomic.Pointer[Buffer]` drops only the phantom generic origin. Seventeen additional direct tests cover syntax ownership and type-origin identity, including mismatched origins, non-instantiated types, and builtin rejection.

Regression controls retain five real dependencies and cover inventory variants and compound aliases. They constrain dependency identity and syntax ownership without changing analysis policy, thresholds, stage membership, suppressions, exclusions or module dependencies.
