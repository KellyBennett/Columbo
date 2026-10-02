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

func TestMain(m *testing.M) {
	os.Setenv("GOFLAGS", "-buildvcs=false")
	os.Setenv("GOWORK", "off")
	os.Setenv("GOOS", "linux")
	os.Setenv("GOARCH", "amd64")
	os.Setenv("CGO_ENABLED", "0")
	os.Exit(m.Run())
}

func fixture(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "go.mod", "module fixture\n\ngo 1.25.1\n")
	write(t, dir, "source.go", src)
	return dir
}
func write(t *testing.T, dir, path, content string) {
	t.Helper()
	if e := os.WriteFile(filepath.Join(dir, path), []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}
func quiet() Config {
	c := Defaults()
	c.History = false
	for _, s := range smells {
		c.Severity[s] = "off"
	}
	return c
}
func investigate(t *testing.T, dir string, c Config) Report {
	t.Helper()
	r, e := Analyze(dir, []string{"./..."}, c)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func one(t *testing.T, r Report, smell string) Case {
	t.Helper()
	found := []Case{}
	for _, c := range r.Cases {
		if c.Smell == smell {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s: want one case, got %d", smell, len(found))
	}
	return found[0]
}
func clueValue(t *testing.T, c Case, kind string) any {
	t.Helper()
	for _, q := range c.Clues {
		if q.Kind == kind {
			return q.Value
		}
	}
	t.Fatalf("missing %s", kind)
	return nil
}
func reconcile(t *testing.T, c Case, kind string) {
	t.Helper()
	sum := 0
	for _, r := range c.Receipts {
		if s, ok := r.(Source); ok && s.Detail.Subject == kind && s.Kind == "metric-contribution" {
			sum += s.Detail.Value.(int)
		}
	}
	want := clueValue(t, c, kind).(int)
	if sum != want {
		t.Fatalf("%s contributions %d != %d", kind, sum, want)
	}
}
func TestAllSevenDefaultFail(t *testing.T) {
	dir, e := filepath.Abs("testdata/all")
	if e != nil {
		t.Fatal(e)
	}
	c := Defaults()
	c.History = false
	r := investigate(t, dir, c)
	for _, s := range smells {
		found := false
		for _, cc := range r.Cases {
			if cc.Smell == s && cc.Verdict == "FAIL" {
				found = true
				if s == "long-function" {
					reconcile(t, cc, "function-lines")
				}
				if s == "high-cognitive-complexity" {
					reconcile(t, cc, "cognitive-complexity")
				}
				if s == "cosmetic-extraction" {
					reconcile(t, cc, "expanded-lines")
					reconcile(t, cc, "expanded-complexity")
				}
			}
		}
		if !found {
			t.Errorf("missing default FAIL %s", s)
		}
	}
	for _, format := range []string{"json", "text"} {
		b, e := Serialize(r, format)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join("testdata", "all", format+".golden")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if e := os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
		}
		want, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(want, b) {
			t.Errorf("%s golden differs", format)
		}
	}
}
func TestConfigMerge(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "config.yml", "severity:\n  long-function: warn\nthresholds:\n  parameters: 9\nexclude: []\n")
	c, e := LoadConfig(filepath.Join(dir, "config.yml"), true)
	if e != nil {
		t.Fatal(e)
	}
	if c.Severity["long-function"] != "warn" || c.Severity["feature-envy"] != "fail" || c.Counts["parameters"] != 9 || c.Counts["dependencies"] != 5 || len(c.Exclude) != 0 {
		t.Fatal(c)
	}
	for _, text := range []string{"", "{}"} {
		write(t, dir, "config.yml", text)
		c, e = LoadConfig(filepath.Join(dir, "config.yml"), true)
		if e != nil || !reflect.DeepEqual(c, Defaults()) {
			t.Fatalf("defaults: %v %v", c, e)
		}
	}
}
func TestConfigInvalid(t *testing.T) {
	for _, s := range []string{"version: 2", "version: 1\nversion: 1", "x: 1", "thresholds: {parameters: null}", "thresholds: {parameters: 3.0}", "thresholds: {parameters: 0}", "thresholds: {cosmetic-min-helpers: 1}", "thresholds: {feature-envy-ratio: 0}", "thresholds: {feature-envy-ratio: .inf}", "thresholds: {cosmetic-parameter-overlap: .nan}", "severity: {long-function: error}", "exclude: ['[']", "{}\n---\n{}", "history: {enabled: yes}", "severity: {<<: {long-function: warn}}", "severity: &s {}\nthresholds: *s", "[]", "null"} {
		t.Run(s, func(t *testing.T) {
			d := t.TempDir()
			write(t, d, "c", s)
			if _, e := LoadConfig(filepath.Join(d, "c"), true); e == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
	for _, n := range []string{"0", "1"} {
		d := t.TempDir()
		write(t, d, "c", "thresholds: {cosmetic-parameter-overlap: "+n+"}")
		if _, e := LoadConfig(filepath.Join(d, "c"), true); e != nil {
			t.Fatal(e)
		}
	}
}
func TestPhysicalLines(t *testing.T) {
	for _, x := range []struct {
		body string
		want int
	}{{"", 0}, {"a:=1;_ = a", 1}, {"\n// comment\n{\n}\n", 0}, {"s := `one\n\nthree`\n_ = s", 4}, {"x := []int{\n1,\n2,\n}\n_ = x", 4}} {
		dir := fixture(t, "package fixture\nfunc F(){"+x.body+"}\n")
		a, e := load(dir, []string{"./..."}, quiet())
		if e != nil {
			t.Fatal(e)
		}
		d := a.declarations[0]
		d.measure(a)
		if d.lines != x.want {
			t.Errorf("%q: %d != %d", x.body, d.lines, x.want)
		}
	}
}
func TestPinnedComplexity(t *testing.T) {
	src := `package fixture
func F(a,b,c bool) { if a && b || c { for a { if b {continue} } } else if b { switch {case c:} } else { select {default:} }; goto label; label: _=func(){ if a {F(a,b,c)} } }
`
	fs := token.NewFileSet()
	f, e := parser.ParseFile(fs, "source.go", src, parser.ParseComments)
	if e != nil {
		t.Fatal(e)
	}
	d := f.Decls[0].(*ast.FuncDecl)
	v := &complexityVisitor{name: d.Name, diagnosticsEnabled: true}
	ast.Walk(v, d)
	expected := gocognit.ScanComplexity(d, true)
	if v.complexity != expected.Complexity || len(v.diagnostics) != len(expected.Diagnostics) {
		t.Fatal("pinned visitor mismatch")
	}
	for i, d := range expected.Diagnostics {
		got := v.diagnostics[i]
		if got.Inc != d.Inc || got.Nesting != d.Nesting || got.Pos != d.Pos || got.Text != d.Text {
			t.Fatal("pinned diagnostic mismatch", i)
		}
	}

	if v.complexity != 17 {
		t.Fatalf("pinned fixture score: %d", v.complexity)
	}
}
func TestSeverityAndExits(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc F(a,b,c,d,e int){}\n")
	for _, severity := range []string{"fail", "warn", "off"} {
		write(t, dir, ".columbo.yml", "history: {enabled: false}\nseverity: {long-parameter-list: "+severity+"}\n")
		var out, err bytes.Buffer
		code := Run([]string{"--format=json"}, dir, "", &out, &err)
		want := 0
		if severity == "fail" {
			want = 1
		}
		if code != want {
			t.Fatalf("%s: exit %d (%s)", severity, code, err.String())
		}
		var doc Report
		if e := json.Unmarshal(out.Bytes(), &doc); e != nil {
			t.Fatal(e)
		}
	}
	write(t, dir, "bad.go", "package fixture\nvar x = missing\n")
	var out, err bytes.Buffer
	if Run(nil, dir, "", &out, &err) != 2 || out.Len() != 0 {
		t.Fatal("fatal did not leave stdout empty")
	}
}

type brokenSink struct{ accepted int }

func (b *brokenSink) Write(p []byte) (int, error) {
	n := 3
	if len(p) < n {
		n = len(p)
	}
	b.accepted += n
	return n, errors.New("sink failure")
}
func TestOutputWriteFailure(t *testing.T) {
	var err bytes.Buffer
	b := &brokenSink{}
	if Run([]string{"--version"}, "/no/module", "", b, &err) != 2 || b.accepted != 3 || !strings.Contains(err.String(), "sink failure") {
		t.Fatal("write failure not fatal")
	}
}
func TestCLI(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"--version=false", "--help"}} {
		var b, e bytes.Buffer
		if Run(args, "/no/module", "test", &b, &e) != 0 || b.Len() == 0 {
			t.Fatal(args, e.String())
		}
	}
	for _, args := range [][]string{{"--unknown"}, {"--version", "--format=bad"}, {"--no-history=wat"}, {"--config"}} {
		var b, e bytes.Buffer
		if Run(args, "/no/module", "", &b, &e) != 2 || b.Len() != 0 {
			t.Fatal(args)
		}
	}
}
func TestSuppressionValidation(t *testing.T) {
	for _, directive := range []string{"// columbo:ignore long-parameter-list -- tiny", "// columbo:ignore fake -- enough justification", "// columbo:ignore data-clump -- enough justification", "// columbo:ignore long-parameter-list -- enough justification\n// columbo:ignore long-parameter-list -- duplicate justification", "// columbo:ignore long-parameter-list -- enough justification\n"} {
		src := "package fixture\n" + directive + "\nfunc F(a,b,c,d,e int){}"
		dir := fixture(t, src)
		if _, e := Analyze(dir, []string{"./..."}, quiet()); e == nil {
			t.Fatal("accepted invalid directive", directive)
		}
	}
}
func TestSuppressionAccounting(t *testing.T) {
	dir := fixture(t, "package fixture\n// columbo:ignore long-parameter-list -- legacy public API compatibility\n// unrelated comment\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	r := investigate(t, dir, c)
	if r.Summary.Suppressed != 1 || r.Summary.Failed != 0 || !one(t, r, "long-parameter-list").Suppressed {
		t.Fatal(r)
	}
	c.Severity["long-parameter-list"] = "off"
	r = investigate(t, dir, c)
	if len(r.Warnings) != 1 || r.Warnings[0].Code != "unused-suppression" {
		t.Fatal(r)
	}
}
func TestGeneratedMarker(t *testing.T) {
	for _, x := range []struct {
		marker   string
		excluded bool
	}{{"// Code generated tool DO NOT EDIT.", true}, {`// Code generated tool DO NOT EDIT\x`, false}} {
		dir := fixture(t, x.marker+"\npackage fixture\nfunc F(a,b,c,d,e int){}\n")
		c := quiet()
		c.Severity["long-parameter-list"] = "fail"
		r := investigate(t, dir, c)
		if (len(r.Cases) == 0) != x.excluded {
			t.Fatal(x, r)
		}
	}
}
func TestBlankFunctionIdentities(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc _(a,b,c,d,e int){}\nfunc _(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	r := investigate(t, dir, c)
	if len(r.Cases) != 2 || r.Cases[0].Symbol != "fixture._#1" || r.Cases[1].Symbol != "fixture._#2" || r.Cases[0].ID == r.Cases[1].ID {
		t.Fatal(r)
	}
}
func TestClumpClosedSupport(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc A(a int,b bool,c string,d float64){}\nfunc B(a int,b bool,c string,d float64){}\nfunc C(a int,b bool,c string,d float64){}\nfunc D(a int,b bool,c string){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	r := investigate(t, dir, c)
	if len(r.Cases) != 2 {
		t.Fatalf("want both closed multisets, got %d", len(r.Cases))
	}
	write(t, dir, "source.go", "package fixture\nfunc A(a int,b bool,c string,d float64){}\nfunc B(a int,b bool,c string,d float64){}\nfunc C(a int,b bool,c string,d float64){}\n")
	r = investigate(t, dir, c)
	if len(r.Cases) != 1 || clueValue(t, r.Cases[0], "clump-size") != 4 {
		t.Fatal(r)
	}
}
func TestStructuralSignatureNames(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc A(f func(a int) (b string),g interface{M(a int) string},x int){}\nfunc B(f func(z int) (w string),g interface{M(z int) string},x int){}\nfunc C(f func(int) string,g interface{M(int) string},x int){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	r := investigate(t, dir, c)
	if len(r.Cases) != 1 {
		t.Fatal(r)
	}
}
func TestModuleAndTests(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc F(a,b,c,d,e int){}\n")
	write(t, dir, "source_test.go", "package fixture\nimport \"testing\"\nfunc TestF(t *testing.T){}\n")
	write(t, dir, "external_test.go", "package fixture_test\nfunc External(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	r := investigate(t, dir, c)
	if len(r.Cases) != 2 {
		t.Fatal(r)
	}
	if _, e := Analyze(dir, []string{"./missing/..."}, c); e == nil {
		t.Fatal("unmatched patterns accepted")
	}
	if _, e := Analyze(t.TempDir(), []string{"./..."}, c); e == nil {
		t.Fatal("no module accepted")
	}
}
func TestHistoryFailure(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	c.History = true
	r := investigate(t, dir, c)
	if len(r.Warnings) != 1 || r.Warnings[0].Message != historyWarning {
		t.Fatal(r)
	}
	id := r.Cases[0].ID
	c.History = false
	r = investigate(t, dir, c)
	if len(r.Warnings) != 0 || r.Cases[0].ID != id {
		t.Fatal(r)
	}
}
func TestDogfoodingRelease(t *testing.T) {
	if ValidatePublicRelease() == nil {
		t.Fatal("provisional public release accepted")
	}
}

var _ io.Writer = (*brokenSink)(nil)
