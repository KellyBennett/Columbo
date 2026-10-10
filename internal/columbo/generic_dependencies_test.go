package columbo

import (
	"reflect"
	"testing"
)

func TestGenericDependencyInventory(t *testing.T) {
	tests := []struct {
		name, source string
		want         []string
	}{
		{"qualified alias", `import("bytes";"sync/atomic")
type Buffer = bytes.Buffer
func F(p *atomic.Pointer[Buffer]) { _ = p.Load() }`, []string{"type:bytes.Buffer", "type:sync/atomic.Pointer[bytes.Buffer]"}},
		{"all explicit type contexts", `type Box[T any] struct{ Value T }
func F(x Box[int], a any) Box[int] {
 var local Box[int]
 _ = Box[int]{Value: 1}
 _ = a.(Box[int])
 _ = Box[int](local)
 return x
}`, []string{"type:fixture.Box[int]"}},
		{"multiple arguments and nesting", `type Box[T any] struct{ Value T }
type Pair[A, B any] struct{ A A; B B }
type Item struct{}
func F(x Pair[Box[Item], Box[int]]) {}`, []string{"type:fixture.Box[fixture.Item]", "type:fixture.Box[int]", "type:fixture.Item", "type:fixture.Pair[fixture.Box[fixture.Item], fixture.Box[int]]"}},
		{"distinct specializations", `type Box[T any] struct{ Value T }
func F(x Box[int], y Box[string]) {}`, []string{"type:fixture.Box[int]", "type:fixture.Box[string]"}},
		{"type parameter specialization", `type Box[T any] struct{ Value T }
func F[T any](x Box[T]) {}`, []string{"type:fixture.Box[T0]"}},
		{"generic alias", `type Box[T any] struct{ Value T }
type Alias[T any] = Box[T]
func F(x Alias[int]) {}`, []string{"type:fixture.Box[int]"}},
		{"parenthesized instantiated type", `type Box[T any] struct{ Value T }
func F(x (Box[int])) {}`, []string{"type:fixture.Box[int]"}},
		{"inferred generic call", `type Box[T any] struct{ Value T }
func F() { _ = makeBox(1) }
func makeBox[T any](x T) Box[T] { return Box[T]{Value:x} }`, []string{"type:fixture.Box[int]"}},
		{"explicit generic call", `type Box[T any] struct{ Value T }
func F() { _ = makeBox[int](1) }
func makeBox[T any](x T) Box[T] { return Box[T]{Value:x} }`, []string{"type:fixture.Box[int]"}},
		{"generic receiver exclusion", `type Box[T any] struct{ Value T }
func (b Box[T]) F(x Box[int]) {}`, []string{}},
		{"ordinary named dependency", `type Plain struct{}
func F(x Plain) {}`, []string{"type:fixture.Plain"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &testHarness{T: t}
			engine := h.loadDependencyFixture(h.fixture("package fixture\n"+tt.source), quiet())
			d := engine.declarations[0]
			d.measure(engine)
			if got := sortedSet(d.deps); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("dependencies: got %v; want %v", got, tt.want)
			}
			for _, receipt := range d.depReceipts {
				if receipt.DependencyIdentity == "type:fixture.Box[T any]" || receipt.DependencyIdentity == "type:sync/atomic.Pointer[T any]" {
					t.Errorf("spurious generic-head receipt: %+v", receipt)
				}
			}
			h.repeatDependencyMeasurement(engine, d)
			h.dependencyReceiptKinds(d)
		})
	}
}

func TestGenericDependencyThreshold(t *testing.T) {
	for _, tt := range []struct {
		name, parameter string
		fail            bool
	}{
		{"five genuine dependencies", "x Box[int], a A, b B, c C, d D", false},
		{"six genuine dependencies", "x Box[int], y Box[string], a A, b B, c C, d D", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := &testHarness{T: t}
			dir := h.fixture("package fixture\ntype Box[T any] struct{ Value T }; type A struct{}; type B struct{}; type C struct{}; type D struct{}\nfunc F(" + tt.parameter + ") {}")
			h.dependencyVerdict(h.investigate(dir, dependencyConfig()), tt.fail)
		})
	}
}
