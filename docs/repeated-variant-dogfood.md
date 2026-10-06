# RVD-001 initial dogfood evidence

The initial production `./...` self-check had 2 FAIL cases from the new rule, 4 duplicate-code WARN cases, and no suppressions. Other default FAIL smells were green. The rule and its 2-site / 2-variant minima remain enabled. The following sections preserve the original evidence and subsequent refactoring results.

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

## Shared resolved value path follow-up

Feature Envy and Repeated Variant Decision now use one resolved value path: a root variable plus resolved field objects. It owns identity, equality and display-key rendering. Caller-specific normalization remains separate: Feature Envy accepts address-taking/dereferencing at each step, while variant chains accept parentheses only. Calls, indexes, package selections, constants and method selections remain outside path resolution.

The useful abstraction removes one decision site from the group, but the architectural case remains:

| Metric | Initial implementation | Shared value path |
| --- | ---: | ---: |
| Supporting ast.Expr sites | 4 | 3 |
| Ident variant support | 3 | 2 |
| SelectorExpr variant support | 4 | 3 |

Current case identity remains `C-499393d3e6`. All other default FAIL smells remain green; the default self-check still reports 2 FAIL, 4 WARN and no suppressions. The types.Type traversal has not been changed.

| Current decision location | Enclosing declaration | Complete site variant set |
| --- | --- | --- |
| `internal/columbo/calls.go:176` (byte 4883) | `github.com/KellyBennett/Columbo/internal/columbo.calleeIdentifier` | `type:go/ast.Expr=*go/ast.Ident`, `type:go/ast.Expr=*go/ast.SelectorExpr` |
| `internal/columbo/metrics.go:316` (byte 9818) | `github.com/KellyBennett/Columbo/internal/columbo.(*dependencyScan).expression` | `type:go/ast.Expr=*go/ast.CompositeLit`, `type:go/ast.Expr=*go/ast.SelectorExpr`, `type:go/ast.Expr=*go/ast.TypeAssertExpr` |
| `internal/columbo/value_paths.go:47` (byte 1454) | `github.com/KellyBennett/Columbo/internal/columbo.(*valuePathResolver).resolve` | `type:go/ast.Expr=*go/ast.Ident`, `type:go/ast.Expr=*go/ast.SelectorExpr` |

Regression tests pin strict versus Feature Envy normalization, shadowed roots, distinct field chains, immutable path extension, and variant exclusion of address/dereference chains. Existing logical-row/summary goldens pass without changes.

## Type visitor exploration

Interface discovery and dependency collection now implement a three-message `typeVisitor`: `named`, `iface`, and `components`. `visitType` owns alias normalization and the single named/interface/structural classification. Each visitor continues to own its recursion and results; discovery also retains its seen set.

| Message | Interface discovery | Dependency collection |
| --- | --- | --- |
| named | Discover named interfaces; traverse the underlying type | Record the named identity; traverse generic type arguments |
| iface | Record the interface; traverse method signatures | Record the interface identity |
| components | Traverse structural components | Traverse structural components |

This small protocol gives semantic type classification one owner without making either analysis depend on the other. A factory and per-type wrappers would add objects without further behavior to own, so the implementation uses direct visitor dispatch. The cost is an extra interface call and navigation step between traversal and its policy methods. The benefit is a common classification boundary with explicitly different traversal policies; this is a modest improvement, not evidence that all repeated switches need visitors.

The `type:go/types.Type` case `C-44ed8eaf6c` clears because only one qualifying decision remains. Default self-check now reports **1 FAIL, 4 WARN, 0 suppressed**. The remaining `ast.Expr` case retains identity `C-499393d3e6`, three supporting sites, Ident support 2, and SelectorExpr support 3. No thresholds, severity settings, or suppressions changed. Its dependency-scanning site has moved to `metrics.go:308` (byte 9733).

A regression fixture combines an alias, an instantiated generic recursive type, an interface-valued field, and nested method-signature interfaces. It confirms that discovery reaches underlying fields and method signatures without looping or duplicating results, while dependency collection preserves named boundaries and follows type arguments. Full Go tests, vet, existing evidence goldens, and the SQL boundary/sqlc checks pass.
