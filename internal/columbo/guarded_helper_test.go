package columbo

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGuardedHelperBoundary(t *testing.T) {
	tests := []struct {
		name, body string
		count      int
	}{
		{"direct", `if x.Quality<50 { baseline(x) }`, 1},
		{"reversed", `if 50>x.Quality { baseline(x) }`, 1},
		{"context", `if x.Quality<50 { baseline(x); x.SellIn--; if x.SellIn<0 { baseline(x) } }`, 1},
		{"pure prefix", `if x.Quality<50 { n:=1; n++; baseline(x); _=n }`, 1},
		{"different subject", `if x.Quality<50 { baseline(y) }`, 0},
		{"different field", `if x.Other<50 { baseline(x) }`, 0},
		{"different bound", `if x.Quality<49 { baseline(x) }`, 0},
		{"different operator", `if x.Quality<=50 { baseline(x) }`, 0},
		{"reassignment", `if x.Quality<50 { x=y; baseline(x) }`, 0},
		{"alias write", `if x.Quality<50 { y.Quality=100; baseline(x) }`, 0},
		{"effect", `if x.Quality<50 { touch(x); baseline(x) }`, 0},
		{"prior mutation", `if x.Quality<50 { x.Quality=100; baseline(x) }`, 0},
		{"control flow", `if x.Quality<50 { if flag { return }; baseline(x) }`, 0},
		{"alias", `z:=x; if x.Quality<50 { baseline(z) }`, 0},
		{"binding", `if x.Quality<50 { f:=baseline; f(x) }`, 0},
		{"deferred", `if x.Quality<50 { defer baseline(x) }`, 0},
		{"closure", `_ = func(){if x.Quality<50 { baseline(x) }}`, 0},
		{"nested guard", `if x.Quality<50 { if flag { baseline(x) } }`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture(guardedPrelude+`func touch(x *Item){};func caller(x,y *Item,flag bool){`+test.body+`}`), quiet())
			require.Len(t, report.GuardedUpdates, test.count)
			if test.count > 0 {
				require.Len(t, report.GuardedUpdates[0].sites, 2)
			}
		})
	}
}
func TestGuardedHelperCalleeBoundaries(t *testing.T) {
	tests := []struct {
		name, helper string
		count        int
	}{
		{"renamed parameter", `func helper(z *Item){if z.Quality<50{z.Quality++}}`, 1},
		{"pure prefix", `func helper(z *Item){n:=1;n++;if z.Quality<50{z.Quality++};_=n}`, 1},
		{"effect before guard", `func helper(z *Item){touch(z);if z.Quality<50{z.Quality++}}`, 0},
		{"reassign before guard", `func helper(z *Item){z=&Item{};if z.Quality<50{z.Quality++}}`, 0},
		{"effect before mutation", `func helper(z *Item){if z.Quality<50{touch(z);z.Quality++}}`, 0},
		{"nested guard", `func helper(z *Item){if z.SellIn<0{if z.Quality<50{z.Quality++}}}`, 0},
		{"different mutation subject", `var other Item;func helper(z *Item){if z.Quality<50{other.Quality++}}`, 0},
		{"recursive prefix", `func helper(z *Item){helper(z);if z.Quality<50{z.Quality++}}`, 0},
		{"one hop only", `func operation(z *Item){if z.Quality<50{z.Quality++}};func helper(z *Item){operation(z)}`, 0},
		{"result boundary", `func helper(z *Item)*Item{if z.Quality<50{z.Quality++};return z}`, 0},
		{"copy subject", `func helper(z Item){if z.Quality<50{z.Quality++}}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			arg := "x"
			if test.name == "copy subject" {
				arg = "*x"
			}
			source := `package fixture;type Item struct{Quality,SellIn int};func touch(x *Item){};` + test.helper + `;func caller(x *Item){if x.Quality<50{helper(` + arg + `)}}`
			report := h.investigate(h.fixture(source), quiet())
			require.Len(t, report.GuardedUpdates, test.count)
		})
	}
}
func TestGuardedHelperEvidence(t *testing.T) {
	h := &testHarness{T: t}
	report := h.investigate(h.fixture(guardedPrelude+`func caller(x *Item){if x.Quality<50{baseline(x);x.SellIn--}}`), quiet())
	require.Len(t, report.GuardedUpdates, 1)
	db := h.snapshot(report)
	rendered, _, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	for _, text := range []string{"guarded-helper-call", "guarded-helper-declaration", "guarded-body", "outer guard is not proven redundant", "x.SellIn--"} {
		require.Contains(t, string(rendered), text)
	}
}

func TestGuardedHelperCrossFileSnapshot(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(guardedPrelude)
	h.write(dir, "caller.go", `package fixture;func caller(x *Item){if x.Quality<50{baseline(x);x.SellIn--}}`)
	report := h.investigate(dir, quiet())
	require.Len(t, report.GuardedUpdates, 1)
	require.Len(t, report.GuardedUpdates[0].sites, 2)
	db := h.snapshot(report)
	rendered, _, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	for _, text := range []string{"caller.go", "source.go", "guarded-helper-declaration", "guarded-helper-call", "caller repeats boundary in fixture.baseline"} {
		require.Contains(t, string(rendered), text)
	}
}
