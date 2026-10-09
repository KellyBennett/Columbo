package columbo

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const categoryPrelude = `package fixture
type Item struct { Name string; Quality int }
func first(x *Item) { x.Quality++ }
func second(x *Item) { x.Quality-- }
`

func TestCategorySelectedBehaviorBounds(t *testing.T) {
	tests := []struct {
		name, body, extra string
		count             int
	}{
		{"raw strings default noop", `switch item.Name { case "Brie": first(item); case "Sulfuras": return; default: second(item) }`, "", 1},
		{"if chain", `if item.Name == "Brie" { first(item) } else if "Sulfuras" == item.Name { return } else { second(item) }`, "", 1},
		{"if implicit default", `if item.Name == "Brie" { first(item) } else if item.Name == "Ordinary" { second(item) }`, "", 1},
		{"same function", `switch item.Name { case "Brie": first(item); default: first(item) }`, "", 0},
		{"distinct subject", `switch item.Name { case "Brie": first(item); default: second(other) }`, "", 0},
		{"different argument position", `switch item.Name { case "Brie": pair(item,other); default: otherPair(other,item) }`, `func pair(a,b *Item){};func otherPair(a,b *Item){}`, 0},
		{"same argument position", `switch item.Name { case "Brie": pair(other,item); default: otherPair(other,item) }`, `func pair(a,b *Item){};func otherPair(a,b *Item){}`, 1},
		{"signature mismatch", `switch item.Name { case "Brie": first(item); default: result(item) }`, `func result(x *Item) int{return x.Quality}`, 0},
		{"shadow subject", `switch item.Name { case "Brie": first(item); default: item:=other; second(item) }`, "", 0},
		{"unrelated calls", `first(item); second(item)`, "", 0},
		{"returned function", `switch item.Name { case "Brie": _=first; default: _=second }`, "", 0},
		{"function variable execution", `f:=first; switch item.Name { case "Brie": f(item); default: second(item) }`, "", 0},
		{"method execution", `switch item.Name { case "Brie": item.one(); default: item.two() }`, `func(x *Item)one(){};func(x *Item)two(){}`, 0},
		{"fallthrough", `switch item.Name { case "Brie": first(item); fallthrough; default: second(item) }`, "", 0},
		{"computed arguments", `switch item.Name { case "Brie": pair(item,makeItem()); default: otherPair(item,other) }`, `func pair(a,b *Item){};func otherPair(a,b *Item){};func makeItem()*Item{return nil}`, 0},
		{"same spelling different selector root", `if item.Name == "Brie" {first(item)} else if other.Name == "Ordinary" {second(item)}`, "", 0},
		{"closure deferred selection", `f:=func(){switch item.Name {case "Brie":first(item);default:second(item)}};_=f`, "", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			cfg := quiet()
			cfg.Staged = true
			source := categoryPrelude + test.extra + "\n" + `func Update(item,other *Item){` + test.body + `}`
			report := h.investigate(h.fixture(source), cfg)
			require.Len(t, report.CategoryBehavior, test.count)
		})
	}
}
func TestCategoryReturnsAndOwnershipBoundary(t *testing.T) {
	tests := []struct {
		name, source string
		count        int
	}{
		{"scalar action result", `func a(x *Item)int{x.Quality++;return x.Quality};func b(x *Item)int{x.Quality--;return x.Quality};func run(x *Item)int{switch x.Name{case "a":return a(x);default:return b(x)}}`, 1},
		{"function factory", `func choose(x *Item)func(*Item){switch x.Name{case "a":return first;default:return second}};func run(x *Item){choose(x)(x)}`, 0},
		{"constructed behavior", `type Owner struct{item *Item};func a(x *Item)*Owner{return &Owner{x}};func b(x *Item)*Owner{return &Owner{x}};func choose(x *Item)*Owner{switch x.Name{case "a":return a(x);default:return b(x)}}`, 0},
		{"callable factory", `func a(x *Item)func(){return func(){first(x)}};func b(x *Item)func(){return func(){second(x)}};func choose(x *Item)func(){switch x.Name{case "a":return a(x);default:return b(x)}}`, 0},
		{"named string", `type Category string;type Thing struct{Category Category};const A Category="a";func a(x *Thing){};func b(x *Thing){};func run(x *Thing){switch x.Category{case A:a(x);default:b(x)}}`, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			cfg := quiet()
			cfg.Staged = true
			report := h.investigate(h.fixture(categoryPrelude+test.source), cfg)
			require.Len(t, report.CategoryBehavior, test.count)
		})
	}
}
func TestCategoryStageEvidenceRoundtrip(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	cfg.Staged = true
	dir := h.fixture(categoryPrelude + `func Update(items []*Item){for _, item:=range items{switch item.Name{case "Brie":first(item);case "Sulfuras":continue;default:second(item)}}}`)
	report := h.investigate(dir, cfg)
	require.Equal(t, "cleared", report.Stages[0].state)
	require.Equal(t, "active", report.Stages[1].state)
	require.Equal(t, 1, report.Stages[1].issues[categoryBehaviorKind])
	require.False(t, report.Stages[1].definition.pending)
	checkStageSnapshot(t, h, report, 1, "Collector category-selected-behavior: 1 issues")
	db := h.snapshot(report)
	rendered, _, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	for _, text := range []string{"candidate-callable-surface", "shared-subject-state", "subject-state-write", "category-selector", `"Brie" ->`, "default/else ->", `"Sulfuras" -> explicit no-op`, "category-call", "selected-callee"} {
		require.Contains(t, string(rendered), text)
	}
	h.write(dir, "source.go", categoryPrelude+`func choose(x *Item)func(*Item){switch x.Name{case "Brie":return first;case "Sulfuras":return func(*Item){};default:return second}};func Update(items []*Item){for _,item:=range items{choose(item)(item)}}`)
	owned := h.investigate(dir, cfg)
	require.Empty(t, owned.CategoryBehavior)
	require.Equal(t, "cleared", owned.Stages[1].state)
	checkStageSnapshot(t, h, owned, 0, "Stage Assign Ownership: cleared")
	cfg.Staged = false
	require.Empty(t, h.investigate(dir, cfg).CategoryBehavior)
}
func TestCategoryLockedByUntangling(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	cfg.Staged = true
	source := tanglePrelude + tangleBody + `type Subject struct{Name string};func a(x *Subject){};func b(x *Subject){};func run(x *Subject){switch x.Name{case "a":a(x);default:b(x)}}`
	report := h.investigate(h.fixture(source), cfg)
	require.NotEmpty(t, report.CategoryBehavior)
	require.Equal(t, "active", report.Stages[0].state)
	require.Equal(t, "locked", report.Stages[1].state)
	checkStageSnapshot(t, h, report, 1, "Stage Assign Ownership: locked")
}

func TestCategoryCrossFileReceiptsAndExclusions(t *testing.T) {
	h := &testHarness{T: t}
	cfg := quiet()
	cfg.Staged = true
	dir := h.fixture(`package fixture
 type Item struct{Name string; Quality int}
 func first(x *Item){x.Quality++}
 func run(x *Item){if x.Name=="a"{first(x)}else{second(x)}}`)
	h.write(dir, "helper.go", `package fixture
 func second(x *Item){x.Quality--}`)
	report := h.investigate(dir, cfg)
	require.Len(t, report.CategoryBehavior, 1)
	checkStageSnapshot(t, h, report, 1, "helper.go:2-2")
	cfg.Exclude = append(cfg.Exclude, "helper.go")
	require.Empty(t, h.investigate(dir, cfg).CategoryBehavior)
}
