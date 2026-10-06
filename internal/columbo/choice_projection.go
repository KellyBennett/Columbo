package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

func (c *choiceConstruction) extend() bool {
	if !choiceProjectionLoop(c.loop) {
		return false
	}
	if !c.sourceCollection() {
		return false
	}
	return c.project(c.loop.Body.List[0])
}
func choiceProjectionLoop(loop *ast.RangeStmt) bool {
	return loop.Tok == token.DEFINE && len(loop.Body.List) == 1 && blankChoiceKey(loop.Key)
}
func (c *choiceConstruction) sourceCollection() bool {
	if !resolveValuePath(c.facts.info, c.loop.X, ast.Unparen).valid() {
		return false
	}
	c.collection = c.facts.fieldKey(c.loop.X)
	return c.collection != ""
}
func blankChoiceKey(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "_"
}
func (c choiceFacts) fieldKey(expr ast.Expr) string {
	selector, ok := unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return choiceFieldKey(c.info.Selections[selector])
}
func choiceFieldKey(selection *types.Selection) string {
	if selection == nil || selection.Kind() != types.FieldVal || len(selection.Index()) != 1 {
		return ""
	}
	return choiceReceiverKey(selection.Recv(), selection.Obj().Name())
}
func (c *choiceConstruction) project(stmt ast.Stmt) bool {
	assignment, ok := stmt.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return false
	}
	if c.representation == "map-keys" {
		return c.mapProjection(assignment)
	}
	return c.sliceProjection(assignment)
}
func (c *choiceConstruction) sameLocal(expr ast.Expr) bool {
	id, ok := unparen(expr).(*ast.Ident)
	return ok && c.facts.info.ObjectOf(id) == c.local
}
func (c *choiceConstruction) mapProjection(stmt *ast.AssignStmt) bool {
	index, ok := unparen(stmt.Lhs[0]).(*ast.IndexExpr)
	return ok && c.sameLocal(index.X) && c.facts.inert(stmt.Rhs[0]) && c.elementProjection(index.Index)
}
func (c *choiceConstruction) sliceProjection(stmt *ast.AssignStmt) bool {
	call, ok := unparen(stmt.Rhs[0]).(*ast.CallExpr)
	if !ok || !c.sameLocal(stmt.Lhs[0]) || len(call.Args) != 2 || call.Ellipsis.IsValid() {
		return false
	}
	if !c.facts.isAppend(call.Fun) || !c.sameLocal(call.Args[0]) {
		return false
	}
	return c.elementProjection(call.Args[1])
}
func (c *choiceConstruction) elementProjection(expr ast.Expr) bool {
	selector, ok := unparen(expr).(*ast.SelectorExpr)
	if !ok || c.loop.Value == nil {
		return false
	}
	if !c.facts.sameVariable(selector.X, c.loop.Value) {
		return false
	}
	if !c.facts.slice(c.loop.X) {
		return false
	}
	c.projection = c.facts.fieldKey(selector)
	return c.projection != ""
}
