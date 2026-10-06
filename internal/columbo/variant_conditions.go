package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

func unparen(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

// Object paths distinguish shadowed identifiers and resolved fields. They are
// local to a chain; cross-site grouping intentionally ignores these names.
type variantExpressionPath struct{ info *types.Info }

func variantDiscriminant(expr ast.Expr, info *types.Info) []types.Object {
	return (&variantExpressionPath{info: info}).objects(expr)
}
func (p *variantExpressionPath) objects(expr ast.Expr) []types.Object {
	switch n := unparen(expr).(type) {
	case *ast.Ident:
		return p.identifier(n)
	case *ast.SelectorExpr:
		return p.selector(n)
	}
	return nil
}
func (p *variantExpressionPath) identifier(n *ast.Ident) []types.Object {
	obj := p.info.ObjectOf(n)
	if _, ok := obj.(*types.Var); ok {
		return []types.Object{obj}
	}
	return nil
}
func (p *variantExpressionPath) selector(n *ast.SelectorExpr) []types.Object {
	root := p.objects(n.X)
	selection := p.info.Selections[n]
	if len(root) == 0 || !fieldVariantSelection(selection) {
		return nil
	}
	return append(root, selection.Obj())
}

type variantCondition struct {
	site         *variantSite
	discriminant []types.Object
}

func (s *variantSite) ifChain(first *ast.IfStmt) bool {
	condition := variantCondition{site: s}
	for branch := first; branch != nil; {
		if !condition.parse(branch.Cond) {
			return false
		}
		branch, _ = branch.Else.(*ast.IfStmt)
	}
	return true
}

func (c *variantCondition) parse(expr ast.Expr) bool {
	binary, ok := unparen(expr).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	if binary.Op == token.LOR {
		return c.parse(binary.X) && c.parse(binary.Y)
	}
	if binary.Op != token.EQL {
		return false
	}
	return c.equality(binary)
}
func (c *variantCondition) equality(binary *ast.BinaryExpr) bool {
	discriminant, constant := c.site.equalityOperands(binary)
	objects := variantDiscriminant(discriminant, c.site.info)
	if len(objects) == 0 {
		return false
	}
	if !c.sameDomain(discriminant, objects) {
		return false
	}
	return c.site.valueArm(constant)
}
func (s *variantSite) equalityOperands(binary *ast.BinaryExpr) (ast.Expr, ast.Expr) {
	left, right := binary.X, binary.Y
	if s.info.Types[right].Value == nil {
		return right, left
	}
	return left, right
}
func (c *variantCondition) sameDomain(expr ast.Expr, objects []types.Object) bool {
	domain := valueVariantDomain(c.site.info.TypeOf(expr))
	if domain == "" {
		return false
	}
	if c.discriminant == nil {
		c.discriminant, c.site.domain = objects, domain
	}
	return domain == c.site.domain && slices.Equal(objects, c.discriminant)
}
