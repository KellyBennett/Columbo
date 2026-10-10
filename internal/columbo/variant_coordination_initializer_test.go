package columbo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVariantCoordinationInitializedDecisions(t *testing.T) {
	tests := []struct {
		name, body      string
		shared, overlap int
	}{
		{"plain if", "n:=0;if x==A {n++};if x==B {n++};_=n", 1, 1},
		{"inert if", "n:=0;if _=0;x==A {n++};if _=0;x==B {n++};_=n", 1, 1},
		{"plain switch", "n:=0;switch x {case A:n++};switch x {case B:n++};_=n", 1, 1},
		{"inert switch", "n:=0;switch _=0;x {case A:n++};switch _=0;x {case B:n++};_=n", 1, 1},
		{"initializer writes only", "n:=0;if n=1;x==A {};switch n=2;x {case B:};_=n", 0, 0},
		{"left initializer write only", "n:=0;if n=1;x==A {};if x==B {n++};_=n", 0, 0},
		{"right initializer write only", "n:=0;if x==A {n=1};switch n=2;x {case B:};_=n", 0, 0},
		{"right initializer read only", "n:=0;if x==A {n=1};if _=n;x==B {};_=n", 0, 0},
		{"shadowed selector", "n:=0;if x==A {n++};if x:=x;x==B {n++};_=n", 0, 0},
		{"fresh selector", "n:=0;if x==A {n++};switch y:=x;y {case B:n++};_=n", 0, 0},
		{"different initializer bindings", "n:=0;if y:=x;y==A {n++};if y:=x;y==B {n++};_=n", 0, 0},
		{"shadowed output", "n:=0;if x==A {n++};if n:=0;x==B {n++};_=n", 0, 0},
		{"conditionless switch", "n:=0;if x==A {n++};switch _=0; {case x==B:n++};_=n", 0, 0},
		{"unsupported predicate", "n:=0;if x==A {n++};if _=0;x==B && len(x)>0 {n++};_=n", 0, 0},
		{"nonconstant case", "n:=0;y:=x;if x==A {n++};switch _=0;x {case y:n++};_=n", 0, 0},
		{"nested refinement", "n:=0;if _=0;x==A {n++;if _=0;x!=B {n++}};_=n", 0, 0},
		{"else refinement", "n:=0;if _=0;x==A {n++} else if _=0;x==B {n++};_=n", 0, 0},
		{"nested initializer remains conditional", "n:=0;if _=0;x==A {if n=1;true {}};if _=0;x==B {n++};_=n", 1, 1},
		{"closure boundary", "n:=0;if _=0;x==A {n++};_=func(){if _=0;x==B {n++}};_=n", 0, 0},
		{"initializer closure boundary", "n:=0;if _=0;x==A {n++};if f:=func(){if x==B {n++}};x==C {_=f};_=n", 0, 0},
		{"scalar transition retained", "n:=0;if x==A {n++};x=B;if _=0;x==B {n++};_=n", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &testHarness{T: t}
			source := variantPrelude + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + variantFunction("Operation", tt.body)
			c := h.one(h.investigate(h.fixture(source), variantConfig()), variantSmell)
			counts := map[string]int{}
			for _, clue := range c.Clues {
				counts[clue.Kind]++
			}
			require.Equal(t, tt.shared, counts["variant-shared-write"])
			require.Equal(t, tt.overlap, counts["variant-state-overlap"])
		})
	}
}

func TestVariantCoordinationInitializerUncertainty(t *testing.T) {
	for _, body := range []string{
		"n:=0;if x=A;x==A {n++};if x=B;x==B {n++};_=n",
		"n:=0;switch x=A;x {case A:n++};switch x=B;x {case B:n++};_=n",
		"n:=0;if x=Kind(A);x==A {n++};switch x=Kind(B);x {case B:n++};_=n",
	} {
		t.Run(body, func(t *testing.T) {
			h := &testHarness{T: t}
			source := variantPrelude + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + variantFunction("Operation", body)
			c := h.one(h.investigate(h.fixture(source), variantConfig()), variantSmell)
			found := 0
			for _, clue := range c.Clues {
				if clue.Kind != "variant-shared-write" && clue.Kind != "variant-state-overlap" {
					continue
				}
				found++
				text := strings.Join(clue.Value.([]string), "\n")
				require.Contains(t, text, "2 possible selector writes")
				if strings.Contains(body, "Kind(") {
					require.Contains(t, text, "2 calls/conversions across span")
				}
			}
			require.Equal(t, 2, found)
		})
	}
}

func TestVariantCoordinationInitializerReceiptsAndIdentity(t *testing.T) {
	h := &testHarness{T: t}
	first := "if _=0;x==A {n++}"
	second := "switch _=0;x {case B:n++}"
	source := variantPrelude + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + variantFunction("Operation", "n:=0;"+first+";"+second+";_=n")
	dir := h.fixture(source)
	report := h.investigate(dir, variantConfig())
	c := h.one(report, variantSmell)
	require.Equal(t, canonical(report), canonical(h.investigate(dir, variantConfig())))
	roots := []string{}
	for _, raw := range c.Receipts {
		receipt := raw.(Source)
		spelling := source[receipt.StartOffset:receipt.EndOffset]
		if receipt.Kind == "variant-coordination-root" {
			roots = append(roots, spelling)
		}
		if receipt.Kind == "variant-effect-write" || receipt.Kind == "variant-effect-read" {
			require.Equal(t, "n", spelling)
		}
	}
	require.ElementsMatch(t, []string{first, second}, roots)
	h.write(dir, "source.go", strings.ReplaceAll(source, "_=0;", ""))
	plain := h.one(h.investigate(dir, variantConfig()), variantSmell)
	require.Equal(t, plain.ID, c.ID)
	require.Equal(t, plain.Verdict, c.Verdict)
}
