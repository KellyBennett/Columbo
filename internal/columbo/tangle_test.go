package columbo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const tanglePrelude = `package fixture
type Item struct { Name string; Quality, SellIn int }
`
const tangleBody = `func update(x *Item) {
 if x.Name != "A" && !(x.Name == "B") {
  x.Quality--
  if x.Name != "B" { x.SellIn-- }
 }
 if x.Name == "A" { x.Quality++ }
}
`

func tanglesFixture(t *testing.T, source string) TangleReport {
	t.Helper()
	h := &testHarness{T: t}
	a, err := load(h.fixture(source), []string{"./..."}, quiet())
	require.NoError(t, err)
	return a.tangles()
}
func TestTanglesBooleanContextAndDeterminism(t *testing.T) {
	a := tanglesFixture(t, tanglePrelude+tangleBody)
	b := tanglesFixture(t, tanglePrelude+tangleBody)
	require.Equal(t, a, b)
	require.Len(t, a.Groups, 1)
	require.Contains(t, a.Groups[0].Sites[0].Condition.Spelling, `!(x.Name == "B")`)
	require.Equal(t, []string{`"A"`, `"B"`}, a.Groups[0].Values)
}

func TestTanglesKeepRecognizedComparisonsBesideUnsupportedLeaves(t *testing.T) {
	source := strings.Replace(tangleBody, `x.Name != "A"`, `x.Quality > 0 && x.Name != "A"`, 1)
	report := tanglesFixture(t, tanglePrelude+source)
	require.Len(t, report.Groups, 1)
	require.Len(t, report.Groups[0].Values, 2)
}
func TestTanglesCounterexamples(t *testing.T) {
	cases := map[string]string{
		"return-validation":    `func f(x *Item) bool { if x.Name!="A" && x.Name!="B" { if x.Name!="B" { return false } };if x.Name=="A" { return true };return false }`,
		"single-output":        strings.ReplaceAll(tangleBody, "x.SellIn--", "x.Quality--"),
		"sibling-decisions":    `func f(x *Item) { if x.Name!="A" && x.Name!="B" { x.Quality-- };if x.Name!="B" { x.SellIn-- };if x.Name=="A" { x.Quality++ } }`,
		"different-receivers":  strings.Replace(strings.Replace(tangleBody, "x *Item", "x,y *Item", 1), `if x.Name != "B" { x.SellIn-- }`, `if y.Name != "B" { y.SellIn-- }`, 1),
		"other-receiver-write": strings.Replace(strings.Replace(tangleBody, "x *Item", "x,y *Item", 1), "x.SellIn--", "y.SellIn--", 1),
		"closure":              strings.Replace(tangleBody, `if x.Name != "B" { x.SellIn-- }`, `f:=func(){ if x.Name != "B" { x.SellIn-- } };f()`, 1),
		"one-repeated-value":   strings.Replace(tangleBody, `x.Name != "B"`, `x.Name != "C"`, 1),
		"different-index":      `func f(xs []*Item,i,j int) { if xs[i].Name!="A" && xs[i].Name!="B" { xs[i].Quality--; if xs[j].Name!="B" { xs[j].SellIn-- } };if xs[i].Name=="A" { xs[i].Quality++ } }`,
		"shadowed-root":        `func f(x *Item) { if x.Name!="A" && x.Name!="B" { x.Quality--;x:=&Item{}; if x.Name!="B" { x.SellIn-- } };if x.Name=="A" { x.Quality++ } }`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) { require.Empty(t, tanglesFixture(t, tanglePrelude+body).Groups) })
	}
}
func TestTanglesIntegerFields(t *testing.T) {
	source := strings.ReplaceAll(tanglePrelude+tangleBody, "Name string", "Name int")
	source = strings.ReplaceAll(strings.ReplaceAll(source, `"A"`, "1"), `"B"`, "2")
	require.Len(t, tanglesFixture(t, source).Groups, 1)
}
func TestTanglesDistinctReceiversStaySeparate(t *testing.T) {
	body := strings.TrimSuffix(strings.TrimPrefix(tangleBody, "func update(x *Item) {"), "}\n")
	source := tanglePrelude + "func update(x,y *Item){" + body + strings.ReplaceAll(body, "x.", "y.") + "}"
	report := tanglesFixture(t, source)
	require.Len(t, report.Groups, 2)
	require.NotEqual(t, report.Groups[0].ID, report.Groups[1].ID)
}

func TestTanglesDispatchLadderIsNotNesting(t *testing.T) {
	body := `func f(x *Item){if x.Name=="A" || x.Name=="B" {x.Quality--} else if x.Name=="A" || x.Name=="B" {x.SellIn--} else if x.Name=="A" || x.Name=="B" {x.Quality++}}`
	require.Empty(t, tanglesFixture(t, tanglePrelude+body).Groups)
}
func TestTanglesExcludeGeneratedTestsAndClosures(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(tanglePrelude + "func outer(x *Item){f:=func(){" + strings.TrimSuffix(strings.TrimPrefix(tangleBody, "func update(x *Item) {"), "}\n") + "};f()}")
	h.write(dir, "generated.go", "// Code generated fixture. DO NOT EDIT.\npackage fixture\n"+tangleBody)
	h.write(dir, "fixture_test.go", "package fixture\n"+strings.Replace(tangleBody, "update(", "testUpdate(", 1))
	a, err := load(dir, []string{"./..."}, quiet())
	require.NoError(t, err)
	require.Empty(t, a.tangles().Groups)
}
