package columbo

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

type overwriteAssignment struct {
	statement  *ast.AssignStmt
	target     types.Object
	identifier bool
	value      ast.Expr
}

func (s *overwriteScan) decodeAssignment(stmt *ast.AssignStmt) *overwriteAssignment {
	if (stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE) || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return nil
	}
	assignment := &overwriteAssignment{statement: stmt, value: stmt.Rhs[0]}
	if id, ok := stmt.Lhs[0].(*ast.Ident); ok {
		assignment.target = s.info.ObjectOf(id)
		assignment.identifier = true
	}
	return assignment
}
func (s *overwriteScan) constant(expr ast.Expr) constant.Value {
	value := s.info.Types[expr].Value
	if value != nil && overwriteScalar(value) {
		return value
	}
	return nil
}
func (w *overwriteWitness) assignment(stmt *ast.AssignStmt) bool {
	assignment := w.scan.decodeAssignment(stmt)
	if assignment == nil {
		return false
	}
	value := w.expression(assignment.value)
	if value == nil || (!assignment.identifier && w.scan.constant(assignment.value) == nil) {
		return false
	}
	w.recordAssignment(assignment, value)
	return true
}
func (w *overwriteWitness) recordAssignment(assignment *overwriteAssignment, value constant.Value) {
	stmt := assignment.statement
	if assignment.identifier {
		w.predecessors[stmt] = w.lastWrites[assignment.target]
		w.lastWrites[assignment.target] = stmt
		w.locals[assignment.target] = value
	}
	w.executed[stmt] = true
	w.receipts = append(w.receipts, w.scan.owner.nodeSource("witness-path-assignment", stmt, Detail{Subject: value.ExactString()}))
}
