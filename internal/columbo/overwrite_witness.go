package columbo

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

type overwriteWitness struct {
	inGuard      bool
	lastWrites   map[types.Object]*ast.AssignStmt
	predecessors map[*ast.AssignStmt]*ast.AssignStmt
	scan         *overwriteScan
	inputs       map[overwriteInput]constant.Value
	locals       map[types.Object]constant.Value
	executed     map[*ast.AssignStmt]bool
	receipts     []Source
}

func (w *overwriteWitness) expression(e ast.Expr) constant.Value {
	if value := w.resolved(e); value != nil {
		return value
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return w.expression(e.X)
	case *ast.BinaryExpr:
		return w.binary(e)
	}
	return nil
}
func (w *overwriteWitness) resolved(e ast.Expr) constant.Value {
	if value := w.scan.constant(e); value != nil {
		return value
	}
	if key, ok := w.scan.input(e); ok {
		return w.inputs[key]
	}
	if id, ok := e.(*ast.Ident); ok && !w.inGuard {
		return w.locals[w.scan.info.ObjectOf(id)]
	}
	return nil
}
func (w *overwriteWitness) binary(e *ast.BinaryExpr) constant.Value {
	left := w.expression(e.X)
	if left == nil {
		return nil
	}
	if e.Op == token.LAND && left.Kind() == constant.Bool && !constant.BoolVal(left) {
		return constant.MakeBool(false)
	}
	right := w.expression(e.Y)
	if right == nil {
		return nil
	}
	return w.operation(e, left, right)
}
func (w *overwriteWitness) operation(e *ast.BinaryExpr, left, right constant.Value) constant.Value {
	if e.Op == token.LAND && left.Kind() == constant.Bool && right.Kind() == constant.Bool {
		return constant.MakeBool(constant.BoolVal(left) && constant.BoolVal(right))
	}
	if left.Kind() != constant.Int || right.Kind() != constant.Int {
		return nil
	}
	return overwriteOperation(e.Op, left, right, w.scan.info.TypeOf(e))
}
func overwriteOperation(op token.Token, left, right constant.Value, t types.Type) constant.Value {
	if guardedComparison(op) {
		return constant.MakeBool(constant.Compare(left, op, right))
	}
	if op == token.SUB {
		return overwriteDifference(left, right, t)
	}
	return nil
}
func overwriteDifference(left, right constant.Value, t types.Type) constant.Value {
	result := constant.BinaryOp(left, token.SUB, right)
	if overwriteFits(result, t) {
		return result
	}
	return nil
}
func overwriteFits(v constant.Value, t types.Type) bool {
	if !overwriteInteger(t) {
		return false
	}
	bits := overwriteBits(t.Underlying().(*types.Basic).Kind())
	number, ok := constant.Int64Val(v)
	if !ok {
		return false
	}
	if bits == 64 {
		return true
	}
	limit := int64(1) << (bits - 1)
	return number >= -limit && number < limit
}
func overwriteBits(kind types.BasicKind) int {
	switch kind {
	case types.Int8:
		return 8
	case types.Int16:
		return 16
	case types.Int64:
		return 64
	}
	return 32
}
func (w *overwriteWitness) prefix(last *ast.IfStmt) bool {
	for _, stmt := range w.scan.owner.fn.Body.List {
		if !w.statement(stmt) {
			return false
		}
		if stmt == last {
			return true
		}
	}
	return false
}
func (w *overwriteWitness) statement(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.IfStmt:
		return w.decision(stmt)
	case *ast.AssignStmt:
		return w.assignment(stmt)
	case *ast.EmptyStmt:
		return true
	}
	return false
}
func (w *overwriteWitness) decision(stmt *ast.IfStmt) bool {
	w.inGuard = true
	value := w.expression(stmt.Cond)
	w.inGuard = false
	if value == nil || value.Kind() != constant.Bool {
		return false
	}
	w.receipts = append(w.receipts, w.scan.owner.nodeSource("witness-path-condition", stmt.Cond, Detail{Subject: value.ExactString()}))
	return !constant.BoolVal(value) || w.block(stmt.Body)
}
func (w *overwriteWitness) block(body *ast.BlockStmt) bool {
	for _, child := range body.List {
		if !w.statement(child) {
			return false
		}
	}
	return true
}
func (s *overwriteScan) witness(first, last overwriteCandidate) *overwriteWitness {
	for _, values := range overwriteSamples(len(s.inputs)) {
		w := s.newWitness(values)
		if w != nil && w.overwrites(first, last) {
			return w
		}
	}
	return nil
}
func overwriteSamples(dimensions int) [][]int {
	samples := [][]int{}
	for first := -2; first <= 8; first++ {
		if dimensions == 1 {
			samples = append(samples, []int{first})
			continue
		}
		for second := -2; second <= 8; second++ {
			samples = append(samples, []int{first, second})
		}
	}
	return samples
}
func (w *overwriteWitness) overwrites(first, last overwriteCandidate) bool {
	return w.prefix(last.guard) && w.executed[first.write] && w.executed[last.write] && w.predecessors[last.write] == first.write
}
func (s *overwriteScan) newWitness(values []int) *overwriteWitness {
	w := &overwriteWitness{lastWrites: map[types.Object]*ast.AssignStmt{}, predecessors: map[*ast.AssignStmt]*ast.AssignStmt{}, scan: s, inputs: map[overwriteInput]constant.Value{}, locals: map[types.Object]constant.Value{}, executed: map[*ast.AssignStmt]bool{}}
	for i, input := range s.inputs {
		value := constant.MakeInt64(int64(values[i]))
		if !overwriteFits(value, input.typ()) {
			return nil
		}
		w.inputs[input] = value
	}
	return w
}
