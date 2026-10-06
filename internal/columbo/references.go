package columbo

import "go/ast"

// referenceVisitor interprets the two syntax forms that name an identifier or
// selected member. Callers own normalization and semantic eligibility.
type referenceVisitor[T any] interface {
	identifier(*ast.Ident) T
	selector(*ast.SelectorExpr) T
}

// visitReference returns the result's zero value for unsupported expressions.
func visitReference[T any](expr ast.Expr, visitor referenceVisitor[T]) T {
	switch n := expr.(type) {
	case *ast.Ident:
		return visitor.identifier(n)
	case *ast.SelectorExpr:
		return visitor.selector(n)
	}
	var zero T
	return zero
}
