package columbo

import (
	"go/ast"
	"go/token"
	"reflect"
)

type roleArm struct {
	labels []ast.Expr
	body   ast.Node
}
type roleArmBuilder func(ast.Node) []roleArm

var roleArmBuilders = map[reflect.Type]roleArmBuilder{
	reflect.TypeOf((*ast.SwitchStmt)(nil)):     valueRoleArms,
	reflect.TypeOf((*ast.TypeSwitchStmt)(nil)): typeRoleArms,
	reflect.TypeOf((*ast.IfStmt)(nil)):         conditionalRoleArms,
}
var roleBoundaries = map[reflect.Type]bool{
	reflect.TypeOf((*ast.FuncLit)(nil)):        true,
	reflect.TypeOf((*ast.IfStmt)(nil)):         true,
	reflect.TypeOf((*ast.SwitchStmt)(nil)):     true,
	reflect.TypeOf((*ast.TypeSwitchStmt)(nil)): true,
	reflect.TypeOf((*ast.ForStmt)(nil)):        true,
	reflect.TypeOf((*ast.RangeStmt)(nil)):      true,
	reflect.TypeOf((*ast.SelectStmt)(nil)):     true,
}

func selectionRoleArms(node ast.Node) []roleArm {
	if builder := roleArmBuilders[reflect.TypeOf(node)]; builder != nil {
		return builder(node)
	}
	return nil
}
func valueRoleArms(node ast.Node) []roleArm {
	n := node.(*ast.SwitchStmt)
	if hasSelectionBranch(n, token.FALLTHROUGH) {
		return nil
	}
	return switchRoleArms(n.Body)
}
func typeRoleArms(node ast.Node) []roleArm { return switchRoleArms(node.(*ast.TypeSwitchStmt).Body) }
func switchRoleArms(body *ast.BlockStmt) []roleArm {
	var arms []roleArm
	for _, stmt := range body.List {
		clause := stmt.(*ast.CaseClause)
		arms = append(arms, roleArm{labels: clause.List, body: clause})
	}
	return arms
}
func conditionalRoleArms(node ast.Node) []roleArm {
	branch := node.(*ast.IfStmt)
	arms := []roleArm{{labels: conditionRoleLabels(branch.Cond), body: branch.Body}}
	if tail, ok := branch.Else.(*ast.IfStmt); ok {
		return append(arms, conditionalRoleArms(tail)...)
	}
	if branch.Else != nil {
		arms = append(arms, roleArm{body: branch.Else})
	}
	return arms
}
func conditionRoleLabels(condition ast.Expr) []ast.Expr {
	var labels []ast.Expr
	ast.Inspect(condition, func(node ast.Node) bool {
		if expr, ok := node.(ast.Expr); ok {
			labels = append(labels, expr)
		}
		return true
	})
	return labels
}
func roleNestedBoundary(node, root ast.Node) bool {
	return node != root && roleBoundaries[reflect.TypeOf(node)]
}
