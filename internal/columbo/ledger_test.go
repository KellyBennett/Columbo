package columbo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"github.com/stretchr/testify/require"
)

var privateFixtureWords = []string{"zebrafinch", "hushhush", "marmalade", "quokka", "wombat", "armadillo", "PRIVATE-JUSTIFICATION", "example.com"}

type ledgerRun struct {
	t    *testing.T
	home string
	ids  []string
	dirs []string
}

func newLedgerRun(t *testing.T) *ledgerRun {
	return &ledgerRun{t: t, home: t.TempDir()}
}

func privateModule(t *testing.T, n int) string {
	dir := t.TempDir()
	write := func(name, text string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644))
	}
	write("go.mod", fmt.Sprintf("module example.com/zebrafinch-%d-hushhush\n\ngo 1.25\n", n))
	var source strings.Builder
	source.WriteString("package marmaladevault\n")
	for i, name := range []string{"A", "B", "C", "D"} {
		if i >= 2 {
			fmt.Fprintf(&source, "\n// columbo:ignore long-function -- PRIVATE-JUSTIFICATION armadillo budget %d%s\n", n, name)
		}
		fmt.Fprintf(&source, "func ComputeWombatPayroll%d%s(a int) int {\n%s\treturn a\n}\n", n, name, strings.Repeat("\ta++\n", 10))
	}
	write("internal/marmaladevault/quokkasecretplans.go", source.String())
	return dir
}

func (r *ledgerRun) analyze(dir string, args ...string) (int, string) {
	output := filepath.Join(r.t.TempDir(), "snapshot.sqlite")
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"--staged=false", "--no-history", "--output", output}, args...), Invocation{Dir: dir, Home: r.home, Stdout: &stdout, Stderr: &stderr})
	r.dirs = append(r.dirs, dir, output)
	if _, err := os.Stat(output); err == nil {
		r.ids = append(r.ids, snapshotCaseIDs(r.t, output)...)
	}
	return code, stderr.String()
}

func snapshotCaseIDs(t *testing.T, path string) []string {
	snapshot, err := OpenSnapshot(path)
	require.NoError(t, err)
	defer snapshot.Close()
	cases, err := snapshot.Queries().SummaryCases(context.Background())
	require.NoError(t, err)
	var ids []string
	for _, c := range cases {
		ids = append(ids, c.ID)
	}
	return ids
}

func (r *ledgerRun) export() (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"calibrate", "--export"}, Invocation{Dir: r.t.TempDir(), Home: r.home, Stdout: &stdout, Stderr: &stderr})
	return code, stdout.String(), stderr.String()
}

func (r *ledgerRun) ledgerFile() string { return filepath.Join(r.home, ".columbo", "ledger.sqlite") }

func TestLedgerIsOptIn(t *testing.T) {
	run := newLedgerRun(t)
	dir := privateModule(t, 1)
	code, _ := run.analyze(dir)
	require.Equal(t, 1, code)
	require.NoDirExists(t, filepath.Join(run.home, ".columbo"), "a run without --ledger writes nothing in the home directory")
	code, stderr := run.analyze(dir, "--ledger")
	require.Equal(t, 1, code)
	require.NotContains(t, stderr, "ledger not recorded")
	require.FileExists(t, run.ledgerFile())
	info, err := os.Stat(run.ledgerFile())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLedgerExportPrivacy(t *testing.T) {
	run := newLedgerRun(t)
	for n := 1; n <= 3; n++ {
		code, stderr := run.analyze(privateModule(t, n), "--ledger")
		require.Equal(t, 1, code, stderr)
	}
	code, document, stderr := run.export()
	require.Equal(t, 0, code, stderr)
	export := decodeExport(t, document)
	require.Contains(t, export.Rules, exportRule{Smell: "long-function", Buckets: []exportBucket{
		{Verdict: "FAIL", Suppressed: false, Excess: "at-limit", Cases: 6, Repos: 3},
		{Verdict: "FAIL", Suppressed: true, Excess: "at-limit", Cases: 6, Repos: 3},
	}}, document)
	for _, forbidden := range slices.Concat(privateFixtureWords, run.ids, run.dirs) {
		require.NotContains(t, strings.ToLower(document), strings.ToLower(forbidden))
	}
	require.NotEmpty(t, run.ids)
	require.NotRegexp(t, regexp.MustCompile(`[0-9a-fA-F]{8,}|[A-Z2-7]{16,}`), document)
	requireClosedVocabulary(t, document)
}

func decodeExport(t *testing.T, document string) ledgerExport {
	decoder := json.NewDecoder(strings.NewReader(document))
	decoder.DisallowUnknownFields()
	var export ledgerExport
	require.NoError(t, decoder.Decode(&export))
	return export
}

func requireClosedVocabulary(t *testing.T, document string) {
	words := slices.Concat(smells, []string{ledgerExportFormat, "FAIL", "WARN", unmeasuredExcess})
	for _, band := range excessBands {
		words = append(words, band.name)
	}
	var value any
	require.NoError(t, json.Unmarshal([]byte(document), &value))
	for _, text := range jsonStrings(value) {
		require.Contains(t, words, text, "every exported string is closed vocabulary")
	}
}

func jsonStrings(value any) []string {
	switch value := value.(type) {
	case string:
		return []string{value}
	case []any:
		var out []string
		for _, item := range value {
			out = append(out, jsonStrings(item)...)
		}
		return out
	case map[string]any:
		var out []string
		for _, item := range value {
			out = append(out, jsonStrings(item)...)
		}
		return out
	}
	return nil
}

func TestLedgerCountsRepeatedRunsOnce(t *testing.T) {
	run := newLedgerRun(t)
	dirs := []string{privateModule(t, 1), privateModule(t, 2), privateModule(t, 3)}
	for _, dir := range dirs {
		run.analyze(dir, "--ledger")
	}
	_, first, _ := run.export()
	for range 5 {
		run.analyze(dirs[0], "--ledger")
	}
	_, repeated, _ := run.export()
	require.Equal(t, first, repeated, "rerunning one repository adds nothing")
	run.analyze(privateModule(t, 4), "--ledger")
	_, widened, _ := run.export()
	require.Contains(t, widened, `"cases": 8,`)
	require.Contains(t, widened, `"repos": 4`)
}

func TestLedgerCountsCheckoutsOfOneModuleOnce(t *testing.T) {
	run := newLedgerRun(t)
	for range 3 {
		run.analyze(privateModule(t, 1), "--ledger")
	}
	_, document, _ := run.export()
	require.Empty(t, decodeExport(t, document).Rules, "three checkouts of one module are one repository")
}

func TestLedgerExportFloor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.sqlite")
	record := func(smell string, repos ...int) {
		for i, repo := range repos {
			shape := snapshotdb.CaseShapesRow{ID: fmt.Sprintf("C-%s-%d", smell, i), Smell: smell, Verdict: "FAIL", Operator: ">", ValueType: "integer", Value: 11, LimitValue: 10}
			require.NoError(t, recordShapes(path, fmt.Sprintf("repo-%d", repo), []snapshotdb.CaseShapesRow{shape}))
		}
	}
	record("long-function", 1, 2, 3, 3)
	record("feature-envy", 1, 2, 3, 3, 3)
	record("data-clump", 1, 1, 2, 2, 2)
	buckets, err := ledgerBuckets(path)
	require.NoError(t, err)
	require.Equal(t, []exportRule{{Smell: "feature-envy", Buckets: []exportBucket{{Verdict: "FAIL", Excess: "at-limit", Cases: 5, Repos: 3}}}}, newLedgerExport(buckets).Rules,
		"4 cases or 2 repositories are dropped; 5 cases in 3 repositories are kept")
	record("long-function", 4)
	record("data-clump", 3)
	buckets, err = ledgerBuckets(path)
	require.NoError(t, err)
	require.Len(t, newLedgerExport(buckets).Rules, 3, "the fifth case and the third repository lift the dropped buckets over the floor")
}

func TestExcessBands(t *testing.T) {
	for _, x := range []struct {
		operator, valueType string
		value, limit        float64
		want                string
	}{
		{">", "integer", 11, 10, "at-limit"},
		{">", "integer", 13, 10, "near"},
		{">", "integer", 14, 10, "over"},
		{">", "integer", 22, 10, "over"},
		{">", "integer", 23, 10, "far"},
		{">=", "integer", 6, 6, "at-limit"},
		{">=", "integer", 7, 6, "near"},
		{">=", "real", 0.6, 0.5, "near"},
		{">", "real", 1.0, 0.5, "over"},
		{">", "integer", 1, 0, "at-limit"},
		{"", "", 0, 0, "unmeasured"},
		{">=", "real", 0.4, 0, "unmeasured"},
	} {
		shape := snapshotdb.CaseShapesRow{Operator: x.operator, ValueType: x.valueType, Value: x.value, LimitValue: x.limit}
		require.Equal(t, x.want, excessName(excessOf(shape)), x)
	}
}

func TestCalibrateCommand(t *testing.T) {
	run := newLedgerRun(t)
	for _, x := range []struct {
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{[]string{"calibrate"}, 2, "", "calibrate requires --export"},
		{[]string{"calibrate", "--help"}, 0, "Usage: columbo calibrate --export", ""},
		{[]string{"calibrate", "--export", "extra"}, 2, "", "calibrate takes no arguments"},
		{[]string{"calibrate", "--export"}, 2, "", "no ledger at " + run.ledgerFile() + "; record one with columbo --ledger"},
		{[]string{"--ledger=wat"}, 2, "", "invalid boolean value"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(x.args, Invocation{Dir: t.TempDir(), Home: run.home, Stdout: &stdout, Stderr: &stderr})
		require.Equal(t, x.code, code, x.args)
		require.Contains(t, stdout.String(), x.stdout, x.args)
		require.Contains(t, stderr.String(), x.stderr, x.args)
	}
}

func TestLedgerFailureKeepsTheVerdict(t *testing.T) {
	run := newLedgerRun(t)
	dir := privateModule(t, 1)
	require.NoError(t, os.MkdirAll(filepath.Dir(run.ledgerFile()), 0o700))
	snapshot := filepath.Join(t.TempDir(), "snapshot.sqlite")
	Run([]string{"--no-history", "--output", snapshot}, Invocation{Dir: dir, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	data, err := os.ReadFile(snapshot)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(run.ledgerFile(), data, 0o600))
	code, stderr := run.analyze(dir, "--ledger")
	require.Equal(t, 1, code, "a ledger failure never changes the exit status")
	require.Contains(t, stderr, "columbo: ledger not recorded: not a Columbo ledger")
	unhomed := &ledgerRun{t: t}
	code, stderr = unhomed.analyze(dir, "--ledger")
	require.Equal(t, 1, code)
	require.Contains(t, stderr, "columbo: ledger not recorded: home directory unknown")
}
