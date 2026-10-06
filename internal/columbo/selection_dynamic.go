package columbo

import "go/ast"

func (s *selectionScan) findDynamicLocals() {
	for {
		before := len(s.dynamicLocals)
		ast.Inspect(s.owner.fn.Body, s.dynamicNode)
		if before == len(s.dynamicLocals) {
			return
		}
	}
}
func (s *selectionScan) dynamicNode(node ast.Node) bool {
	if _, ok := node.(*ast.FuncLit); ok {
		return false
	}
	if assignment, ok := node.(*ast.AssignStmt); ok {
		s.dynamicAssignment(assignment.Lhs, assignment.Rhs)
	}
	if spec, ok := node.(*ast.ValueSpec); ok {
		s.dynamicDeclaration(spec)
	}
	return true
}
func selectionNames(names []*ast.Ident) []ast.Expr {
	expressions := make([]ast.Expr, len(names))
	for i, name := range names {
		expressions[i] = name
	}
	return expressions
}
func (s *selectionScan) dynamicAssignment(left, right []ast.Expr) {
	for _, expr := range right {
		if s.unsafeExpression(expr) {
			s.markDynamic(left)
			return
		}
	}
}
func (s *selectionScan) markDynamic(left []ast.Expr) {
	for _, expr := range left {
		if obj := s.local(expr); obj != nil {
			s.dynamicLocals[obj] = true
		}
	}
}

func (s *selectionScan) dynamicDeclaration(spec *ast.ValueSpec) {
	s.dynamicAssignment(selectionNames(spec.Names), spec.Values)
}
