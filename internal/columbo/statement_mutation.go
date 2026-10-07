package columbo

import (
	"go/ast"
	"go/token"
)

type statementMutation struct {
	targets []ast.Expr
	reads   []ast.Expr
}

func mutationOf(node ast.Node) (statementMutation, bool) {
	switch n := node.(type) {
	case *ast.AssignStmt:
		return assignmentMutation(n), true
	case *ast.IncDecStmt:
		return statementMutation{[]ast.Expr{n.X}, []ast.Expr{n.X}}, true
	}
	return statementMutation{}, false
}

func assignmentMutation(n *ast.AssignStmt) statementMutation {
	m := statementMutation{targets: n.Lhs, reads: n.Rhs}
	if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
		m.reads = append(append([]ast.Expr{}, n.Lhs...), n.Rhs...)
	}
	return m
}
