package columbo

import (
	"go/ast"
	"go/token"
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

type variantCondition struct {
	site         *variantSite
	discriminant resolvedValuePath
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
	path := resolveValuePath(c.site.info, discriminant, ast.Unparen)
	if !path.valid() {
		return false
	}
	if !c.sameDomain(discriminant, path) {
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
func (c *variantCondition) sameDomain(expr ast.Expr, path resolvedValuePath) bool {
	domain := valueVariantDomain(c.site.info.TypeOf(expr))
	if domain == "" {
		return false
	}
	if !c.discriminant.valid() {
		c.discriminant, c.site.domain = path, domain
	}
	return domain == c.site.domain && path.same(c.discriminant)
}
