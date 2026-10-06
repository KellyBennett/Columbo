package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
)

// Each scan is confined to one named declaration. Unsupported indirect writes
// invalidate affected locals; no callee body or closure supplies origins.
type selectionScan struct {
	dynamicLocals map[types.Object]bool
	probing       bool
	siteIndex     map[selectionKey]*selectionSite
	steps         map[reflect.Type]selectionStep
	owner         *declaration
	info          *types.Info
	minimum       int
	sites         []*selectionSite
	unsafeLocals  map[types.Object]bool
}

func newSelectionScan(d *declaration, minimum int) *selectionScan {
	s := &selectionScan{steps: selectionSteps, dynamicLocals: map[types.Object]bool{}, siteIndex: map[selectionKey]*selectionSite{}, owner: d, info: d.file.typeInfo(), minimum: minimum, unsafeLocals: map[types.Object]bool{}}
	s.findIndirectWrites()
	s.findDynamicLocals()
	return s
}
func (s *selectionScan) local(expr ast.Expr) types.Object {
	id, ok := unparen(expr).(*ast.Ident)
	if !ok {
		return nil
	}
	obj, ok := s.info.ObjectOf(id).(*types.Var)
	if !ok || !localSelectionObject(obj) || s.unsafeLocals[obj] {
		return nil
	}
	return obj
}
func (s *selectionScan) findIndirectWrites() { ast.Inspect(s.owner.fn.Body, s.indirectNode) }
func (s *selectionScan) indirectNode(n ast.Node) bool {
	if unary, ok := n.(*ast.UnaryExpr); ok {
		s.addressTaken(unary)
	}
	if literal, ok := n.(*ast.FuncLit); ok {
		s.invalidateCaptures(literal)
		return false
	}
	return true
}
func (s *selectionScan) addressTaken(unary *ast.UnaryExpr) {
	if unary.Op != token.AND {
		return
	}
	s.markIndirect([]ast.Expr{unary.X})
}
func (s *selectionScan) markIndirect(expressions []ast.Expr) {
	for _, expr := range expressions {
		if obj := s.local(expr); obj != nil {
			s.unsafeLocals[obj] = true
		}
	}
}
func (s *selectionScan) invalidateCaptures(literal *ast.FuncLit) {
	writes := selectionWrites{nested: true, assignment: s.markIndirect}
	ast.Inspect(literal.Body, writes.visit)
}
func (s *selectionScan) value(expr ast.Expr, env selectionEnv) selectedValue {
	expr = unparen(expr)
	if obj := s.local(expr); obj != nil && isInterfaceValue(obj.Type()) {
		if value, ok := env[obj]; ok {
			return value
		}
		return unknownSelectedValue()
	}
	if value, ok := s.conversion(expr, env); ok {
		return value
	}
	return s.origin(expr)
}
func (s *selectionScan) conversion(expr ast.Expr, env selectionEnv) (selectedValue, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !s.info.Types[call.Fun].IsType() {
		return selectedValue{}, false
	}
	if !isInterfaceValue(s.info.TypeOf(call)) {
		return selectedValue{}, false
	}
	return s.value(call.Args[0], env), true
}
func (s *selectionScan) origin(expr ast.Expr) selectedValue {
	v := emptySelectedValue()
	if s.isUniverse(expr, "nil") {
		return v
	}
	name := selectedImplementation(s.info.TypeOf(expr))
	if name == "" || s.unsafeExpression(expr) {
		return unknownSelectedValue()
	}
	v.origins[expr] = name
	return v
}
func (s *selectionScan) unsafeExpression(expr ast.Expr) bool {
	unsafe := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && s.dynamicObject(id) {
			unsafe = true
		}
		return true
	})
	return unsafe
}
func (s *selectionScan) dynamicObject(id *ast.Ident) bool {
	obj := s.info.ObjectOf(id)
	return s.dynamicLocals[obj] || dynamicSelectionObject(obj)
}
func (s *selectionScan) isUniverse(expr ast.Expr, name string) bool {
	id, ok := unparen(expr).(*ast.Ident)
	return ok && s.info.ObjectOf(id) == types.Universe.Lookup(name)
}
func (s *selectionScan) assign(left, right []ast.Expr, node ast.Node, env selectionEnv) {
	values := make([]selectedValue, len(left))
	for i := range left {
		values[i] = unknownSelectedValue()
		if len(right) == len(left) {
			values[i] = s.value(right[i], env)
		}
	}
	for i, expr := range left {
		s.bind(expr, values[i], node, env)
	}
}
func (s *selectionScan) bind(expr ast.Expr, value selectedValue, node ast.Node, env selectionEnv) {
	obj := s.local(expr)
	if obj == nil || !isInterfaceValue(obj.Type()) {
		return
	}
	env[obj] = value.withFlow(node)
}
func (s *selectionScan) declaration(stmt *ast.DeclStmt, env selectionEnv) {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, spec := range decl.Specs {
		if value, ok := spec.(*ast.ValueSpec); ok {
			s.declare(value, env)
		}
	}
}
func (s *selectionScan) declare(spec *ast.ValueSpec, env selectionEnv) {
	names, values := spec.Names, spec.Values
	left := make([]ast.Expr, len(names))
	for i, name := range names {
		left[i] = name
	}
	s.expressions(values, env)
	s.assign(left, values, spec, env)
	if len(values) == 0 {
		for _, name := range names {
			if obj := s.local(name); obj != nil {
				env[obj] = emptySelectedValue()
			}
		}
	}
}
func (s *selectionScan) expressions(exprs []ast.Expr, env selectionEnv) {
	for _, expr := range exprs {
		s.calls(expr, env)
	}
}
func (s *selectionScan) calls(node ast.Node, env selectionEnv) {
	if node == nil || s.probing {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			s.message(call, env)
		}
		return true
	})
}
func (s *selectionScan) message(call *ast.CallExpr, env selectionEnv) {
	receiver, method := s.messageReceiver(call)
	if receiver == nil {
		return
	}
	value := s.value(receiver, env)
	if value.unknown {
		return
	}
	use := selectedUse{call: call, message: selectedMessage(method), value: value}
	for site, flow := range value.traces {
		use.flow = flow
		site.use(use, s.minimum)
	}
}
func (s *selectionScan) interfaceSelection(expr ast.Expr) *types.Selection {
	selector, ok := unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selection := s.info.Selections[selector]
	if selection == nil || selectedRole(selection.Recv()) == "" {
		return nil
	}
	return selection
}
func (s *selectionScan) messageReceiver(call *ast.CallExpr) (ast.Expr, *types.Func) {
	selection := s.interfaceSelection(call.Fun)
	if selection == nil {
		return nil, nil
	}
	return interfaceReceiver(call, selection.Kind()), selectionMethod(selection)
}
func interfaceReceiver(call *ast.CallExpr, kind types.SelectionKind) ast.Expr {
	if kind == types.MethodExpr {
		return call.Args[0]
	}
	return unparen(call.Fun).(*ast.SelectorExpr).X
}
func selectionMethod(selection *types.Selection) *types.Func { return selection.Obj().(*types.Func) }

// A use is an observation of one receiver value, with its physical invocation
// and alias route. The site retains only the origins supporting that observation.
type selectedUse struct {
	call    *ast.CallExpr
	message string
	value   selectedValue
	flow    []ast.Node
}

func (use selectedUse) origins(site *selectionSite) map[ast.Expr]string {
	origins := map[ast.Expr]string{}
	for origin, name := range use.value.origins {
		if _, ok := site.origins[origin]; ok {
			origins[origin] = name
		}
	}
	return origins
}
func (site *selectionSite) use(use selectedUse, minimum int) {
	origins := use.origins(site)
	if len((selectedValue{origins: origins}).implementations()) < minimum {
		return
	}
	for origin, name := range origins {
		site.usedOrigins[origin] = name
	}
	site.messages[use.call] = use.message
	use.value.origins = origins
	site.observations = append(site.observations, use)
	for _, node := range use.flow {
		site.flows[node] = true
	}
}
func (s *selectionScan) returns(stmt *ast.ReturnStmt, env selectionEnv) {
	s.expressions(stmt.Results, env)
	for _, expr := range stmt.Results {
		s.returnValue(s.value(expr, env))
	}
	if len(stmt.Results) == 0 {
		s.namedReturns(env)
	}
}
func (s *selectionScan) namedReturns(env selectionEnv) {
	results := s.owner.signature.Results()
	for i := 0; i < results.Len(); i++ {
		s.returnValue(env[results.At(i)])
	}
}
func (s *selectionScan) returnValue(value selectedValue) {
	if s.probing {
		return
	}
	for site := range value.traces {
		site.returned = true
	}
}

func localSelectionObject(obj types.Object) bool {
	return obj.Pkg() != nil && obj.Parent() != obj.Pkg().Scope()
}

func dynamicSelectionObject(obj types.Object) bool {
	return obj != nil && obj.Pkg() != nil && (obj.Pkg().Path() == "unsafe" || obj.Pkg().Path() == "reflect")
}
