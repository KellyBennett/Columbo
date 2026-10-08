package columbo

import (
	"go/ast"
	"go/types"
)

type guardedPrefix struct {
	facts guardedFacts
	reads map[types.Object]bool
}

func (owner *declaration) guardedPrefix(node *ast.IfStmt, mutation *guardedMutation, index int) bool {
	prefix := newGuardedPrefix(owner.file.typeInfo())
	return prefix.preservesUntil(node, mutation, index)
}
func newGuardedPrefix(info *types.Info) guardedPrefix {
	return guardedPrefix{guardedFacts{info}, map[types.Object]bool{}}
}
func (prefix guardedPrefix) preservesUntil(node *ast.IfStmt, mutation *guardedMutation, index int) bool {
	prefix.observe(node.Cond)
	prefix.observe(mutation.node)
	return prefix.preservesAll(node.Body.List[:index])
}
func (prefix guardedPrefix) preservesAll(statements []ast.Stmt) bool {
	for _, statement := range statements {
		if !prefix.preserves(statement) {
			return false
		}
	}
	return true
}
func (prefix guardedPrefix) observe(node ast.Node) {
	ast.Inspect(node, func(node ast.Node) bool {
		if name, ok := node.(*ast.Ident); ok {
			prefix.reads[prefix.facts.info.ObjectOf(name)] = true
		}
		return true
	})
}
func (prefix guardedPrefix) preserves(statement ast.Stmt) bool {
	if mutation := guardedMutationOf(statement); mutation != nil {
		return prefix.mutation(*mutation)
	}
	return prefix.otherStatement(statement)
}
func (prefix guardedPrefix) mutation(mutation guardedMutation) bool {
	return prefix.independent(mutation.target) && (mutation.value == nil || prefix.facts.pure(mutation.value))
}
func (prefix guardedPrefix) otherStatement(statement ast.Stmt) bool {
	switch node := statement.(type) {
	case *ast.EmptyStmt:
		return true
	case *ast.AssignStmt:
		return prefix.assignment(node)
	case *ast.DeclStmt:
		return prefix.declaration(node)
	}
	return false
}
func (prefix guardedPrefix) independent(expr ast.Expr) bool {
	name, ok := unparen(expr).(*ast.Ident)
	if !ok {
		return false
	}
	object := prefix.facts.info.ObjectOf(name)
	return guardedLocalScalar(object) && !prefix.reads[object]
}
func (prefix guardedPrefix) assignment(node *ast.AssignStmt) bool {
	for _, target := range node.Lhs {
		if !prefix.independent(target) {
			return false
		}
	}
	return prefix.pureValues(node.Rhs)
}
func (prefix guardedPrefix) pureValues(values []ast.Expr) bool {
	for _, value := range values {
		if !prefix.facts.pure(value) {
			return false
		}
	}
	return true
}
func (prefix guardedPrefix) declaration(node *ast.DeclStmt) bool {
	group, ok := node.Decl.(*ast.GenDecl)
	if !ok {
		return false
	}
	for _, spec := range group.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok || !prefix.value(value) {
			return false
		}
	}
	return true
}
func (prefix guardedPrefix) value(value *ast.ValueSpec) bool {
	for _, name := range value.Names {
		if !prefix.independent(name) {
			return false
		}
	}
	return prefix.pureValues(value.Values)
}

func guardedLocalScalar(object types.Object) bool {
	variable, ok := object.(*types.Var)
	return ok && guardedScalar(variable.Type()) && variable.Pkg() != nil && variable.Parent() != variable.Pkg().Scope()
}
