package columbo

import (
	"go/ast"
	"go/token"
)

func walkBooleanLeaves(expr ast.Expr, visit func(ast.Expr) bool) bool {
	expr = ast.Unparen(expr)
	if n, ok := expr.(*ast.BinaryExpr); ok && booleanConnector(n.Op) {
		return walkBooleanLeaves(n.X, visit) && walkBooleanLeaves(n.Y, visit)
	}
	return visit(expr)
}

func booleanConnector(op token.Token) bool {
	return op == token.LAND || op == token.LOR
}
