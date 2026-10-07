package columbo

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStagedFreshTransitions(t *testing.T) {
	h := &testHarness{T: t}
	tangled := `n:=0; switch x {case A:n=1;case B:n=2}; switch x {case A:n++;case B:n+=2};_=n`
	gathered := `n:=0; switch x {case A:n=1;n++;case B:n=2;n+=2};_=n`
	label := variantFunction("Label", variantSwitch("A,B"))
	dir := h.fixture(variantPrelude + variantFunction("Operation", tangled) + label)
	cfg := variantConfig()
	cfg.Staged = true
	before := h.investigate(dir, cfg)
	require.Equal(t, "active", before.Stages[0].state)
	require.Equal(t, "locked", before.Stages[1].state)
	require.NotEmpty(t, before.Coordination)
	checkStageSnapshot(t, h, before, 1, "Stage Untangle Behavior: active")
	h.write(dir, "source.go", variantPrelude+variantFunction("Operation", gathered)+label)
	after := h.investigate(dir, cfg)
	require.Equal(t, "cleared", after.Stages[0].state)
	require.Equal(t, "active", after.Stages[1].state)
	require.Empty(t, after.Coordination)
	require.Contains(t, after.Stages[1].definition.task, "Review lead (not a gate):")
	require.Contains(t, after.Stages[1].definition.task, "source.go:")
	h.one(after, variantSmell)
	checkStageSnapshot(t, h, after, 0, "Pending definition; this is not completion.")
	h.write(dir, "source.go", variantPrelude+variantFunction("Operation", tangled)+label)
	require.Equal(t, "active", h.investigate(dir, cfg).Stages[0].state)
}
func checkStageSnapshot(t *testing.T, h *testHarness, report Report, exit int, text string) {
	t.Helper()
	db := h.snapshot(report)
	rendered, code, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Equal(t, exit, code)
	require.Contains(t, string(rendered), text)
	require.NotContains(t, string(rendered), "CASE ")
	require.Equal(t, len(report.Cases), h.sqlCount(db, "SELECT count(*) FROM cases"))
	require.Equal(t, report.Stages[0].issueCount(), h.sqlCount(db, "SELECT count(*) FROM advisory_groups WHERE kind IN ('nested-field-decision','variant-coordination')"))
}
func TestStagedIndependentOfSeverityWithUnchangedEligibility(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(variantPrelude + variantFunction("Operation", `n:=0;switch x {case A:n=1;case B:n=2};switch x {case A:n++;case B:n+=2};_=n`))
	cfg := quiet()
	cfg.Staged = true
	report := h.investigate(dir, cfg)
	require.Empty(t, report.Cases)
	require.NotEmpty(t, report.Coordination)
	checkStageSnapshot(t, h, report, 1, "Collector variant-coordination: 2 issues")
	cfg.Counts["repeated-variant-sites"] = 3
	require.Empty(t, h.investigate(dir, cfg).Coordination)
}
func TestStagedNestedCollectorAndLegacyMode(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(tanglePrelude + tangleBody)
	cfg := quiet()
	legacy := h.investigate(dir, cfg)
	require.Nil(t, legacy.Stages)
	cfg.Staged = true
	report := h.investigate(dir, cfg)
	require.Equal(t, 1, report.Stages[0].issues["nested-field-decision"])
	checkStageSnapshot(t, h, report, 1, "Collector nested-field-decision: 1 issues")
	var out, stderr bytes.Buffer
	code := Run([]string{"--staged", "--no-history", "--output", filepath.Join(dir, "cli.sqlite")}, Invocation{Dir: dir, Stdout: &out, Stderr: &stderr})
	require.Equal(t, 1, code, stderr.String())
	require.Contains(t, out.String(), "Stage Untangle Behavior: active")
}
func TestOrderedStageMembership(t *testing.T) {
	definitions := []stageDefinition{
		{id: "first", collectors: []string{"a", "b"}},
		{id: "second", collectors: []string{"c"}},
		{id: "terminal", pending: true},
	}
	result := evaluateStages(definitions, []advisoryGroup{{kind: "c"}})
	require.Equal(t, "cleared", result[0].state)
	require.Equal(t, "active", result[1].state)
	require.Equal(t, "locked", result[2].state)
	result = evaluateStages(definitions, nil)
	require.Equal(t, "active", result[2].state)
	result = evaluateStages(definitions, []advisoryGroup{{kind: "b"}, {kind: "c"}})
	require.Equal(t, "active", result[0].state)
	require.Equal(t, "locked", result[1].state)
}

func TestStagedSuppressionDoesNotClearCollector(t *testing.T) {
	h := &testHarness{T: t}
	directive := "//columbo:ignore repeated-variant-decision -- accepted legacy case\n"
	body := variantFunction("Operation", `n:=0;switch x {case A:n=1;case B:n=2};switch x {case A:n++;case B:n+=2};_=n`)
	cfg := variantConfig()
	cfg.Staged = true
	report := h.investigate(h.fixture(variantPrelude+directive+body), cfg)
	require.Equal(t, 1, report.Summary.Suppressed)
	require.Equal(t, 0, report.Summary.Failed)
	require.NotEmpty(t, report.Coordination)
	checkStageSnapshot(t, h, report, 1, "Stage Untangle Behavior: active")
}
