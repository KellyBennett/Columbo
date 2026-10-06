package columbo

import (
	"go/ast"
	"go/types"
)

type choiceFacts struct{ info *types.Info }

func (c choiceFacts) object(expr ast.Expr) types.Object {
	return visitReference(unparen(expr), choiceObjects{c.info})
}

type choiceObjects struct{ info *types.Info }

func (c choiceObjects) identifier(id *ast.Ident) types.Object { return c.info.ObjectOf(id) }
func (c choiceObjects) selector(expr *ast.SelectorExpr) types.Object {
	return c.info.ObjectOf(expr.Sel)
}
func (c choiceFacts) localDefinition(expr ast.Expr) *types.Var {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	local, _ := c.info.Defs[id].(*types.Var)
	return local
}
func choiceRepresentation(t types.Type) string {
	switch t.Underlying().(type) {
	case *types.Map:
		return "map-keys"
	case *types.Slice:
		return "slice-elements"
	}
	return ""
}
func (c choiceFacts) mapKey(expr ast.Expr) ast.Expr {
	pair, ok := expr.(*ast.KeyValueExpr)
	if !ok || !c.inert(pair.Value) {
		return nil
	}
	return pair.Key
}
func choiceConstant(obj types.Object) (ChoiceConstant, bool) {
	constant, ok := obj.(*types.Const)
	if !ok || constant.Pkg() == nil {
		return ChoiceConstant{}, false
	}
	return ChoiceConstant{constant.Pkg().Path() + "." + constant.Name(), constant.Val().ExactString()}, true
}
func (c choiceFacts) inert(expr ast.Expr) bool {
	if _, ok := unparen(expr).(*ast.BasicLit); ok {
		return true
	}
	return visitReference(unparen(expr), choiceInert{c.info})
}

type choiceInert struct{ info *types.Info }

func (c choiceInert) identifier(id *ast.Ident) bool { return c.info.ObjectOf(id) != nil }
func (c choiceInert) selector(expr *ast.SelectorExpr) bool {
	return (choiceFacts{c.info}).inert(expr.X)
}
func choiceReceiverKey(t types.Type, name string) string {
	receiver, ok := stripPointer(t).(*types.Named)
	if !ok {
		return ""
	}
	return canonicalType(receiver, nil) + "." + name
}
func (c choiceFacts) isAppend(expr ast.Expr) bool {
	return c.object(expr) == types.Universe.Lookup("append")
}
func (c choiceFacts) slice(expr ast.Expr) bool {
	_, ok := c.info.TypeOf(expr).Underlying().(*types.Slice)
	return ok
}

func (c choiceFacts) sameVariable(left, right ast.Expr) bool {
	a, ok := unparen(left).(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := unparen(right).(*ast.Ident)
	return ok && c.info.ObjectOf(a) != nil && c.info.ObjectOf(a) == c.info.ObjectOf(b)
}
