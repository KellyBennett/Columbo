package columbo

import (
	"go/ast"
	"go/types"
	"maps"
)

func (a *engine) selectionRole(c *Case, site *selectionSite) { a.recordRole(site.roleGroup(), c) }
func (site *selectionSite) roleGroup() *roleGroup {
	g := newRoleGroup()
	g.receipts = append(g.receipts, site.receipt("role-selection", site.decision, site.role))
	for _, use := range site.observations {
		use.observeRole(g, site)
	}
	return g
}
func (site *selectionSite) roleSelection(call *ast.CallExpr) *types.Selection {
	selector, ok := unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	return site.owner.file.typeInfo().Selections[selector]
}
func (use selectedUse) observeRole(g *roleGroup, site *selectionSite) {
	selection := site.roleSelection(use.call)
	if selection == nil {
		return
	}
	g.roles[selectedRole(selection.Recv())] = selection.Recv()
	for origin := range use.value.origins {
		site.observeRoleOrigin(g, use, origin)
	}
}
func (site *selectionSite) observeRoleOrigin(g *roleGroup, use selectedUse, origin ast.Expr) {
	invocation := site.owner.roleInvocation(use.call)
	if invocation == nil {
		return
	}
	invocation.receiver = origin
	invocation.observe(g, site.owner, use.call)
	g.receipts = append(g.receipts, site.originRoleReceipt(origin))
}
func (site *selectionSite) originRoleReceipt(origin ast.Expr) Source {
	t := site.owner.file.typeInfo().TypeOf(origin)
	return site.receipt("role-origin", origin, roleImplementation(t))
}

type roleObserver struct {
	owner *declaration
	group *roleGroup
	root  ast.Node
}

func observedRoleArm(d *declaration, arm roleArm) *roleGroup {
	observer := roleObserver{owner: d, group: newRoleGroup(), root: arm.body}
	ast.Inspect(arm.body, observer.visit)
	return observer.group
}
func (o *roleObserver) visit(node ast.Node) bool {
	if roleNestedBoundary(node, o.root) {
		return false
	}
	if call, ok := node.(*ast.CallExpr); ok {
		o.call(call)
	}
	return true
}
func (o *roleObserver) call(call *ast.CallExpr) {
	invocation := o.owner.roleInvocation(call)
	if invocation != nil {
		invocation.observe(o.group, o.owner, call)
	}
}
func (i *roleInvocation) observe(g *roleGroup, owner *declaration, call *ast.CallExpr) {
	t := i.implementation()
	if t == nil {
		return
	}
	receipt := owner.roleReceipt(call, roleImplementation(t))
	receipt.RoleMessage = selectedMessage(i.method)
	g.observe(t, i.method, receipt)
	i.recordUsedRole(g)
}
func (d *declaration) roleReceipt(node ast.Node, implementation string) Source {
	receipt := d.source("role-message", node.Pos(), node.End(), Detail{Subject: implementation})
	return receipt
}

type roleInvocation struct {
	info     *types.Info
	receiver ast.Expr
	method   *types.Func
	role     types.Type
}

func (d *declaration) roleInvocation(call *ast.CallExpr) *roleInvocation {
	info := d.file.typeInfo()
	selection := roleCallSelection(info, call.Fun)
	if selection == nil {
		return nil
	}
	invocation := &roleInvocation{info: info}
	invocation.bind(selection, call)
	return invocation
}
func roleCallSelection(info *types.Info, expression ast.Expr) *types.Selection {
	selector, ok := unparen(expression).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selection := info.Selections[selector]
	if selection == nil || selection.Kind() == types.FieldVal {
		return nil
	}
	return selection
}
func (i *roleInvocation) implementation() types.Type {
	if !i.staticReceiver() {
		return nil
	}
	t := i.info.TypeOf(i.receiver)
	if selectedImplementation(t) == "" {
		return nil
	}
	if i.requiresAddress(t) {
		t = types.NewPointer(t)
	}
	return t
}
func (i *roleInvocation) requiresAddress(t types.Type) bool {
	return !roleSupports(t, i.method) && i.info.Types[i.receiver].Addressable()
}
func (i *roleInvocation) staticReceiver() bool {
	valid := true
	ast.Inspect(i.receiver, func(node ast.Node) bool {
		if i.dynamicNode(node) {
			valid = false
		}
		return true
	})
	return valid
}
func (i *roleInvocation) dynamicNode(node ast.Node) bool {
	if _, assertion := node.(*ast.TypeAssertExpr); assertion {
		return true
	}
	id, ok := node.(*ast.Ident)
	return ok && dynamicSelectionObject(i.info.ObjectOf(id))
}

type branchRoleScan struct {
	engine *engine
	owner  *declaration
}

func (a *engine) branchRoles(d *declaration) {
	scan := branchRoleScan{engine: a, owner: d}
	ast.Inspect(d.fn.Body, scan.visit)
}
func (s *branchRoleScan) visit(node ast.Node) bool {
	if _, closure := node.(*ast.FuncLit); closure {
		return false
	}
	arms := selectionRoleArms(node)
	if len(arms) < 2 {
		return true
	}
	g := s.group(arms)
	if g == nil {
		return true
	}
	s.record(g, node)
	return true
}
func (s *branchRoleScan) group(arms []roleArm) *roleGroup {
	g := newRoleGroup()
	for _, arm := range arms {
		observed := observedRoleArm(s.owner, arm)
		if len(observed.players) != 1 {
			return nil
		}
		g.include(observed)
	}
	return g
}
func (g *roleGroup) include(other *roleGroup) {
	maps.Copy(g.roles, other.roles)
	for key, player := range other.players {
		g.includePlayer(key, player)
	}
	g.receipts = append(g.receipts, other.receipts...)
}
func (g *roleGroup) includePlayer(key string, player *rolePlayer) {
	current := g.players[key]
	if current == nil {
		g.players[key] = player
		return
	}
	for message, method := range player.messages {
		current.messages[message] = method
	}
	current.receipts = append(current.receipts, player.receipts...)
}
func (s *branchRoleScan) record(g *roleGroup, node ast.Node) {
	g.receipts = append(g.receipts, s.owner.source("role-selection", node.Pos(), node.End(), Detail{Subject: selectionSmell}))
	s.engine.recordRole(g, nil)
}
func (i *roleInvocation) bind(selection *types.Selection, call *ast.CallExpr) {
	i.receiver = interfaceReceiver(call, selection.Kind())
	i.method = selectionMethod(selection)
	i.unwrapConversions()
}
func (i *roleInvocation) unwrapConversions() {
	i.role = i.info.TypeOf(i.receiver)
	for {
		operand := roleConversionOperand(i.info, i.receiver)
		if operand == nil {
			return
		}
		i.receiver = operand
	}
}
func roleConversionOperand(info *types.Info, expr ast.Expr) ast.Expr {
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !info.Types[call.Fun].IsType() || !isInterfaceValue(info.TypeOf(call)) {
		return nil
	}
	return call.Args[0]
}
func (i *roleInvocation) recordUsedRole(g *roleGroup) {
	if key := selectedRole(i.role); key != "" {
		g.roles[key] = i.role
	}
}
