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
func TestClusterBoundaries(t *testing.T) {
	for _, x := range []struct {
		name, body string
		want       bool
	}{{"ordinary", "one(a,b);two(a,b)", true}, {"assignments", "one(a,b);x:=1;_ = x;two(a,b)", true}, {"opposite-if", "if a>0 {one(a,b)} else {two(a,b)}", false}, {"loop-boundary", "one(a,b);for a>0 {two(a,b)}", false}, {"return-boundary", "one(a,b);return;two(a,b)", false}, {"go-boundary", "go one(a,b);two(a,b)", false}, {"defer-boundary", "defer one(a,b);two(a,b)", false}, {"literal-cluster-exclusion", "_ = func(){one(a,b);two(a,b)}", false}, {"builtin-exception", "one(a,b);println(a);two(a,b)", true}} {
		t.Run(x.name, func(t *testing.T) {
			dir := fixture(t, "package fixture\nfunc Parent(a,b int){"+x.body+"}\n"+helpers)
			r := investigate(t, dir, cosmeticConfig())
			if (len(r.Cases) > 0) != x.want {
				t.Fatal(r)
			}
			if len(r.Cases) > 0 {
				c := one(t, r, "cosmetic-extraction")
				reconcile(t, c, "expanded-lines")
				reconcile(t, c, "expanded-complexity")
			}
		})
	}
}
func TestHelperForwarding(t *testing.T) {
	for _, x := range []struct {
		args      string
		threshold float64
		want      bool
	}{{"a,a", .5, true}, {"a,a", .5000000001, false}, {"a,b", 1, true}} {
		dir := fixture(t, "package fixture\nfunc Parent(a,b int){one("+x.args+");two("+x.args+")}\n"+helpers)
		c := cosmeticConfig()
		c.Ratios["cosmetic-parameter-overlap"] = x.threshold
		r := investigate(t, dir, c)
		if (len(r.Cases) > 0) != x.want {
			t.Fatal(x, r)
		}
	}
}
func TestHelperReferences(t *testing.T) {
	for _, extra := range []string{"\nvar value = one\n", "\nfunc Other(){one(1,2)}\n"} {
		dir := fixture(t, "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n"+helpers+extra)
		if r := investigate(t, dir, cosmeticConfig()); len(r.Cases) != 0 {
			t.Fatal(r)
		}
	}
	dir := fixture(t, "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n"+helpers)
	write(t, dir, "excluded_generated.go", "package fixture\nfunc Other(){one(1,2)}\n")
	if r := investigate(t, dir, cosmeticConfig()); len(r.Cases) != 0 {
		t.Fatal("excluded caller did not disqualify", r)
	}
}
func TestReachableClusterOwnership(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Ancestor(a,b int){middle(a,b)}\nfunc middle(x,y int){one(x,y);two(x,y)}\n"+helpers)
	r := investigate(t, dir, cosmeticConfig())
	if len(r.Cases) != 2 {
		t.Fatal(r)
	}
	for _, c := range r.Cases {
		if len(c.Clusters) != 1 || c.Clusters[0].Owner != "fixture.middle" {
			t.Fatal(c)
		}
		for _, q := range c.Clues {
			if q.Kind == "parent-input-set" {
				for _, s := range q.Value.([]string) {
					if !strings.HasPrefix(s, "fixture.middle:") {
						t.Fatal(q)
					}
				}
			}
		}
	}
}
func TestClusterJSONOrder(t *testing.T) {
	src := "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b);if a>0{};three(a,b);four(a,b)}\n" + helpers + strings.ReplaceAll(strings.ReplaceAll(helpers, "one", "three"), "two", "four")
	dir := fixture(t, src)
	c := one(t, investigate(t, dir, cosmeticConfig()), "cosmetic-extraction")
	if len(c.Clusters) != 2 {
		t.Fatal(c)
	}
	if c.Clusters[0].Members[0].Helper != "fixture.one" || c.Clusters[1].Members[0].Helper != "fixture.three" || c.Clusters[0].Members[1].CallOffset >= c.Clusters[1].Members[0].CallOffset {
		t.Fatal(c.Clusters)
	}
}
func TestExpandedLines(t *testing.T) {
	for _, x := range []struct {
		body string
		want int
	}{{"one(a,b)", 3}, {"x:=one(a,b);_ = x", 4}, {"one(a,b);two(a,b)", 6}} {
		h := helpers
		if strings.Contains(x.body, "x:=") {
			h = strings.ReplaceAll(h, "func one(a,b int) {", "func one(a,b int) int {")
			h = strings.ReplaceAll(h, "println(a)\n}\nfunc two", "return a\n}\nfunc two")
		}
		dir := fixture(t, "package fixture\nfunc Parent(a,b int){"+x.body+"}\n"+h)
		a, e := load(dir, []string{"./..."}, quiet())
		if e != nil {
			t.Fatal(e)
		}
		for _, d := range a.declarations {
			d.measure(a)
		}
		d := a.declarations[0]
		ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
		if len(ss) != x.want {
			t.Errorf("%s: got %d != %d", x.body, len(ss), x.want)
		}
	}
}
func TestExpandedComplexity(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Parent(a,b bool){if a {if b {helper(a)}}}\nfunc helper(a bool){if a {return}}\n")
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	d := a.declarations[0]
	n, rs := a.expandedComplexity(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}}, 0)
	if n != 6 {
		t.Fatalf("expanded complexity %d != 6", n)
	}
	copied := false
	for _, r := range rs {
		if len(r.Detail.ExpansionSites) > 0 && r.Detail.Value == 3 && r.Detail.Nesting == 2 {
			copied = true
		}
	}
	if !copied {
		t.Fatal(rs)
	}
}
func TestCosmeticSeverityAndPolicy(t *testing.T) {
	dir := fixture(t, "package fixture\n// columbo:ignore cosmetic-extraction -- accepted during policy review\nfunc Parent(a,b int){one(a,b);two(a,b)}\n"+helpers)
	c := cosmeticConfig()
	c.Severity["cosmetic-extraction"] = "warn"
	r := investigate(t, dir, c)
	cc := one(t, r, "cosmetic-extraction")
	if cc.Verdict != "WARN" || !cc.Suppressed || len(cc.PolicyReviews) != 1 || cc.PolicyReviews[0] != policy {
		t.Fatal(cc)
	}
	id := cc.ID
	cc.PolicyReviews = []PolicyReview{}
	got, _ := identity(cc.Smell, cc.File, cc.Symbol, "")
	if got != id {
		t.Fatal("review changed identity")
	}
	for _, format := range []string{"text", "json"} {
		b, _ := Serialize(r, format)
		path := filepath.Join("testdata", "suppressed-"+format+".golden")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			os.WriteFile(path, b, 0600)
		}
		want, e := os.ReadFile(path)
		if e != nil || string(want) != string(b) {
			t.Fatalf("suppressed %s golden: %v", format, e)
		}
	}
	c.Severity["cosmetic-extraction"] = "off"
	r = investigate(t, dir, c)
	if len(r.Cases) != 0 || len(r.Warnings) != 1 {
		t.Fatal(r)
	}
}
func TestIdentitiesAndSchema(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	r := investigate(t, dir, c)
	id := r.Cases[0].ID
	write(t, dir, "source.go", "package fixture\n\n// shift\nfunc F(a,b,c,d,e int){}\n")
	c.Severity["long-parameter-list"] = "warn"
	r = investigate(t, dir, c)
	if r.Cases[0].ID != id {
		t.Fatal("identity moved")
	}
	b, e := Serialize(r, "json")
	if e != nil {
		t.Fatal(e)
	}
	var obj map[string]json.RawMessage
	if e = json.Unmarshal(b, &obj); e != nil {
		t.Fatal(e)
	}
	if len(obj) != 5 {
		t.Fatal(obj)
	}
	if strings.Contains(string(b), `"clusters":null`) || strings.Contains(string(b), `"policy_reviews":null`) {
		t.Fatal(string(b))
	}
	id1, _ := identity("x", "a|b", "c", "")
	id2, _ := identity("x", "a", "b|c", "")
	if id1 == id2 {
		t.Fatal("identity delimiter collision")
	}
}
func TestFeatureEnvyValuesAndSelectors(t *testing.T) {
	dir := fixture(t, `package fixture
type Own struct{N int; Child Foreign}
type Foreign struct{N int}
func (f Foreign) M(){}
func (o Own) F(x Foreign) { x.M();x.M();x.M();x.M();x.M(); y:=x; y.M();y.M();y.M();y.M();y.M(); x=Foreign{};x.M(); _ = o.Child.N+o.Child.N+o.Child.N+o.Child.N+o.Child.N; _=Foreign.M; _=func(){ x.M() } }
`)
	c := quiet()
	c.Severity["feature-envy"] = "fail"
	c.Ratios["feature-envy-ratio"] = .5
	r := investigate(t, dir, c)
	cc := one(t, r, "feature-envy")
	counts := []int{}
	for _, q := range cc.Clues {
		if q.Kind == "foreign-accesses" {
			counts = append(counts, q.Value.(int))
		}
	}
	if !reflect.DeepEqual(counts, []int{5, 6, 5}) {
		t.Fatal(counts)
	}
}
func TestDependencyIdentitiesAndExemptions(t *testing.T) {
	dir := fixture(t, `package fixture
import "bytes"
type Private struct{}
type Shared struct{}
type Interface interface{M()}
type Alias = Interface
func F(a,b Alias,c *bytes.Buffer,d Private,e Shared) { _=a;_=b;_=c;_=d;_=e }
`)
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	d := a.declarations[0]
	d.measure(a)
	for _, want := range []string{"type:fixture.Interface", "package:bytes", "type:bytes.Buffer", "type:fixture.Private", "type:fixture.Shared"} {
		if !d.deps[want] {
			t.Errorf("missing %s in %v", want, d.deps)
		}
	}
	write(t, dir, "source.go", strings.ReplaceAll(string(read(t, filepath.Join(dir, "source.go"))), "Private", "private"))
	a, e = load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	d = a.declarations[0]
	d.measure(a)
	if d.deps["type:fixture.private"] {
		t.Fatal(d.deps)
	}
	write(t, dir, "other.go", "package fixture\nvar P private\n")
	a, e = load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range a.declarations {
		d.measure(a)
		if !d.deps["type:fixture.private"] {
			t.Fatal(d.deps)
		}
	}
}
func read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestExclusionsAndErrors(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc F(a,b,c,d,e int){}\n")
	write(t, dir, "hidden_generated.go", "package fixture\n// columbo:ignore invalid\nfunc G(a,b,c,d,e int){}\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	r := investigate(t, dir, c)
	if len(r.Cases) != 1 {
		t.Fatal(r)
	}
	write(t, dir, "hidden_generated.go", "package fixture\nvar X=missing\n")
	if _, e := Analyze(dir, []string{"./..."}, c); e == nil {
		t.Fatal("excluded type error accepted")
	}
}
func TestHistoryProvenance(t *testing.T) {
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("git unavailable")
	}
	dir := fixture(t, "package fixture\nfunc F(a,b,c,d,e int){}\n")
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
	r := investigate(t, dir, c)
	cc := one(t, r, "long-parameter-list")
	found := false
	for _, rr := range cc.Receipts {
		if h, ok := rr.(History); ok {
			if h.Commit != hash || h.CommittedAt != 946684800 || !reflect.DeepEqual(h.Files, []string{"source.go"}) {
				t.Fatal(h)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing provenance", r)
	}
}
func TestOverlapComparisonEvidence(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Parent(a,b int){one(a,a);two(a,b)}\n"+helpers)
	c := cosmeticConfig()
	c.Ratios["cosmetic-parameter-overlap"] = .75
	cc := one(t, investigate(t, dir, c), "cosmetic-extraction")
	for _, q := range cc.Clues {
		if q.Kind == "parameter-overlap" {
			if strings.HasSuffix(q.Subject, ":mean") {
				if q.Limit != .75 || q.Operator != ">=" {
					t.Fatal(q)
				}
			} else if q.Limit != nil || q.Operator != nil {
				t.Fatal(q)
			}
		}
	}
}
func TestCallableScope(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Bodyless(a,b,c,d,e int)\nfunc F(){_ = func(a,b,c,d,e int){};var v interface{M(a,b,c,d,e int)};_=v}\n")
	write(t, dir, "body.s", "// bodyless external declaration fixture\n")
	c := quiet()
	c.Severity["long-parameter-list"] = "fail"
	cc := one(t, investigate(t, dir, c), "long-parameter-list")
	if cc.Symbol != "fixture.Bodyless" {
		t.Fatal(cc)
	}
}
func TestClumpNormalization(t *testing.T) {
	dir := fixture(t, "package fixture\ntype Alias = int\nfunc A[T any](a T,b Alias,c ...byte){}\nfunc B[X any](a X,b int,c []uint8){}\nfunc C[Z any](a Z,b int,c []byte){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	one(t, investigate(t, dir, c), "data-clump")
}
func TestThresholdBoundaries(t *testing.T) {
	for _, x := range []struct {
		kind, smell string
		src         string
		v           int64
	}{{"function-lines", "long-function", "func F(){println(1)\nprintln(2)}", 2}, {"parameters", "long-parameter-list", "func F(a,b int){}", 2}, {"cognitive-complexity", "high-cognitive-complexity", "func F(a bool){if a {if a {}}}", 3}, {"dependencies", "excessive-dependencies", "type A struct{};type B struct{};func F(a A,b B){}", 2}} {
		t.Run(x.kind, func(t *testing.T) {
			dir := fixture(t, "package fixture\n"+x.src+"\n")
			c := quiet()
			c.Severity[x.smell] = "fail"
			c.Counts[x.kind] = x.v
			r := investigate(t, dir, c)
			if len(r.Cases) != 0 {
				t.Fatal(r)
			}
			c.Counts[x.kind] = x.v - 1
			r = investigate(t, dir, c)
			one(t, r, x.smell)
		})
	}
}
func TestCosmeticMeaningfulButStrict(t *testing.T) {
	src := "package fixture\nfunc Checkout(a,b int){authorizePayment(a,b);reserveInventory(a,b)}\n" + strings.ReplaceAll(strings.ReplaceAll(helpers, "one", "authorizePayment"), "two", "reserveInventory")
	dir := fixture(t, src)
	cc := one(t, investigate(t, dir, cosmeticConfig()), "cosmetic-extraction")
	if len(cc.PolicyReviews) != 1 {
		t.Fatal(cc)
	}
}
func TestGenericHelperInterface(t *testing.T) {
	for _, x := range []struct {
		arg       string
		candidate bool
	}{{"int", false}, {"string", true}} {
		src := `package fixture
type Known interface{process(int)}
type Box[T any] struct{}
func (b Box[T]) process(x T){}
func Parent(b Box[` + x.arg + `],x ` + x.arg + `){b.process(x)}
`
		dir := fixture(t, src)
		a, e := load(dir, []string{"./..."}, quiet())
		if e != nil {
			t.Fatal(e)
		}
		for _, d := range a.declarations {
			if d.fn.Name.Name == "process" && d.candidate != x.candidate {
				t.Fatal(x, d.candidate)
			}
		}
	}
}
func TestExpansionCopyEvidence(t *testing.T) {
	dir := fixture(t, "package fixture\nfunc Parent(a,b int){outer(inner(a,b),b);two(a,b)}\nfunc outer(a,b int){_=func(){if a>0{println(a)}}}\nfunc inner(a,b int)int{if a>0{return a};return b}\n"+strings.ReplaceAll(helpers, "func one(a,b int)", "func unused(a,b int)"))
	a, e := load(dir, []string{"./..."}, quiet())
	if e != nil {
		t.Fatal(e)
	}
	d := a.declarations[0]
	ss := a.expandedLines(d, []*declaration{d}, expansion{[]string{d.symbol}, []Site{}})
	seen := map[string]bool{}
	for _, r := range ss {
		if len(r.Detail.Expansion) > 0 {
			k := r.Detail.Expansion[len(r.Detail.Expansion)-1] + ":" + strconv.Itoa(r.StartLine) + ":" + canonical(r.Detail.ExpansionSites)
			if seen[k] {
				t.Fatal("copy path duplicate", r)
			}
			seen[k] = true
		}
	}
}

var _ ast.Node
