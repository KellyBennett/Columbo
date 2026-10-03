// Adapted from github.com/uudashr/gocognit v1.2.0 (BSD-3-Clause).
// The only traversal extension is the CallExpr interception hook.
package columbo

import (
	"go/ast"
	"go/token"
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

// Visit implements the ast.Visitor interface.
func (v *complexityVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.IfStmt:
		return v.visitIfStmt(n)
	case *ast.SwitchStmt:
		return v.visitSwitchStmt(n)
	case *ast.TypeSwitchStmt:
		return v.visitTypeSwitchStmt(n)
	case *ast.SelectStmt:
		return v.visitSelectStmt(n)
	case *ast.ForStmt:
		return v.visitForStmt(n)
	case *ast.RangeStmt:
		return v.visitRangeStmt(n)
	case *ast.FuncLit:
		return v.visitFuncLit(n)
	case *ast.BranchStmt:
		return v.visitBranchStmt(n)
	case *ast.BinaryExpr:
		return v.visitBinaryExpr(n)
	case *ast.CallExpr:
		return v.visitCallExpr(n)
	}

	return v
}

func (v *complexityVisitor) visitIfStmt(n *ast.IfStmt) ast.Visitor {
	v.incIfComplexity(n, "if", n.Pos())

	if n := n.Init; n != nil {
		ast.Walk(v, n)
	}

	ast.Walk(v, n.Cond)

	v.incNesting()
	ast.Walk(v, n.Body)
	v.decNesting()

	if _, ok := n.Else.(*ast.BlockStmt); ok {
		v.incComplexity("else", n.Else.Pos())

		ast.Walk(v, n.Else)
	} else if _, ok := n.Else.(*ast.IfStmt); ok {
		v.markAsElseNode(n.Else)
		ast.Walk(v, n.Else)
	}

	return nil
}

func (v *complexityVisitor) visitSwitchStmt(n *ast.SwitchStmt) ast.Visitor {
	v.nestIncComplexity("switch", n.Pos())

	if n := n.Init; n != nil {
		ast.Walk(v, n)
	}

	if n := n.Tag; n != nil {
		ast.Walk(v, n)
	}

	v.incNesting()
	ast.Walk(v, n.Body)
	v.decNesting()

	return nil
}

func (v *complexityVisitor) visitTypeSwitchStmt(n *ast.TypeSwitchStmt) ast.Visitor {
	v.nestIncComplexity("switch", n.Pos())

	if n := n.Init; n != nil {
		ast.Walk(v, n)
	}

	if n := n.Assign; n != nil {
		ast.Walk(v, n)
	}

	v.incNesting()
	ast.Walk(v, n.Body)
	v.decNesting()

	return nil
}

func (v *complexityVisitor) visitSelectStmt(n *ast.SelectStmt) ast.Visitor {
	v.nestIncComplexity("select", n.Pos())

	v.incNesting()
	ast.Walk(v, n.Body)
	v.decNesting()

	return nil
}

func (v *complexityVisitor) visitForStmt(n *ast.ForStmt) ast.Visitor {
	v.nestIncComplexity("for", n.Pos())

	if n := n.Init; n != nil {
		ast.Walk(v, n)
	}

	if n := n.Cond; n != nil {
		ast.Walk(v, n)
	}

	if n := n.Post; n != nil {
		ast.Walk(v, n)
	}

	v.incNesting()
	ast.Walk(v, n.Body)
	v.decNesting()

	return nil
}

func (v *complexityVisitor) visitRangeStmt(n *ast.RangeStmt) ast.Visitor {
	v.nestIncComplexity("for", n.Pos())

	if n := n.Key; n != nil {
		ast.Walk(v, n)
	}

	if n := n.Value; n != nil {
		ast.Walk(v, n)
	}

	ast.Walk(v, n.X)

	v.incNesting()
	ast.Walk(v, n.Body)
	v.decNesting()

	return nil
}

func (v *complexityVisitor) visitFuncLit(n *ast.FuncLit) ast.Visitor {
	ast.Walk(v, n.Type)

	v.incNesting()
	ast.Walk(v, n.Body)
	v.decNesting()

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
	if callIdent, ok := n.Fun.(*ast.Ident); ok {
		obj, name := callIdent.Obj, callIdent.Name
		if obj == v.name.Obj && name == v.name.Name {
			// called by same function directly (direct recursion)
			v.incComplexity(name, n.Pos())
		}
	}

	return v
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
