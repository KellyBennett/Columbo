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

func TestFixedToolchain(t *testing.T) {
	if runtime.Version() != "go1.25.1" {
		t.Fatalf("acceptance fixtures require go1.25.1, got %s", runtime.Version())
	}
}
func TestMaximalClusters(t *testing.T) {
	src := "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b);three(a,a)}\n" + helpers + "\nfunc three(a,b int){println(a)}\n"
	dir := fixture(t, src)
	c := cosmeticConfig()
	c.Ratios["cosmetic-parameter-overlap"] = .9
	r := investigate(t, dir, c)
	if len(r.Cases) > 0 {
		t.Fatal("searched passing subsequence", r)
	}
}
func TestNamedInterfaceDependencies(t *testing.T) {
	dir := fixture(t, `package fixture
type Named[T any] interface{M(T)}
type Alias = Named[int]
func F(a Alias,b interface{},c interface{M(int)},d Named[string]){}
`)
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	d := a.declarations[0]
	d.measure(a)
	for _, s := range []string{"type:fixture.Named[int]", "type:fixture.Named[string]", "interface:interface{}", "interface:interface{M(int)}"} {
		if !d.deps[s] {
			t.Error("missing", s, d.deps)
		}
	}
	for s := range d.deps {
		if strings.HasPrefix(s, "interface:fixture.Named") {
			t.Fatal(s)
		}
	}
}
func TestDependencySiteRanges(t *testing.T) {
	src := `package fixture
import "bytes"
func F(b *bytes.Buffer) []byte { x := b.Bytes();return append(x,b.Bytes()...) }
`
	dir := fixture(t, src)
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
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
		if !sites[want] {
			t.Error("missing designated range", want, sites)
		}
	}
	seen := map[string]bool{}
	for _, r := range d.depReceipts {
		k := canonical(r)
		if seen[k] {
			t.Fatal("duplicate dependency receipt")
		}
		seen[k] = true
	}
}
func TestComplexityReceipts(t *testing.T) {
	dir := fixture(t, `package fixture
func F(a,b,c,d bool){ if a && b || c && d {} }
`)
	c := quiet()
	c.Severity["high-cognitive-complexity"] = "fail"
	c.Counts["cognitive-complexity"] = 1
	cc := one(t, investigate(t, dir, c), "high-cognitive-complexity")
	reconcile(t, cc, "cognitive-complexity")
	aggregated := false
	for _, rr := range cc.Receipts {
		if r, ok := rr.(Source); ok && r.Detail.Subject == "cognitive-complexity" {
			if r.Detail.Value == 3 && r.Detail.Nesting == 0 && string(read(t, filepath.Join(dir, r.File))[r.StartOffset:r.EndOffset]) == "||" {
				aggregated = true
			}
		}
	}
	if !aggregated {
		t.Fatal(cc.Receipts)
	}
}
func TestCosmeticDistinctBoundaries(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Parent(a,b int){one(a,a);two(b,b)}\n"+helpers)
	c := cosmeticConfig()
	c.Ratios["cosmetic-parameter-overlap"] = .75
	if r := investigate(t, dir, c); len(r.Cases) > 0 {
		t.Fatal(r)
	}
}
func TestExpansionBoundaries(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Parent(a bool){helper(a)}\nfunc helper(a bool){if a {Parent(a)}}\n")
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	d := a.declarations[0]
	n, _ := a.expandedComplexity(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}}, 0)
	if n != 1 {
		t.Fatalf("cycle cutoff score %d", n)
	}
	ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
	if len(ss) != 1 {
		t.Fatal(ss)
	}
}
func TestHistoryWarningGoldens(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	c.History = true
	r := investigate(t, dir, c)
	for _, format := range []string{"text", "json"} {
		b, e := Serialize(r, format)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join("testdata", "history-"+format+".golden")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			os.WriteFile(path, b, 0600)
		}
		if !bytes.Equal(read(t, path), b) {
			t.Fatalf("%s history golden", format)
		}
	}
}
func TestCgoPhysicalSource(t *testing.T) {
	t.Setenv("CGO_ENABLED", "1")
	dir := fixture(t, "package fixture\n/* int answer(void) { return 42; } */\nimport \"C\"\nfunc Answer() int{return int(C.answer())}\n")
	var out, err bytes.Buffer
	code := Run([]string{"--no-history"}, dir, "", &out, &err)
	if code != 2 || out.Len() != 0 || (!strings.Contains(err.String(), "physical-source/type correspondence") && !strings.Contains(err.String(), "package loading failed")) {
		t.Fatalf("cgo correspondence failed nonatomically: %d %s %s", code, out.String(), err.String())
	}
}
func TestInterfaceDispatchNotCandidate(t *testing.T) {
	dir := fixture(t, "package fixture\ntype I interface{work(int)}\ntype Box struct{}\nfunc (Box) work(x int){}\nfunc Parent(i I){i.work(1)}\n")
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range a.declarations {
		if d.fn.Name.Name == "work" && d.candidate {
			t.Fatal("interface dispatch resolved statically")
		}
	}
}
func TestHelperCallsFromTests(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n"+helpers)
	write(t, dir, "source_test.go", "package fixture\nfunc Example(){one(1,2)}\n")
	r := investigate(t, dir, cosmeticConfig())
	if len(r.Cases) != 0 {
		t.Fatal("test caller did not disqualify helper", r)
	}
}
func TestExpansionChildCopies(t *testing.T) {
	dir := fixture(t, `package fixture
func Parent(a int){outer(func()int{return inner(a)}())}
func outer(f int){println(f)}
func inner(a int)int{return a}
`)
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	d := a.declarations[0]
	ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
	paths := map[string]bool{}
	for _, r := range ss {
		if len(r.Detail.Expansion) > 0 && strings.HasSuffix(r.Detail.Expansion[len(r.Detail.Expansion)-1], "inner") {
			paths[canonical(r.Detail.ExpansionSites)] = true
		}
	}
	if len(paths) != 1 {
		t.Fatal(paths)
	}
}
func TestCanonicalTypeMatchesGoFormatting(t *testing.T) {
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, nil, "ignored", types.NewSlice(types.Typ[types.Byte]))), types.NewTuple(types.NewVar(token.NoPos, nil, "ignored", types.Typ[types.Rune])), false)
	want := "func([]uint8) int32"
	if got := canonicalType(sig, nil); got != want {
		t.Fatalf("%s != %s", got, want)
	}
}
func TestClusterSchema(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n"+helpers)
	r := investigate(t, dir, cosmeticConfig())
	b, _ := Serialize(r, "json")
	var m map[string]any
	if e := json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	cc := m["cases"].([]any)[0].(map[string]any)
	if len(cc) != 16 {
		t.Fatal("case field count", len(cc))
	}
	cluster := cc["clusters"].([]any)[0].(map[string]any)
	if len(cluster) != 4 {
		t.Fatal(cluster)
	}
	member := cluster["members"].([]any)[0].(map[string]any)
	if len(member) != 2 {
		t.Fatal(member)
	}
}

var _ ast.Node

func TestLargeClosedClump(t *testing.T) {
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
	dir := fixture(t, src.String())
	c := quiet()
	c.Severity["data-clump"] = "fail"
	cc := one(t, investigate(t, dir, c), "data-clump")
	if clueValue(t, cc, "clump-size") != 32 {
		t.Fatal(cc)
	}
}
