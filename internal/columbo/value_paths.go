package columbo

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"
)

// resolvedValuePath owns the identity of a variable-rooted field chain. Resolved
// objects distinguish shadowed roots and fields with the same source spelling.
type resolvedValuePath struct {
	root   *types.Var
	fields []*types.Var
}

func (p resolvedValuePath) valid() bool { return p.root != nil }
func (p resolvedValuePath) same(other resolvedValuePath) bool {
	return p.valid() && other.valid() && p.root == other.root && slices.Equal(p.fields, other.fields)
}
func (p resolvedValuePath) withField(field *types.Var) resolvedValuePath {
	if !p.valid() || field == nil {
		return resolvedValuePath{}
	}
	p.fields = append(slices.Clone(p.fields), field)
	return p
}
func (p resolvedValuePath) key(rootIdentity string) string {
	names := []string{rootIdentity}
	for _, field := range p.fields {
		names = append(names, field.Name())
	}
	return strings.Join(names, ".")
}

// The caller owns normalization policy. Both callers share root/field resolution
// while only Feature Envy normalizes address-taking and dereferencing.
type valuePathResolver struct {
	info      *types.Info
	normalize func(ast.Expr) ast.Expr
}

func resolveValuePath(info *types.Info, expr ast.Expr, normalize func(ast.Expr) ast.Expr) resolvedValuePath {
	return (&valuePathResolver{info: info, normalize: normalize}).resolve(expr)
}
func (r *valuePathResolver) resolve(expr ast.Expr) resolvedValuePath {
	switch n := r.normalize(expr).(type) {
	case *ast.Ident:
		return r.identifier(n)
	case *ast.SelectorExpr:
		return r.selector(n)
	}
	return resolvedValuePath{}
}
func (r *valuePathResolver) identifier(id *ast.Ident) resolvedValuePath {
	variable, _ := r.info.ObjectOf(id).(*types.Var)
	return resolvedValuePath{root: variable}
}
func (r *valuePathResolver) selector(expr *ast.SelectorExpr) resolvedValuePath {
	field := selectedValueField(r.info.Selections[expr])
	if field == nil {
		return resolvedValuePath{}
	}
	return r.resolve(expr.X).withField(field)
}
func selectedValueField(selection *types.Selection) *types.Var {
	if selection == nil || selection.Kind() != types.FieldVal {
		return nil
	}
	field, _ := selection.Obj().(*types.Var)
	return field
}
