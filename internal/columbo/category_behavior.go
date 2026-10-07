package columbo

import (
	"go/ast"
	"go/types"
	"strconv"
)

const categoryBehaviorKind = "category-selected-behavior"
const categoryBehaviorLead = "This caller appears to select between category-specific implementations of the same operation. Give that operation an explicit owner and separate category selection from invoking the behavior. Functions or objects can express ownership; preserve default and explicit no-op behavior."
const categoryBehaviorLimits = "Bounded lexical evidence, not proof of a design defect or common semantics. Direct string/integer field switch or equality if/else; single direct free-function call/return per action branch; identical non-generic, non-variadic signatures; same resolved selector subject in the same argument position. No alias, mutation, interprocedural flow or exhaustiveness inference. Returning a selected function/object is not direct execution."

type categoryAction struct {
	branch   categoryBranch
	call     *ast.CallExpr
	target   *declaration
	position int
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
	actions, ok := scan.actions(selection, subject)
	if !ok || !compatibleCategoryActions(actions) {
		return
	}
	scan.groups = append(scan.groups, scan.evidence(selection, subject, actions))
}
func (scan *categoryBehaviorScan) actions(selection categorySelection, subject resolvedValuePath) ([]categoryAction, bool) {
	actions := []categoryAction{}
	for _, branch := range selection.branches {
		action, ok := scan.branchAction(branch, subject)
		if !ok {
			return nil, false
		}
		if action.call != nil {
			actions = append(actions, action)
		}
	}
	return actions, true
}
func (scan *categoryBehaviorScan) branchAction(branch categoryBranch, subject resolvedValuePath) (categoryAction, bool) {
	if !branch.constantLabels(scan.owner.file.typeInfo()) {
		return categoryAction{}, false
	}
	call, ok := branch.action()
	if !ok || call == nil {
		return categoryAction{}, ok
	}
	return scan.resolveAction(branch, call, subject)
}
func (scan *categoryBehaviorScan) resolveAction(branch categoryBranch, call *ast.CallExpr, subject resolvedValuePath) (categoryAction, bool) {
	target := scan.engine.calls[call]
	if target == nil || !target.file.included || target.hasReceiver() || !target.hasBody() {
		return categoryAction{}, false
	}
	signature := target.signature
	if signature.Variadic() || signature.TypeParams().Len() != 0 || !categoryActionResults(signature) || !categoryArguments(scan.owner.file.typeInfo(), call) {
		return categoryAction{}, false
	}
	for position, argument := range call.Args {
		if subject.same(categoryPath(scan.owner.file.typeInfo(), argument)) {
			return categoryAction{branch, call, target, position}, true
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
