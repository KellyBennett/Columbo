package columbo

import (
	"go/ast"
	"go/types"
	"sort"
)

type categorySplitArm struct {
	owner    *declaration
	decision categorySplitDecision
	branch   categoryBranch
	effects  coordinationEffects
	updates  []ast.Node
}

func (p categorySplitPair) arm(decision categorySplitDecision, branch categoryBranch) categorySplitArm {
	owner := p.scan.owner
	arm := categorySplitArm{owner: owner, decision: decision, branch: branch, effects: coordinationEffects{owner: owner, info: owner.file.typeInfo(), reads: map[string]Source{}, writes: map[string]Source{}}}
	for _, stmt := range branch.body {
		ast.Inspect(stmt, arm.update)
	}
	return arm
}
func (a *categorySplitArm) update(node ast.Node) bool {
	if coordinationClosure(node) {
		return false
	}
	mutation, ok := mutationOf(node)
	if !ok {
		return true
	}
	if a.scalarMutation(mutation) {
		a.updates = append(a.updates, node)
	}
	return false
}
func (a *categorySplitArm) scalarMutation(mutation statementMutation) bool {
	if len(mutation.targets) != 1 {
		return false
	}
	target := mutation.targets[0]
	if !splitNumeric(a.effects.info.TypeOf(target)) || coordinationKey(categoryPath(a.effects.info, target)) == "" {
		return false
	}
	a.effects.write(target)
	a.effects.readAll(mutation.reads)
	return true
}
func splitNumeric(typ types.Type) bool {
	if typ == nil {
		return false
	}
	basic, ok := typ.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsNumeric != 0
}
func splitSharedLabels(a, b categoryBranch, info *types.Info) []string {
	values := map[string]bool{}
	for _, label := range a.labels {
		values[info.Types[label].Value.ExactString()] = true
	}
	result := map[string]bool{}
	for _, label := range b.labels {
		value := info.Types[label].Value.ExactString()
		if values[value] {
			result[value] = true
		}
	}
	return splitSortedKeys(result)
}
func splitSortedKeys[T any](values map[string]T) []string {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func splitDistinctWrites(a, b coordinationEffects) bool {
	if len(a.writes) == 0 || len(b.writes) == 0 {
		return false
	}
	for key := range a.writes {
		if _, ok := b.writes[key]; ok {
			return false
		}
	}
	return true
}
func splitSharedInputs(a, b coordinationEffects, selector string) []string {
	keys := []string{}
	for key := range a.reads {
		if key != selector && splitInput(a, b, key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
func splitInput(a, b coordinationEffects, key string) bool {
	_, read := b.reads[key]
	_, leftWrite := a.writes[key]
	_, rightWrite := b.writes[key]
	return read && !leftWrite && !rightWrite
}
