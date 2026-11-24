package feedback

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/KellyBennett/Columbo/internal/columbo"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"github.com/stretchr/testify/require"
)

const clumpBaseline = `package roster

func Enroll(orgID string, sectionID string, termID string) {}
func Drop(orgID string, sectionID string, termID string)   {}
func Notify(orgID string, sectionID string, termID string) {}
func Audit(orgID string, sectionID string, termID string)  {}
`

func TestTargetClassificationOnRealSnapshots(t *testing.T) {
	baseline := analyzeSource(t, clumpBaseline)
	require.Len(t, baseline, 1)
	target := baseline[0]
	require.Equal(t, "data-clump", target.Smell)
	require.Equal(t, 4.0, metricValue(t, target, "clump-occurrences"))
	for _, test := range []struct {
		name, source string
		status       TargetStatus
		occurrences  float64
	}{
		{"refactor removes the clump", `package roster

type Section struct{ Org, ID, Term string }

func Enroll(s Section) {}
func Drop(s Section)   {}
func Notify(s Section) {}
func Audit(s Section)  {}
`, TargetCleared, 0},
		{"refactor narrows the clump but leaves it over the limit", `package roster

type Section struct{ Org, ID, Term string }

func Enroll(s Section)                                     {}
func Drop(orgID string, sectionID string, termID string)   {}
func Notify(orgID string, sectionID string, termID string) {}
func Audit(orgID string, sectionID string, termID string)  {}
`, TargetPersistedChanged, 3},
		{"refactor leaves the target untouched", clumpBaseline + "\nfunc Unrelated() {}\n", TargetPersisted, 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome := classifyTarget(target.ID, baseline, analyzeSource(t, test.source), nil)
			require.Equal(t, test.status, outcome.Status, outcome.Reason)
			require.Equal(t, target.ID, outcome.Baseline.ID)
			require.Empty(t, outcome.NewCases)
			if test.status == TargetCleared {
				require.Nil(t, outcome.Match)
				return
			}
			require.Equal(t, test.occurrences, metricValue(t, *outcome.Match, "clump-occurrences"))
			if test.status == TargetPersistedChanged {
				require.NotEqual(t, target.ID, outcome.Match.ID, "narrowing changed the case ID")
				require.Contains(t, outcome.Reason, "rule 3")
				require.Contains(t, outcome.Reason, "shares 3 declaration symbol(s)")
			}
		})
	}
}

// analyzeSource analyzes a one-file module with the default policy and loads
// its cases through the snapshot and cases.json path the pilot runner uses.
func analyzeSource(t *testing.T, source string) []AnalyzedCase {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.invalid/roster\n\ngo 1.25.1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roster.go"), []byte(source), 0o600))
	config := columbo.Defaults()
	config.History, config.Staged = false, false
	report, err := columbo.Analyze(dir, []string{"./..."}, config)
	require.NoError(t, err)
	prefix := filepath.Join(t.TempDir(), "analysis")
	require.NoError(t, columbo.WriteSnapshot(prefix+".sqlite", report, "test"))
	snapshot, err := snapshotdb.Open(prefix + ".sqlite")
	require.NoError(t, err)
	defer snapshot.Close()
	cases, err := loadAnalyzedCases(context.Background(), snapshot.Queries())
	require.NoError(t, err)
	require.NoError(t, saveJSON(casesPath(prefix), cases))
	loaded, err := readAnalyzedCases(CommandResult{Exit: 1, EvidenceValid: true}, prefix)
	require.NoError(t, err)
	return loaded
}

func metricValue(t *testing.T, c AnalyzedCase, kind string) float64 {
	t.Helper()
	for _, metric := range c.Metrics {
		if metric.Kind == kind {
			value, ok := metric.NumericValue.(float64)
			require.True(t, ok, "metric %s has value %v", kind, metric.NumericValue)
			return value
		}
	}
	t.Fatalf("case %s has no %s metric", c.ID, kind)
	return 0
}

func syntheticCase(id, smell, path string, symbols ...string) AnalyzedCase {
	c := AnalyzedCase{SummaryCasesRow: snapshotdb.SummaryCasesRow{ID: id, Smell: smell, Symbol: symbols[0], Path: path}}
	for _, symbol := range symbols {
		c.Declarations = append(c.Declarations, CaseDeclaration{Symbol: symbol, Path: path, Role: "clump-support"})
	}
	return c
}

func TestTargetSuccessorExcludesPreexistingCasesAndCountsNewOnes(t *testing.T) {
	target := syntheticCase("C-target", "data-clump", "a.go", "p.A", "p.B", "p.C")
	neighbor := syntheticCase("C-neighbor", "data-clump", "a.go", "p.P", "p.Q", "p.R")
	spawned := syntheticCase("C-spawned", "long-function", "a.go", "p.A")
	baseline := []AnalyzedCase{target, neighbor}

	outcome := classifyTarget("C-target", baseline, []AnalyzedCase{neighbor, spawned}, nil)
	require.Equal(t, TargetCleared, outcome.Status, "a pre-existing same-smell case in the same file is not the successor")
	require.Equal(t, []string{"C-spawned"}, outcome.NewCases)

	pathOnly := syntheticCase("C-path", "data-clump", "a.go", "p.X", "p.Y", "p.Z")
	symbolOverlap := syntheticCase("C-symbols", "data-clump", "b.go", "p.B", "p.C", "p.D")
	outcome = classifyTarget("C-target", baseline, []AnalyzedCase{neighbor, pathOnly, symbolOverlap, spawned}, nil)
	require.Equal(t, TargetPersistedChanged, outcome.Status)
	require.Equal(t, "C-symbols", outcome.Match.ID, "symbol overlap is preferred over a primary-file match")
	require.Equal(t, []string{"C-path", "C-spawned"}, outcome.NewCases)

	moved := syntheticCase("C-moved", "data-clump", "renamed.go", "p.X", "p.Y", "p.Z")
	outcome = classifyTarget("C-target", baseline, []AnalyzedCase{moved}, map[string]string{"a.go": "renamed.go"})
	require.Equal(t, TargetPersistedChanged, outcome.Status)
	require.Equal(t, "C-moved", outcome.Match.ID)
	require.Contains(t, outcome.Reason, "primary file renamed.go")
	require.Equal(t, TargetCleared, classifyTarget("C-target", baseline, []AnalyzedCase{moved}, nil).Status)

	outcome = classifyTarget("C-missing", baseline, []AnalyzedCase{target, spawned}, nil)
	require.Equal(t, TargetUnknown, outcome.Status)
	require.Contains(t, outcome.Reason, "rule 1")
}

func TestTargetIsUnknownWithoutValidAnalyzerEvidence(t *testing.T) {
	root := t.TempDir()
	trial := pilotTrial{root: root, finding: PilotCase{CaseID: "C-target"}}
	trial.result.BaselineColumbo = CommandResult{Exit: 1, EvidenceValid: true}
	require.NoError(t, saveJSON(casesPath(filepath.Join(root, "baseline-columbo")), []AnalyzedCase{syntheticCase("C-target", "data-clump", "a.go", "p.A")}))
	outcome := trial.observeTarget(CommandResult{Exit: 2}, filepath.Join(root, "attempt-1-columbo"), nil, nil)
	require.Equal(t, TargetUnknown, outcome.Status)
	require.Contains(t, outcome.Reason, "lacks valid evidence")
	require.Nil(t, outcome.NewCases)
}

func TestRenamedPathsAreModuleRelative(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	_, err := git(context.Background(), "", "init", "--quiet", dir)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "mod"), 0o700))
	body := "package p\n\n// A long enough body for rename detection to pair the files.\nfunc One() {}\nfunc Two() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mod", "old.go"), []byte(body), 0o600))
	before := commitFiles(t, dir)
	require.NoError(t, os.Rename(filepath.Join(dir, "mod", "old.go"), filepath.Join(dir, "mod", "new.go")))
	renames, err := candidateRenames(context.Background(), dir, before, "mod")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"old.go": "new.go"}, renames)
}

func TestReportShowsRunnerObservedTargetSeparatelyFromAgentJudgment(t *testing.T) {
	root := t.TempDir()
	limit := 3.0
	target := syntheticCase("C-target", "data-clump", "a.go", "p.A", "p.B", "p.C")
	target.Metrics = []snapshotdb.SummaryMetricsRow{{Kind: "clump-occurrences", NumericValue: 4.0, LimitValue: limit}}
	successor := syntheticCase("C-successor", "data-clump", "a.go", "p.B", "p.C")
	successor.Metrics = []snapshotdb.SummaryMetricsRow{{Kind: "clump-occurrences", NumericValue: 3.0, LimitValue: limit}}
	narrowed := classifyTarget("C-target", []AnalyzedCase{target}, []AnalyzedCase{successor, syntheticCase("C-spawned", "long-function", "b.go", "p.D")}, nil)
	untouched := classifyTarget("C-target", []AnalyzedCase{target}, []AnalyzedCase{target}, nil)
	explored := PilotCase{ID: "explored", PR: 1, CaseID: "C-target", Rule: "data-clump", Reaction: "thumbs-down"}
	actual := PilotCase{ID: "actual", PR: 2, CaseID: "C-target", Rule: "data-clump", Reaction: "thumbs-up"}
	unobserved := PilotCase{ID: "unobserved", PR: 3, CaseID: "C-other", Rule: "data-clump", Reaction: "unreacted"}
	require.NoError(t, saveJSON(filepath.Join(root, "trials", explored.ID, "result.json"), TrialResult{Attempts: []RefactoringAttempt{
		{Number: 1, Target: &TargetOutcome{Status: TargetUnknown, Reason: "evidence unavailable: analyzer failed"}, Unformatted: []string{"a.go", "b.go"}},
		{Number: 2, Target: &narrowed},
	}, ActualTarget: &untouched}))
	require.NoError(t, saveJSON(filepath.Join(root, "trials", actual.ID, "result.json"), TrialResult{ActualTarget: &untouched}))
	require.NoError(t, pilotSummary(root, PilotPlan{Cases: []PilotCase{explored, actual, unobserved}}))
	data, err := os.ReadFile(filepath.Join(root, "report.md"))
	require.NoError(t, err)
	report := string(data)
	require.Contains(t, report, "| Behavior checks | Target (runner-observed) | Evidence |")
	require.Contains(t, report, "| not verified | persisted-changed | [trial](trials/explored/result.json) |", "exploratory rows show the last attempt")
	require.Contains(t, report, "| not verified | persisted | [trial](trials/actual/result.json) |", "actual-changes rows show the team revision")
	require.Contains(t, report, "| not verified | n/a | [trial](trials/unobserved/result.json) |")
	require.Contains(t, report, "* Runner-observed target (not agent judgment), attempt 1: unknown. evidence unavailable: analyzer failed. Matched case: none. Metrics: unavailable. New cases: unknown.")
	require.Contains(t, report, "* Runner-observed target (not agent judgment), attempt 2: persisted-changed. rule 3: new data-clump case C-successor shares 2 declaration symbol(s) with the target. Matched case: C-successor. Metrics: clump-occurrences 4→3 (limit 3). New cases: 1.")
	require.Contains(t, report, "* Runner-observed target (not agent judgment), team revision: persisted. rule 2: the same case ID is present in this revision. Matched case: C-target. Metrics: clump-occurrences 4→4 (limit 3). New cases: 0.")
	require.Contains(t, report, "* Unformatted Go files after attempt 1 (runner gofmt -l): a.go, b.go")
}
