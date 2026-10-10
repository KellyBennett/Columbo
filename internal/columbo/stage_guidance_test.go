package columbo

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReviewTaskGuidanceContract(t *testing.T) {
	for _, definition := range refactoringStages() {
		t.Run(definition.id, func(t *testing.T) {
			for _, obligation := range []string{
				"source-backed review obligations, not edit or count targets",
				"Attempt observable characterization on unchanged code before claiming blocked",
				"prerequisite exceptions",
				"honest changed/validated, retained or unresolved dispositions",
				"Do not suppress findings",
				"Later review after current-phase dispositions",
				"never clears locked tool stages or CI",
				"docs/experimental-review-recipe.md",
			} {
				require.Contains(t, definition.task, obligation)
			}
			require.Equal(t, 1, strings.Count(definition.task, reviewTask))
			require.NotContains(t, definition.task, "remove the current collector findings")
			require.NotContains(t, definition.task, "Resolve the current collector evidence")
		})
	}
	require.Contains(t, untangleTask, "temporary duplication is acceptable")
	require.Contains(t, ownershipTask, "non-gating review leads")
	require.Contains(t, ownershipTask, "Preserve defaults and no-op behavior")
	require.Contains(t, consolidationTask, "every eligible duplicate-code group")
	require.Contains(t, consolidationTask, "surrounding conditions, evaluation order and effects")
}

func TestReviewGuidanceKeepsStageMembership(t *testing.T) {
	definitions := refactoringStages()
	for i := range definitions {
		definitions[i].task = ""
	}
	require.Equal(t, []stageDefinition{
		{id: "untangle-behavior", name: "Untangle Behavior", collectors: []string{"nested-field-decision", "variant-coordination"}},
		{id: "assign-ownership", name: "Assign Ownership", collectors: []string{categoryBehaviorKind}, contextSmells: []string{variantSmell, selectionSmell}},
		{id: "consolidate-shared-behavior", name: "Consolidate Shared Behavior", collectors: []string{guardedUpdateKind}},
	}, definitions)
}

func TestReviewGuidanceDoesNotClearOrUnlockStages(t *testing.T) {
	tests := []struct {
		name   string
		kinds  []string
		states []string
		issues []int
	}{
		{"no findings", nil, []string{"cleared", "cleared", "cleared"}, []int{0, 0, 0}},
		{"retained untangling locks zero counts", []string{"nested-field-decision"}, []string{"active", "locked", "locked"}, []int{1, 0, 0}},
		{"unresolved ownership locks zero count", []string{categoryBehaviorKind}, []string{"cleared", "active", "locked"}, []int{0, 1, 0}},
		{"unresolved consolidation stays active", []string{guardedUpdateKind}, []string{"cleared", "cleared", "active"}, []int{0, 0, 1}},
		{"reopened untangling retains locked findings", []string{"variant-coordination", categoryBehaviorKind, guardedUpdateKind}, []string{"active", "locked", "locked"}, []int{1, 1, 1}},
		{"repeated groups still count", []string{guardedUpdateKind, guardedUpdateKind}, []string{"cleared", "cleared", "active"}, []int{0, 0, 2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := []advisoryGroup{}
			for _, kind := range test.kinds {
				groups = append(groups, advisoryGroup{kind: kind})
			}
			definitions := refactoringStages()
			withGuidance := evaluateStages(definitions, groups)
			require.Equal(t, test.states, stageStates(Report{Stages: withGuidance}))
			for i := range definitions {
				require.Equal(t, test.issues[i], withGuidance[i].issueCount())
				definitions[i].task = ""
			}
			withoutGuidance := evaluateStages(definitions, groups)
			for i := range withGuidance {
				require.Equal(t, withoutGuidance[i].state, withGuidance[i].state)
				require.Equal(t, withoutGuidance[i].issues, withGuidance[i].issues)
			}
		})
	}
}

func TestReviewGuidanceRendersOnlyActiveTask(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(variantPrelude + variantFunction("Operation", `n:=0;switch x {case A:n=1;case B:n=2};switch x {case A:n++;case B:n+=2};_=n`))
	cfg := variantConfig()
	cfg.Staged = true
	report := h.investigate(dir, cfg)
	db := h.snapshot(report)
	rendered, code, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Equal(t, 1, code)
	require.Equal(t, []string{"active", "locked", "locked"}, stageStates(report))
	require.Contains(t, string(rendered), untangleTask)
	require.NotContains(t, string(rendered), ownershipTask)
	require.NotContains(t, string(rendered), consolidationTask)
	require.Contains(t, string(rendered), "Stage Assign Ownership: locked")
	require.Contains(t, string(rendered), "Stage Consolidate Shared Behavior: locked")
	require.Equal(t, 3, h.sqlCount(db, "SELECT count(*) FROM refactoring_stages WHERE task LIKE '%docs/experimental-review-recipe.md%'"))
}

func TestReviewRecipeFrozenInstructions(t *testing.T) {
	for name, expected := range map[string]string{
		"phase.txt":             "4972c83396d8ae31555ea595615ca1a461d8b199d73a669d61a94ad0437ce4c5",
		"common-assignment.txt": "b203d6c2c8d43608f2440f753e890d5b457e945aebfae6a3fc89f3d5b4853ce8",
	} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "docs", "experimental-review", name))
			require.NoError(t, err)
			require.Equal(t, expected, fmt.Sprintf("%x", sha256.Sum256(data)))
		})
	}
	_, err := os.Stat(filepath.Join("..", "..", "docs", "experimental-review-recipe.md"))
	require.NoError(t, err)
}
