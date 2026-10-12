package columbo

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uudashr/gocognit"
)

type testHarness struct{ *testing.T }

func TestMain(m *testing.M) {
	os.Setenv("GOFLAGS", "-buildvcs=false")
	os.Setenv("GOWORK", "off")
	os.Setenv("GOOS", "linux")
	os.Setenv("GOARCH", "amd64")
	os.Setenv("CGO_ENABLED", "0")
	os.Exit(m.Run())
}

func (t *testHarness) fixture(src string) string {
	t.Helper()
	dir := t.TempDir()
	t.write(dir, "go.mod", "module fixture\n\ngo 1.25.1\n")
	t.write(dir, "source.go", src)
	return dir
}
func (t *testHarness) write(dir, path, content string) {
	t.Helper()
	e := os.WriteFile(filepath.Join(dir, path), []byte(content), 0600)
	require.NoError(t.T, e)
}

// quiet disables ordinary smell severities and staged presentation so focused
// collector fixtures can opt into only the behavior they exercise.
func quiet() Config {
	c := Defaults()
	c.History = false
	c.Staged = false
	for _, s := range smells {
		c.Severity[s] = "off"
	}
	return c
}
func longParameterConfig() Config {
	config := quiet()
	config.Severity["long-parameter-list"] = "fail"
	return config
}

// investigate exercises the lower-level detector pipeline for focused fixtures.
// Public Analyze tests cover source inspection and its additional evidence.
func (t *testHarness) investigate(dir string, c Config) Report {
	t.Helper()
	a, err := load(dir, []string{"./..."}, c)
	require.NoError(t.T, err)
	r, err := a.Analyze()
	require.NoError(t.T, err)
	return r
}
func (t *testHarness) one(r Report, smell string) Case {
	t.Helper()
	found := []Case{}
	for _, c := range r.Cases {
		if c.Smell == smell {
			found = append(found, c)
		}
	}
	require.Len(t.T, found, 1, "%s: want one case", smell)
	return found[0]
}
func (t *testHarness) clueValue(c Case, kind string) any {
	t.Helper()
	for _, q := range c.Clues {
		if q.Kind == kind {
			return q.Value
		}
	}
	require.FailNow(t.T, "missing clue", "kind: %s", kind)
	return nil
}
func (t *testHarness) reconcile(c Case, kind string) {
	t.Helper()
	sum := 0
	for _, r := range c.Receipts {
		if s, ok := r.(Source); ok && s.Detail.Subject == kind && s.Kind == "metric-contribution" {
			sum += s.Detail.Value.(int)
		}
	}
	want := t.clueValue(c, kind).(int)
	require.Equal(t.T, want, sum, "%s contributions", kind)
}
func (t *testHarness) TestAllSevenDefaultFail() {
	dir, e := filepath.Abs("testdata/all")
	require.NoError(t.T, e)
	c := Defaults()
	c.History = false
	// Test the default severity policy in ordinary diagnostic mode.
	c.Staged = false
	r := t.investigate(dir, c)
	for _, smell := range smells {
		if c.Severity[smell] == "fail" {
			t.verifyDefaultSmell(r, smell)
		}
	}
	t.snapshotGoldens(r, filepath.Join("testdata", "all"))
}
func (t *testHarness) verifyDefaultSmell(r Report, smell string) {
	found := false
	for _, c := range r.Cases {
		if c.Smell == smell && c.Verdict == "FAIL" {
			found = true
			t.reconcileDefaultCase(c)
		}
	}
	assert.True(t.T, found, "missing default FAIL %s", smell)
}
func (t *testHarness) reconcileDefaultCase(c Case) {
	switch c.Smell {
	case "long-function":
		t.reconcile(c, "function-lines")
	case "high-cognitive-complexity":
		t.reconcile(c, "cognitive-complexity")
	case "cosmetic-extraction":
		t.reconcile(c, "expanded-lines")
		t.reconcile(c, "expanded-complexity")
	}
}

// snapshotGoldens checks the published relational evidence and its SQL-only
// compact presentation independently. Physical SQLite bytes are not canonical.
func (t *testHarness) snapshotGoldens(r Report, base string) {
	t.Helper()
	db := t.snapshot(r)
	t.goldenBytes(base+"-rows.golden", t.logicalRows(db))
	summary, exitCode, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t.T, err)
	require.Equal(t.T, failureExit(r.Summary.Failed), exitCode, "stored outcome exit")
	require.Equal(t.T, r.Summary.Failed, t.sqlCount(db, `SELECT failed FROM summary`), "stored failure count")
	t.goldenBytes(base+"-summary.golden", summary)
}
func (t *testHarness) snapshot(r Report) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.sqlite")
	err := WriteSnapshot(path, r, "test")
	require.NoError(t.T, err)
	db, err := openTestSnapshot(path)
	require.NoError(t.T, err)
	t.Cleanup(func() { assert.NoError(t.T, db.Close(), "close snapshot") })
	return db
}
func (t *testHarness) goldenBytes(path string, data []byte) {
	t.Helper()
	g, name := goldenFile(t.T, path)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		err := g.Update(t.T, name, data)
		require.NoError(t.T, err)
	}
	g.Assert(t.T, name, data)
}

// logicalRows pins every stable column, null, typed value and ordinal. Tables
// and views are discovered through SQLite, and rows use a total explicit order.
func (t *testHarness) logicalRows(db *sql.DB) []byte {
	t.Helper()
	names := t.sqlStrings(db, `SELECT name FROM sqlite_schema WHERE type IN ('table','view') AND name NOT GLOB 'sqlite_*' ORDER BY name`)
	var output strings.Builder
	for _, name := range names {
		table := logicalTable{test: t, db: db, name: name}
		table.writeTo(&output)
	}
	return []byte(strings.TrimRight(output.String(), "\n") + "\n")
}

// logicalTable owns discovery, deterministic row order and cell presentation.
type logicalTable struct {
	test *testHarness
	db   *sql.DB
	name string
}

func (table logicalTable) quotedName() string {
	return `"` + strings.ReplaceAll(table.name, `"`, `""`) + `"`
}
func (table logicalTable) columns() []string {
	rows, err := table.db.Query("SELECT * FROM " + table.quotedName() + " LIMIT 0")
	require.NoError(table.test.T, err)
	defer rows.Close()
	columns, err := rows.Columns()
	require.NoError(table.test.T, err)
	return columns
}
func (table logicalTable) orderedRows(columns []string) *sql.Rows {
	order := make([]string, len(columns))
	for i := range order {
		order[i] = strconv.Itoa(i + 1)
	}
	rows, err := table.db.Query("SELECT * FROM " + table.quotedName() + " ORDER BY " + strings.Join(order, ","))
	require.NoError(table.test.T, err)
	return rows
}
func (table logicalTable) writeTo(output *strings.Builder) {
	columns := table.columns()
	rows := table.orderedRows(columns)
	defer rows.Close()
	fmt.Fprintf(output, "[%s] %s\n", table.name, strings.Join(columns, " | "))
	for rows.Next() {
		output.WriteString(table.formattedRow(rows, len(columns)) + "\n")
	}
	require.NoError(table.test.T, rows.Err())
	output.WriteByte('\n')
}
func (table logicalTable) formattedRow(rows *sql.Rows, count int) string {
	values := make([]any, count)
	pointers := make([]any, count)
	for i := range values {
		pointers[i] = &values[i]
	}
	require.NoError(table.test.T, rows.Scan(pointers...))
	parts := make([]string, count)
	for i, value := range values {
		parts[i] = logicalValue(value)
	}
	return strings.Join(parts, " | ")
}
func failureExit(failed int) int {
	if failed > 0 {
		return 1
	}
	return 0
}
func logicalValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return strconv.Quote(v)
	case []byte:
		return fmt.Sprintf("x'%x'", v)
	default:
		return logicalNumber(value)
	}
}
func logicalNumber(value any) string {
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'g', -1, 64)
	}
	return fmt.Sprint(value)
}
func goldenFile(t testing.TB, path string) (*goldie.Goldie, string) {
	return goldie.New(t, goldie.WithFixtureDir(filepath.Dir(path))), strings.TrimSuffix(filepath.Base(path), ".golden")
}
func TestAllSevenDefaultFail(t *testing.T) { (&testHarness{T: t}).TestAllSevenDefaultFail() }

func (t *testHarness) TestConfigMerge() {
	dir := t.TempDir()
	t.write(dir, "config.yml", "severity:\n  long-function: warn\nthresholds:\n  parameters: 9\nexclude: []\n")
	c, e := LoadConfig(filepath.Join(dir, "config.yml"), true)
	require.NoError(t.T, e)
	require.Equal(t.T, "warn", c.Severity["long-function"])
	require.Equal(t.T, "fail", c.Severity["feature-envy"])
	require.Equal(t.T, int64(9), c.Counts["parameters"])
	require.Equal(t.T, int64(5), c.Counts["dependencies"])
	require.Empty(t.T, c.Exclude)
	for _, text := range []string{"", "{}"} {
		t.write(dir, "config.yml", text)
		c, e = LoadConfig(filepath.Join(dir, "config.yml"), true)
		require.NoError(t.T, e)
		require.Equal(t.T, Defaults(), c, "defaults")
	}
}
func TestConfigMerge(t *testing.T) { (&testHarness{T: t}).TestConfigMerge() }

func (t *testHarness) TestConfigInvalid() {
	for _, s := range []string{"version: 2", "version: 1\nversion: 1", "x: 1", "thresholds: {parameters: null}", "thresholds: {parameters: 3.0}", "thresholds: {parameters: 0}", "thresholds: {cosmetic-min-helpers: 1}", "thresholds: {feature-envy-ratio: 0}", "thresholds: {feature-envy-ratio: .inf}", "thresholds: {cosmetic-parameter-overlap: .nan}", "severity: {long-function: error}", "exclude: ['[']", "{}\n---\n{}", "history: {enabled: yes}", "severity: {<<: {long-function: warn}}", "severity: &s {}\nthresholds: *s", "[]", "null"} {
		t.Run(s, func(raw *testing.T) {
			t := &testHarness{T: raw}
			d := t.TempDir()
			t.write(d, "c", s)
			_, e := LoadConfig(filepath.Join(d, "c"), true)
			require.Error(t.T, e, "accepted invalid config")
		})
	}
}
func (t *testHarness) TestConfigRatioEndpoints() {
	for _, n := range []string{"0", "1"} {
		d := t.TempDir()
		t.write(d, "c", "thresholds: {cosmetic-parameter-overlap: "+n+"}")
		_, e := LoadConfig(filepath.Join(d, "c"), true)
		require.NoError(t.T, e)
	}
}
func TestConfigRatioEndpoints(t *testing.T) { (&testHarness{T: t}).TestConfigRatioEndpoints() }
func TestConfigInvalid(t *testing.T)        { (&testHarness{T: t}).TestConfigInvalid() }

func (t *testHarness) TestPhysicalLines() {
	for _, x := range []struct {
		body string
		want int
	}{{"", 0}, {"a:=1;_ = a", 1}, {"\n// comment\n{\n}\n", 0}, {"s := `one\n\nthree`\n_ = s", 4}, {"x := []int{\n1,\n2,\n}\n_ = x", 4}} {
		dir := t.fixture("package fixture\nfunc F(){" + x.body + "}\n")
		a, e := load(dir, []string{"./..."}, quiet())
		require.NoError(t.T, e)
		require.NotEmpty(t.T, a.declarations)
		d := a.declarations[0]
		d.measure(a)
		assert.Equal(t.T, x.want, d.lines, "body: %q", x.body)
	}
}
func TestPhysicalLines(t *testing.T) { (&testHarness{T: t}).TestPhysicalLines() }

// Exercise every specialized visitor and optional header, including expressions
// whose nested function literals expose traversal order and nesting mistakes.
var complexitySources = []string{
	acceptanceSource0,
	`package p; func F() { if x := F(); x { F() } else if a && (b || c) { F() } else { if d { F() } } }`,
	`package p; func F() { switch x := func() int { if a { return 1 }; return 0 }(); x { case 1: if b { F() } }; switch { case a: F() } }`,
	`package p; func F() { switch x := func() any { if a { return nil }; return b }(); y := x.(type) { case int: if b { F() } }; switch x.(type) { default: F() } }`,
	`package p; func F() { select { case x := <-func() chan int { if a { F() }; return c }(): if x > 0 { F() }; default: F() } }`,
	`package p; func F() { Loop: for i := func() int { if a { F() }; return 0 }(); a && b; func() { if c { F() } }() { if d { continue Loop }; break Loop }; for { break }; goto End; End: }`,
	`package p; func F() { for k, v = range func() []int { if a { F() }; return xs }() { if b { F() } }; for range xs { F() } }`,
	`package p; func F() { _ = func() { if a && b || c && d { F() } }; obj.F(); { F := func() {}; F() }; F() }`,
}

func (t *testHarness) TestPinnedComplexity() {
	for i, source := range complexitySources {
		score := t.pinnedComplexity(source)
		if i == 0 {
			require.Equal(t.T, 17, score, "pinned fixture score")
		}
	}
}
func (t *testHarness) complexityDeclaration(source string) *ast.FuncDecl {
	f, err := parser.ParseFile(token.NewFileSet(), "source.go", source, parser.ParseComments)
	require.NoError(t.T, err)
	require.NotEmpty(t.T, f.Decls)
	fn, ok := f.Decls[0].(*ast.FuncDecl)
	require.True(t.T, ok, "first declaration must be a function")
	return fn
}
func (t *testHarness) pinnedComplexity(source string) int {
	fn := t.complexityDeclaration(source)
	got := scanComplexity(fn)
	expected := gocognit.ScanComplexity(fn, true)
	require.Equal(t.T, expected.Complexity, got.complexity, "pinned complexity mismatch: %s", source)
	require.Len(t.T, got.diagnostics, len(expected.Diagnostics), "pinned diagnostics mismatch: %s", source)
	t.pinnedDiagnostics(got.diagnostics, expected)
	return got.complexity
}
func (t *testHarness) pinnedDiagnostics(got []diagnostic, expected gocognit.ScanResult) {
	for i, d := range expected.Diagnostics {
		t.complexityDiagnostic(got[i], diagnostic{d.Inc, d.Nesting, d.Text, d.Pos}, i)
	}
}
func (t *testHarness) complexityDiagnostic(got diagnostic, expected diagnostic, index int) {
	require.Equal(t.T, expected, got, "pinned diagnostic mismatch: %d", index)
}
func TestPinnedComplexity(t *testing.T) { (&testHarness{T: t}).TestPinnedComplexity() }

func (t *testHarness) TestSeverityAndExits() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	for _, severity := range []string{"fail", "warn", "off"} {
		t.severityExit(dir, severity)
	}
	t.malformedPackageExit(dir)
}
func (t *testHarness) severityExit(dir, severity string) {
	t.write(dir, ".columbo.yml", "history: {enabled: false}\nseverity: {long-parameter-list: "+severity+"}\n")
	var out, err bytes.Buffer
	path := filepath.Join(t.TempDir(), "report-"+severity+".sqlite")
	code := Run([]string{"--staged=false", "--output=" + path}, Invocation{Dir: dir, Version: "", Stdout: &out, Stderr: &err})
	want := 0
	if severity == "fail" {
		want = 1
	}
	require.Equal(t.T, want, code, "%s: %s", severity, err.String())
	t.requireSeveritySnapshot(path, out.Bytes(), want)
}
func (t *testHarness) requireSeveritySnapshot(path string, stdout []byte, want int) {
	db, err := openTestSnapshot(path)
	require.NoError(t.T, err)
	defer db.Close()
	summary, exitCode, err := RenderSnapshot(testDatabaseQueries(db), path)
	require.NoError(t.T, err)
	require.Equal(t.T, want, exitCode, "exit did not follow stored outcomes")
	require.Equal(t.T, summary, stdout, "CLI summary disagrees with snapshot")
}
func (t *testHarness) malformedPackageExit(dir string) {
	t.write(dir, "bad.go", "package fixture\nvar x = missing\n")
	var out, err bytes.Buffer
	path := filepath.Join(t.TempDir(), "report.sqlite")
	code := Run([]string{"--output=" + path}, Invocation{Dir: dir, Version: "", Stdout: &out, Stderr: &err})
	require.Equal(t.T, 2, code, "malformed package must be fatal")
	require.Empty(t.T, out.Bytes(), "fatal did not leave stdout empty")
}
func TestSeverityAndExits(t *testing.T) { (&testHarness{T: t}).TestSeverityAndExits() }

type brokenSink struct{ accepted int }

func (b *brokenSink) Write(p []byte) (int, error) {
	n := 3
	if len(p) < n {
		n = len(p)
	}
	b.accepted += n
	return n, errors.New("sink failure")
}
func (t *testHarness) TestOutputWriteFailure() {
	var err bytes.Buffer
	b := &brokenSink{}
	code := Run([]string{"--version"}, Invocation{Dir: "/no/module", Version: "", Stdout: b, Stderr: &err})
	require.Equal(t.T, 2, code, "write failure must be fatal")
	require.Equal(t.T, 3, b.accepted)
	require.Contains(t.T, err.String(), "sink failure")
}
func TestOutputWriteFailure(t *testing.T) { (&testHarness{T: t}).TestOutputWriteFailure() }

func (t *testHarness) TestSuppressionValidation() {
	for _, directive := range []string{"// columbo:ignore long-parameter-list -- tiny", "// columbo:ignore fake -- enough justification", "// columbo:ignore data-clump -- enough justification", "// columbo:ignore long-parameter-list -- enough justification\n// columbo:ignore long-parameter-list -- duplicate justification", "// columbo:ignore long-parameter-list -- enough justification\n"} {
		src := "package fixture\n" + directive + "\nfunc F(a,b,c,d,e int){}"
		dir := t.fixture(src)
		_, e := Analyze(dir, []string{"./..."}, quiet())
		require.Error(t.T, e, "accepted invalid directive: %s", directive)
	}
}
func TestSuppressionValidation(t *testing.T) { (&testHarness{T: t}).TestSuppressionValidation() }

func (t *testHarness) TestSuppressionAccounting() {
	dir := t.fixture("package fixture\n// columbo:ignore long-parameter-list -- legacy public API compatibility\n// unrelated comment\nfunc F(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	r := t.investigate(dir, c)
	require.Equal(t.T, 1, r.Summary.Suppressed)
	require.Zero(t.T, r.Summary.Failed)
	require.True(t.T, t.one(r, "long-parameter-list").Suppressed)
	c.Severity["long-parameter-list"] = "off"
	r = t.investigate(dir, c)
	require.Len(t.T, r.Warnings, 1)
	require.Equal(t.T, "unused-suppression", r.Warnings[0].Code)
}
func TestSuppressionAccounting(t *testing.T) { (&testHarness{T: t}).TestSuppressionAccounting() }

func (t *testHarness) TestGeneratedMarker() {
	for _, x := range []struct {
		marker   string
		excluded bool
	}{{"// Code generated tool DO NOT EDIT.", true}, {`// Code generated tool DO NOT EDIT\x`, false}} {
		dir := t.fixture(x.marker + "\npackage fixture\nfunc F(a,b,c,d,e int){}\n")
		c := quiet()
		c.Severity["long-parameter-list"] = "fail"
		r := t.investigate(dir, c)
		if x.excluded {
			require.Empty(t.T, r.Cases, "generated marker: %s", x.marker)
		} else {
			require.NotEmpty(t.T, r.Cases, "generated marker: %s", x.marker)
		}
	}
}
func TestGeneratedMarker(t *testing.T) { (&testHarness{T: t}).TestGeneratedMarker() }

func (t *testHarness) TestBlankFunctionIdentities() {
	dir := t.fixture("package fixture\nfunc _(a,b,c,d,e int){}\nfunc _(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	r := t.investigate(dir, c)
	require.Len(t.T, r.Cases, 2)
	require.Equal(t.T, "fixture._#1", r.Cases[0].Symbol)
	require.Equal(t.T, "fixture._#2", r.Cases[1].Symbol)
	require.NotEqual(t.T, r.Cases[0].ID, r.Cases[1].ID)
}
func TestBlankFunctionIdentities(t *testing.T) { (&testHarness{T: t}).TestBlankFunctionIdentities() }

func (t *testHarness) TestClumpClosedSupport() {
	dir := t.fixture("package fixture\nfunc A(a int,b bool,c string,d float64){}\nfunc B(a int,b bool,c string,d float64){}\nfunc C(a int,b bool,c string,d float64){}\nfunc D(a int,b bool,c string){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	r := t.investigate(dir, c)
	require.Len(t.T, r.Cases, 2, "want both closed multisets")
	t.write(dir, "source.go", "package fixture\nfunc A(a int,b bool,c string,d float64){}\nfunc B(a int,b bool,c string,d float64){}\nfunc C(a int,b bool,c string,d float64){}\n")
	r = t.investigate(dir, c)
	require.Len(t.T, r.Cases, 1)
	require.Equal(t.T, 4, t.clueValue(r.Cases[0], "clump-size"))
}
func TestClumpClosedSupport(t *testing.T) { (&testHarness{T: t}).TestClumpClosedSupport() }

func (t *testHarness) TestStructuralSignatureNames() {
	dir := t.fixture("package fixture\nfunc A(f func(a int) (b string),g interface{M(a int) string},x int){}\nfunc B(f func(z int) (w string),g interface{M(z int) string},x int){}\nfunc C(f func(int) string,g interface{M(int) string},x int){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	r := t.investigate(dir, c)
	require.Len(t.T, r.Cases, 1)
}
func TestStructuralSignatureNames(t *testing.T) { (&testHarness{T: t}).TestStructuralSignatureNames() }

func (t *testHarness) TestModuleAndTests() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	t.write(dir, "source_test.go", "package fixture\nimport \"testing\"\nfunc TestF(t *testing.T){}\n")
	t.write(dir, "external_test.go", "package fixture_test\nfunc External(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	r := t.investigate(dir, c)
	require.Len(t.T, r.Cases, 1)
	_, e := Analyze(dir, []string{"./missing/..."}, c)
	require.Error(t.T, e, "unmatched patterns accepted")
	_, e = Analyze(t.TempDir(), []string{"./..."}, c)
	require.Error(t.T, e, "no module accepted")
}
func TestModuleAndTests(t *testing.T) { (&testHarness{T: t}).TestModuleAndTests() }

func (t *testHarness) TestHistoryFailure() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	c.History = true
	r := t.investigate(dir, c)
	require.Len(t.T, r.Warnings, 1)
	require.Equal(t.T, historyWarning, r.Warnings[0].Message)
	require.NotEmpty(t.T, r.Cases)
	id := r.Cases[0].ID
	c.History = false
	r = t.investigate(dir, c)
	require.Empty(t.T, r.Warnings)
	require.NotEmpty(t.T, r.Cases)
	require.Equal(t.T, id, r.Cases[0].ID)
}
func TestHistoryFailure(t *testing.T) { (&testHarness{T: t}).TestHistoryFailure() }

func (t *testHarness) TestDogfoodingRelease() {
	require.Error(t.T, ValidatePublicRelease(), "provisional public release accepted")
}
func TestDogfoodingRelease(t *testing.T) { (&testHarness{T: t}).TestDogfoodingRelease() }

var _ io.Writer = (*brokenSink)(nil)

const acceptanceSource0 = `package fixture
func F(a,b,c bool) { if a && b || c { for a { if b {continue} } } else if b { switch {case c:} } else { select {default:} }; goto label; label: _=func(){ if a {F(a,b,c)} } }
`

// These aliases preserve existing call sites and their fatal/nonfatal behavior.
// Sprint preserves messages whose first argument is a report or an error.
func (t *testHarness) require(ok bool, args ...any) {
	t.Helper()
	require.True(t.T, ok, fmt.Sprint(args...))
}
func (t *testHarness) requiref(ok bool, format string, args ...any) {
	t.Helper()
	require.Truef(t.T, ok, format, args...)
}
func (t *testHarness) check(ok bool, args ...any) {
	t.Helper()
	assert.True(t.T, ok, fmt.Sprint(args...))
}
func (t *testHarness) checkf(ok bool, format string, args ...any) {
	t.Helper()
	assert.Truef(t.T, ok, format, args...)
}

// Positions are byte offsets plus the token.File base (1), including the
// zero-width semicolons Go inserts at newlines and UTF-8 literal bytes.
var sourceTokenBoundaries = []lexToken{
	{1, 8, token.PACKAGE}, {9, 10, token.IDENT}, {10, 10, token.SEMICOLON},
	{11, 18, token.COMMENT}, {19, 22, token.VAR}, {23, 24, token.IDENT},
	{25, 26, token.ASSIGN}, {27, 31, token.STRING}, {31, 31, token.SEMICOLON},
}

func (t *testHarness) TestSourceTokenBoundaries() {
	got := t.sourceTokens("package p\n// note\nvar X = \"é\"\n")
	require.Equal(t.T, sourceTokenBoundaries, got, "physical token boundaries")
}
func (t *testHarness) sourceTokens(source string) []lexToken {
	tf := token.NewFileSet().AddFile("source.go", -1, len(source))
	f := &file{tf: tf, data: []byte(source)}
	f.scan()
	return f.tokens
}
func TestSourceTokenBoundaries(t *testing.T) { (&testHarness{T: t}).TestSourceTokenBoundaries() }

var leadingGeneratedSources = []struct {
	source string
	want   bool
}{
	{"// license\r\n// Code generated tool DO NOT EDIT.\r\npackage p", true},
	{"\ufeff\n// Code generated tool DO NOT EDIT.\npackage p", true},
	{"/* Code generated tool DO NOT EDIT. */\npackage p", false},
	{"// Code generated tool DO NOT EDIT.x\npackage p", false},
	{"package p\n// Code generated tool DO NOT EDIT.\n", false},
	{"// ordinary comment\n", false},
	{"", false},
}

func (t *testHarness) TestLeadingGeneratedSources() {
	for _, x := range leadingGeneratedSources {
		require.Equal(t.T, x.want, generated([]byte(x.source)), "leading generated marker: %s", x.source)
	}
}
func TestLeadingGeneratedSources(t *testing.T) { (&testHarness{T: t}).TestLeadingGeneratedSources() }

var configDocumentCases = []struct{ source, message string }{
	{"", ""},
	{" \n# comment only\n", ""},
	{"{}", ""},
	{"[]", "configuration root must be a mapping"},
	{"null", "configuration root must be a mapping"},
	{"{}\n---\n{}", "configuration must contain exactly one YAML document"},
	{"{}\n---\n{", "configuration must contain exactly one YAML document"},
	{"unknown: 1", "unknown configuration key unknown"},
	{"unknown: 1\nseverity: {long-function: null}", "nulls, aliases, anchors and merge keys are not allowed"},
	{"severity: {long-function: warn, long-function: fail}", "duplicate key long-function"},
}

func (t *testHarness) TestConfigDocumentErrors() {
	for _, x := range configDocumentCases {
		t.configDocumentError(x.source, x.message)
	}
}
func (t *testHarness) configDocumentError(source, message string) {
	_, err := decodeConfig([]byte(source), Defaults())
	if message == "" {
		require.NoError(t.T, err, "source: %s", source)
		return
	}
	require.EqualError(t.T, err, message, "source: %s", source)
}
func TestConfigDocumentErrors(t *testing.T) { (&testHarness{T: t}).TestConfigDocumentErrors() }
