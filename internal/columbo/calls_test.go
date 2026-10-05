package columbo

import (
	"go/ast"
	"go/types"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// These expectations cover the analyzer's call boundary, including expressions
// that must remain primitive or dynamic rather than acquiring a declaration.
type staticCallResult struct {
	name      string
	primitive bool
}
type staticCallEvidence struct {
	file  *file
	calls map[string]staticCallResult
}

func (t *testHarness) TestStaticCalleeBoundaries() {
	a, err := load(t.fixture(staticCalleeSource), []string{"./..."}, quiet())
	t.require(err == nil, err)
	evidence := &staticCallEvidence{a.files[0], map[string]staticCallResult{}}
	evidence.scan()
	evidence.check(t)
}
func TestStaticCalleeBoundaries(t *testing.T) { (&testHarness{T: t}).TestStaticCalleeBoundaries() }
func (e *staticCallEvidence) scan()           { ast.Inspect(e.file.ast, e.visit) }
func (e *staticCallEvidence) check(t *testHarness) {
	t.require(reflect.DeepEqual(staticCalleeWant, e.calls), e.calls)
}
func (e *staticCallEvidence) visit(node ast.Node) bool {
	if call, ok := node.(*ast.CallExpr); ok {
		e.record(call)
	}
	return true
}
func (e *staticCallEvidence) record(call *ast.CallExpr) {
	info := e.file.typeInfo()
	e.calls[e.text(call.Fun)] = staticCallResult{staticCallName(info, call), primitiveCall(info, call)}
}
func (e *staticCallEvidence) text(expr ast.Expr) string {
	return string(e.file.data[e.file.tf.Offset(expr.Pos()):e.file.tf.Offset(expr.End())])
}
func staticCallName(info *types.Info, call *ast.CallExpr) string {
	if fn := callObject(info, call); fn != nil {
		return fn.Origin().Name()
	}
	return ""
}

var staticCalleeWant = map[string]staticCallResult{
	"direct":               {"direct", false},
	"((direct))":           {"direct", false},
	"generic[int]":         {"generic", false},
	"((generic[int]))":     {"generic", false},
	"generic":              {"generic", false},
	"pair[int,string]":     {"pair", false},
	"((pair[int,string]))": {"pair", false},
	"b.work":               {"work", false},
	"((b.work))":           {"work", false},
	"Box[int].work":        {"work", false},
	"((Box[int].work))":    {"work", false},
	"embedded.work":        {"work", false},
	"math.Abs":             {"Abs", false},
	"i.work":               {"", false},
	"I.work":               {"", false},
	"alias.work":           {"", false},
	"Alias.work":           {"", false},
	"constrained.work":     {"", false},
	"f":                    {"", false},
	"fs[0]":                {"", false},
	"b.callback":           {"", false},
	"func(){}":             {"", false},
	"int":                  {"", true},
	"((int))":              {"", true},
	"Handler":              {"", true},
	"len":                  {"", true},
	"((len))":              {"", true},
}

const staticCalleeSource = `package fixture
import "math"
func direct(x int){}
func generic[T any](x T){}
func pair[T,U any](x T,y U){}
type I interface{work(int)}
type Alias = I
type Handler func(int)
type Box[T any] struct{callback func(int)}
func(Box[T]) work(x T){}
type Embedded struct{Box[int]}
func via[T interface{work(int)}](constrained T){constrained.work(1)}
func Calls(b Box[int], embedded Embedded, i I, alias Alias, f func(int), fs []func(int)){
 direct(1)
 ((direct))(1)
 generic[int](1)
 ((generic[int]))(1)
 generic(1)
 pair[int,string](1,"x")
 ((pair[int,string]))(1,"x")
 b.work(1)
 ((b.work))(1)
 Box[int].work(b,1)
 ((Box[int].work))(b,1)
 embedded.work(1)
 _ = math.Abs(-1)
 i.work(1)
 I.work(i,1)
 alias.work(1)
 Alias.work(alias,1)
 f(1)
 fs[0](1)
 b.callback(1)
 func(){}()
 _ = int(1)
 _ = ((int))(1)
 _ = Handler(f)
 _ = len(fs)
 _ = ((len))(fs)
}
`

// Test files must not affect a generic helper's direct calls;
// Origin still joins instantiations to the one physical source declaration.
func (t *testHarness) TestGenericParenthesizedHelperJoins() {
	dir := t.fixture(genericHelperSource)
	t.write(dir, "source_test.go", "package fixture\nfunc Example(){}\n")
	a, err := load(dir, []string{"./..."}, quiet())
	t.require(err == nil, err)
	t.requireGenericJoins(a)
	t.requireGenericCluster(t.one(t.investigate(dir, cosmeticConfig()), "cosmetic-extraction"))
}
func TestGenericParenthesizedHelperJoins(t *testing.T) {
	(&testHarness{T: t}).TestGenericParenthesizedHelperJoins()
}
func (t *testHarness) requireGenericJoins(a *engine) {
	t.require(len(a.declarations) == 3, "production declarations", a.declarations)
	t.require(len(a.calls) == 2, "physical calls", a.calls)
	for _, target := range a.calls {
		t.require(target.candidate, target.symbol)
	}
}
func (t *testHarness) requireGenericCluster(c Case) {
	require.Len(t.T, c.Clusters, 1)
	members := c.Clusters[0].Members
	require.Len(t.T, members, 2)
	require.Equal(t.T, "fixture.one", members[0].Helper)
	require.Equal(t.T, "fixture.two", members[1].Helper)
	require.Less(t.T, members[0].CallOffset, members[1].CallOffset)
	t.reconcile(c, "expanded-lines")
	t.reconcile(c, "expanded-complexity")
}

const genericHelperSource = `package fixture
func Parent(a,b int){((one[int]))(((a)),((b)));((two[int,int]))((a),(((b))))}
func one[T any](a,b T){
 println(a)
 println(b)
 println(a)
}
func two[T,U any](a T,b U){
 println(a)
 println(b)
 println(a)
}
`
