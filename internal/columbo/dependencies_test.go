package columbo

import (
	"go/token"
	"go/types"
	"reflect"
	"strings"
	"testing"
)

type dependencyScenario struct {
	name, source string
	want         []string
	fail         bool
}

var dependencyScenarios = []dependencyScenario{
	{"parser adapter", `package fixture
import("go/ast";"go/parser";"go/token")
func F(fs *token.FileSet,p string,b []byte)(*ast.File,error){return parser.ParseFile(fs,p,b,parser.ParseComments)}`, []string{"type:go/ast.File", "type:go/parser.Mode", "type:go/token.FileSet"}, false},
	{"method constructor", `package fixture
import "go/types"
func F(m *types.Func)*types.Func{return types.NewFunc(m.Pos(),m.Pkg(),m.Name(),m.Type().(*types.Signature))}`, []string{"type:go/token.Pos", "type:go/types.Func", "type:go/types.Package", "type:go/types.Signature", "type:go/types.Type"}, false},
	{"JSON adapter", `package fixture
import("bytes";"encoding/json")
func F(v any)([]byte,error){var b bytes.Buffer;e:=json.NewEncoder(&b);e.SetEscapeHTML(false);if err:=e.Encode(v);err!=nil{return nil,err};return b.Bytes(),nil}`, []string{"type:bytes.Buffer", "type:encoding/json.Encoder", "type:io.Writer"}, false},
	{"package operations", `package fixture
import("fmt";"math";"os";"path/filepath";"strconv";"strings")
func F(){s:=os.Getenv("X");s=strings.TrimSpace(s);n,_:=strconv.Atoi(s);_=filepath.Clean(s);_=fmt.Sprint(n);_=math.Abs(float64(n))}`, []string{"package:fmt", "package:math", "package:os", "package:path/filepath", "package:strconv", "package:strings"}, true},
	{"alias and generic owners", `package fixture
import("bytes";"sync/atomic")
type Buffer = bytes.Buffer
func F(p *atomic.Pointer[Buffer]){_=p.Load()}`, []string{"type:bytes.Buffer", "type:sync/atomic.Pointer[T any]", "type:sync/atomic.Pointer[bytes.Buffer]"}, false},
	{"nonempty interface", `package fixture
func F(s interface{Run()error}){_=s.Run()}`, []string{"interface:interface{Run() error}"}, false},
	{"named interface", `package fixture
type Service interface{Run()error}
func F(s Service){_=s.Run()}`, []string{"type:fixture.Service"}, false},
}

func TestDependencyScoring(t *testing.T) {
	for _, scenario := range dependencyScenarios {
		t.Run(scenario.name, func(t *testing.T) { scenario.verify(&testHarness{T: t}) })
	}
}
func (s dependencyScenario) verify(h *testHarness) {
	dir := h.fixture(s.source)
	config := dependencyConfig()
	engine := h.loadDependencyFixture(dir, config)
	declaration := engine.declarations[0]
	h.dependencyMeasurement(engine, declaration, s.want)
	h.dependencyVerdict(h.investigate(dir, config), s.fail)
	if s.name == "parser adapter" {
		h.dependencyInventory(declaration)
	}
}
func dependencyConfig() Config {
	config := quiet()
	config.Severity["excessive-dependencies"] = "fail"
	return config
}
func (h *testHarness) loadDependencyFixture(dir string, config Config) *engine {
	engine, err := load(dir, []string{"./..."}, config)
	h.require(err == nil, err)
	return engine
}
func (h *testHarness) dependencyMeasurement(engine *engine, declaration *declaration, want []string) {
	declaration.measure(engine)
	h.require(reflect.DeepEqual(sortedSet(declaration.deps), want), declaration.deps)
	h.repeatDependencyMeasurement(engine, declaration)
	h.dependencyReceiptKinds(declaration)
}

type dependencySnapshot struct {
	dependencies []string
	complexity   []Source
}

func captureDependencies(declaration *declaration) dependencySnapshot {
	return dependencySnapshot{sortedSet(declaration.deps), declaration.complexityReceipts}
}
func (h *testHarness) repeatDependencyMeasurement(engine *engine, declaration *declaration) {
	before := captureDependencies(declaration)
	declaration.measure(engine)
	after := captureDependencies(declaration)
	h.require(reflect.DeepEqual(after.dependencies, before.dependencies), "measurement is not repeatable")
	h.require(reflect.DeepEqual(after.complexity, before.complexity), "repeat measurement duplicated complexity receipts")
}
func (h *testHarness) dependencyReceiptKinds(declaration *declaration) {
	for _, receipt := range declaration.depReceipts {
		h.require((receipt.Kind == "dependency") == declaration.deps[receipt.Detail.Subject], receipt)
	}
}
func (h *testHarness) dependencyVerdict(report Report, fail bool) {
	h.require((report.Summary.Failed > 0) == fail, report)
	h.require(report.Summary.Suppressed == 0, report)
}
func (h *testHarness) dependencyInventory(declaration *declaration) {
	inventory := map[string]bool{}
	for _, receipt := range declaration.depReceipts {
		if receipt.Kind == "dependency-inventory" {
			inventory[receipt.Detail.Subject] = true
		}
	}
	h.require(inventory["type:error"] && inventory["interface:interface{}"] && inventory["package:go/parser"], inventory)
}

const cosmeticPlumbingSource = `package fixture
type A struct{}
type B struct{}
func Parent(a,b int){one(a,b);two(a,b)}
func sink(v any)error{return nil}
func one(a,b int){var x A;_=x;_=sink(a);println(b)}
func two(a,b int){var x B;_=x;_=sink(a);println(b)}`

func (h *testHarness) cosmeticPlumbingReport(overlap float64) Report {
	dir := h.fixture(cosmeticPlumbingSource)
	config := cosmeticConfig()
	config.Counts["function-lines"] = 1
	config.Ratios["cosmetic-dependency-overlap"] = overlap
	return h.investigate(dir, config)
}
func (h *testHarness) TestCosmeticDependencyPlumbing() {
	report := h.cosmeticPlumbingReport(.75)
	h.require(len(report.Cases) == 0, "error and any must not create shared collaborator overlap", report)
	c := h.one(h.cosmeticPlumbingReport(0), "cosmetic-extraction")
	h.noScoredPlumbingOverlap(c)
	h.cosmeticPlumbingInventory(c)
}
func (h *testHarness) noScoredPlumbingOverlap(c Case) {
	for _, clue := range c.Clues {
		if clue.Kind == "dependency-overlap" {
			h.require(clue.Value == float64(0), clue)
		}
	}
}
func (h *testHarness) cosmeticPlumbingInventory(c Case) {
	inventory := false
	for _, receipt := range c.Receipts {
		if source, ok := receipt.(Source); ok && source.Kind == "dependency-inventory" && source.Detail.Subject == "type:error" {
			inventory = true
		}
	}
	h.require(inventory, "cosmetic receipts lost plumbing evidence")
}
func TestCosmeticDependencyPlumbing(t *testing.T) {
	(&testHarness{T: t}).TestCosmeticDependencyPlumbing()
}

func (t *testHarness) measuredDependencySets(dir string) []map[string]bool {
	t.Helper()
	engine := t.loadDependencyFixture(dir, quiet())
	sets := []map[string]bool{}
	for _, declaration := range engine.declarations {
		declaration.measure(engine)
		sets = append(sets, declaration.deps)
	}
	return sets
}
func (t *testHarness) requirePublicDependencies() {
	t.Helper()
	dir := t.fixture(helpersSource1)
	deps := t.measuredDependencySets(dir)[0]
	for _, want := range []string{"type:fixture.Interface", "type:bytes.Buffer", "type:fixture.Private", "type:fixture.Shared"} {
		t.checkf(deps[want], "missing %s in %v", want, deps)
	}
}
func (t *testHarness) requirePrivateDependencyExemptions() {
	t.Helper()
	dir := t.fixture(strings.ReplaceAll(helpersSource1, "Private", "private"))
	deps := t.measuredDependencySets(dir)[0]
	t.require(!deps["type:fixture.private"], deps)
	t.write(dir, "other.go", "package fixture\nvar P private\n")
	sets := t.measuredDependencySets(dir)
	t.require(len(sets) > 0, "missing measured declarations")
	for _, deps := range sets {
		t.require(deps["type:fixture.private"], deps)
	}
}

// evaluatedType constructs Go type fixtures using the standard type checker.
func evaluatedType(expression string) (types.Type, error) {
	value, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expression)
	return value.Type, err
}
