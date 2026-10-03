package columbo

import (
	"encoding/json"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
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
	for _, x := range []struct {
		name, body string
		want       bool
	}{{"ordinary", "one(a,b);two(a,b)", true}, {"assignments", "one(a,b);x:=1;_ = x;two(a,b)", true}, {"opposite-if", "if a>0 {one(a,b)} else {two(a,b)}", false}, {"loop-boundary", "one(a,b);for a>0 {two(a,b)}", false}, {"return-boundary", "one(a,b);return;two(a,b)", false}, {"go-boundary", "go one(a,b);two(a,b)", false}, {"defer-boundary", "defer one(a,b);two(a,b)", false}, {"literal-cluster-exclusion", "_ = func(){one(a,b);two(a,b)}", false}, {"builtin-exception", "one(a,b);println(a);two(a,b)", true}} {
		t.Run(x.name, func(raw *testing.T) {
			t := &testHarness{T: raw}
			dir := t.fixture("package fixture\nfunc Parent(a,b int){" + x.body + "}\n" + helpers)
			r := t.investigate(dir, cosmeticConfig())
			t.require((len(r.Cases) > 0) == x.want, r)
			if len(r.Cases) > 0 {
				c := t.one(r, "cosmetic-extraction")
				t.reconcile(c, "expanded-lines")
				t.reconcile(c, "expanded-complexity")
			}
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
		t.require(len(c.Clusters) == 1 && c.Clusters[0].Owner == "fixture.middle", c)
		for _, q := range c.Clues {
			if q.Kind == "parent-input-set" {
				for _, s := range q.Value.([]string) {
					t.require(strings.HasPrefix(s, "fixture.middle:"), q)
				}
			}
		}
	}
}
func TestReachableClusterOwnership(t *testing.T) {
	(&testHarness{T: t}).TestReachableClusterOwnership()
}

func (t *testHarness) TestClusterJSONOrder() {
	src := "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b);if a>0{};three(a,b);four(a,b)}\n" + helpers + strings.ReplaceAll(strings.ReplaceAll(helpers, "one", "three"), "two", "four")
	dir := t.fixture(src)
	c := t.one(t.investigate(dir, cosmeticConfig()), "cosmetic-extraction")
	t.require(len(c.Clusters) == 2, c)
	t.require(c.Clusters[0].Members[0].Helper == "fixture.one" && c.Clusters[1].Members[0].Helper == "fixture.three" && c.Clusters[0].Members[1].CallOffset < c.Clusters[1].Members[0].CallOffset, c.Clusters)
}
func TestClusterJSONOrder(t *testing.T) { (&testHarness{T: t}).TestClusterJSONOrder() }

func (t *testHarness) TestExpandedLines() {
	for _, x := range []struct {
		body string
		want int
	}{{"one(a,b)", 3}, {"x:=one(a,b);_ = x", 4}, {"one(a,b);two(a,b)", 6}} {
		h := helpers
		if strings.Contains(x.body, "x:=") {
			h = strings.ReplaceAll(h, "func one(a,b int) {", "func one(a,b int) int {")
			h = strings.ReplaceAll(h, "println(a)\n}\nfunc two", "return a\n}\nfunc two")
		}
		dir := t.fixture("package fixture\nfunc Parent(a,b int){" + x.body + "}\n" + h)
		a, e := load(dir, []string{"./..."}, quiet())
		t.require(e == nil, e)
		for _, d := range a.declarations {
			d.measure(a)
		}
		d := a.declarations[0]
		ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
		t.checkf(len(ss) == x.want, "%s: got %d != %d", x.body, len(ss), x.want)
	}
}
func TestExpandedLines(t *testing.T) { (&testHarness{T: t}).TestExpandedLines() }

func (t *testHarness) TestExpandedComplexity() {
	dir := t.fixture("package fixture\nfunc Parent(a,b bool){if a {if b {helper(a)}}}\nfunc helper(a bool){if a {return}}\n")
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d := a.declarations[0]
	n, rs := a.expandedComplexity(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}}, 0)
	t.requiref(n == 6, "expanded complexity %d != 6", n)
	copied := false
	for _, r := range rs {
		if len(r.Detail.ExpansionSites) > 0 && r.Detail.Value == 3 && r.Detail.Nesting == 2 {
			copied = true
		}
	}
	t.require(copied, rs)
}
func TestExpandedComplexity(t *testing.T) { (&testHarness{T: t}).TestExpandedComplexity() }

func (t *testHarness) TestCosmeticSeverityAndPolicy() {
	dir := t.fixture("package fixture\n// columbo:ignore cosmetic-extraction -- accepted during policy review\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers)
	c := cosmeticConfig()
	c.Severity["cosmetic-extraction"] = "warn"
	r := t.investigate(dir, c)
	cc := t.one(r, "cosmetic-extraction")
	t.require(cc.Verdict == "WARN" && cc.Suppressed && len(cc.PolicyReviews) == 1 && cc.PolicyReviews[0] == policy, cc)
	id := cc.ID
	cc.PolicyReviews = []PolicyReview{}
	got, _ := identity(cc.Smell, cc.File, cc.Symbol, "")
	t.require(got == id, "review changed identity")
	for _, format := range []string{"text", "json"} {
		b, _ := Serialize(r, format)
		path := filepath.Join("testdata", "suppressed-"+format+".golden")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			os.WriteFile(path, b, 0600)
		}
		want, e := os.ReadFile(path)
		t.requiref(e == nil && string(want) == string(b), "suppressed %s golden: %v", format, e)
	}
	c.Severity["cosmetic-extraction"] = "off"
	r = t.investigate(dir, c)
	t.require(len(r.Cases) == 0 && len(r.Warnings) == 1, r)
}
func TestCosmeticSeverityAndPolicy(t *testing.T) {
	(&testHarness{T: t}).TestCosmeticSeverityAndPolicy()
}

func (t *testHarness) TestIdentitiesAndSchema() {
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	r := t.investigate(dir, c)
	id := r.Cases[0].ID
	t.write(dir, "source.go", "package fixture\n\n// shift\nfunc F(a,b,c,d,e int){}\n")
	c.Severity["long-parameter-list"] = "warn"
	r = t.investigate(dir, c)
	t.require(r.Cases[0].ID == id, "identity moved")
	b, e := Serialize(r, "json")
	t.require(e == nil, e)
	var obj map[string]json.RawMessage
	e = json.Unmarshal(b, &obj)
	t.require(e == nil, e)
	t.require(len(obj) == 5, obj)
	t.require(!(strings.Contains(string(b), `"clusters":null`)) && !(strings.Contains(string(b), `"policy_reviews":null`)), string(b))
	id1, _ := identity("x", "a|b", "c", "")
	id2, _ := identity("x", "a", "b|c", "")
	t.require(id1 != id2, "identity delimiter collision")
}
func TestIdentitiesAndSchema(t *testing.T) { (&testHarness{T: t}).TestIdentitiesAndSchema() }

func (t *testHarness) TestFeatureEnvyValuesAndSelectors() {
	dir := t.fixture(helpersSource0)
	c := quiet()
	c.Severity["feature-envy"] = "fail"
	c.Ratios["feature-envy-ratio"] = .5
	r := t.investigate(dir, c)
	cc := t.one(r, "feature-envy")
	counts := []int{}
	for _, q := range cc.Clues {
		if q.Kind == "foreign-accesses" {
			counts = append(counts, q.Value.(int))
		}
	}
	t.require(reflect.DeepEqual(counts, []int{5, 6, 5}), counts)
}
func TestFeatureEnvyValuesAndSelectors(t *testing.T) {
	(&testHarness{T: t}).TestFeatureEnvyValuesAndSelectors()
}

func (t *testHarness) TestDependencyIdentitiesAndExemptions() {
	dir := t.fixture(helpersSource1)
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d := a.declarations[0]
	d.measure(a)
	for _, want := range []string{"type:fixture.Interface", "type:bytes.Buffer", "type:fixture.Private", "type:fixture.Shared"} {
		t.checkf(d.deps[want], "missing %s in %v", want, d.deps)
	}
	t.write(dir, "source.go", strings.ReplaceAll(string(t.read(filepath.Join(dir, "source.go"))), "Private", "private"))
	a, e = load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d = a.declarations[0]
	d.measure(a)
	t.require(!(d.deps["type:fixture.private"]), d.deps)
	t.write(dir, "other.go", "package fixture\nvar P private\n")
	a, e = load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	for _, d := range a.declarations {
		d.measure(a)
		t.require(d.deps["type:fixture.private"], d.deps)
	}
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
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("git unavailable")
	}
	dir := t.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
		b, e := cmd.CombinedOutput()
		if e != nil {
			t.Skipf("git sandbox unavailable: %s", b)
		}
		return strings.TrimSpace(string(b))
	}
	run("init")
	run("add", ".")
	run("commit", "-m", "fixture")
	hash := run("rev-parse", "HEAD")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	c.History = true
	r := t.investigate(dir, c)
	cc := t.one(r, "long-parameter-list")
	found := false
	for _, rr := range cc.Receipts {
		if h, ok := rr.(History); ok {
			t.require(h.Commit == hash && h.CommittedAt == 946684800 && reflect.DeepEqual(h.Files, []string{"source.go"}), h)
			found = true
		}
	}
	t.require(found, "missing provenance", r)
}
func TestHistoryProvenance(t *testing.T) { (&testHarness{T: t}).TestHistoryProvenance() }

func (t *testHarness) TestOverlapComparisonEvidence() {
	dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,a);two(a,b)}\n" + helpers)
	c := cosmeticConfig()
	c.Ratios["cosmetic-parameter-overlap"] = .75
	cc := t.one(t.investigate(dir, c), "cosmetic-extraction")
	for _, q := range cc.Clues {
		if q.Kind == "parameter-overlap" {
			if strings.HasSuffix(q.Subject, ":mean") {
				t.require(q.Limit == .75 && q.Operator == ">=", q)
			} else {
				t.require(q.Limit == nil && q.Operator == nil, q)
			}
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
	for _, x := range []struct {
		kind, smell string
		src         string
		v           int64
	}{{"function-lines", "long-function", "func F(){println(1)\nprintln(2)}", 2}, {"parameters", "long-parameter-list", "func F(a,b int){}", 2}, {"cognitive-complexity", "high-cognitive-complexity", "func F(a bool){if a {if a {}}}", 3}, {"dependencies", "excessive-dependencies", "type A struct{};type B struct{};func F(a A,b B){}", 2}} {
		t.Run(x.kind, func(raw *testing.T) {
			t := &testHarness{T: raw}
			dir := t.fixture("package fixture\n" + x.src + "\n")
			c := quiet()
			c.Severity[x.smell] = "fail"
			c.Counts[x.kind] = x.v
			r := t.investigate(dir, c)
			t.require(len(r.Cases) == 0, r)
			c.Counts[x.kind] = x.v - 1
			r = t.investigate(dir, c)
			t.one(r, x.smell)
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
	dir := t.fixture("package fixture\nfunc Parent(a,b int){outer(inner(a,b),b);two(a,b)}\nfunc outer(a,b int){_=func(){if a>0{println(a)}}}\nfunc inner(a,b int)int{if a>0{return a};return b}\n" + strings.ReplaceAll(helpers, "func one(a,b int)", "func unused(a,b int)"))
	a, e := load(dir, []string{"./..."}, quiet())
	t.require(e == nil, e)
	d := a.declarations[0]
	ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
	seen := map[string]bool{}
	for _, r := range ss {
		if len(r.Detail.Expansion) > 0 {
			k := r.Detail.Expansion[len(r.Detail.Expansion)-1] + ":" + strconv.Itoa(r.StartLine) + ":" + canonical(r.Detail.ExpansionSites)
			t.require(!(seen[k]), "copy path duplicate", r)
			seen[k] = true
		}
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
