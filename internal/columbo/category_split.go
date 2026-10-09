package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

const categorySplitKind = "category-split-updates"
const categorySplitLead = "Separate decisions explicitly select this category and update different resolved numeric output paths using a shared lexical input. Review whether the behavior belongs together in a category path or intentionally belongs to independent policies."
const categorySplitLimits = "Observational evidence only; independent pricing, rewards or other policies can have identical structure. Same resolved field path does not prove the same runtime value. Calls, conversions, aliases, pointer effects and path feasibility are not resolved. No defect, safe movement, ownership or polymorphism is inferred. No stage gate."

type categorySplitDecision struct {
	node      ast.Stmt
	selection categorySelection
	path      resolvedValuePath
}
type categorySplitScan struct {
	owner  *declaration
	groups []advisoryGroup
}
type categorySplitPair struct {
	scan        *categorySplitScan
	left, right categorySplitDecision
	block       *ast.BlockStmt
}

func (a *engine) categorySplitAdvisories() []advisoryGroup {
	groups := []advisoryGroup{}
	for _, owner := range a.declarations {
		if !owner.file.included || !owner.hasBody() {
			continue
		}
		scan := categorySplitScan{owner: owner}
		owner.inspectBody(scan.visit)
		groups = append(groups, scan.groups...)
	}
	return groups
}
func (s *categorySplitScan) visit(node ast.Node) bool {
	if coordinationClosure(node) {
		return false
	}
	if block, ok := node.(*ast.BlockStmt); ok {
		s.block(block)
	}
	return true
}
func (s *categorySplitScan) block(block *ast.BlockStmt) {
	decisions := s.decisions(block)
	for i, left := range decisions {
		for _, right := range decisions[i+1:] {
			pair := categorySplitPair{s, left, right, block}
			pair.collect()
		}
	}
}
func (s *categorySplitScan) decisions(block *ast.BlockStmt) []categorySplitDecision {
	result := []categorySplitDecision{}
	for _, stmt := range block.List {
		selection := splitSelection(stmt, s.owner.file.typeInfo())
		if s.eligible(selection) {
			result = append(result, categorySplitDecision{stmt, selection, categoryPath(s.owner.file.typeInfo(), selection.selector)})
		}
	}
	return result
}
func splitSelection(stmt ast.Stmt, info *types.Info) categorySelection {
	if node, ok := stmt.(*ast.SwitchStmt); ok {
		return splitSwitch(node)
	}
	if node, ok := stmt.(*ast.IfStmt); ok {
		return categoryIf(node, info)
	}
	return categorySelection{}
}
func splitSwitch(node *ast.SwitchStmt) categorySelection {
	if node.Init != nil || splitFallthrough(node) {
		return categorySelection{}
	}
	return categorySwitch(node)
}
func (s *categorySplitScan) eligible(selection categorySelection) bool {
	info := s.owner.file.typeInfo()
	if !selection.subject(info).valid() {
		return false
	}
	for _, branch := range selection.branches {
		if !branch.constantLabels(info) {
			return false
		}
	}
	return true
}
func splitFallthrough(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if coordinationClosure(n) {
			return false
		}
		if branch, ok := n.(*ast.BranchStmt); ok && branch.Tok == token.FALLTHROUGH {
			found = true
		}
		return true
	})
	return found
}
func (p categorySplitPair) collect() {
	if !p.left.path.same(p.right.path) || p.selectorWritten() {
		return
	}
	for _, first := range p.left.selection.branches {
		for _, second := range p.right.selection.branches {
			p.branches(first, second)
		}
	}
}
func (p categorySplitPair) selectorWritten() bool {
	written := false
	p.span(func(node ast.Node) {
		mutation, _ := mutationOf(node)
		for _, target := range mutation.targets {
			if coordinationMayReplace(categoryPath(p.scan.owner.file.typeInfo(), target), p.left.path) {
				written = true
			}
		}
	})
	return written
}
func (p categorySplitPair) span(visit func(ast.Node)) {
	for _, stmt := range p.block.List {
		if stmt.Pos() >= p.left.node.Pos() && stmt.End() <= p.right.node.End() {
			splitInspect(stmt, visit)
		}
	}
}
func splitInspect(node ast.Node, visit func(ast.Node)) {
	ast.Inspect(node, func(node ast.Node) bool {
		if node == nil || coordinationClosure(node) {
			return false
		}
		visit(node)
		return true
	})
}
func (p categorySplitPair) branches(first, second categoryBranch) {
	a, b := p.arm(p.left, first), p.arm(p.right, second)
	inputs := splitSharedInputs(a.effects, b.effects, coordinationKey(p.left.path))
	if len(inputs) == 0 || !splitDistinctWrites(a.effects, b.effects) {
		return
	}
	candidate := categorySplitCandidate{pair: p, first: a, second: b, inputs: inputs}
	for _, label := range splitSharedLabels(first, second, p.scan.owner.file.typeInfo()) {
		p.scan.groups = append(p.scan.groups, candidate.evidence(label))
	}
}
