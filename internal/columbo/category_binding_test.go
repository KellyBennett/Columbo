package columbo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCategoryLocalBindingBounds(t *testing.T) {
	tests := []struct {
		name, branch string
		count        int
	}{
		{"short declaration", `update := first; update(item)`, 1},
		{"var declaration", `var update = first; update(item)`, 1},
		{"typed declaration", `var update func(*Item) = first; update(item)`, 1},
		{"parentheses", `update := (first); (update)(item)`, 1},
		{"shadowed function name", `first := first; first(item)`, 1},
		{"same target", `update := second; update(item)`, 0},
		{"other subject", `update := first; update(other)`, 0},
		{"reassigned", `update := first; update = second; update(item)`, 0},
		{"alias chain", `update := first; next := update; next(item)`, 0},
		{"closure", `update := func(x *Item) { first(x) }; update(item)`, 0},
		{"deferred", `update := first; defer update(item)`, 0},
		{"goroutine", `update := first; go update(item)`, 0},
		{"escape", `update := first; sink(update, item)`, 0},
		{"uninvoked", `update := first; _ = update`, 0},
		{"shadowed subject", `item := other; first(item)`, 0},
		{"parameter function", `update := incoming; update(item)`, 0},
		{"method value", `update := item.change; update(item)`, 0},
		{"generic instance", `update := generic[Item]; update(item)`, 0},
		{"multiple bindings", `update, spare := first, second; update(item); _ = spare`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			cfg := quiet()
			cfg.Staged = true
			source := categoryPrelude + `func sink(f func(*Item), x *Item) { f(x) };func (x *Item) change(y *Item) {};func generic[T any](x *T) {};func run(item, other *Item, incoming func(*Item)) { switch item.Name { case "a": ` + test.branch + `; default: second(item) } }`
			report := h.investigate(h.fixture(source), cfg)
			require.Len(t, report.CategoryBehavior, test.count)
		})
	}
}

func TestCategoryPartialEvidence(t *testing.T) {
	tests := []struct {
		name, source string
		count        int
	}{
		{"unknown branch", `switch item.Name {case "a":first(item); case "b":second(item);default: unknown(item)}`, 1},
		{"unknown if branch", `if item.Name=="a" {first(item)} else if item.Name=="b" {second(item)} else {unknown(item)}`, 1},
		{"unknown multi statement", `switch item.Name {case "a":first(item); case "b":second(item);default: first(item);second(item)}`, 1},
		{"runtime label", `switch item.Name {case "a":first(item); case "b":second(item);case other.Name: first(item)}`, 0},
		{"one known action", `switch item.Name {case "a":first(item);default: unknown(item)}`, 0},
		{"same known action", `switch item.Name {case "a":first(item);case "b":first(item);default: unknown(item)}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			cfg := quiet()
			cfg.Staged = true
			report := h.investigate(h.fixture(categoryPrelude+`func run(item,other *Item,unknown func(*Item)) {`+test.source+`}`), cfg)
			require.Len(t, report.CategoryBehavior, test.count)
			if test.count == 0 {
				return
			}
			rendered, _, err := RenderSnapshot(testDatabaseQueries(h.snapshot(report)), "report.sqlite")
			require.NoError(t, err)
			require.Contains(t, string(rendered), "category-unknown-branch")
			require.NotContains(t, string(rendered), "category-no-op")
		})
	}
}

func TestCategoryBindingEvidenceAndFactoryBoundary(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	cfg.Staged = true
	dir := h.fixture(categoryPrelude + `func run(item *Item) {switch item.Name {case "a":update:=first;update(item);default:second(item)}}`)
	report := h.investigate(dir, cfg)
	require.Len(t, report.CategoryBehavior, 1)
	checkStageSnapshot(t, h, report, 1, "Stage Assign Ownership: active")
	rendered, _, err := RenderSnapshot(testDatabaseQueries(h.snapshot(report)), "report.sqlite")
	require.NoError(t, err)
	for _, text := range []string{"category-function-binding", "update:=first", "category-call", "update(item)", "selected-callee"} {
		require.Contains(t, string(rendered), text)
	}
	h.write(dir, "source.go", categoryPrelude+`func choose(item *Item)func(*Item){switch item.Name{case "a":update:=first;return update;default:return second}};func run(item *Item){update:=choose(item);update(item)}`)
	owned := h.investigate(dir, cfg)
	require.Empty(t, owned.CategoryBehavior)
	checkStageSnapshot(t, h, owned, 0, "Stage Assign Ownership: cleared")
}

func TestCategoryBoundScalarReturn(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	cfg.Staged = true
	report := h.investigate(h.fixture(categoryPrelude+`func a(x *Item)int{return x.Quality};func b(x *Item)int{return -x.Quality};func run(item *Item)int{if item.Name=="a"{update:=a;return update(item)}else{var update=b;return update(item)}}`), cfg)
	require.Len(t, report.CategoryBehavior, 1)
}
