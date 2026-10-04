package columbo

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const helpers = `
func one(a,b int) {
 println(a)
 println(b)
 println(a)
}
func two(a,b int) {
 println(a)
 println(b)
 println(a)
}
`

func cosmeticConfig() Config {
	c := quiet()
	c.Severity["cosmetic-extraction"] = "fail"
	c.Counts["function-lines"] = 2
	c.Ratios["cosmetic-dependency-overlap"] = 0
	return c
}
func (t *testHarness) TestClusterBoundaries() {
	for _, scenario := range clusterBoundaryCases {
		t.Run(scenario.name, func(raw *testing.T) {
			(&testHarness{T: raw}).checkClusterBoundary(scenario)
		})
	}
}
func TestClusterBoundaries(t *testing.T) { (&testHarness{T: t}).TestClusterBoundaries() }

func (t *testHarness) TestHelperForwarding() {
	for _, x := range []struct {
		args      string
		threshold float64
		want      bool
	}{{"a,a", .5, true}, {"a,a", .5000000001, false}, {"a,b", 1, true}} {
		dir := t.fixture("package fixture\nfunc Parent(a,b int){one(" + x.args + ");two(" + x.args + ")}\n" + helpers)
		c := cosmeticConfig()
		c.Ratios["cosmetic-parameter-overlap"] = x.threshold
		r := t.investigate(dir, c)
		t.require((len(r.Cases) > 0) == x.want, x, r)
	}
}
func TestHelperForwarding(t *testing.T) { (&testHarness{T: t}).TestHelperForwarding() }

func (t *testHarness) TestHelperReferences() {
	for _, extra := range []string{"\nvar value = one\n", "\nfunc Other(){one(1,2)}\n"} {
		dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers + extra)
		r := t.investigate(dir, cosmeticConfig())
		t.require(len(r.Cases) == 0, r)
	}
	dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers)
	t.write(dir, "excluded_generated.go", "package fixture\nfunc Other(){one(1,2)}\n")
	r := t.investigate(dir, cosmeticConfig())
	t.require(len(r.Cases) == 0, "excluded caller did not disqualify", r)
}
func TestHelperReferences(t *testing.T) { (&testHarness{T: t}).TestHelperReferences() }

func (t *testHarness) TestReachableClusterOwnership() {
	dir := t.fixture("package fixture\nfunc Ancestor(a,b int){middle(a,b)}\nfunc middle(x,y int){one(x,y);two(x,y)}\n" + helpers)
	r := t.investigate(dir, cosmeticConfig())
	t.require(len(r.Cases) == 2, r)
	for _, c := range r.Cases {
		t.requireOwnedCluster(c, "fixture.middle")
	}
}
func TestReachableClusterOwnership(t *testing.T) {
	(&testHarness{T: t}).TestReachableClusterOwnership()
}

func (t *testHarness) TestClusterJSONOrder() {
	src := orderedClusterSource()
	dir := t.fixture(src)
	c := t.one(t.investigate(dir, cosmeticConfig()), "cosmetic-extraction")
	t.require(len(c.Clusters) == 2, c)
	t.require(c.Clusters[0].Members[0].Helper == "fixture.one" && c.Clusters[1].Members[0].Helper == "fixture.three" && c.Clusters[0].Members[1].CallOffset < c.Clusters[1].Members[0].CallOffset, c.Clusters)
}
func TestClusterJSONOrder(t *testing.T) { (&testHarness{T: t}).TestClusterJSONOrder() }

type expandedLineCase struct {
	body string
	want int
}

var expandedLineCases = []expandedLineCase{{"one(a,b)", 3}, {"x:=one(a,b);_ = x", 4}, {"one(a,b);two(a,b)", 6}}

func (c expandedLineCase) source() string {
	h := helpers
	if strings.Contains(c.body, "x:=") {
		h = strings.ReplaceAll(h, "func one(a,b int) {", "func one(a,b int) int {")
		h = strings.ReplaceAll(h, "println(a)\n}\nfunc two", "return a\n}\nfunc two")
	}
	return "package fixture\nfunc Parent(a,b int){" + c.body + "}\n" + h
}
func (t *testHarness) TestExpandedLines() {
	for _, c := range expandedLineCases {
		f := t.expansionFixture(c.source())
		assert.Len(t.T, f.lines().sources, c.want, c.body)
	}
}
func TestExpandedLines(t *testing.T) { (&testHarness{T: t}).TestExpandedLines() }

func (t *testHarness) TestExpandedComplexity() {
	f := t.expansionFixture("package fixture\nfunc Parent(a,b bool){if a {if b {helper(a)}}}\nfunc helper(a bool){if a {return}}\n")
	score, evidence := f.complexity()
	require.Equal(t.T, 6, score, "expanded complexity")
	require.True(t.T, evidence.hasCopiedContribution(3, 2), "%+v", evidence.sources)
}
func TestExpandedComplexity(t *testing.T) { (&testHarness{T: t}).TestExpandedComplexity() }

func (t *testHarness) TestCosmeticSeverityAndPolicy() {
	dir := t.fixture("package fixture\n// columbo:ignore cosmetic-extraction -- accepted during policy review\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers)
	c := cosmeticConfig()
	c.Severity["cosmetic-extraction"] = "warn"
	r := t.investigate(dir, c)
	cc := t.one(r, "cosmetic-extraction")
	t.requireCosmeticPolicy(cc)
	for _, format := range []string{"text", "json"} {
		t.golden(r, filepath.Join("testdata", "suppressed-"+format+".golden"), format)
	}
	t.requireDisabledCosmeticSuppression(dir, c)
}
func TestCosmeticSeverityAndPolicy(t *testing.T) {
	(&testHarness{T: t}).TestCosmeticSeverityAndPolicy()
}

func (t *testHarness) TestIdentitiesAndSchema() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	r := t.investigate(dir, c)
	id := r.Cases[0].ID
	t.write(dir, "source.go", "package fixture\n\n// shift\nfunc F(a,b,c,d,e int){}\n")
	c.Severity["long-parameter-list"] = "warn"
	r = t.investigate(dir, c)
	t.require(r.Cases[0].ID == id, "identity moved")
	t.requireReportSchema(r)
	t.requireIdentityDelimiterBoundaries()
}
func TestIdentitiesAndSchema(t *testing.T) { (&testHarness{T: t}).TestIdentitiesAndSchema() }

func (t *testHarness) TestFeatureEnvyValuesAndSelectors() {
	dir := t.fixture(helpersSource0)
	c := quiet()
	c.Severity["feature-envy"] = "fail"
	c.Ratios["feature-envy-ratio"] = .5
	r := t.investigate(dir, c)
	cc := t.one(r, "feature-envy")
	t.require(reflect.DeepEqual(t.foreignAccessCounts(cc), []int{5, 6, 5}), cc)

}
func TestFeatureEnvyValuesAndSelectors(t *testing.T) {
	(&testHarness{T: t}).TestFeatureEnvyValuesAndSelectors()
}

func (t *testHarness) TestDependencyIdentitiesAndExemptions() {
	t.Run("public identities", func(raw *testing.T) { (&testHarness{T: raw}).requirePublicDependencies() })
	t.Run("private exemptions", func(raw *testing.T) { (&testHarness{T: raw}).requirePrivateDependencyExemptions() })
}
func TestDependencyIdentitiesAndExemptions(t *testing.T) {
	(&testHarness{T: t}).TestDependencyIdentitiesAndExemptions()
}

func (t *testHarness) read(path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	t.require(e == nil, e)
	return b
}
func (t *testHarness) TestExclusionsAndErrors() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	t.write(dir, "hidden_generated.go", "package fixture\n// columbo:ignore invalid\nfunc G(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	r := t.investigate(dir, c)
	t.require(len(r.Cases) == 1, r)
	t.write(dir, "hidden_generated.go", "package fixture\nvar X=missing\n")
	_, e := Analyze(dir, []string{"./..."}, c)
	t.require(e != nil, "excluded type error accepted")
}
func TestExclusionsAndErrors(t *testing.T) { (&testHarness{T: t}).TestExclusionsAndErrors() }

func (t *testHarness) TestHistoryProvenance() {
	fixture := t.committedFixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	c.History = true
	finding := t.one(t.investigate(fixture.dir, c), "long-parameter-list")
	t.requireHistory(finding, fixture.run(t, "rev-parse", "HEAD"))
}
func TestHistoryProvenance(t *testing.T) { (&testHarness{T: t}).TestHistoryProvenance() }

func (t *testHarness) TestOverlapComparisonEvidence() {
	dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,a);two(a,b)}\n" + helpers)
	c := cosmeticConfig()
	c.Ratios["cosmetic-parameter-overlap"] = .75
	cc := t.one(t.investigate(dir, c), "cosmetic-extraction")
	for _, q := range cc.Clues {
		if q.Kind == "parameter-overlap" {
			t.requireOverlapComparison(q)
		}
	}
}
func TestOverlapComparisonEvidence(t *testing.T) {
	(&testHarness{T: t}).TestOverlapComparisonEvidence()
}

func (t *testHarness) TestCallableScope() {
	dir := t.fixture("package fixture\nfunc Bodyless(a,b,c,d,e int)\nfunc F(){_ = func(a,b,c,d,e int){};var v interface{M(a,b,c,d,e int)};_=v}\n")
	t.write(dir, "body.s", "// bodyless external declaration fixture\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	cc := t.one(t.investigate(dir, c), "long-parameter-list")
	t.require(cc.Symbol == "fixture.Bodyless", cc)
}
func TestCallableScope(t *testing.T) { (&testHarness{T: t}).TestCallableScope() }

func (t *testHarness) TestClumpNormalization() {
	dir := t.fixture("package fixture\ntype Alias = int\nfunc A[T any](a T,b Alias,c ...byte){}\nfunc B[X any](a X,b int,c []uint8){}\nfunc C[Z any](a Z,b int,c []byte){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	t.one(t.investigate(dir, c), "data-clump")
}
func TestClumpNormalization(t *testing.T) { (&testHarness{T: t}).TestClumpNormalization() }

func (t *testHarness) TestThresholdBoundaries() {
	for _, scenario := range thresholdCases {
		t.Run(scenario.kind, func(raw *testing.T) {
			(&testHarness{T: raw}).checkThresholdBoundary(scenario)
		})
	}
}
func TestThresholdBoundaries(t *testing.T) { (&testHarness{T: t}).TestThresholdBoundaries() }

func (t *testHarness) TestCosmeticMeaningfulButStrict() {
	src := "package fixture\nfunc Checkout(a,b int){authorizePayment(a,b);reserveInventory(a,b)}\n" + strings.ReplaceAll(strings.ReplaceAll(helpers, "one", "authorizePayment"), "two", "reserveInventory")
	dir := t.fixture(src)
	cc := t.one(t.investigate(dir, cosmeticConfig()), "cosmetic-extraction")
	t.require(len(cc.PolicyReviews) == 1, cc)
}
func TestCosmeticMeaningfulButStrict(t *testing.T) {
	(&testHarness{T: t}).TestCosmeticMeaningfulButStrict()
}

func (t *testHarness) TestGenericHelperInterface() {
	for _, x := range []struct {
		arg       string
		candidate bool
	}{{"int", false}, {"string", true}} {
		src := helpersSource2 + x.arg + `],x ` + x.arg + helpersSource3
		dir := t.fixture(src)
		a, e := load(dir, []string{"./..."}, quiet())
		t.require(e == nil, e)
		for _, d := range a.declarations {
			t.require(d.fn.Name.Name != "process" || d.candidate == x.candidate, x, d.candidate)
		}
	}
}
func TestGenericHelperInterface(t *testing.T) { (&testHarness{T: t}).TestGenericHelperInterface() }

func (t *testHarness) TestExpansionCopyEvidence() {
	f := t.expansionFixture("package fixture\nfunc Parent(a,b int){outer(inner(a,b),b);two(a,b)}\nfunc outer(a,b int){_=func(){if a>0{println(a)}}}\nfunc inner(a,b int)int{if a>0{return a};return b}\n" + strings.ReplaceAll(helpers, "func one(a,b int)", "func unused(a,b int)"))
	keys := f.lines().copyKeys()
	require.NotEmpty(t.T, keys, "expected expanded copy provenance")
	seen := map[string]bool{}
	for _, key := range keys {
		require.False(t.T, seen[key], "copy path duplicate: %s", key)
		seen[key] = true
	}
}
func TestExpansionCopyEvidence(t *testing.T) { (&testHarness{T: t}).TestExpansionCopyEvidence() }

var _ ast.Node

const helpersSource0 = `package fixture
type Own struct{N int; Child Foreign}
type Foreign struct{N int}
func (f Foreign) M(){}
func (o Own) F(x Foreign) { x.M();x.M();x.M();x.M();x.M(); y:=x; y.M();y.M();y.M();y.M();y.M(); x=Foreign{};x.M(); _ = o.Child.N+o.Child.N+o.Child.N+o.Child.N+o.Child.N; _=Foreign.M; _=func(){ x.M() } }
`

const helpersSource1 = `package fixture
import "bytes"
type Private struct{}
type Shared struct{}
type Interface interface{M()}
type Alias = Interface
func F(a,b Alias,c *bytes.Buffer,d Private,e Shared) { _=a;_=b;_=c;_=d;_=e }
`

const helpersSource2 = `package fixture
type Known interface{process(int)}
type Box[T any] struct{}
func (b Box[T]) process(x T){}
func Parent(b Box[`

const helpersSource3 = `){b.process(x)}
`

func (t *testHarness) foreignAccessCounts(c Case) []int {
	counts := []int{}
	for _, clue := range c.Clues {
		if clue.Kind == "foreign-accesses" {
			counts = append(counts, clue.Value.(int))
		}
	}
	return counts
}

const stableAccessSource = `package fixture
type Own struct { N int; Child Foreign }
type Foreign struct { N int }
type Numeric int
func (Foreign) M() {}
func (Numeric) M() {}
func get() Foreign { return Foreign{} }
func (o Own) F(p *Foreign, x Foreign, xs []Foreign, n Numeric) {
 _ = (*p).N
 _ = (&x).N
 _ = (x).N
 _ = o.Child.N
 _ = get().N
 _ = xs[0].N
 _ = Foreign.M
 _ = func() { _ = x.N }
 (-n).M()
 n.M()
}
`

func (t *testHarness) TestFeatureEnvyStableAccessPaths() {
	dir := t.fixture(stableAccessSource)
	config := quiet()
	config.Severity["feature-envy"] = "fail"
	config.Counts["feature-envy-foreign-accesses"] = 1
	config.Ratios["feature-envy-ratio"] = .1
	c := t.one(t.investigate(dir, config), "feature-envy")
	t.require(reflect.DeepEqual(t.foreignAccessCounts(c), []int{1, 1, 1, 2}), c)
	t.require(t.clueValue(c, "own-accesses") == 1, c)
}
func TestFeatureEnvyStableAccessPaths(t *testing.T) {
	(&testHarness{T: t}).TestFeatureEnvyStableAccessPaths()
}
