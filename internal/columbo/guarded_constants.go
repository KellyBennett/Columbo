package columbo

import (
	"go/ast"
	"go/types"
)

func (normalizer *guardedNormalizer) constant(expr ast.Expr) string {
	value := normalizer.owner.file.typeInfo().Types[expr]
	if value.Value == nil || !guardedScalar(value.Type) {
		return ""
	}
	return guardedForm("constant", normalizer.typeName(expr), value.Value.ExactString())
}
func (normalizer *guardedNormalizer) namedConstant(expr ast.Expr, object *types.Const) string {
	identity := object.Name()
	if object.Pkg() != nil {
		identity = normalizer.objectIdentity(object)
	}
	return guardedForm("named-constant", normalizer.typeName(expr), identity, object.Val().ExactString())
}
func (normalizer *guardedNormalizer) selector(expr *ast.SelectorExpr) string {
	reference := normalizer.resolved(expr, normalizer.owner.file.typeInfo().ObjectOf(expr.Sel))
	if reference != "" {
		return reference
	}
	return normalizer.sibling(expr)
}
func (normalizer *guardedNormalizer) sibling(expr *ast.SelectorExpr) string {
	path := (guardedFacts{normalizer.owner.file.typeInfo()}).path(expr)
	if path.root != normalizer.subject.root || len(path.fields) != 1 {
		return ""
	}
	return guardedForm("subject-field", normalizer.typeName(expr), normalizer.objectIdentity(path.fields[0]))
}
func (normalizer *guardedNormalizer) objectIdentity(object types.Object) string {
	return object.Pkg().Path() + "." + object.Name() + "@" + guardedLocation(normalizer.fset.PositionFor(object.Pos(), false))
}
func (normalizer *guardedNormalizer) one(target ast.Expr) string {
	return guardedForm("constant", normalizer.typeName(target), "1")
}

func (normalizer *guardedNormalizer) conversion(node *ast.CallExpr) string {
	if len(node.Args) != 1 || normalizer.constant(node) == "" || !normalizer.owner.file.typeInfo().Types[node.Fun].IsType() {
		return ""
	}
	if _, literal := unparen(node.Args[0]).(*ast.BasicLit); literal {
		return normalizer.constant(node)
	}
	operand := normalizer.expression(node.Args[0])
	if operand == "" {
		return ""
	}
	return guardedForm("constant conversion", normalizer.typeName(node), operand)
}
