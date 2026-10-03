package columbo

import (
	"go/ast"
	"go/types"
)

// callGraph owns the reference universe and helper eligibility. Declaration calls
// and whole-file references are separate traversals because package initializers
// cannot become lexical helper clusters.
type callGraph struct {
	engine     *engine
	counts     map[*declaration]int
	firstClass map[*declaration]bool
	direct     map[*ast.Ident]bool
	unique     map[*declaration]*ast.CallExpr
}

func (a *engine) findCalls() {
	graph := &callGraph{a, map[*declaration]int{}, map[*declaration]bool{}, map[*ast.Ident]bool{}, map[*declaration]*ast.CallExpr{}}
	graph.collect()
	graph.classify()
}
func (g *callGraph) collect() {
	for _, d := range g.engine.declarations {
		g.declarationCalls(d)
	}
	for _, f := range g.engine.files {
		g.fileReferences(f)
	}
}
func (g *callGraph) declarationCalls(d *declaration) {
	if !d.hasBody() {
		return
	}
	scan := &declarationCalls{graph: g, owner: d, info: d.file.typeInfo()}
	d.inspectBody(scan.visit)
}

type declarationCalls struct {
	graph *callGraph
	owner *declaration
	info  *types.Info
}

func (s *declarationCalls) visit(n ast.Node) bool {
	if call, ok := n.(*ast.CallExpr); ok {
		s.call(call)
	}
	return true
}
func (s *declarationCalls) call(call *ast.CallExpr) {
	target := s.graph.resolve(s.info, call)
	if target == nil {
		return
	}
	s.graph.record(call, target, s.owner)
	if id := calleeIdentifier(call.Fun); id != nil {
		s.graph.direct[id] = true
	}
}
func (g *callGraph) record(call *ast.CallExpr, target, owner *declaration) {
	g.engine.calls[call] = target
	g.engine.callOwner[call] = owner
	g.counts[target]++
	g.unique[target] = call
}
func (g *callGraph) fileReferences(f *file) {
	scan := &fileReferences{graph: g, info: f.typeInfo()}
	ast.Inspect(f.ast, scan.visit)
}

type fileReferences struct {
	graph *callGraph
	info  *types.Info
}

func (s *fileReferences) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.CallExpr:
		s.packageCall(n)
	case *ast.Ident:
		s.identifier(n)
	}
	return true
}
func (s *fileReferences) packageCall(call *ast.CallExpr) {
	if s.graph.engine.calls[call] != nil {
		return
	}
	if target := s.graph.resolve(s.info, call); target != nil {
		s.graph.counts[target]++
		s.graph.firstClass[target] = true
	}
}
func (s *fileReferences) identifier(id *ast.Ident) {
	target := s.graph.referenced(s.info, id)
	if target != nil && !s.graph.direct[id] {
		s.graph.firstClass[target] = true
	}
}
func (g *callGraph) resolve(info *types.Info, call *ast.CallExpr) *declaration {
	return g.engine.target(callObject(info, call))
}
func (g *callGraph) referenced(info *types.Info, id *ast.Ident) *declaration {
	return g.engine.target((&callResolver{info}).identifier(id))
}
func (g *callGraph) classify() {
	for _, d := range g.engine.declarations {
		d.candidate = d.helperEligible(g.counts[d], g.firstClass[d]) && !g.interfaceMethod(d)
	}
}
func (d *declaration) helperEligible(count int, firstClass bool) bool {
	return d.file.included && d.fn.Body != nil && d.obj != nil && !d.obj.Exported() && d.fn.Name.Name != "_" && count == 1 && !firstClass
}
func (g *callGraph) interfaceMethod(d *declaration) bool {
	if !d.hasReceiver() || g.unique[d] == nil {
		return false
	}
	call := g.unique[d]
	owner := g.engine.callOwner[call]
	receiver := owner.methodReceiver(call)
	return receiver != nil && implementsMethod(receiver, d.methodName(), g.engine.interfaces)
}
func (d *declaration) methodReceiver(call *ast.CallExpr) types.Type {
	return selectedReceiver(d.file.typeInfo(), call)
}
func selectedReceiver(info *types.Info, call *ast.CallExpr) types.Type {
	sel := (&callResolver{info}).selection(calleeBase(call.Fun))
	if sel == nil {
		return nil
	}
	return stripPointer(sel.Recv())
}
func (r *callResolver) selection(e ast.Expr) *types.Selection {
	expr, ok := e.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	return r.info.Selections[expr]
}
func implementsMethod(receiver types.Type, name string, interfaces []types.Type) bool {
	for _, t := range interfaces {
		iface, ok := t.Underlying().(*types.Interface)
		if ok && interfaceHasMethod(iface, name) && (types.Implements(receiver, iface) || types.Implements(types.NewPointer(receiver), iface)) {
			return true
		}
	}
	return false
}
func interfaceHasMethod(iface *types.Interface, name string) bool {
	for i := 0; i < iface.NumMethods(); i++ {
		if iface.Method(i).Name() == name {
			return true
		}
	}
	return false
}

// Callee resolution is shared with helper clustering and forwarding receipts.
func calleeBase(e ast.Expr) ast.Expr {
	e = unparen(e)
	switch x := e.(type) {
	case *ast.IndexExpr:
		return unparen(x.X)
	case *ast.IndexListExpr:
		return unparen(x.X)
	}
	return e
}
func calleeIdentifier(e ast.Expr) *ast.Ident {
	switch x := calleeBase(e).(type) {
	case *ast.Ident:
		return x
	case *ast.SelectorExpr:
		return x.Sel
	}
	return nil
}

type callResolver struct{ info *types.Info }

func callObject(info *types.Info, call *ast.CallExpr) *types.Func {
	return (&callResolver{info}).resolve(calleeBase(call.Fun))
}
func (r *callResolver) resolve(e ast.Expr) *types.Func {
	switch x := e.(type) {
	case *ast.Ident:
		return r.identifier(x)
	case *ast.SelectorExpr:
		return r.selector(x)
	}
	return nil
}
func (r *callResolver) identifier(id *ast.Ident) *types.Func {
	fn, _ := r.info.Uses[id].(*types.Func)
	return fn
}
func (r *callResolver) selector(expr *ast.SelectorExpr) *types.Func {
	if interfaceSelection(r.info.Selections[expr]) {
		return nil
	}
	return r.identifier(expr.Sel)
}
func interfaceSelection(sel *types.Selection) bool {
	if sel == nil {
		return false
	}
	_, yes := types.Unalias(stripPointer(sel.Recv())).Underlying().(*types.Interface)
	return yes
}

func primitiveCall(info *types.Info, call *ast.CallExpr) bool {
	r := &callResolver{info}
	return r.isType(call.Fun) || r.builtin(call.Fun)
}
func (r *callResolver) isType(e ast.Expr) bool { return r.info.Types[e].IsType() }
func (r *callResolver) builtin(e ast.Expr) bool {
	id, ok := unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = r.info.Uses[id].(*types.Builtin)
	return ok
}
func inputVariable(info *types.Info, e ast.Expr) *types.Var {
	id, ok := unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	v, _ := info.Uses[id].(*types.Var)
	return v
}

func forwardedReceiver(info *types.Info, call *ast.CallExpr) ast.Expr {
	return (&callResolver{info}).methodValueReceiver(unparen(call.Fun))
}
func (r *callResolver) methodValueReceiver(e ast.Expr) ast.Expr {
	expr, ok := e.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	sel := r.info.Selections[expr]
	if sel == nil || sel.Kind() != types.MethodVal {
		return nil
	}
	return expr.X
}
