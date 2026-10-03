package columbo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func (t *testHarness) TestFixedToolchain() {
	t.requiref(runtime.Version() == "go1.25.1", "acceptance fixtures require go1.25.1, got %s", runtime.Version())
}
func TestFixedToolchain(t *testing.T) { (&testHarness{T: t}).TestFixedToolchain() }

func (t *testHarness) TestMaximalClusters() {
	src := "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b);three(a,a)}\n" + helpers + "\nfunc three(a,b int){println(a)}\n"
	dir := t.fixture(src)
	c := cosmeticConfig()
	c.Ratios["cosmetic-parameter-overlap"] = .9
	r := t.investigate(dir, c)
	t.require(len(r.Cases) <= 0, "searched passing subsequence", r)
}
func TestMaximalClusters(t *testing.T) { (&testHarness{T: t}).TestMaximalClusters() }

func (t *testHarness) TestNamedInterfaceDependencies() {
	dir := t.fixture(edgeSource0)
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d := a.declarations[0]
	d.measure(a)
	for _, s := range []string{"type:fixture.Named[int]", "type:fixture.Named[string]", "interface:interface{M(int)}"} {
		t.check(d.deps[s], "missing", s, d.deps)
	}
	for s := range d.deps {
		t.require(!(strings.HasPrefix(s, "interface:fixture.Named")), s)
	}
}
func TestNamedInterfaceDependencies(t *testing.T) {
	(&testHarness{T: t}).TestNamedInterfaceDependencies()
}

func (t *testHarness) TestDependencySiteRanges() {
	src := edgeSource1
	dir := t.fixture(src)
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d := a.declarations[0]
	d.measure(a)
	sites := map[string]bool{}
	for _, r := range d.depReceipts {
		if r.Detail.Subject == "type:bytes.Buffer" {
			text := string(d.file.data[r.StartOffset:r.EndOffset])
			sites[text] = true
		}
	}
	for _, want := range []string{"*bytes.Buffer", "bytes.Buffer", "b.Bytes"} {
		t.check(sites[want], "missing designated range", want, sites)
	}
	seen := map[string]bool{}
	for _, r := range d.depReceipts {
		k := canonical(r)
		t.require(!(seen[k]), "duplicate dependency receipt")
		seen[k] = true
	}
}
func TestDependencySiteRanges(t *testing.T) { (&testHarness{T: t}).TestDependencySiteRanges() }

func (t *testHarness) TestComplexityReceipts() {
	dir := t.fixture(edgeSource2)
	c := quiet()
	c.Severity["high-cognitive-complexity"] = "fail"
	c.Counts["cognitive-complexity"] = 1
	cc := t.one(t.investigate(dir, c), "high-cognitive-complexity")
	t.reconcile(cc, "cognitive-complexity")
	aggregated := false
	for _, rr := range cc.Receipts {
		if r, ok := rr.(Source); ok && r.Detail.Subject == "cognitive-complexity" {
			if r.Detail.Value == 3 && r.Detail.Nesting == 0 && string(t.read(filepath.Join(dir, r.File))[r.StartOffset:r.EndOffset]) == "||" {
				aggregated = true
			}
		}
	}
	t.require(aggregated, cc.Receipts)
}
func TestComplexityReceipts(t *testing.T) { (&testHarness{T: t}).TestComplexityReceipts() }

func (t *testHarness) TestCosmeticDistinctBoundaries() {
	dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,a);two(b,b)}\n" + helpers)
	c := cosmeticConfig()
	c.Ratios["cosmetic-parameter-overlap"] = .75
	r := t.investigate(dir, c)
	t.require(len(r.Cases) <= 0, r)
}
func TestCosmeticDistinctBoundaries(t *testing.T) {
	(&testHarness{T: t}).TestCosmeticDistinctBoundaries()
}

func (t *testHarness) TestExpansionBoundaries() {
	dir := t.fixture("package fixture\nfunc Parent(a bool){helper(a)}\nfunc helper(a bool){if a {Parent(a)}}\n")
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d := a.declarations[0]
	n, _ := a.expandedComplexity(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}}, 0)
	t.requiref(n == 1, "cycle cutoff score %d", n)
	ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
	t.require(len(ss) == 1, ss)
}
func TestExpansionBoundaries(t *testing.T) { (&testHarness{T: t}).TestExpansionBoundaries() }

func (t *testHarness) TestHistoryWarningGoldens() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	c.History = true
	r := t.investigate(dir, c)
	for _, format := range []string{"text", "json"} {
		b, e := Serialize(r, format)
		t.require(e == nil, e)
		path := filepath.Join("testdata", "history-"+format+".golden")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			os.WriteFile(path, b, 0600)
		}
		t.requiref(bytes.Equal(t.read(path), b), "%s history golden", format)
	}
}
func TestHistoryWarningGoldens(t *testing.T) { (&testHarness{T: t}).TestHistoryWarningGoldens() }

func (t *testHarness) TestCgoPhysicalSource() {
	t.Setenv("CGO_ENABLED", "1")
	dir := t.fixture("package fixture\n/* int answer(void) { return 42; } */\nimport \"C\"\nfunc Answer() int{return int(C.answer())}\n")
	var out, err bytes.Buffer
	code := Run([]string{"--no-history"}, Invocation{Dir: dir, Version: "", Stdout: &out, Stderr: &err})
	t.requiref(code == 2 && out.Len() == 0 && (strings.Contains(err.String(), "physical-source/type correspondence") || strings.Contains(err.String(), "package loading failed")), "cgo correspondence failed nonatomically: %d %s %s", code, out.String(), err.String())
}
func TestCgoPhysicalSource(t *testing.T) { (&testHarness{T: t}).TestCgoPhysicalSource() }

func (t *testHarness) TestInterfaceDispatchNotCandidate() {
	dir := t.fixture("package fixture\ntype I interface{work(int)}\ntype Box struct{}\nfunc (Box) work(x int){}\nfunc Parent(i I){i.work(1)}\n")
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	for _, d := range a.declarations {
		t.require(d.fn.Name.Name != "work" || !(d.candidate), "interface dispatch resolved statically")
	}
}
func TestInterfaceDispatchNotCandidate(t *testing.T) {
	(&testHarness{T: t}).TestInterfaceDispatchNotCandidate()
}

func (t *testHarness) TestHelperCallsFromTests() {
	dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers)
	t.write(dir, "source_test.go", "package fixture\nfunc Example(){one(1,2)}\n")
	r := t.investigate(dir, cosmeticConfig())
	t.require(len(r.Cases) == 0, "test caller did not disqualify helper", r)
}
func TestHelperCallsFromTests(t *testing.T) { (&testHarness{T: t}).TestHelperCallsFromTests() }

func (t *testHarness) TestExpansionChildCopies() {
	dir := t.fixture(edgeSource3)
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d := a.declarations[0]
	ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
	paths := map[string]bool{}
	for _, r := range ss {
		if len(r.Detail.Expansion) > 0 && strings.HasSuffix(r.Detail.Expansion[len(r.Detail.Expansion)-1], "inner") {
			paths[canonical(r.Detail.ExpansionSites)] = true
		}
	}
	t.require(len(paths) == 1, paths)
}
func TestExpansionChildCopies(t *testing.T) { (&testHarness{T: t}).TestExpansionChildCopies() }

func (t *testHarness) TestCanonicalTypeMatchesGoFormatting() {
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, nil, "ignored", types.NewSlice(types.Typ[types.Byte]))), types.NewTuple(types.NewVar(token.NoPos, nil, "ignored", types.Typ[types.Rune])), false)
	want := "func([]uint8) int32"
	got := canonicalType(sig, nil)
	t.requiref(got == want, "%s != %s", got, want)
}
func TestCanonicalTypeMatchesGoFormatting(t *testing.T) {
	(&testHarness{T: t}).TestCanonicalTypeMatchesGoFormatting()
}

func (t *testHarness) TestClusterSchema() {
	dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers)
	r := t.investigate(dir, cosmeticConfig())
	b, _ := Serialize(r, "json")
	var m map[string]any
	e := json.Unmarshal(b, &m)
	t.require(e == nil, e)
	cc := m["cases"].([]any)[0].(map[string]any)
	t.require(len(cc) == 16, "case field count", len(cc))
	cluster := cc["clusters"].([]any)[0].(map[string]any)
	t.require(len(cluster) == 4, cluster)
	member := cluster["members"].([]any)[0].(map[string]any)
	t.require(len(member) == 2, member)
}
func TestClusterSchema(t *testing.T) { (&testHarness{T: t}).TestClusterSchema() }

var _ ast.Node

func (t *testHarness) TestLargeClosedClump() {
	var src strings.Builder
	src.WriteString("package fixture\n")
	params := []string{}
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("A%02d", i)
		fmt.Fprintf(&src, "type %s struct{}\n", name)
		params = append(params, fmt.Sprintf("p%d %s", i, name))
	}
	for _, name := range []string{"One", "Two", "Three"} {
		fmt.Fprintf(&src, "func %s(%s){}\n", name, strings.Join(params, ","))
	}
	dir := t.fixture(src.String())
	c := quiet()
	c.Severity["data-clump"] = "fail"
	cc := t.one(t.investigate(dir, c), "data-clump")
	t.require(t.clueValue(cc, "clump-size") == 32, cc)
}
func TestLargeClosedClump(t *testing.T) { (&testHarness{T: t}).TestLargeClosedClump() }

const edgeSource0 = `package fixture
type Named[T any] interface{M(T)}
type Alias = Named[int]
func F(a Alias,b interface{},c interface{M(int)},d Named[string]){}
`

const edgeSource1 = `package fixture
import "bytes"
func F(b *bytes.Buffer) []byte { x := b.Bytes();return append(x,b.Bytes()...) }
`

const edgeSource2 = `package fixture
func F(a,b,c,d bool){ if a && b || c && d {} }
`

const edgeSource3 = `package fixture
func Parent(a int){outer(func()int{return inner(a)}())}
func outer(f int){println(f)}
func inner(a int)int{return a}
`
