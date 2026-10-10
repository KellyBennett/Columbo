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

const untangleTask = "Review behavioral categories for coherent paths or methods. Preserve behavior, ordering and boundaries; temporary duplication is acceptable. Defer ownership and sharing to later review phases." + reviewTask
const ownershipTask = "Review category-selected behavior and non-gating review leads for coherent ownership. Separate selection from invocation when it reduces duplicated knowledge. Preserve defaults and no-op behavior." + reviewTask
const consolidationTask = "Review every eligible duplicate-code group for shared responsibility. Preserve intentional differences, surrounding conditions, evaluation order and effects; evidence does not prove safe extraction or outer-guard removal." + reviewTask
const reviewTask = " Treat findings as source-backed review obligations, not edit or count targets. Do not suppress findings. Attempt observable characterization on unchanged code before claiming blocked; see docs/experimental-review-recipe.md for prerequisite exceptions and honest changed/validated, retained or unresolved dispositions. Later review after current-phase dispositions never clears locked tool stages or CI."

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
