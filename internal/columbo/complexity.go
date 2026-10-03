// Adapted from github.com/uudashr/gocognit v1.2.0 (BSD-3-Clause).
// The only traversal extension is the CallExpr interception hook.
package columbo

import (
	"go/ast"
	"go/token"
	"reflect"
)

type diagnostic struct {
	Inc     int
	Nesting int
	Text    string
	Pos     token.Pos
}

type complexityVisitor struct {
	name            *ast.Ident
	complexity      int
	nesting         int
	elseNodes       map[ast.Node]bool
	calculatedExprs map[ast.Expr]bool

	diagnosticsEnabled bool
	diagnostics        []diagnostic
	hook               func(*complexityVisitor, *ast.CallExpr) bool
}

func (v *complexityVisitor) incNesting() {
	v.nesting++
}

func (v *complexityVisitor) decNesting() {
	v.nesting--
}

func (v *complexityVisitor) incComplexity(text string, pos token.Pos) {
	v.complexity++

	if !v.diagnosticsEnabled {
		return
	}

	v.diagnostics = append(v.diagnostics, diagnostic{
		Inc:  1,
		Text: text,
		Pos:  pos,
	})
}

func (v *complexityVisitor) nestIncComplexity(text string, pos token.Pos) {
	v.complexity += (v.nesting + 1)

	if !v.diagnosticsEnabled {
		return
	}

	v.diagnostics = append(v.diagnostics, diagnostic{
		Inc:     v.nesting + 1,
		Nesting: v.nesting,
		Text:    text,
		Pos:     pos,
	})
}

func (v *complexityVisitor) markAsElseNode(n ast.Node) {
	if v.elseNodes == nil {
		v.elseNodes = make(map[ast.Node]bool)
	}

	v.elseNodes[n] = true
}

func (v *complexityVisitor) markedAsElseNode(n ast.Node) bool {
	if v.elseNodes == nil {
		return false
	}

	return v.elseNodes[n]
}

func (v *complexityVisitor) markCalculated(e ast.Expr) {
	if v.calculatedExprs == nil {
		v.calculatedExprs = make(map[ast.Expr]bool)
	}

	v.calculatedExprs[e] = true
}

func (v *complexityVisitor) isCalculated(e ast.Expr) bool {
	if v.calculatedExprs == nil {
		return false
	}

	return v.calculatedExprs[e]
}

// Each specialized handler owns its node's traversal; other nodes use ast.Walk.
type complexityHandler func(*complexityVisitor, ast.Node) ast.Visitor

func handlerFor[T ast.Node](visit func(*complexityVisitor, T) ast.Visitor) complexityHandler {
	return func(v *complexityVisitor, n ast.Node) ast.Visitor { return visit(v, n.(T)) }
}

var complexityHandlers = map[reflect.Type]complexityHandler{
	reflect.TypeOf((*ast.IfStmt)(nil)):         handlerFor((*complexityVisitor).visitIfStmt),
	reflect.TypeOf((*ast.SwitchStmt)(nil)):     handlerFor((*complexityVisitor).visitSwitchStmt),
	reflect.TypeOf((*ast.TypeSwitchStmt)(nil)): handlerFor((*complexityVisitor).visitTypeSwitchStmt),
	reflect.TypeOf((*ast.SelectStmt)(nil)):     handlerFor((*complexityVisitor).visitSelectStmt),
	reflect.TypeOf((*ast.ForStmt)(nil)):        handlerFor((*complexityVisitor).visitForStmt),
	reflect.TypeOf((*ast.RangeStmt)(nil)):      handlerFor((*complexityVisitor).visitRangeStmt),
	reflect.TypeOf((*ast.FuncLit)(nil)):        handlerFor((*complexityVisitor).visitFuncLit),
	reflect.TypeOf((*ast.BranchStmt)(nil)):     handlerFor((*complexityVisitor).visitBranchStmt),
	reflect.TypeOf((*ast.BinaryExpr)(nil)):     handlerFor((*complexityVisitor).visitBinaryExpr),
	reflect.TypeOf((*ast.CallExpr)(nil)):       handlerFor((*complexityVisitor).visitCallExpr),
}

// Visit implements the ast.Visitor interface.
func (v *complexityVisitor) Visit(n ast.Node) ast.Visitor {
	if visit := complexityHandlers[reflect.TypeOf(n)]; visit != nil {
		return visit(v, n)
	}
	return v
}

func (v *complexityVisitor) walkOptional(n ast.Node) {
	if n != nil {
		v.walk(n)
	}
}
func (v *complexityVisitor) walkNested(n ast.Node) {
	v.incNesting()
	v.walk(n)
	v.decNesting()
}

func (v *complexityVisitor) visitIfStmt(n *ast.IfStmt) ast.Visitor {
	v.incIfComplexity(n, "if", n.Pos())
	v.ifCondition(n)
	v.ifBranches(n)
	return nil
}
func (v *complexityVisitor) ifBranches(n *ast.IfStmt) {
	v.walkNested(n.Body)
	v.walkElse(n.Else)
}
func (v *complexityVisitor) ifCondition(n *ast.IfStmt) {
	v.walkOptional(n.Init)
	v.walk(n.Cond)
}
func (v *complexityVisitor) walkElse(n ast.Stmt) {
	switch n.(type) {
	case *ast.BlockStmt:
		v.incComplexity("else", n.Pos())
		v.walk(n)
	case *ast.IfStmt:
		v.markAsElseNode(n)
		v.walk(n)
	}
}
func (v *complexityVisitor) visitSwitchStmt(n *ast.SwitchStmt) ast.Visitor {
	v.nestIncComplexity("switch", n.Pos())
	v.switchHeader(n)
	v.walkNested(n.Body)
	return nil
}
func (v *complexityVisitor) switchHeader(n *ast.SwitchStmt) {
	v.walkOptional(n.Init)
	v.walkOptional(n.Tag)
}
func (v *complexityVisitor) visitTypeSwitchStmt(n *ast.TypeSwitchStmt) ast.Visitor {
	v.nestIncComplexity("switch", n.Pos())
	v.typeSwitchHeader(n)
	v.walkNested(n.Body)
	return nil
}
func (v *complexityVisitor) typeSwitchHeader(n *ast.TypeSwitchStmt) {
	v.walkOptional(n.Init)
	v.walkOptional(n.Assign)
}
func (v *complexityVisitor) visitSelectStmt(n *ast.SelectStmt) ast.Visitor {
	v.nestIncComplexity("select", n.Pos())
	v.walkNested(n.Body)
	return nil
}
func (v *complexityVisitor) visitForStmt(n *ast.ForStmt) ast.Visitor {
	v.nestIncComplexity("for", n.Pos())
	v.forHeader(n)
	v.walkNested(n.Body)
	return nil
}
func (v *complexityVisitor) forHeader(n *ast.ForStmt) {
	v.walkOptional(n.Init)
	v.walkOptional(n.Cond)
	v.walkOptional(n.Post)
}
func (v *complexityVisitor) visitRangeStmt(n *ast.RangeStmt) ast.Visitor {
	v.nestIncComplexity("for", n.Pos())
	v.rangeHeader(n)
	v.walkNested(n.Body)
	return nil
}
func (v *complexityVisitor) rangeHeader(n *ast.RangeStmt) {
	v.walkOptional(n.Key)
	v.walkOptional(n.Value)
	v.walk(n.X)
}
func (v *complexityVisitor) visitFuncLit(n *ast.FuncLit) ast.Visitor {
	v.walk(n.Type)
	v.walkNested(n.Body)
	return nil
}

func (v *complexityVisitor) visitBranchStmt(n *ast.BranchStmt) ast.Visitor {
	if n.Label != nil {
		v.incComplexity(n.Tok.String(), n.Pos())
	}

	return v
}

func (v *complexityVisitor) visitBinaryExpr(n *ast.BinaryExpr) ast.Visitor {
	if isBinaryLogicalOp(n.Op) && !v.isCalculated(n) {
		ops := v.collectBinaryOps(n)

		var lastOp token.Token
		for _, op := range ops {
			if lastOp != op {
				v.incComplexity(op.String(), n.OpPos)
				lastOp = op
			}
		}
	}

	return v
}

func (v *complexityVisitor) visitCallExpr(n *ast.CallExpr) ast.Visitor {
	if v.hook != nil && v.hook(v, n) {
		return nil
	}
	if name, recursive := v.recursiveName(n.Fun); recursive {
		v.incComplexity(name, n.Pos())
	}
	return v
}
func (v *complexityVisitor) recursiveName(expr ast.Expr) (string, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", false
	}
	return ident.Name, ident.Obj == v.name.Obj && ident.Name == v.name.Name
}

func (v *complexityVisitor) collectBinaryOps(exp ast.Expr) []token.Token {
	v.markCalculated(exp)

	if exp, ok := exp.(*ast.BinaryExpr); ok {
		return mergeBinaryOps(v.collectBinaryOps(exp.X), exp.Op, v.collectBinaryOps(exp.Y))
	}
	return nil
}

func (v *complexityVisitor) incIfComplexity(n *ast.IfStmt, text string, pos token.Pos) {
	if v.markedAsElseNode(n) {
		v.incComplexity(text, pos)
	} else {
		v.nestIncComplexity(text, pos)
	}
}

func mergeBinaryOps(x []token.Token, op token.Token, y []token.Token) []token.Token {
	var out []token.Token
	out = append(out, x...)

	if isBinaryLogicalOp(op) {
		out = append(out, op)
	}

	out = append(out, y...)
	return out
}

func isBinaryLogicalOp(op token.Token) bool {
	return op == token.LAND || op == token.LOR
}

func (d diagnostic) position() token.Pos { return d.Pos }
func (d diagnostic) detail(kind string) Detail {
	return Detail{Subject: kind, Value: d.Inc, Nesting: d.Nesting}
}

func scanComplexity(fn *ast.FuncDecl) *complexityVisitor {
	v := &complexityVisitor{name: fn.Name, diagnosticsEnabled: true}
	ast.Walk(v, fn)
	return v
}

func expansionVisitor(name *ast.Ident, depth int, hook func(*complexityVisitor, *ast.CallExpr) bool) *complexityVisitor {
	return &complexityVisitor{name: name, nesting: depth, diagnosticsEnabled: true, hook: hook}
}
func (v *complexityVisitor) walk(n ast.Node) { ast.Walk(v, n) }
func (v *complexityVisitor) walkOperands(call *ast.CallExpr) {
	v.walk(call.Fun)
	for _, arg := range call.Args {
		v.walk(arg)
	}
}
