package columbo

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsolidationFreshTransitionsAndLockedEvidence(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	cfg.Staged = true
	repeated := guardedPrelude + `func another(x *Item){if x.Quality<50{baseline(x)}}`
	dir := h.fixture(repeated)
	active := h.investigate(dir, cfg)
	require.Equal(t, []string{"cleared", "cleared", "active"}, stageStates(active))
	require.Equal(t, 1, active.Stages[2].issues[guardedUpdateKind])
	checkStageSnapshot(t, h, active, 1, "Collector repeated-guarded-update: 1 issues")
	h.write(dir, "source.go", guardedPrelude+`func another(x *Item){baseline(x)}`)
	cleared := h.investigate(dir, cfg)
	require.Equal(t, []string{"cleared", "cleared", "cleared"}, stageStates(cleared))
	checkStageSnapshot(t, h, cleared, 0, "Stage Consolidate Shared Behavior: cleared")
	h.write(dir, "source.go", repeated)
	require.Equal(t, stageStates(active), stageStates(h.investigate(dir, cfg)))
	h.write(dir, "selection.go", `package fixture;type Category struct{Name string};func first(*Category){};func second(*Category){};func selectAndRun(x *Category){switch x.Name{case "a":first(x);default:second(x)}}`)
	locked := h.investigate(dir, cfg)
	require.Equal(t, []string{"cleared", "active", "locked"}, stageStates(locked))
	db := h.snapshot(locked)
	require.Equal(t, 1, h.sqlCount(db, "SELECT count(*) FROM advisory_groups WHERE kind='repeated-guarded-update'"))
	require.Equal(t, 0, h.sqlCount(db, "SELECT count(*) FROM active_stage_issues WHERE kind='repeated-guarded-update'"))
	require.Equal(t, 1, locked.Stages[2].issueCount())
}

func stageStates(report Report) []string {
	states := []string{}
	for _, stage := range report.Stages {
		states = append(states, stage.state)
	}
	return states
}

func TestConsolidationCLIAndOrdinaryMode(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(guardedPrelude + `func another(x *Item){if x.Quality<50{x.Quality++}}`)
	var out, stderr bytes.Buffer
	code := Run([]string{"--staged", "--no-history", "--output", filepath.Join(dir, "cli.sqlite")}, Invocation{Dir: dir, Stdout: &out, Stderr: &stderr})
	require.Equal(t, 1, code, stderr.String())
	for _, text := range []string{"Stage Consolidate Shared Behavior: active", consolidationTask, guardedUpdateKind} {
		require.Contains(t, out.String(), text)
	}
	legacy := h.investigate(dir, quiet())
	require.Nil(t, legacy.Stages)
	require.Empty(t, legacy.Cases)
	require.Len(t, legacy.GuardedUpdates, 1)
	rendered, code, err := RenderSnapshot(testDatabaseQueries(h.snapshot(legacy)), "report.sqlite")
	require.NoError(t, err)
	require.Zero(t, code)
	require.Contains(t, string(rendered), guardedUpdateKind)
}

func TestConsolidationGateCountsGroupsAndFirstUnclearedStage(t *testing.T) {
	tests := []struct {
		name   string
		kinds  []string
		states []string
		issues int
	}{
		{"none", nil, []string{"cleared", "cleared", "cleared"}, 0},
		{"one group", []string{guardedUpdateKind}, []string{"cleared", "cleared", "active"}, 1},
		{"two groups", []string{guardedUpdateKind, guardedUpdateKind}, []string{"cleared", "cleared", "active"}, 2},
		{"ownership reopens", []string{categoryBehaviorKind, guardedUpdateKind}, []string{"cleared", "active", "locked"}, 1},
		{"untangling reopens", []string{"nested-field-decision", guardedUpdateKind}, []string{"active", "locked", "locked"}, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := []advisoryGroup{}
			for _, kind := range test.kinds {
				groups = append(groups, advisoryGroup{kind: kind})
			}
			report := Report{Stages: evaluateStages(refactoringStages(), groups)}
			require.Equal(t, test.states, stageStates(report))
			require.Equal(t, test.issues, report.Stages[2].issueCount())
			require.False(t, report.Stages[2].definition.pending)
		})
	}
}
