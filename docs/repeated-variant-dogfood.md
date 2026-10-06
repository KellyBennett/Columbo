# RVD-001 initial dogfood evidence

The default production `./...` self-check has 2 FAIL cases from the new rule, 4 duplicate-code WARN cases, and no suppressions. Other default FAIL smells are green. The rule and its 2-site / 2-variant minima remain enabled. These are review leads under RVD-001; no automatic refactor or new suppression was applied to the typed interpreter decisions.

## type:go/ast.Expr

Case: `C-499393d3e6`.

| Repeated variant | Distinct-site support |
| --- | ---: |
| `type:go/ast.Expr=*go/ast.Ident` | 3 |
| `type:go/ast.Expr=*go/ast.SelectorExpr` | 4 |

| Decision location | Enclosing declaration | Complete site variant set |
| --- | --- | --- |
| `internal/columbo/calls.go:176` (byte 4883) | `github.com/KellyBennett/Columbo/internal/columbo.calleeIdentifier` | `type:go/ast.Expr=*go/ast.Ident`, `type:go/ast.Expr=*go/ast.SelectorExpr` |
| `internal/columbo/envy.go:40` (byte 867) | `github.com/KellyBennett/Columbo/internal/columbo.(*valueResolver).resolve` | `type:go/ast.Expr=*go/ast.Ident`, `type:go/ast.Expr=*go/ast.SelectorExpr` |
| `internal/columbo/metrics.go:316` (byte 9818) | `github.com/KellyBennett/Columbo/internal/columbo.(*dependencyScan).expression` | `type:go/ast.Expr=*go/ast.CompositeLit`, `type:go/ast.Expr=*go/ast.SelectorExpr`, `type:go/ast.Expr=*go/ast.TypeAssertExpr` |
| `internal/columbo/variant_conditions.go:28` (byte 620) | `github.com/KellyBennett/Columbo/internal/columbo.(*variantExpressionPath).objects` | `type:go/ast.Expr=*go/ast.Ident`, `type:go/ast.Expr=*go/ast.SelectorExpr` |

## type:go/types.Type

Case: `C-44ed8eaf6c`.

| Repeated variant | Distinct-site support |
| --- | ---: |
| `type:go/types.Type=*go/types.Interface` | 2 |
| `type:go/types.Type=*go/types.Named` | 2 |

| Decision location | Enclosing declaration | Complete site variant set |
| --- | --- | --- |
| `internal/columbo/interfaces.go:51` (byte 1233) | `github.com/KellyBennett/Columbo/internal/columbo.(*interfaceDiscovery).components` | `type:go/types.Type=*go/types.Interface`, `type:go/types.Type=*go/types.Named` |
| `internal/columbo/metrics.go:190` (byte 6360) | `github.com/KellyBennett/Columbo/internal/columbo.(*dependencyCollector).walk` | `type:go/types.Type=*go/types.Interface`, `type:go/types.Type=*go/types.Named` |

## Review before clearing

Both domains describe Go syntax/type interpretation. Some decisions perform distinct structural traversals and may be reasonable despite sharing the taxonomy. Review the complete sets and operations before deciding whether common roles, one dispatch boundary, or data representation would help. RVD-001 intentionally keeps these cases failing while this architectural choice remains unresolved.

Validation: Go suite and vet; SQL access guard and its negative regression fixtures; pinned sqlc generation; published SQLite queries; action runner tests; GitHub annotation tests, including all variant evidence and policy metadata.
