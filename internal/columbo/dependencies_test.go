package columbo

import (
	"reflect"
	"testing"
)

func TestDependencyScoring(t *testing.T) {
	scenarios := []struct {
		name, source string
		want         []string
		fail         bool
	}{
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
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			h := &testHarness{T: t}
			dir := h.fixture(scenario.source)
			c := quiet()
			c.Severity["excessive-dependencies"] = "fail"
			a, e := load(dir, []string{"./..."}, c)
			h.require(e == nil, e)
			d := a.declarations[0]
			d.measure(a)
			h.require(reflect.DeepEqual(sortedSet(d.deps), scenario.want), d.deps)
			before := sortedSet(d.deps)
			complexityEvidence := d.complexityReceipts
			d.measure(a)
			h.require(reflect.DeepEqual(sortedSet(d.deps), before), "measurement is not repeatable")
			h.require(reflect.DeepEqual(d.complexityReceipts, complexityEvidence), "repeat measurement duplicated complexity receipts")
			for _, r := range d.depReceipts {
				h.require((r.Kind == "dependency") == d.deps[r.Detail.Subject], r)
			}
			report := h.investigate(dir, c)
			h.require((report.Summary.Failed > 0) == scenario.fail, report)
			h.require(report.Summary.Suppressed == 0, report)
			if scenario.name == "parser adapter" {
				inventory := map[string]bool{}
				for _, r := range d.depReceipts {
					if r.Kind == "dependency-inventory" {
						inventory[r.Detail.Subject] = true
					}
				}
				h.require(inventory["type:error"] && inventory["interface:interface{}"] && inventory["package:go/parser"], inventory)
			}
		})
	}
}

func TestCosmeticDependencyPlumbing(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(`package fixture
type A struct{}
type B struct{}
func Parent(a,b int){one(a,b);two(a,b)}
func sink(v any)error{return nil}
func one(a,b int){var x A;_=x;_=sink(a);println(b)}
func two(a,b int){var x B;_=x;_=sink(a);println(b)}`)
	c := cosmeticConfig()
	c.Counts["function-lines"] = 1
	c.Ratios["cosmetic-dependency-overlap"] = .75
	r := h.investigate(dir, c)
	h.require(len(r.Cases) == 0, "error and any must not create shared collaborator overlap", r)
	c.Ratios["cosmetic-dependency-overlap"] = 0
	cc := h.one(h.investigate(dir, c), "cosmetic-extraction")
	for _, q := range cc.Clues {
		if q.Kind == "dependency-overlap" {
			h.require(q.Value == float64(0), q)
		}
	}
	inventory := false
	for _, rr := range cc.Receipts {
		if r, ok := rr.(Source); ok && r.Kind == "dependency-inventory" && r.Detail.Subject == "type:error" {
			inventory = true
		}
	}
	h.require(inventory, "cosmetic receipts lost plumbing evidence")
}
