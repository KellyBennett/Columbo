package columbo

import "go/ast"

type referenceVisitor[T any] interface {
	identifier(*ast.Ident) T
	selector(*ast.SelectorExpr) T
}

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

func visitUnary[T any](expr ast.Expr, visit func(*ast.UnaryExpr) T) (T, bool) {
	node, ok := expr.(*ast.UnaryExpr)
	if ok {
		return visit(node), true
	}
	var zero T
	return zero, false
}
