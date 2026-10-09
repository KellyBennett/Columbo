package columbo

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const guardedPrelude = `package fixture
 type Item struct { Quality, Other, SellIn int }
 type Other struct { Quality int }
 func baseline(item *Item) { if item.Quality < 50 { item.Quality++ } }
`

func TestGuardedUpdateBoundaries(t *testing.T) {
	tests := []struct {
		name, source string
		count        int
	}{
		{"same schema distinct parameter", `func another(x *Item){if x.Quality<50{x.Quality++}}`, 1},
		{"named constant identity", `const limit=25*2;func another(x *Item){if limit>x.Quality{x.Quality++}}`, 0},
		{"compound before", `func another(x *Item){if x.SellIn<0 && x.Quality<50{x.Quality++}}`, 1},
		{"compound after", `func another(x *Item){if x.Quality<50 && x.SellIn<0{x.Quality++}}`, 1},
		{"receiver", `func(x *Item) another(){if x.Quality<50{x.Quality++}}`, 1},
		{"different receiver", `func(x *Other) another(){if x.Quality<50{x.Quality++}}`, 0},
		{"different field", `func another(x *Item){if x.Other<50{x.Other++}}`, 0},
		{"distinct subject", `func another(x,y *Item){if x.Quality<50{y.Quality++}}`, 0},
		{"bound mismatch", `func another(x *Item){if x.Quality<49{x.Quality++}}`, 0},
		{"decrement mismatch", `func another(x *Item){if x.Quality>0{x.Quality--}}`, 0},
		{"direction mismatch", `func another(x *Item){if x.Quality<50{x.Quality--}}`, 0},
		{"inclusive mismatch", `func another(x *Item){if x.Quality<=50{x.Quality++}}`, 0},
		{"equality mismatch", `func another(x *Item){if x.Quality!=50{x.Quality++}}`, 0},
		{"variable bound", `func another(x *Item,limit int){if x.Quality<limit{x.Quality++}}`, 0},
		{"constant or", `func another(x *Item){if x.Quality<50 && (true || false){x.Quality++}}`, 0},
		{"or", `func another(x *Item){if x.SellIn<0 || x.Quality<50{x.Quality++}}`, 0},
		{"call before", `func check(x *Item)bool{x.Quality=0;return true};func another(x *Item){if check(x)&&x.Quality<50{x.Quality++}}`, 0},
		{"call after", `func check(x *Item)bool{x.Quality=100;return true};func another(x *Item){if x.Quality<50&&check(x){x.Quality++}}`, 0},
		{"alias subject", `func another(x *Item){y:=x;if x.Quality<50{y.Quality++}}`, 0},
		{"shadowed subject", `func another(x *Item){if x.Quality<50{x:=&Item{};x.Quality++}}`, 0},
		{"additional mutation", `func another(x *Item){if x.Quality<50{x.Quality=100;x.Quality++}}`, 0},
		{"else", `func another(x *Item){if x.Quality<50{x.Quality++}else{x.Quality=0}}`, 0},
		{"init", `func another(x *Item){if y:=x;y.Quality<50{y.Quality++}}`, 0},
		{"closure", `func another(x *Item){_=func(){if x.Quality<50{x.Quality++}}}`, 0},
		{"promoted field", `type Wrap struct{Item};func another(x *Wrap){if x.Quality<50{x.Quality++}}`, 0},
		{"nested field", `type Wrap struct{Child *Item};func another(x *Wrap){if x.Child.Quality<50{x.Child.Quality++}}`, 0},
		{"compound assignment normalization", `func another(x *Item){if x.Quality<50{x.Quality+=1}}`, 1},
		{"same name local scalar", `func another(){Quality:=0;if Quality<50{Quality++}}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture(guardedPrelude+test.source), quiet())
			require.Len(t, report.GuardedUpdates, test.count)
		})
	}
}
func TestGuardedUpdateDirectionsAndTypes(t *testing.T) {
	h := &testHarness{T: t}
	report := h.investigate(h.fixture(guardedPrelude+`
 func lower(a,b *Item){if a.Quality>0{a.Quality--};if 0<b.Quality{b.Quality--}}
 type Count int
 type Named struct{Quality Count}
 func named(a,b *Named){if a.Quality<Count(50){a.Quality++};if b.Quality<50{b.Quality++}}
 `), quiet())
	require.Len(t, report.GuardedUpdates, 2)
	for _, group := range report.GuardedUpdates {
		require.Len(t, group.sites, 2)
	}
}
func TestGuardedUpdateSnapshotAndStages(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	dir := h.fixture(guardedPrelude + `func another(x *Item){if x.SellIn<0{if x.SellIn<5&&x.Quality<50{x.Quality++}}}`)
	report := h.investigate(dir, cfg)
	require.Len(t, report.GuardedUpdates, 1)
	db := h.snapshot(report)
	rendered, code, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Zero(t, code)
	for _, text := range []string{guardedUpdateKind, "bound-guard", "surrounding-condition", "x.SellIn<5&&x.Quality<50", "guarded-update", "resolved-field"} {
		require.Contains(t, string(rendered), text)
	}
	cfg.Staged = true
	staged := h.investigate(dir, cfg)
	require.Len(t, staged.GuardedUpdates, 1)
	require.Len(t, staged.Stages, 3)
	for _, stage := range staged.Stages[:2] {
		require.Equal(t, "cleared", stage.state)
	}
	require.Equal(t, "active", staged.Stages[2].state)
	checkStageSnapshot(t, h, staged, 1, "Stage Consolidate Shared Behavior: active")
	cfg.Exclude = append(cfg.Exclude, "source.go")
	require.Empty(t, h.investigate(dir, cfg).GuardedUpdates)
}
