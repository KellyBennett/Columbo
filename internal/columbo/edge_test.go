package columbo

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go/ast"
	"go/token"
	"go/types"
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

// Receipt evidence keeps the bytes and their physical ranges together.
type dependencyReceiptEvidence struct {
	data     []byte
	receipts []Source
}

func receiptEvidence(d *declaration) dependencyReceiptEvidence {
	return dependencyReceiptEvidence{d.file.data, d.depReceipts}
}
func (t *testHarness) measuredDependencyEvidence(source string) dependencyReceiptEvidence {
	t.Helper()
	engine, err := load(t.fixture(source), []string{"./..."}, quiet())
	require.NoError(t.T, err)
	declaration := engine.declarations[0]
	declaration.measure(engine)
	return receiptEvidence(declaration)
}
func (e dependencyReceiptEvidence) designatedTexts(subject string) []string {
	texts := []string{}
	for _, receipt := range e.receipts {
		if receipt.Detail.Subject == subject {
			texts = append(texts, string(e.data[receipt.StartOffset:receipt.EndOffset]))
		}
	}
	return texts
}
func (e dependencyReceiptEvidence) uniqueCount() int {
	unique := map[string]bool{}
	for _, receipt := range e.receipts {
		unique[canonical(receipt)] = true
	}
	return len(unique)
}
func (t *testHarness) TestDependencySiteRanges() {
	evidence := t.measuredDependencyEvidence(edgeSource1)
	texts := evidence.designatedTexts("type:bytes.Buffer")
	for _, want := range []string{"*bytes.Buffer", "bytes.Buffer", "b.Bytes"} {
		assert.Contains(t.T, texts, want)
	}
	require.Len(t.T, evidence.receipts, evidence.uniqueCount(), "duplicate dependency receipt")
}
func TestDependencySiteRanges(t *testing.T) { (&testHarness{T: t}).TestDependencySiteRanges() }

// Complexity sites retain the physical operator and its measured contribution.
type complexitySite struct {
	file, text     string
	value, nesting any
}

func complexitySites(c Case, data []byte) []complexitySite {
	sites := []complexitySite{}
	for _, receipt := range c.Receipts {
		source, ok := receipt.(Source)
		if !ok || source.Detail.Subject != "cognitive-complexity" {
			continue
		}
		sites = append(sites, complexitySite{source.File, string(data[source.StartOffset:source.EndOffset]), source.Detail.Value, source.Detail.Nesting})
	}
	return sites
}

func (t *testHarness) TestComplexityReceipts() {
	dir := t.fixture(edgeSource2)
	c := quiet()
	c.Severity["high-cognitive-complexity"] = "fail"
	c.Counts["cognitive-complexity"] = 1
	cc := t.one(t.investigate(dir, c), "high-cognitive-complexity")
	t.reconcile(cc, "cognitive-complexity")
	assert.Contains(t.T, complexitySites(cc, []byte(edgeSource2)), complexitySite{"source.go", "||", 3, 0})
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
	f := t.expansionFixture("package fixture\nfunc Parent(a bool){helper(a)}\nfunc helper(a bool){if a {Parent(a)}}\n")
	score, _ := f.complexity()
	require.Equal(t.T, 1, score, "cycle cutoff score")
	require.Len(t.T, f.lines().sources, 1)
}
func TestExpansionBoundaries(t *testing.T) { (&testHarness{T: t}).TestExpansionBoundaries() }

func (t *testHarness) TestHistoryWarningGoldens() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	c.History = true
	r := t.investigate(dir, c)
	for _, format := range []string{"text", "json"} {
		t.golden(r, filepath.Join("testdata", "history-"+format+".golden"), format)
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
	f := t.expansionFixture(edgeSource3)
	paths := f.lines().childPaths("inner")
	require.Len(t.T, paths, 1)
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
	m := t.reportDocument(r)
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

func (t *testHarness) TestRepeatedCaseIdentity() {
	a := newEngine("", "", quiet())
	id, raw := identity("long-function", "f.go", "fixture.F", "lines")
	t.require(a.claimIdentity(id, raw) == nil, "initial identity")
	t.require(a.claimIdentity(id, raw) == nil && len(a.identities) == 1, "repeated identity")
}
func TestRepeatedCaseIdentity(t *testing.T) { (&testHarness{T: t}).TestRepeatedCaseIdentity() }

func (t *testHarness) TestConflictingCaseIdentity() {
	a := newEngine("", "", quiet())
	id, raw := identity("long-function", "f.go", "fixture.F", "lines")
	a.identities[id] = "different raw identity"
	err := a.claimIdentity(id, raw)
	t.require(err != nil && a.identities[id] == "different raw identity", "identity collision must fail without replacing the previous entry", err)
}
func TestConflictingCaseIdentity(t *testing.T) { (&testHarness{T: t}).TestConflictingCaseIdentity() }

func (t *testHarness) TestDisabledCaseSkipsIdentity() {
	a := newEngine("", "", quiet())
	got, err := a.newCase(nil, "long-function", "unused")
	t.require(got == nil && err == nil && len(a.identities) == 0, "disabled case must skip declaration and identity work", got, err)
}
func TestDisabledCaseSkipsIdentity(t *testing.T) {
	(&testHarness{T: t}).TestDisabledCaseSkipsIdentity()
}
