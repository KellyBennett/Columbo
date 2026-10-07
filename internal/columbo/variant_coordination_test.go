package columbo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVariantCoordination(t *testing.T) {
	cases := []struct {
		name, body   string
		shared, flow int
	}{
		{"shared accumulator", `n:=0; if x==A {n++}; if x!=B {n+=2}; _=n`, 1, 1},
		{"local calculation", `n,m:=0,0; if x==A {n=1}; if x==B {m=n+1}; _,_=n,m`, 0, 1},
		{"nested refinement", `n:=0; if x==A {n++; if x!=B {n++}}; _=n`, 0, 0},
		{"else refinement", `n:=0; if x==A {n++} else if x==B {n+=2}; _=n`, 0, 0},
		{"unrelated outputs", `n,m:=0,0; if x==A {n=1}; if x==B {m=2}; _,_=n,m`, 0, 0},
		{"different values", `n:=0; y:=x; if x==A {n++}; if y==B {n++}; _=n`, 0, 0},
		{"different receivers", `a,b:=Source{x},Source{x}; n:=0; if a.Kind==A {n++}; if b.Kind==B {n++}; _=n`, 0, 0},
		{"shadowed outputs", `if x==A {n:=1;_ = n}; if x==B {n:=2;_=n}`, 0, 0},
		{"closure boundary", `n:=0; if x==A {n++}; _=func(){if x==B {n++}}; _=n`, 0, 0},
		{"boolean capabilities", `a,b:=true,false;n:=0;if a {n++};if b {n++};_=n`, 0, 0},
		{"unsupported leaf rejects decision", `n:=0;if x==A && len(x)>0 {n++};if x==B {n++};_=n`, 0, 0},
		{"negation remains unsupported", `n:=0;if !(x==A) {n++};if x==B {n++};_=n`, 0, 0},
		{"switch and comparison", `n:=0;switch x {case A:n++;case B:n+=2};if x!=C {n++};_=n`, 1, 1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			h := &testHarness{T: t}
			source := variantPrelude + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + variantFunction("Operation", tt.body)
			c := h.one(h.investigate(h.fixture(source), variantConfig()), variantSmell)
			counts := map[string]int{}
			for _, clue := range c.Clues {
				counts[clue.Kind]++
				if strings.HasPrefix(clue.Kind, "variant-shared-") || clue.Kind == "variant-write-read" {
					require.Len(t, clue.SupportingReceipts, 4)
				}
			}
			require.Equal(t, tt.shared, counts["variant-shared-write"])
			require.Equal(t, tt.flow, counts["variant-write-read"])
		})
	}
}

func TestVariantCoordinationUncertainty(t *testing.T) {
	h := &testHarness{T: t}
	source := variantPrelude + variantFunction("SeedOne", variantSwitch("A,B")) + variantFunction("SeedTwo", variantSwitch("A,B")) + variantFunction("Operation", `n,m:=0,0;if x==A {n=1}; x=B; n=2; println(n); if x==B {m=n};_,_=n,m`)
	c := h.one(h.investigate(h.fixture(source), variantConfig()), variantSmell)
	for _, clue := range c.Clues {
		if clue.Kind == "variant-write-read" {
			require.Contains(t, strings.Join(clue.Value.([]string), "\n"), "1 calls/conversions across span; 1 possible selector writes; 1 intervening location writes")
			return
		}
	}
	t.Fatal("missing lexical link with uncertainty")
}

func TestVariantCoordinationSnapshot(t *testing.T) {
	h := &testHarness{T: t}
	source := variantPrelude + variantFunction("Operation", `n:=0; switch x {case A:n=1;case B:n=2}; switch x {case A:n++;case B:n+=2};_=n`)
	dir := h.fixture(source)
	r := h.investigate(dir, variantConfig())
	require.Equal(t, canonical(r), canonical(h.investigate(dir, variantConfig())))
	db := h.snapshot(r)
	text, code, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Equal(t, 1, code)
	require.Contains(t, string(text), "variant-shared-write")
	require.Contains(t, string(text), "variant-write-read")
	require.Contains(t, string(text), "lexical evidence only")
	require.Equal(t, 2, h.sqlCount(db, "SELECT COUNT(*) FROM clues WHERE kind IN ('variant-shared-write', 'variant-write-read')"))
	h.write(dir, "source.go", variantPrelude+variantFunction("Operation", `n:=0;switch x {case A:n=1;case B:n=2};switch x {case A:println(1);case B:println(2)};_=n`))
	other := h.one(h.investigate(dir, variantConfig()), variantSmell)
	require.Equal(t, r.Cases[0].ID, other.ID)
	require.Equal(t, r.Cases[0].Verdict, other.Verdict)
}
