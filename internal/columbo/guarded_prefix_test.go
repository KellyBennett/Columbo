package columbo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGuardedBodyPrefixBoundaries(t *testing.T) {
	tests := []struct {
		name, body string
		count      int
	}{
		{"later call", `x.Quality++; effect(x)`, 1},
		{"later write", `x.Quality++; x.Quality = 100`, 1},
		{"second increment lacks a fresh check", `x.Quality++; x.Quality++`, 1},
		{"later return", `x.Quality++; return`, 1},
		{"independent local", `n := 1; n++; x.Quality++; _ = n`, 1},
		{"independent declared local", `var n int; n = 1; x.Quality++; _ = n`, 1},
		{"independent scalar parameter", `spare++; x.Quality++`, 1},
		{"long harmless prefix and suffix", `n := 1; n++; spare++; n+=spare; var m int; m=n; x.Quality++; effect(x); x.Quality=100; _=m; _=n`, 1},
		{"global write", `global++; x.Quality++`, 0},
		{"call before", `effect(x); x.Quality++`, 0},
		{"call in assignment", `spare = effect(x); x.Quality++`, 0},
		{"call in declaration", `n := effect(x); x.Quality++; _ = n`, 0},
		{"checked value changed", `x.Quality = 0; x.Quality++`, 0},
		{"field changed away from bound", `x.Quality--; x.Quality++`, 0},
		{"possible alias write", `other.Quality = 0; x.Quality++`, 0},
		{"subject reassigned", `x = other; x.Quality++`, 0},
		{"subject shadowed", `x := other; x.Quality++`, 0},
		{"indirect write", `*x = *other; x.Quality++`, 0},
		{"conditional prefix", `if spare > 0 { return }; x.Quality++`, 0},
		{"return prefix", `return; x.Quality++`, 0},
		{"deferred call", `defer effect(x); x.Quality++`, 0},
		{"goroutine call", `go effect(x); x.Quality++`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			source := guardedPrelude + `var global int;func effect(x *Item) int {x.Quality=100;return 1};func another(x, other *Item, spare int){if x.Quality < 50 {` + test.body + `}}`
			report := h.investigate(h.fixture(source), quiet())
			require.Len(t, report.GuardedUpdates, test.count)
			if test.count > 0 {
				require.Len(t, report.GuardedUpdates[0].sites, 2)
			}
		})
	}
}

func TestGuardedPrefixPreservesInputsAndConjunction(t *testing.T) {
	for _, prefix := range []string{`amount++`, `fee++`, `spare++`, `x.SellIn--`} {
		t.Run(prefix, func(t *testing.T) {
			h := &testHarness{T: t}
			source := `package fixture;type Item struct{Quality,SellIn int};func first(x *Item,amount,fee,spare int){if x.Quality>amount && spare>0 {x.Quality-=fee}};func second(x *Item,amount,fee,spare int){if x.Quality>amount && spare>0 {` + prefix + `;x.Quality-=fee}}`
			require.Empty(t, h.investigate(h.fixture(source), quiet()).GuardedUpdates)
		})
	}
}

func TestGuardedBodyEvidenceAndSevenQualitySteps(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	cfg.Staged = true
	source := guardedPrelude + `func brie(x *Item){if x.SellIn<0&&x.Quality<50{x.Quality++}}
func passes(x *Item){if x.Quality<50{x.Quality++;if x.SellIn<11&&x.Quality<50{x.Quality++};if x.SellIn<6&&x.Quality<50{x.Quality++}}}
func ordinary(x *Item){if x.Quality>0{x.Quality--};x.SellIn--;if x.SellIn<0&&x.Quality>0{x.Quality--}}`
	dir := h.fixture(source)
	report := h.investigate(dir, cfg)
	require.Len(t, report.GuardedUpdates, 2)
	counts := []int{}
	for _, group := range report.GuardedUpdates {
		counts = append(counts, len(group.sites))
	}
	require.ElementsMatch(t, []int{5, 2}, counts)
	for _, stage := range report.Stages[:2] {
		require.Equal(t, "cleared", stage.state)
	}
	cfg.Staged = false
	report = h.investigate(dir, cfg)
	rendered, _, err := RenderSnapshot(testDatabaseQueries(h.snapshot(report)), "report.sqlite")
	require.NoError(t, err)
	for _, text := range []string{"guarded-body", "not a whole-if replacement", "surrounding-condition", "x.SellIn<11", "x.SellIn<6"} {
		require.Contains(t, string(rendered), text)
	}
}
