package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

type categorySelection struct {
	selector ast.Expr
	branches []categoryBranch
}
type categoryBranch struct {
	node   ast.Node
	labels []ast.Expr
	body   []ast.Stmt
}

func categorySwitch(node *ast.SwitchStmt) categorySelection {
	selection := categorySelection{selector: node.Tag}
	for _, stmt := range node.Body.List {
		clause := stmt.(*ast.CaseClause)
		selection.branches = append(selection.branches, categoryBranch{clause, clause.List, clause.Body})
	}
	return selection
}
func categoryIf(node *ast.IfStmt, info *types.Info) categorySelection {
	selection := categorySelection{}
	for node != nil {
		if !selection.addIf(node, info) {
			return categorySelection{}
		}
		node = selection.ifTail(node)
	}
	return selection
}
func (selection *categorySelection) addIf(node *ast.IfStmt, info *types.Info) bool {
	selector, label := categoryEquality(node.Cond, info)
	if selector == nil || node.Init != nil {
		return false
	}
	if selection.selector == nil {
		selection.selector = selector
	}
	if !categoryPath(info, selection.selector).same(categoryPath(info, selector)) {
		return false
	}
	selection.branches = append(selection.branches, categoryBranch{node.Cond, []ast.Expr{label}, node.Body.List})
	return true
}
func (selection *categorySelection) ifTail(node *ast.IfStmt) *ast.IfStmt {
	if tail, ok := node.Else.(*ast.BlockStmt); ok {
		selection.branches = append(selection.branches, categoryBranch{tail, nil, tail.List})
	}
	tail, _ := node.Else.(*ast.IfStmt)
	return tail
}
func categoryEquality(expr ast.Expr, info *types.Info) (ast.Expr, ast.Expr) {
	binary := categoryComparison(expr)
	if binary == nil {
		return nil, nil
	}
	if info.Types[binary.Y].Value != nil {
		return binary.X, binary.Y
	}
	if info.Types[binary.X].Value != nil {
		return binary.Y, binary.X
	}
	return nil, nil
}
func categoryPath(info *types.Info, expr ast.Expr) resolvedValuePath {
	if expr == nil {
		return resolvedValuePath{}
	}
	return resolveValuePath(info, expr, ast.Unparen)
}
func (selection categorySelection) subject(info *types.Info) resolvedValuePath {
	path := categoryPath(info, selection.selector)
	if len(path.fields) == 0 {
		return resolvedValuePath{}
	}
	if !categoryFieldType(info, selection.selector) {
		return resolvedValuePath{}
	}
	path.fields = path.fields[:len(path.fields)-1]
	return path
}
func (branch categoryBranch) constantLabels(info *types.Info) bool {
	for _, label := range branch.labels {
		if info.Types[label].Value == nil {
			return false
		}
	}
	return true
}

func (branch categoryBranch) action() (*ast.CallExpr, bool) {
	if len(branch.body) == 0 {
		return nil, true
	}
	if len(branch.body) != 1 {
		return nil, false
	}
	return categoryStatementAction(branch.body[0])
}
func categoryStatementAction(stmt ast.Stmt) (*ast.CallExpr, bool) {
	if expression, ok := stmt.(*ast.ExprStmt); ok {
		return categoryExpressionAction(expression.X)
	}
	if result, ok := stmt.(*ast.ReturnStmt); ok {
		return categoryReturnAction(result)
	}
	return nil, categoryNoOp(stmt)
}
func categoryExpressionAction(expr ast.Expr) (*ast.CallExpr, bool) {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	return call, ok
}
func categoryReturnAction(result *ast.ReturnStmt) (*ast.CallExpr, bool) {
	if len(result.Results) == 0 {
		return nil, true
	}
	if len(result.Results) != 1 {
		return nil, false
	}
	return categoryExpressionAction(result.Results[0])
}
func categoryNoOp(stmt ast.Stmt) bool {
	if control, ok := stmt.(*ast.BranchStmt); ok {
		return control.Label == nil && (control.Tok == token.CONTINUE || control.Tok == token.BREAK)
	}
	_, empty := stmt.(*ast.EmptyStmt)
	return empty
}

func categoryComparison(expr ast.Expr) *ast.BinaryExpr {
	binary, ok := ast.Unparen(expr).(*ast.BinaryExpr)
	if !ok || binary.Op != token.EQL {
		return nil
	}
	return binary
}
func categoryFieldType(info *types.Info, expr ast.Expr) bool {
	basic, ok := info.TypeOf(expr).Underlying().(*types.Basic)
	return ok && basic.Info()&(types.IsString|types.IsInteger) != 0
}
