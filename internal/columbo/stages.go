package columbo

type stageDefinition struct {
	id, name, task string
	collectors     []string
	contextSmells  []string
	pending        bool
}
type stageResult struct {
	definition stageDefinition
	state      string
	issues     map[string]int
}

func refactoringStages() []stageDefinition {
	return []stageDefinition{
		{id: "untangle-behavior", name: "Untangle Behavior", task: untangleTask,
			collectors: []string{"nested-field-decision", "variant-coordination"}},
		{id: "assign-ownership", name: "Assign Ownership", task: ownershipTask, collectors: []string{categoryBehaviorKind}, contextSmells: []string{variantSmell, selectionSmell}},
		{id: "consolidate-shared-behavior", name: "Consolidate Shared Behavior", task: consolidationTask, collectors: []string{guardedUpdateKind}},
	}
}

const untangleTask = "Gather each behavioral category into coherent paths or methods, preserving behavior, ordering and boundaries. Temporary duplication is acceptable. Do only enough to remove the current collector findings, then rerun Columbo for fresh evidence. Defer ownership, sharing and deduplication decisions."
const ownershipTask = "Assign an explicit owner to category-selected behavior and separate category selection from invocation. Functions and objects can both express ownership. Preserve default and no-op behavior. Clearing this initial bounded collector is not proof that all architectural ownership is correct."

const consolidationTask = "Consolidate repeated responsibilities into shared implementations while preserving behavior and the guarantees of earlier stages. Resolve the current collector evidence, then rerun Columbo. Preserve surrounding conditions, evaluation order and effects; evidence is not proof of safe extraction or outer-guard removal. Clearing this bounded collector is not proof that all shared responsibilities have been consolidated."

type stageEvaluation struct {
	groups []advisoryGroup
	active bool
}

func evaluateStages(definitions []stageDefinition, groups []advisoryGroup) []stageResult {
	evaluation := stageEvaluation{groups: groups}
	results := []stageResult{}
	for _, definition := range definitions {
		results = append(results, evaluation.evaluate(definition))
	}
	return results
}
func (e *stageEvaluation) evaluate(definition stageDefinition) stageResult {
	result := stageResult{definition: definition, state: "locked", issues: map[string]int{}}
	for _, collector := range definition.collectors {
		result.issues[collector] = collectorIssues(e.groups, collector)
	}
	if !e.active {
		result.selectState()
		e.active = result.state == "active"
	}
	return result
}
func (r *stageResult) selectState() {
	r.state = "cleared"
	if r.definition.pending || r.issueCount() > 0 {
		r.state = "active"
	}
}
func collectorIssues(groups []advisoryGroup, collector string) int {
	count := 0
	for _, group := range groups {
		if group.kind == collector {
			count++
		}
	}
	return count
}
func (r stageResult) issueCount() int {
	count := 0
	for _, n := range r.issues {
		count += n
	}
	return count
}
func (a *engine) collectStages() {
	if !a.config.Staged {
		return
	}
	a.report.Coordination = a.coordinationAdvisories()
	a.report.CategoryBehavior = a.categoryBehaviorAdvisories()
	a.report.Stages = evaluateStages(a.report.stageDefinitions(), a.report.advisories())
}
