package columbo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCLIProducesFreshStagedSourceEvidence(t *testing.T) {
	root := (&testHarness{T: t}).fixture(tanglePrelude + tangleBody + "\nfunc Excessive(a,b,c,d,e int) {}\n")
	output := t.TempDir()
	t.Setenv("TMPDIR", output)
	var identity string
	for run := 0; run < 2; run++ {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"./..."}, Invocation{Dir: root, Stdout: &stdout, Stderr: &stderr})
		require.Equal(t, 1, code, stderr.String())
		assert.Contains(t, stdout.String(), "Stage Untangle Behavior: active")
		assert.Contains(t, stdout.String(), "Stage Assign Ownership: locked")
		assert.Contains(t, stdout.String(), "source_receiver_seeds")
		assert.Contains(t, stdout.String(), "source_receiver_siblings")
		assert.Contains(t, stdout.String(), "source_storage_paths")
		assert.Contains(t, stdout.String(), "source_one_hop_calls")
		assert.Contains(t, stdout.String(), "verify source_files.sha256 and source_ranges.sha256")
		assert.Contains(t, stdout.String(), "do not prove aliases, ownership, lifetime or effects")
		assert.NotContains(t, stdout.String(), "docs/refactoring-stages.md")
		_, database, found := strings.Cut(stdout.String(), "Database: ")
		require.True(t, found)
		path := strings.TrimSpace(database)
		require.Equal(t, output, filepath.Dir(path))
		snapshot, err := OpenSnapshot(path)
		require.NoError(t, err)
		q := snapshot.Queries()
		scan, err := q.InspectionIdentity(context.Background())
		require.NoError(t, err)
		assert.Equal(t, root, scan.Root)
		assert.EqualValues(t, 1, scan.Staged)
		require.NotNil(t, scan.ConfigPath)
		assert.Equal(t, ".columbo.yml", *scan.ConfigPath)
		assert.Nil(t, scan.ConfigSha256)
		if run == 0 {
			identity = scan.SourceSha256
		}
		assert.Equal(t, identity, scan.SourceSha256)
		stages, err := q.SummaryStages(context.Background())
		require.NoError(t, err)
		require.Len(t, stages, 3)
		assert.Equal(t, "active", stages[0].State)
		cases, err := q.SummaryCases(context.Background())
		require.NoError(t, err)
		assert.NotEmpty(t, cases)
		require.NoError(t, snapshot.Close())
	}
	files, err := filepath.Glob(filepath.Join(output, "columbo-*.sqlite"))
	require.NoError(t, err)
	assert.Len(t, files, 2)
	sourceOutputs, err := filepath.Glob(filepath.Join(root, "*.sqlite"))
	require.NoError(t, err)
	assert.Empty(t, sourceOutputs)
}

func TestDefaultCLIAllowsExplicitDiagnosticProjection(t *testing.T) {
	root := (&testHarness{T: t}).fixture("package fixture\nfunc Excessive(a,b,c,d,e int) {}\n")
	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		staged bool
	}{
		{name: "default", code: 0, staged: true},
		{name: "diagnostic", args: []string{"--staged=false"}, code: 1, staged: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.sqlite")
			args := []string{"--no-history", "--output", path}
			args = append(args, tc.args...)
			var stdout, stderr bytes.Buffer
			code := Run(args, Invocation{Dir: root, Stdout: &stdout, Stderr: &stderr})
			require.Equal(t, tc.code, code, stderr.String())
			snapshot, err := OpenSnapshot(path)
			require.NoError(t, err)
			q := snapshot.Queries()
			scan, err := q.InspectionIdentity(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.staged, scan.Staged != 0)
			cases, err := q.SummaryCases(context.Background())
			require.NoError(t, err)
			require.Len(t, cases, 1)
			assert.Equal(t, "FAIL", cases[0].Verdict)
			assert.Contains(t, stdout.String(), "Source evidence:")
			assert.Equal(t, !tc.staged, strings.Contains(stdout.String(), "CASE "))
			require.NoError(t, snapshot.Close())
		})
	}
}

func TestDefaultCLIPreservesSubdirectoryScopeAndConfiguration(t *testing.T) {
	root := (&testHarness{T: t}).fixture("package fixture\nfunc Root() {}\n")
	dir := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "child.go"), []byte("package child\nfunc Child() {}\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".columbo.yml"), []byte("invalid: true\n"), 0600))
	config := []byte("thresholds:\n  function-lines: 11\nhistory:\n  enabled: false\n")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: ".columbo.yml"},
		{name: "settings.yml", args: []string{"--config", "settings.yml"}},
		{name: "absolute.yml", args: []string{"--config", filepath.Join(dir, "absolute.yml")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, tc.name), config, 0600))
			path := filepath.Join(t.TempDir(), "report.sqlite")
			args := []string{"--output", path}
			args = append(args, tc.args...)
			var stdout, stderr bytes.Buffer
			require.Zero(t, Run(args, Invocation{Dir: dir, Stdout: &stdout, Stderr: &stderr}), stderr.String())
			snapshot, err := OpenSnapshot(path)
			require.NoError(t, err)
			q := snapshot.Queries()
			scan, err := q.InspectionIdentity(context.Background())
			require.NoError(t, err)
			assert.Equal(t, root, scan.Root)
			require.NotNil(t, scan.ConfigPath)
			assert.Equal(t, "child/"+tc.name, *scan.ConfigPath)
			require.NotNil(t, scan.ConfigSha256)
			assert.Equal(t, inspectionHash(config), *scan.ConfigSha256)
			environment, err := q.InspectionEnvironment(context.Background())
			require.NoError(t, err)
			assert.Contains(t, environment, snapshotdb.InspectionEnvironmentRow{Name: "PWD", Value: dir})
			patterns, err := q.InspectionPatterns(context.Background())
			require.NoError(t, err)
			assert.Equal(t, []snapshotdb.InspectionPatternsRow{{Ordinal: 0, Pattern: "./..."}}, patterns)
			files, err := q.InspectionFiles(context.Background())
			require.NoError(t, err)
			var selected []string
			for _, file := range files {
				if file.SelectedRuntime != 0 {
					selected = append(selected, file.Path)
				}
			}
			assert.Equal(t, []string{"child/child.go"}, selected)
			require.NoError(t, snapshot.Close())
		})
	}
}

func TestDefaultAnalyzeCapturesSuppliedConfigurationWithoutInventingFileProvenance(t *testing.T) {
	root := (&testHarness{T: t}).fixture("package fixture\ntype Owner struct{ Value int }\nfunc (o *Owner) Read() int { return o.Value }\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".columbo.yml"), []byte("invalid: true\n"), 0600))
	cfg := Defaults()
	require.True(t, cfg.Staged)
	cfg.History = false
	cfg.Counts["function-lines"] = 11
	report, err := Analyze(root, nil, cfg)
	require.NoError(t, err)
	require.NotNil(t, report.inspection)
	require.Len(t, report.Stages, 3)
	assert.Empty(t, report.inspection.ConfigFile)
	assert.Empty(t, report.inspection.ConfigSHA)
	assert.Equal(t, cfg, report.inspection.Config)
	inside := filepath.Join(root, "report.sqlite")
	require.ErrorContains(t, WriteSnapshot(inside, report, "test"), "outside source root")
	_, err = os.Stat(inside)
	require.ErrorIs(t, err, os.ErrNotExist)
	path := filepath.Join(t.TempDir(), "report.sqlite")
	require.NoError(t, WriteSnapshot(path, report, "test"))
	snapshot, err := OpenSnapshot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, snapshot.Close()) })
	q := snapshot.Queries()
	scan, err := q.InspectionIdentity(context.Background())
	require.NoError(t, err)
	assert.Nil(t, scan.ConfigPath)
	assert.Nil(t, scan.ConfigSha256)
	counts, err := q.InspectionConfigCounts(context.Background())
	require.NoError(t, err)
	assert.Contains(t, counts, snapshotdb.InspectionConfigCountsRow{Name: "function-lines", Value: 11})
	declarations, err := q.InspectionDeclarations(context.Background())
	require.NoError(t, err)
	require.Len(t, declarations, 1)
	assert.Equal(t, "fixture.(*Owner).Read", declarations[0].Symbol)
}

func TestDefaultCLIHasNoSourceInspectionOffSwitch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--inspect-source=false"}, Invocation{Dir: t.TempDir(), Stdout: &stdout, Stderr: &stderr})
	require.Equal(t, 2, code)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "flag provided but not defined: -inspect-source")
}

func TestDefaultAnalysisOwnsConfigurationAndPackagePatterns(t *testing.T) {
	root := (&testHarness{T: t}).fixture("package fixture\nfunc Excessive(a,b,c,d,e int) {}\n")
	config := Defaults()
	config.History = false
	patterns := []string{"."}
	report, err := Analyze(root, patterns, config)
	require.NoError(t, err)
	require.Len(t, report.Cases, 1)
	assert.Equal(t, "FAIL", report.Cases[0].Verdict)
	config.Counts["parameters"] = 999
	config.Severity["long-parameter-list"] = "off"
	config.Ratios["feature-envy-ratio"] = 99
	config.Exclude[0] = "**/*.go"
	patterns[0] = "./missing"
	path := filepath.Join(t.TempDir(), "report.sqlite")
	require.NoError(t, WriteSnapshot(path, report, "test"))
	snapshot, err := OpenSnapshot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, snapshot.Close()) })
	q := snapshot.Queries()
	counts, err := q.InspectionConfigCounts(context.Background())
	require.NoError(t, err)
	assert.Contains(t, counts, snapshotdb.InspectionConfigCountsRow{Name: "parameters", Value: 4})
	severities, err := q.InspectionConfigSeverity(context.Background())
	require.NoError(t, err)
	assert.Contains(t, severities, snapshotdb.InspectionConfigSeverityRow{Smell: "long-parameter-list", Severity: "fail"})
	ratios, err := q.InspectionConfigRatios(context.Background())
	require.NoError(t, err)
	assert.Contains(t, ratios, snapshotdb.InspectionConfigRatiosRow{Name: "feature-envy-ratio", Value: 2})
	exclusions, err := q.InspectionConfigExclusions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []snapshotdb.InspectionConfigExclusionsRow{
		{Ordinal: 0, Pattern: "**/*_generated.go"},
		{Ordinal: 1, Pattern: "**/vendor/**"},
	}, exclusions)
	selected, err := q.InspectionPatterns(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []snapshotdb.InspectionPatternsRow{{Ordinal: 0, Pattern: "."}}, selected)
	metrics, err := q.SummaryMetrics(context.Background(), snapshotdb.SummaryMetricsParams{CaseID: report.Cases[0].ID})
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	assert.Equal(t, "parameters", metrics[0].Kind)
	assert.EqualValues(t, 5, metrics[0].NumericValue)
	assert.EqualValues(t, 4, metrics[0].LimitValue)
}

func TestDefaultStageOnlyFindingsRemainInspectable(t *testing.T) {
	source := tanglePrelude + strings.Replace(tangleBody, "func update(x *Item)", "func (x *Item) update()", 1)
	root := (&testHarness{T: t}).fixture(source)
	config := Defaults()
	config.History = false
	report, err := Analyze(root, nil, config)
	require.NoError(t, err)
	require.Empty(t, report.Cases)
	require.Equal(t, "active", report.Stages[0].state)
	path := filepath.Join(t.TempDir(), "report.sqlite")
	require.NoError(t, WriteSnapshot(path, report, "test"))
	snapshot, err := OpenSnapshot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, snapshot.Close()) })
	q := snapshot.Queries()
	seeds, err := q.InspectionReceiverSeeds(context.Background())
	require.NoError(t, err)
	assert.Empty(t, seeds)
	declarations, err := q.InspectionDeclarations(context.Background())
	require.NoError(t, err)
	require.Len(t, declarations, 1)
	assert.Equal(t, "fixture.(*Item).update", declarations[0].Symbol)
	assert.Equal(t, "method", declarations[0].Kind)
	output, code, err := RenderSnapshot(q, path)
	require.NoError(t, err)
	assert.Equal(t, 1, code)
	assert.Contains(t, string(output), "unsuppressed ordinary non-prose method-linked cases")
	assert.Contains(t, string(output), "For stage-only findings, use advisory_receipts with source_declarations, source_calls and source_fields")
}

func TestDefaultInspectionLoaderRejectsChangedModuleBoundary(t *testing.T) {
	root := (&testHarness{T: t}).fixture("package fixture\n")
	dir := filepath.Join(root, "nested")
	require.NoError(t, os.Mkdir(dir, 0700))
	module := filepath.Join(dir, "go.mod")
	require.NoError(t, os.WriteFile(module, []byte("module nested\n\ngo 1.25.1\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nested.go"), []byte("package nested\nfunc Nested() {}\n"), 0600))
	input, err := validateInspectionInput(dir, "")
	require.NoError(t, err)
	require.NoError(t, input.capture())
	require.NoError(t, os.Remove(module))
	a, err := input.loadEngine([]string{"."})
	require.ErrorContains(t, err, "source module changed during inspection")
	assert.Nil(t, a)
}
