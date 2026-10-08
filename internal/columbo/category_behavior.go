package columbo

import (
	"go/ast"
	"go/types"
	"strconv"
)

const categoryBehaviorKind = "category-selected-behavior"
const categoryBehaviorLead = "This caller appears to select between category-specific implementations of the same operation. Give that operation an explicit owner and separate category selection from invoking the behavior. Functions or objects can express ownership; preserve default and explicit no-op behavior."
const categoryBehaviorLimits = "Bounded lexical evidence, not proof of a design defect or common semantics. Direct string/integer field switch or equality if/else; at least two understood action branches, each a direct free-function call/return or an immediately invoked branch-local function binding; identical non-generic, non-variadic signatures; same resolved selector subject in the same argument position. Unsupported branches are retained as unknown context, never no-ops. No general alias, mutation, interprocedural flow or exhaustiveness inference. Returning a selected function/object is not direct execution."

type categoryAction struct {
	branch   categoryBranch
	call     *ast.CallExpr
	target   *declaration
	position int
	binding  ast.Node
}
type categoryBehaviorScan struct {
	engine  *engine
	owner   *declaration
	groups  []advisoryGroup
	ifTails map[*ast.IfStmt]bool
}

func (a *engine) categoryBehaviorAdvisories() []advisoryGroup {
	groups := []advisoryGroup{}
	for _, owner := range a.declarations {
		if !owner.file.included {
			continue
		}
		scan := categoryBehaviorScan{engine: a, owner: owner, ifTails: map[*ast.IfStmt]bool{}}
		owner.inspectBody(scan.visit)
		groups = append(groups, scan.groups...)
	}
	return groups
}
func (scan *categoryBehaviorScan) visit(node ast.Node) bool {
	if _, closure := node.(*ast.FuncLit); closure {
		return false
	}
	if decision, ok := node.(*ast.SwitchStmt); ok {
		scan.collect(categorySwitch(decision))
	}
	if decision, ok := node.(*ast.IfStmt); ok {
		scan.visitIf(decision)
	}
	return true
}
func (scan *categoryBehaviorScan) visitIf(node *ast.IfStmt) {
	if tail, ok := node.Else.(*ast.IfStmt); ok {
		scan.ifTails[tail] = true
	}
	if !scan.ifTails[node] {
		scan.collect(categoryIf(node, scan.owner.file.typeInfo()))
	}
}
func (scan *categoryBehaviorScan) collect(selection categorySelection) {
	subject := selection.subject(scan.owner.file.typeInfo())
	if !subject.valid() {
		return
	}
	actions := scan.actions(selection, subject)
	if !compatibleCategoryActions(actions) {
		return
	}
	scan.groups = append(scan.groups, scan.evidence(selection, subject, actions))
}
func (scan *categoryBehaviorScan) actions(selection categorySelection, subject resolvedValuePath) []categoryAction {
	actions := []categoryAction{}
	for _, branch := range selection.branches {
		if !branch.constantLabels(scan.owner.file.typeInfo()) {
			return nil
		}
		action, ok := scan.branchAction(branch, subject)
		if ok && action.call != nil {
			actions = append(actions, action)
		}
	}
	return actions
}
func (scan *categoryBehaviorScan) branchAction(branch categoryBranch, subject resolvedValuePath) (categoryAction, bool) {
	invocation, ok := branch.invocation(scan.owner.file.typeInfo())
	if !ok || invocation.call == nil {
		return categoryAction{}, ok
	}
	action := categoryAction{branch: branch, call: invocation.call, target: invocation.declaration(scan.engine), binding: invocation.binding}
	if !action.valid(scan.owner.file.typeInfo()) {
		return categoryAction{}, false
	}
	return action.onSubject(scan.owner.file.typeInfo(), subject)
}
func (action categoryAction) valid(info *types.Info) bool {
	target := action.target
	if target == nil || !target.file.included || target.hasReceiver() || !target.hasBody() {
		return false
	}
	signature := target.signature
	return !signature.Variadic() && signature.TypeParams().Len() == 0 && categoryActionResults(signature) && categoryArguments(info, action.call)
}
func (action categoryAction) onSubject(info *types.Info, subject resolvedValuePath) (categoryAction, bool) {
	for position, argument := range action.call.Args {
		if subject.same(categoryPath(info, argument)) {
			action.position = position
			return action, true
		}
	}
	return categoryAction{}, false
}
func compatibleCategoryActions(actions []categoryAction) bool {
	if len(actions) < 2 {
		return false
	}
	first := actions[0]
	distinct := false
	for _, action := range actions[1:] {
		if action.position != first.position || !types.Identical(action.target.signature, first.target.signature) {
			return false
		}
		distinct = distinct || action.target.obj != first.target.obj
	}
	return distinct
}
func (scan *categoryBehaviorScan) evidence(selection categorySelection, subject resolvedValuePath, actions []categoryAction) advisoryGroup {
	group := scan.categoryGroup(selection, subject)
	group.categoryActions(actions)
	group.sites = append([]advisorySite{scan.selectionSite(selection, actions)}, group.sites...)
	group.addSharedCategoryState()
	return group
}
func (scan *categoryBehaviorScan) categoryGroup(selection categorySelection, subject resolvedValuePath) advisoryGroup {
	owner := scan.owner
	key := strconv.Itoa(owner.file.tf.Offset(selection.selector.Pos()))
	id, _ := identity(categoryBehaviorKind, owner.file.rel, owner.symbol, key)
	return advisoryGroup{id: id, kind: categoryBehaviorKind, subject: subject.key(owner.variable(subject.root)), lead: categoryBehaviorLead, limits: categoryBehaviorLimits}
}
func (group *advisoryGroup) categoryActions(actions []categoryAction) {
	group.values = []advisoryValue{{"candidate-callable-surface", "", types.TypeString(actions[0].target.signature, nil)}, {"subject-argument-index", "", strconv.Itoa(actions[0].position)}}
	for _, action := range actions {
		group.sites = append(group.sites, action.target.categoryCalleeSite(action.position))
	}
}
