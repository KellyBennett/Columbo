package columbo

import (
	"go/ast"
	"go/token"
)

// expansionContext owns the recursive call path shared by both virtual metrics.
// Each child gets its own trace; the stack disqualifies recursive expansion.
type expansionContext struct {
	engine *engine
	owner  *declaration
	stack  []*declaration
	trace  expansion
}

func (c *expansionContext) target(call *ast.CallExpr) *declaration {
	target := c.engine.calls[call]
	if target == nil || !target.candidate || onStack(c.stack, target) {
		return nil
	}
	return target
}
func (c *expansionContext) child(target *declaration, call *ast.CallExpr) expansionContext {
	return expansionContext{c.engine, target, append(c.stack, target), extendTrace(c.trace, target, c.owner, call)}
}
func (c *expansionContext) receiptTrace() *expansion {
	if len(c.trace.sites) == 0 {
		return nil
	}
	return &c.trace
}

type lineExpansion struct {
	context expansionContext
	removed map[token.Pos]bool
	copies  []Source
}

func (a *engine) expandedLines(d *declaration, stack []*declaration, trace expansion) []Source {
	e := &lineExpansion{context: expansionContext{a, d, stack, trace}, removed: map[token.Pos]bool{}}
	return e.run()
}
func (e *lineExpansion) run() []Source {
	e.visit(e.context.owner.bodyNode())
	original := e.context.owner.linesEvidence("expanded-lines", e.removed, e.context.receiptTrace())
	return append(original, e.copies...)
}
func (e *lineExpansion) visit(n ast.Node) {
	if n == nil {
		return
	}
	if call, ok := n.(*ast.CallExpr); ok {
		e.operands(call)
		e.expandCall(call)
		return
	}
	directChildren(n, e.visit)
}
func directChildren(parent ast.Node, visit func(ast.Node)) {
	ast.Inspect(parent, func(n ast.Node) bool {
		if n == parent {
			return true
		}
		if n != nil {
			visit(n)
		}
		return false
	})
}
func (e *lineExpansion) operands(call *ast.CallExpr) {
	e.visit(call.Fun)
	for _, arg := range call.Args {
		e.visit(arg)
	}
}
func (e *lineExpansion) expandCall(call *ast.CallExpr) {
	target := e.context.target(call)
	if target == nil {
		return
	}
	e.remove(call)
	child := &lineExpansion{context: e.context.child(target, call), removed: map[token.Pos]bool{}}
	e.copies = append(e.copies, child.run()...)
}
func (e *lineExpansion) remove(call *ast.CallExpr) {
	mask := replacementMask(call)
	for _, pos := range e.context.owner.file.tokenPositions(call.Pos(), call.End()) {
		if !mask.retains(pos) {
			e.removed[pos] = true
		}
	}
}

// A replaced call loses its own syntax while retaining nested-call token ranges.
// Nested calls are expanded first, so their masks and copied evidence survive.
type tokenRange struct{ start, end token.Pos }
type callMask struct {
	root     *ast.CallExpr
	retained []tokenRange
}

func replacementMask(call *ast.CallExpr) *callMask {
	mask := &callMask{root: call}
	ast.Inspect(call, mask.visit)
	return mask
}
func (m *callMask) visit(n ast.Node) bool {
	if n == m.root {
		return true
	}
	if call, ok := n.(*ast.CallExpr); ok {
		m.retained = append(m.retained, tokenRange{call.Pos(), call.End()})
		return false
	}
	return true
}
func (m *callMask) retains(pos token.Pos) bool {
	for _, span := range m.retained {
		if pos >= span.start && pos < span.end {
			return true
		}
	}
	return false
}

type complexityExpansion struct {
	context  expansionContext
	visitor  *complexityVisitor
	receipts []Source
}

func (a *engine) expandedComplexity(d *declaration, stack []*declaration, trace expansion, depth int) (int, []Source) {
	e := &complexityExpansion{context: expansionContext{a, d, stack, trace}, receipts: []Source{}}
	return e.run(depth)
}
func (e *complexityExpansion) run(depth int) (int, []Source) {
	e.visitor = e.newVisitor(depth)
	e.visitor.walk(e.context.owner.bodyNode())
	e.diagnostics()
	return e.visitor.complexity, e.receipts
}
func (e *complexityExpansion) newVisitor(depth int) *complexityVisitor {
	return expansionVisitor(e.context.owner.functionName(), depth, e.hook)
}
func (e *complexityExpansion) hook(v *complexityVisitor, call *ast.CallExpr) bool {
	target := e.context.target(call)
	if target == nil {
		return false
	}
	v.walkOperands(call)
	child := &complexityExpansion{context: e.context.child(target, call), receipts: []Source{}}
	n, receipts := child.run(v.nesting)
	v.complexity += n
	e.receipts = append(e.receipts, receipts...)
	return true
}
func (e *complexityExpansion) diagnostics() {
	for _, event := range e.visitor.diagnostics {
		receipt := e.context.owner.event(event, "expanded-complexity").withExpansion(e.context.receiptTrace())
		e.receipts = append(e.receipts, receipt)
	}
}
