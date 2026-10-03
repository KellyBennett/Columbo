package columbo

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/uudashr/gocognit"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
	t.require(e == nil, e)
}
func quiet() Config {
	c := Defaults()
	c.History = false
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
func (t *testHarness) investigate(dir string, c Config) Report {
	t.Helper()
	r, e := Analyze(dir, []string{"./..."}, c)
	t.require(e == nil, e)
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
	t.requiref(len(found) == 1, "%s: want one case, got %d", smell, len(found))
	return found[0]
}
func (t *testHarness) clueValue(c Case, kind string) any {
	t.Helper()
	for _, q := range c.Clues {
		if q.Kind == kind {
			return q.Value
		}
	}
	t.Fatalf("missing %s", kind)
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
	t.requiref(sum == want, "%s contributions %d != %d", kind, sum, want)
}
func (t *testHarness) TestAllSevenDefaultFail() {
	dir, e := filepath.Abs("testdata/all")
	t.require(e == nil, e)
	c := Defaults()
	c.History = false
	r := t.investigate(dir, c)
	for _, smell := range smells {
		t.verifyDefaultSmell(r, smell)
	}
	for _, format := range []string{"json", "text"} {
		t.golden(r, filepath.Join("testdata", "all", format+".golden"), format)
	}
}
func (t *testHarness) verifyDefaultSmell(r Report, smell string) {
	found := false
	for _, c := range r.Cases {
		if c.Smell == smell && c.Verdict == "FAIL" {
			found = true
			t.reconcileDefaultCase(c)
		}
	}
	t.checkf(found, "missing default FAIL %s", smell)
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
func (t *testHarness) golden(r Report, path, format string) {
	b, e := Serialize(r, format)
	t.require(e == nil, e)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		t.writeBytes(path, b)
	}
	t.check(bytes.Equal(t.read(path), b), format+" golden differs")
}
func (t *testHarness) writeBytes(path string, b []byte) {
	e := os.WriteFile(path, b, 0600)
	t.require(e == nil, e)
}
func TestAllSevenDefaultFail(t *testing.T) { (&testHarness{T: t}).TestAllSevenDefaultFail() }

func (t *testHarness) TestConfigMerge() {
	dir := t.TempDir()
	t.write(dir, "config.yml", "severity:\n  long-function: warn\nthresholds:\n  parameters: 9\nexclude: []\n")
	c, e := LoadConfig(filepath.Join(dir, "config.yml"), true)
	t.require(e == nil, e)
	t.require(c.Severity["long-function"] == "warn" && c.Severity["feature-envy"] == "fail" && c.Counts["parameters"] == 9 && c.Counts["dependencies"] == 5 && len(c.Exclude) == 0, c)
	for _, text := range []string{"", "{}"} {
		t.write(dir, "config.yml", text)
		c, e = LoadConfig(filepath.Join(dir, "config.yml"), true)
		t.requiref(e == nil && reflect.DeepEqual(c, Defaults()), "defaults: %v %v", c, e)
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
			t.require(e != nil, "accepted invalid config")
		})
	}
}
func (t *testHarness) TestConfigRatioEndpoints() {
	for _, n := range []string{"0", "1"} {
		d := t.TempDir()
		t.write(d, "c", "thresholds: {cosmetic-parameter-overlap: "+n+"}")
		_, e := LoadConfig(filepath.Join(d, "c"), true)
		t.require(e == nil, e)
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
		t.require(e == nil, e)
		d := a.declarations[0]
		d.measure(a)
		t.checkf(d.lines == x.want, "%q: %d != %d", x.body, d.lines, x.want)
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
			t.requiref(score == 17, "pinned fixture score: %d", score)
		}
	}
}
func (t *testHarness) complexityDeclaration(source string) *ast.FuncDecl {
	f, err := parser.ParseFile(token.NewFileSet(), "source.go", source, parser.ParseComments)
	t.require(err == nil, err)
	return f.Decls[0].(*ast.FuncDecl)
}
func (t *testHarness) pinnedComplexity(source string) int {
	fn := t.complexityDeclaration(source)
	got := scanComplexity(fn)
	expected := gocognit.ScanComplexity(fn, true)
	t.require(got.complexity == expected.Complexity && len(got.diagnostics) == len(expected.Diagnostics), "pinned visitor mismatch", source)
	t.pinnedDiagnostics(got.diagnostics, expected)
	return got.complexity
}
func (t *testHarness) pinnedDiagnostics(got []diagnostic, expected gocognit.ScanResult) {
	for i, d := range expected.Diagnostics {
		t.complexityDiagnostic(got[i], diagnostic{d.Inc, d.Nesting, d.Text, d.Pos}, i)
	}
}
func (t *testHarness) complexityDiagnostic(got diagnostic, expected diagnostic, index int) {
	t.require(got.Inc == expected.Inc && got.Nesting == expected.Nesting && got.Pos == expected.Pos && got.Text == expected.Text, "pinned diagnostic mismatch", index)
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
	code := Run([]string{"--format=json"}, Invocation{Dir: dir, Version: "", Stdout: &out, Stderr: &err})
	want := 0
	if severity == "fail" {
		want = 1
	}
	t.requiref(code == want, "%s: exit %d (%s)", severity, code, err.String())
	t.validReportJSON(out.Bytes())
}
func (t *testHarness) validReportJSON(data []byte) {
	var report Report
	err := json.Unmarshal(data, &report)
	t.require(err == nil, err)
}
func (t *testHarness) malformedPackageExit(dir string) {
	t.write(dir, "bad.go", "package fixture\nvar x = missing\n")
	var out, err bytes.Buffer
	t.require(Run(nil, Invocation{Dir: dir, Version: "", Stdout: &out, Stderr: &err}) == 2 && out.Len() == 0, "fatal did not leave stdout empty")
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
	t.require(Run([]string{"--version"}, Invocation{Dir: "/no/module", Version: "", Stdout: b, Stderr: &err}) == 2 && b.accepted == 3 && strings.Contains(err.String(), "sink failure"), "write failure not fatal")
}
func TestOutputWriteFailure(t *testing.T) { (&testHarness{T: t}).TestOutputWriteFailure() }

func (t *testHarness) TestCLI() {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"--version=false", "--help"}} {
		var b, e bytes.Buffer
		t.require(Run(args, Invocation{Dir: "/no/module", Version: "test", Stdout: &b, Stderr: &e}) == 0 && b.Len() != 0, args, e.String())
	}
	for _, args := range [][]string{{"--unknown"}, {"--version", "--format=bad"}, {"--no-history=wat"}, {"--config"}} {
		var b, e bytes.Buffer
		t.require(Run(args, Invocation{Dir: "/no/module", Version: "", Stdout: &b, Stderr: &e}) == 2 && b.Len() == 0, args)
	}
}
func TestCLI(t *testing.T) { (&testHarness{T: t}).TestCLI() }

func (t *testHarness) TestSuppressionValidation() {
	for _, directive := range []string{"// columbo:ignore long-parameter-list -- tiny", "// columbo:ignore fake -- enough justification", "// columbo:ignore data-clump -- enough justification", "// columbo:ignore long-parameter-list -- enough justification\n// columbo:ignore long-parameter-list -- duplicate justification", "// columbo:ignore long-parameter-list -- enough justification\n"} {
		src := "package fixture\n" + directive + "\nfunc F(a,b,c,d,e int){}"
		dir := t.fixture(src)
		_, e := Analyze(dir, []string{"./..."}, quiet())
		t.require(e != nil, "accepted invalid directive", directive)
	}
}
func TestSuppressionValidation(t *testing.T) { (&testHarness{T: t}).TestSuppressionValidation() }

func (t *testHarness) TestSuppressionAccounting() {
	dir := t.fixture("package fixture\n// columbo:ignore long-parameter-list -- legacy public API compatibility\n// unrelated comment\nfunc F(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	r := t.investigate(dir, c)
	t.require(r.Summary.Suppressed == 1 && r.Summary.Failed == 0 && t.one(r, "long-parameter-list").Suppressed, r)
	c.Severity["long-parameter-list"] = "off"
	r = t.investigate(dir, c)
	t.require(len(r.Warnings) == 1 && r.Warnings[0].Code == "unused-suppression", r)
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
		t.require((len(r.Cases) == 0) == x.excluded, x, r)
	}
}
func TestGeneratedMarker(t *testing.T) { (&testHarness{T: t}).TestGeneratedMarker() }

func (t *testHarness) TestBlankFunctionIdentities() {
	dir := t.fixture("package fixture\nfunc _(a,b,c,d,e int){}\nfunc _(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	r := t.investigate(dir, c)
	t.require(len(r.Cases) == 2 && r.Cases[0].Symbol == "fixture._#1" && r.Cases[1].Symbol == "fixture._#2" && r.Cases[0].ID != r.Cases[1].ID, r)
}
func TestBlankFunctionIdentities(t *testing.T) { (&testHarness{T: t}).TestBlankFunctionIdentities() }

func (t *testHarness) TestClumpClosedSupport() {
	dir := t.fixture("package fixture\nfunc A(a int,b bool,c string,d float64){}\nfunc B(a int,b bool,c string,d float64){}\nfunc C(a int,b bool,c string,d float64){}\nfunc D(a int,b bool,c string){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	r := t.investigate(dir, c)
	t.requiref(len(r.Cases) == 2, "want both closed multisets, got %d", len(r.Cases))
	t.write(dir, "source.go", "package fixture\nfunc A(a int,b bool,c string,d float64){}\nfunc B(a int,b bool,c string,d float64){}\nfunc C(a int,b bool,c string,d float64){}\n")
	r = t.investigate(dir, c)
	t.require(len(r.Cases) == 1 && t.clueValue(r.Cases[0], "clump-size") == 4, r)
}
func TestClumpClosedSupport(t *testing.T) { (&testHarness{T: t}).TestClumpClosedSupport() }

func (t *testHarness) TestStructuralSignatureNames() {
	dir := t.fixture("package fixture\nfunc A(f func(a int) (b string),g interface{M(a int) string},x int){}\nfunc B(f func(z int) (w string),g interface{M(z int) string},x int){}\nfunc C(f func(int) string,g interface{M(int) string},x int){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	r := t.investigate(dir, c)
	t.require(len(r.Cases) == 1, r)
}
func TestStructuralSignatureNames(t *testing.T) { (&testHarness{T: t}).TestStructuralSignatureNames() }

func (t *testHarness) TestModuleAndTests() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	t.write(dir, "source_test.go", "package fixture\nimport \"testing\"\nfunc TestF(t *testing.T){}\n")
	t.write(dir, "external_test.go", "package fixture_test\nfunc External(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	r := t.investigate(dir, c)
	t.require(len(r.Cases) == 2, r)
	_, e := Analyze(dir, []string{"./missing/..."}, c)
	t.require(e != nil, "unmatched patterns accepted")
	_, e = Analyze(t.TempDir(), []string{"./..."}, c)
	t.require(e != nil, "no module accepted")
}
func TestModuleAndTests(t *testing.T) { (&testHarness{T: t}).TestModuleAndTests() }

func (t *testHarness) TestHistoryFailure() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	c.History = true
	r := t.investigate(dir, c)
	t.require(len(r.Warnings) == 1 && r.Warnings[0].Message == historyWarning, r)
	id := r.Cases[0].ID
	c.History = false
	r = t.investigate(dir, c)
	t.require(len(r.Warnings) == 0 && r.Cases[0].ID == id, r)
}
func TestHistoryFailure(t *testing.T) { (&testHarness{T: t}).TestHistoryFailure() }

func (t *testHarness) TestDogfoodingRelease() {
	t.require(ValidatePublicRelease() != nil, "provisional public release accepted")
}
func TestDogfoodingRelease(t *testing.T) { (&testHarness{T: t}).TestDogfoodingRelease() }

var _ io.Writer = (*brokenSink)(nil)

const acceptanceSource0 = `package fixture
func F(a,b,c bool) { if a && b || c { for a { if b {continue} } } else if b { switch {case c:} } else { select {default:} }; goto label; label: _=func(){ if a {F(a,b,c)} } }
`

func (t *testHarness) require(ok bool, args ...any) {
	t.Helper()
	if !ok {
		t.Fatal(args...)
	}
}
func (t *testHarness) requiref(ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}
func (t *testHarness) check(ok bool, args ...any) {
	t.Helper()
	if !ok {
		t.Error(args...)
	}
}
func (t *testHarness) checkf(ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Errorf(format, args...)
	}
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
	t.require(reflect.DeepEqual(got, sourceTokenBoundaries), "physical token boundaries", got)
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
		t.require(generated([]byte(x.source)) == x.want, "leading generated marker", x)
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
		t.require(err == nil, source, err)
		return
	}
	t.require(err != nil, "expected configuration error", source)
	t.require(err.Error() == message, source, err)
}
func TestConfigDocumentErrors(t *testing.T) { (&testHarness{T: t}).TestConfigDocumentErrors() }
