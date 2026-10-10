package columbo

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestVariantCoordinationNonFallthrough(t *testing.T) {
	tests := []struct {
		name, body string
		want       int
	}{
		{"return local", "n:=0;if x==A {n++;return};if x==B {n++};_=n", 0},
		{"scalar transition", "n:=0;if x==A {n++;x=B};if x==B {n++};_=n", 1},
		{"selector assignment then return", "n:=0;if x==A {n++;x=B;return};if x==B {n++};_=n", 0},
		{"initialized return", "n:=0;if _=0;x==A {n++;return};if _=0;x==B {n++};_=n", 0},
		{"fallthrough", "n:=0;if x==A {n++};if x==B {n++};_=n", 1},
		{"missing nested else", "n:=0;if x==A {n++;if n>1 {return}};if x==B {n++};_=n", 1},
		{"mixed nested exits conservatively retained", "n:=0;for {if x==A {n++;if n>1{return}else{break}};if x==B {n++};break};_=n", 1},
		{"else effects retained", "n:=0;if x==A {n++;return}else{n++};if x==B {n++};_=n", 1},
		{"fresh loop break", "for {n:=0;if x==A {n++;break};if x==B {n++};_=n;break}", 0},
		{"single loop no reentry", "n:=0;for {if x==A {n++;break};if x==B {n++};break};_=n", 0},
		{"outer loop reentry retained", "n:=0;for range 2 {for {if x==A {n++;break};if x==B {n++};break}};_=n", 1},
		{"fresh state outer reentry", "for range 2 {for {n:=0;if x==A {n++;break};if x==B {n++};_=n;break}}", 0},
		{"labeled outer break", "Outer: for {n:=0;for {if x==A {n++;break Outer};if x==B {n++};break};_=n}", 0},
		{"unlabeled inner break later outside", "for {n:=0;for {if x==A {n++;break};break};if x==B {n++};_=n;break}", 1},
		{"labeled inner break later outside", "for {n:=0;Inner:for {if x==A {n++;break Inner};break};if x==B {n++};_=n;break}", 1},
		{"switch break later outside", "n:=0;switch {default:if x==A {n++;break}};if x==B {n++};_=n", 1},
		{"switch break inside", "switch {default:n:=0;if x==A {n++;break};if x==B {n++};_=n}", 0},
		{"select break later outside", "n:=0;select {default:if x==A {n++;break}};if x==B {n++};_=n", 1},
		{"select break inside", "select {default:n:=0;if x==A {n++;break};if x==B {n++};_=n}", 0},
		{"nested switch break cannot bypass final return", "n:=0;if x==A {n++;switch{default:break};return};if x==B {n++};_=n", 0},
		{"nested loop break cannot bypass final return", "n:=0;if x==A {n++;for{break};return};if x==B {n++};_=n", 0},
		{"early enclosing break bypasses return", "n:=0;for {if x==A {n++;if n>1 {break};return};break};if x==B {n++};_=n", 1},
		{"early labeled break bypasses return", "n:=0;Outer:for {if x==A {n++;for{break Outer};return};break};if x==B {n++};_=n", 1},
		{"continue unproven", "n:=0;for range 2 {if x==A {n++;continue};if x==B {n++}};_=n", 1},
		{"goto bypasses return", "n:=0;if x==A {n++;goto Later;return};Later:if x==B {n++};_=n", 1},
		{"goto reentry", "n:=0;Again:for {if x==A {n++;break};if x==B {n++};break};if n>0 {goto Again};_=n", 1},
		{"panic is not proof", "n:=0;if x==A {n++;panic(n)};if x==B {n++};_=n", 1},
		{"unrelated call before exit", "n:=0;if x==A {n++;println(n);return};if x==B {n++};_=n", 0},
		{"defer receives copy", "n:=0;defer println(n);if x==A {n++;return};if x==B {n++};_=n", 0},
		{"capture retained", "n:=0;f:=func(){n++};_=f;if x==A {n++;return};if x==B {n++};_=n", 1},
		{"address retained", "n:=0;p:=&n;_=p;if x==A {n++;return};if x==B {n++};_=n", 1},
		{"go receives copy", "n:=0;go println(n);if x==A {n++;return};if x==B {n++};_=n", 0},
		{"field storage retained", "s:=struct{n int}{};if x==A {s.n++;return};if x==B {s.n++}", 1},
		{"struct slot return", "n:=struct{v int}{};if x==A {n=struct{v int}{1};return};if x==B {n=n};_=n", 0},
		{"slice header return", "n:=[]int{};if x==A {n=[]int{1};return};if x==B {n=n};_=n", 0},
		{"slice header single break", "n:=[]int{};for {if x==A {n=[]int{1};break};if x==B {n=n};break};_=n", 0},
		{"slice header outer reentry", "n:=[]int{};for range 2 {for {if x==A {n=[]int{1};break};if x==B {n=n};break}};_=n", 1},
		{"struct slot falling through", "n:=struct{v int}{};if x==A {n=struct{v int}{1}};if x==B {n=n};_=n", 1},
		{"slice header captured", "n:=[]int{};f:=func(){_=n};_=f;if x==A {n=[]int{1};return};if x==B {n=n};_=n", 1},
		{"struct field no continuation retained", "n:=struct{v int}{};if x==A {n.v=1;return};if x==B {n.v=n.v}", 1},

		{"implicit pointer receiver escape", "n:=Box{};n.Keep();if x==A {n=Box{1};return};if x==B {n=n};_=n", 1},
		{"implicit method value escape", "n:=Box{};f:=n.Keep;_=f;if x==A {n=Box{1};return};if x==B {n=n};_=n", 1},
		{"value receiver copies slot", "n:=Box{};n.Copy();if x==A {n=Box{1};return};if x==B {n=n};_=n", 0},
		{"field address escapes slot", "n:=Box{};p:=&n.V;_=p;if x==A {n=Box{1};return};if x==B {n=n};_=n", 1},

		{"unrelated address", "n:=0;other:=0;p:=&other;_=p;if x==A {n++;return};if x==B {n++};_=n", 0},
		{"unrelated pointer receiver", "n:=0;other:=Box{};other.Keep();if x==A {n++;return};if x==B {n++};_=n", 0},
		{"unrelated deferred pointer receiver", "n:=0;other:=Box{};defer other.Keep();if x==A {n++;return};if x==B {n++};_=n", 0},
		{"unrelated capture", "n:=0;other:=0;f:=func(){other++};_=f;if x==A {n++;return};if x==B {n++};_=n", 0},
		{"deferred capture", "n:=0;defer func(){n++}();if x==A {n++;return};if x==B {n++};_=n", 1},
		{"goroutine capture", "n:=0;go func(){n++}();if x==A {n++;return};if x==B {n++};_=n", 1},
		{"shadowed capture is distinct", "n:=0;f:=func(){n:=0;n++;_=n};_=f;if x==A {n++;return};if x==B {n++};_=n", 0},

		{"address of composite copies slot", "n:=0;copy:=&struct{Value int}{Value:n};_=copy;if x==A {n++;return};if x==B {n++};_=n", 0},
		{"composite nested address escapes slot", "n:=0;copy:=&struct{Ptr *int}{Ptr:&n};_=copy;if x==A {n++;return};if x==B {n++};_=n", 1},
		{"composite copies aggregate slot", "n:=Box{};copy:=&struct{Value Box}{Value:n};_=copy;if x==A {n=Box{1};return};if x==B {n=n};_=n", 0},
		{"composite nested field address escapes slot", "n:=Box{};copy:=&struct{Ptr *int}{Ptr:&n.V};_=copy;if x==A {n=Box{1};return};if x==B {n=n};_=n", 1},

		{"global storage retained", "if x==A {global++;return};if x==B {global++}", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &testHarness{T: t}
			source := variantPrelude + "var global int;type Box struct{V int};var saved *Box;func(b *Box)Keep(){saved=b};func(b Box)Copy(){}\n" + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + variantFunction("Operation", tt.body)
			c := h.one(h.investigate(h.fixture(source), variantConfig()), variantSmell)
			counts := map[string]int{}
			for _, clue := range c.Clues {
				counts[clue.Kind]++
			}
			require.Equal(t, tt.want, counts["variant-shared-write"])
			require.Equal(t, tt.want, counts["variant-state-overlap"])
		})
	}
}

func TestVariantCoordinationReturnInteractions(t *testing.T) {
	for _, fn := range []string{
		`func Operation(x Kind)(n int){defer func(){if n>0{x=B;Operation(x)}}();if x==A{n++;return n};if x==B{n++};return n}`,
		`func Operation(x Kind)(n int){if x==A{n++;return observe(n)};if x==B{n++};return n}`,
		`func Operation(x Kind){if x==A{global++;Operation(B);return};if x==B{global++}}`,
	} {
		t.Run(fn, func(t *testing.T) {
			h := &testHarness{T: t}
			source := variantPrelude + "var global int;func observe(n int)int{return n}\n" + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + fn
			c := h.one(h.investigate(h.fixture(source), variantConfig()), variantSmell)
			counts := map[string]int{}
			for _, clue := range c.Clues {
				counts[clue.Kind]++
			}
			require.Equal(t, 1, counts["variant-shared-write"])
			require.Equal(t, 1, counts["variant-state-overlap"])
		})
	}
}

func TestVariantCoordinationNonFallthroughReceipts(t *testing.T) {
	h := &testHarness{T: t}
	first := "if _=0;x==A {n=struct{v int}{1};n.v++;return}"
	second := "if _=0;x==B {n=n;n.v++}"
	source := variantPrelude + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + variantFunction("Operation", "n:=struct{v int}{};"+first+";"+second+";_=n")
	dir := h.fixture(source)
	report := h.investigate(dir, variantConfig())
	c := h.one(report, variantSmell)
	require.Equal(t, canonical(report), canonical(h.investigate(dir, variantConfig())))
	counts := map[string]int{}
	for _, clue := range c.Clues {
		counts[clue.Kind]++
	}
	require.Equal(t, 1, counts["variant-shared-write"])
	require.Equal(t, 1, counts["variant-state-overlap"])
	roots := []string{}
	for _, raw := range c.Receipts {
		receipt := raw.(Source)
		spelling := source[receipt.StartOffset:receipt.EndOffset]
		if receipt.Kind == "variant-coordination-root" {
			roots = append(roots, spelling)
		}
		if receipt.Kind == "variant-effect-write" || receipt.Kind == "variant-effect-read" {
			require.Equal(t, "n.v", spelling)
		}
	}
	require.ElementsMatch(t, []string{first, second}, roots)
}
