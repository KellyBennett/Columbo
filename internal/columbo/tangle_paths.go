package columbo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

type tangleFacts struct{ info *types.Info }

func (f tangleFacts) path(expr ast.Expr) string {
	expr = unparen(expr)
	if n, ok := expr.(*ast.IndexExpr); ok {
		return tangleJoin(f.path(n.X), f.index(n.Index), "[")
	}
	if n, ok := expr.(*ast.StarExpr); ok {
		return tangleJoin(f.path(n.X), "*", "")
	}
	return visitReference(expr, f)
}
func (f tangleFacts) identifier(id *ast.Ident) string {
	v, ok := f.info.ObjectOf(id).(*types.Var)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%p", v)
}
func (f tangleFacts) selector(expr *ast.SelectorExpr) string { return f.selectedPath(expr) }
func tangleJoin(root, part, separator string) string {
	if root == "" || part == "" {
		return ""
	}
	return root + separator + part
}
func (f tangleFacts) selectedPath(n *ast.SelectorExpr) string {
	field := choiceFieldKey(f.info.Selections[n])
	return tangleJoin(f.path(n.X), field, ".")
}
func (f tangleFacts) index(expr ast.Expr) string {
	if value := f.info.Types[expr].Value; value != nil {
		return value.ExactString()
	}
	return f.path(expr)
}
func (f tangleFacts) field(expr ast.Expr) (string, string) {
	selector, ok := unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	return choiceFieldKey(f.info.Selections[selector]), f.path(selector.X)
}
func (f tangleFacts) comparisons(expr ast.Expr) []tangleComparison {
	expr = unparen(expr)
	if n, ok := expr.(*ast.UnaryExpr); ok && n.Op == token.NOT {
		return f.comparisons(n.X)
	}
	n, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	return f.binary(n)
}
func (f tangleFacts) binary(n *ast.BinaryExpr) []tangleComparison {
	op := n.Op
	if op == token.LAND || op == token.LOR {
		return append(f.comparisons(n.X), f.comparisons(n.Y)...)
	}
	if op == token.EQL || op == token.NEQ {
		return f.comparison(n)
	}
	return nil
}

type tangleComparison struct {
	field, receiver, value string
	node                   *ast.BinaryExpr
}

func (f tangleFacts) comparison(n *ast.BinaryExpr) []tangleComparison {
	expr, literal := n.X, n.Y
	if f.info.Types[literal].Value == nil {
		expr, literal = literal, expr
	}
	comparison := f.operands(expr, literal)
	if comparison == nil {
		return nil
	}
	comparison.node = n
	return []tangleComparison{*comparison}
}
func (f tangleFacts) operands(expr, literal ast.Expr) *tangleComparison {
	value := f.info.Types[literal].Value
	field, receiver := f.field(expr)
	if value == nil || field == "" || receiver == "" || !tangleScalar(f.info.TypeOf(expr)) {
		return nil
	}
	return &tangleComparison{field: field, receiver: receiver, value: value.ExactString()}
}
func tangleScalar(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&(types.IsString|types.IsInteger) != 0
}
