package columbo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

func coordinationKey(p resolvedValuePath) string {
	if !p.valid() || p.root.Name() == "_" {
		return ""
	}
	return p.key(fmt.Sprintf("%s@%d", p.root.Name(), p.root.Pos()))
}
func coordinationAccess(owner *declaration, expr ast.Expr, p resolvedValuePath, kind string) Source {
	return owner.source(kind, expr.Pos(), expr.End(), Detail{Subject: p.key(p.root.Name())})
}

type coordinationEffects struct {
	owner         *declaration
	info          *types.Info
	writes, reads map[string]Source
}

func (e *coordinationEffects) access(expr ast.Expr, kind string, into map[string]Source) bool {
	p := resolveValuePath(e.info, expr, ast.Unparen)
	key := coordinationKey(p)
	if key == "" {
		return false
	}
	into[key] = coordinationAccess(e.owner, expr, p, kind)
	return true
}
func (e *coordinationEffects) read(node ast.Node) {
	ast.Inspect(node, e.readVisit)
}
func (e *coordinationEffects) readVisit(node ast.Node) bool {
	if coordinationClosure(node) {
		return false
	}
	expr, ok := node.(ast.Expr)
	return !ok || !e.access(expr, "variant-effect-read", e.reads)
}
func (e *coordinationEffects) readAll(expressions []ast.Expr) {
	for _, expr := range expressions {
		e.read(expr)
	}
}
func (e *coordinationEffects) write(expr ast.Expr) {
	e.access(expr, "variant-effect-write", e.writes)
}
func (e *coordinationEffects) assignment(n *ast.AssignStmt) bool {
	for _, lhs := range n.Lhs {
		e.write(lhs)
		if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
			e.read(lhs)
		}
	}
	e.readAll(n.Rhs)
	return false
}
func (e *coordinationEffects) declaration(n *ast.ValueSpec) bool {
	for _, name := range n.Names {
		e.write(name)
	}
	e.readAll(n.Values)
	return false
}
func (e *coordinationEffects) visit(node ast.Node) bool {
	if coordinationClosure(node) {
		return false
	}
	return e.statement(node)
}
func (e *coordinationEffects) statement(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.AssignStmt:
		return e.assignment(n)
	case *ast.IncDecStmt:
		e.write(n.X)
		e.read(n.X)
		return false
	case *ast.ValueSpec:
		return e.declaration(n)
	}
	return e.expressionStatement(node)
}
func (e *coordinationEffects) expressionStatement(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.ExprStmt:
		e.read(n.X)
		return false
	case *ast.ReturnStmt:
		e.readAll(n.Results)
		return false
	}
	e.conditionRead(node)
	return true
}
func (e *coordinationEffects) conditionRead(node ast.Node) {
	switch n := node.(type) {
	case *ast.IfStmt:
		e.read(n.Cond)
	case *ast.SwitchStmt:
		e.readTag(n)
	}
}

func (e *coordinationEffects) readTag(n *ast.SwitchStmt) {
	if n.Tag != nil {
		e.read(n.Tag)
	}
}

type coordinationUncertainty struct {
	info                         *types.Info
	left, right                  coordinationDecision
	location                     string
	calls, mutations, overwrites int
}

func (u *coordinationUncertainty) visit(node ast.Node) bool {
	if node == nil || coordinationClosure(node) {
		return false
	}
	if node.End() < u.left.node.Pos() || node.Pos() > u.right.node.End() {
		return false
	}
	if _, call := node.(*ast.CallExpr); call {
		u.calls++
	}
	u.writes(node)
	return true
}
func (u *coordinationUncertainty) writes(node ast.Node) {
	switch n := node.(type) {
	case *ast.AssignStmt:
		for _, target := range n.Lhs {
			u.target(target, node)
		}
	case *ast.IncDecStmt:
		u.target(n.X, node)
	}
}
func (u *coordinationUncertainty) target(expr ast.Expr, node ast.Node) {
	p := resolveValuePath(u.info, expr, ast.Unparen)
	if !p.valid() {
		return
	}
	if coordinationMayReplace(p, u.left.selector) {
		u.mutations++
	}
	if node.Pos() >= u.left.node.End() && node.End() <= u.right.node.Pos() && coordinationKey(p) == u.location {
		u.overwrites++
	}
}
func coordinationMayReplace(write, selector resolvedValuePath) bool {
	if write.root != selector.root || len(write.fields) > len(selector.fields) {
		return false
	}
	for i, field := range write.fields {
		if field != selector.fields[i] {
			return false
		}
	}
	return true
}
func (u coordinationUncertainty) text() string {
	return fmt.Sprintf("uncertainty: %d calls/conversions across span; %d possible selector writes; %d intervening location writes", u.calls, u.mutations, u.overwrites)
}
