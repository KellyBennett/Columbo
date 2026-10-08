package columbo

import (
	"go/ast"
	"go/token"
)

type guardedMutation struct {
	node          ast.Node
	target, value ast.Expr
	operator      token.Token
}

func guardedMutationOf(node ast.Stmt) *guardedMutation {
	switch statement := node.(type) {
	case *ast.AssignStmt:
		return guardedAssignment(statement)
	case *ast.IncDecStmt:
		return guardedIncrement(statement)
	}
	return nil
}
func guardedAssignment(node *ast.AssignStmt) *guardedMutation {
	if len(node.Lhs) != 1 || len(node.Rhs) != 1 || node.Tok == token.DEFINE {
		return nil
	}
	return &guardedMutation{node, node.Lhs[0], node.Rhs[0], node.Tok}
}
func guardedIncrement(node *ast.IncDecStmt) *guardedMutation {
	operation := token.ADD_ASSIGN
	if node.Tok == token.DEC {
		operation = token.SUB_ASSIGN
	}
	return &guardedMutation{node: node, target: node.X, operator: operation}
}

var guardedCompoundOperators = map[token.Token]token.Token{
	token.ADD_ASSIGN: token.ADD, token.SUB_ASSIGN: token.SUB, token.MUL_ASSIGN: token.MUL,
	token.QUO_ASSIGN: token.QUO, token.REM_ASSIGN: token.REM, token.AND_ASSIGN: token.AND,
	token.OR_ASSIGN: token.OR, token.XOR_ASSIGN: token.XOR, token.SHL_ASSIGN: token.SHL,
	token.SHR_ASSIGN: token.SHR, token.AND_NOT_ASSIGN: token.AND_NOT,
}

func (mutation guardedMutation) write(normalizer *guardedNormalizer) string {
	value := normalizer.expression(mutation.value)
	if mutation.value == nil {
		value = normalizer.one(mutation.target)
	}
	if value == "" || mutation.operator == token.ASSIGN {
		return value
	}
	operation, ok := guardedCompoundOperators[mutation.operator]
	if !ok {
		return ""
	}
	return normalizer.binaryForm(mutation.target, operation, "field", value)
}
