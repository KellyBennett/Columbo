# Dogfooding audit

Keep the current smell definitions, thresholds, and CI enforcement while collecting
examples. A single cohesive adapter is not enough evidence to change the dependency
rule. Review temporary suppressions after more examples have been examined.

## Temporary suppression

| Function | Rule | Count / limit | Reason | Later review |
| --- | --- | --- | --- | --- |
| `parsePhysicalFile` in `internal/columbo/source.go` | Excessive Dependencies | 8 / 5 | Required Go parser API types in a single-call forwarding adapter. Approved for temporary suppression during dogfooding. | Keep, remove, refactor, or revise the counting rule based on the accumulated examples. |

The eight identities are `package:go/ast`, `package:go/parser`,
`package:go/token`, `type:go/ast.File`, `type:go/token.FileSet`,
`type:go/parser.Mode`, `type:error`, and `interface:interface{}`. The empty
interface comes from the parser's source argument signature; this adapter supplies
a byte slice. Primitive `string` and `byte` do not contribute identities.

## Examples to inspect next

These are observations, not approved exceptions. Their findings remain enforced.

| Function | Count / limit | Possible pattern to investigate |
| --- | --- | --- |
| `(*typeNormalizer).method` in `internal/columbo/metrics.go` | 6 / 5 | A single type-construction expression exposes several types from the same Go tooling API. |
| `serializeJSON` in `internal/columbo/model.go` | 7 / 5 | A small serialization routine counts buffer/encoder types, their packages, `io.Writer`, `error`, and the input empty interface. |

For each new example, record the code and exact dependency identities, distinguish
required API plumbing from mixed responsibilities, and consider a concrete
refactor before proposing a suppression. Look for recurring patterns across
different responsibilities before deciding whether the rule should change.
