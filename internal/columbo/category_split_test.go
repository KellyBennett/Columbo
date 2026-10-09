package columbo

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

const splitPrelude = `package fixture
import "math"
type Play struct { Kind string; Audience int; Other int }
func opaque(){}
func run(play, other *Play) { a,b:=0,0; _,_=a,b; _=math.Floor
`
const splitFirst = `switch play.Kind {case "comedy":a=300+play.Audience;case "tragedy":a=400}`
const splitSecond = `if play.Kind=="comedy" {b+=play.Audience/5}`

func TestCategorySplitBounds(t *testing.T) {
	cases := []struct {
		name, body string
		count      int
	}{
		{"pricing and rewards", splitFirst + `;` + splitSecond, 1},
		{"independent policies remain ambiguous", `if play.Kind=="comedy" {a=play.Audience*50}; if play.Kind=="comedy" {b=play.Audience/5}`, 1},
		{"calls conversions intervening calculation", splitFirst + `;b+=int(math.Max(float64(play.Audience)-30,0));if play.Kind=="comedy" {b+=int(math.Floor(float64(play.Audience)/5))}`, 1},
		{"opaque call uncertainty", splitFirst + `;opaque();` + splitSecond, 1},
		{"integer category", `switch play.Audience{case 1:a=play.Other;case 2:a=0};if play.Audience==1{b=play.Other/5}`, 1},
		{"constant alias", `const C="comedy";if C==play.Kind{a=play.Audience};if play.Kind=="comedy"{b=play.Audience/5}`, 1},
		{"different subject", splitFirst + `;if other.Kind=="comedy"{b=play.Audience}`, 0},
		{"different field", splitFirst + `;if play.Audience==5{b=play.Audience}`, 0},
		{"different category", splitFirst + `;if play.Kind=="history"{b=play.Audience}`, 0},
		{"disjoint input", splitFirst + `;if play.Kind=="comedy"{b=play.Other}`, 0},
		{"same destination", splitFirst + `;if play.Kind=="comedy"{a+=play.Audience}`, 0},
		{"gathered category", `switch play.Kind{case "comedy":a=play.Audience;b=play.Audience/5;case "tragedy":a=0}`, 0},
		{"logging calls only", `if play.Kind=="comedy"{opaque()};if play.Kind=="comedy"{opaque()}`, 0},
		{"selector replacement", splitFirst + `;play.Kind="tragedy";` + splitSecond, 0},
		{"root replacement", splitFirst + `;play=other;` + splitSecond, 0},
		{"nested pair", `if play.Kind=="comedy"{a=play.Audience;if play.Kind=="comedy"{b=play.Audience}}`, 0},
		{"different block", splitFirst + `;{` + splitSecond + `}`, 0},
		{"closure", `_ = func(){` + splitFirst + `;` + splitSecond + `}`, 0},
		{"alias selector", splitFirst + `;alias:=play;if alias.Kind=="comedy"{b=play.Audience}`, 0},
		{"runtime category", `switch play.Kind{case other.Kind:a=play.Audience};` + splitSecond, 0},
		{"fallthrough", `switch play.Kind{case "comedy":a=play.Audience;fallthrough;case "tragedy":a++};` + splitSecond, 0},
		{"boolean predicate", splitFirst + `;if play.Kind=="comedy"&&play.Audience>0{b=play.Audience}`, 0},
		{"shadowing if initializer", `if play:=other;play.Kind=="comedy"{a=play.Audience}else if play.Kind=="tragedy"{a=play.Audience};` + splitSecond, 0},
		{"multiassignment unrelated rhs", `var s,t string;_,_=s,t;if play.Kind=="comedy"{a,s=1,play.Kind};if play.Kind=="comedy"{b,t=2,play.Kind}`, 0},
		{"discarded read does not support output", `if play.Kind=="comedy"{a=1;_=play.Audience};if play.Kind=="comedy"{b=2;_=play.Audience}`, 0},
		{"multiassignment numeric rhs", `if play.Kind=="comedy"{a,play.Other=1,play.Audience};if play.Kind=="comedy"{b,other.Other=2,play.Audience}`, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture(splitPrelude+test.body+`}`), quiet())
			require.Len(t, report.CategorySplit, test.count)
			for _, group := range report.CategorySplit {
				require.Contains(t, group.limits, "independent")
				require.Contains(t, group.limits, "same runtime value")
			}
		})
	}
}
func TestCategorySplitSnapshotNonGating(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(splitPrelude + splitFirst + `;opaque();b+=int(math.Max(float64(play.Audience)-30,0));if play.Kind=="comedy"{b+=int(math.Floor(float64(play.Audience)/5))}}`)
	cfg := quiet()
	report := h.investigate(dir, cfg)
	require.Len(t, report.CategorySplit, 1)
	db := h.snapshot(report)
	rendered, code, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Zero(t, code)
	for _, text := range []string{categorySplitKind, "independent policies", "runtime value stability unknown", "split-decision", "split-category-arm", "split-scalar-write", "split-update-context", "split-shared-input-read", "split-intervening-statement", "split-call-or-conversion", "math.Max", "math.Floor", "purity not inferred", "opaque()"} {
		require.Contains(t, string(rendered), text)
	}
	require.Equal(t, 1, h.sqlCount(db, `SELECT count(*) FROM advisory_groups WHERE kind='category-split-updates'`))
	cfg.Staged = true
	staged := h.investigate(dir, cfg)
	require.Len(t, staged.CategorySplit, 1)
	require.Equal(t, []string{"cleared", "cleared", "cleared"}, stageStates(staged))
	stagedDB := h.snapshot(staged)
	stagedText, code, err := RenderSnapshot(testDatabaseQueries(stagedDB), "staged.sqlite")
	require.NoError(t, err)
	require.Zero(t, code)
	require.NotContains(t, string(stagedText), categorySplitKind)
	require.Zero(t, h.sqlCount(stagedDB, `SELECT count(*) FROM active_stage_issues WHERE kind='category-split-updates'`))
	require.Equal(t, 1, h.sqlCount(stagedDB, `SELECT count(*) FROM advisory_groups WHERE kind='category-split-updates'`))
	cfg.Exclude = append(cfg.Exclude, "source.go")
	require.Empty(t, h.investigate(dir, cfg).CategorySplit)
}
func TestCategorySplitPromotedFieldAndShadow(t *testing.T) {
	h := &testHarness{T: t}
	for _, source := range []string{
		`type Embedded struct{Kind string};type Wrap struct{Embedded;N int};func f(r *Wrap){a,b:=0,0;if r.Kind=="a"{a=r.N};if r.Embedded.Kind=="a"{b=r.N};_,_=a,b}`,
		`type X struct{Kind string;N int};func f(r,other *X){a,b:=0,0;if r.Kind=="a"{a=r.N};if r.Kind=="other"{}else if r:=other;r.Kind=="a"{b=r.N};_,_=a,b}`,
	} {
		require.Empty(t, h.investigate(h.fixture("package fixture;"+source), quiet()).CategorySplit)
	}
}
func TestCategorySplitDeterministicEvidence(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(splitPrelude + splitFirst + `;` + splitSecond + `}`)
	first, second := h.investigate(dir, quiet()), h.investigate(dir, quiet())
	require.Equal(t, first.CategorySplit, second.CategorySplit)
	require.True(t, strings.Contains(first.CategorySplit[0].values[0].value, "comedy"))
}

func TestCategorySplitRepeatedCategorySnapshot(t *testing.T) {
	h := &testHarness{T: t}
	source := splitPrelude + `c:=0;_=c;if play.Kind=="comedy"{a=play.Audience}else if play.Kind=="comedy"{c=play.Audience};` + splitSecond + `}`
	report := h.investigate(h.fixture(source), quiet())
	require.Len(t, report.CategorySplit, 2)
	require.NotEqual(t, report.CategorySplit[0].id, report.CategorySplit[1].id)
	db := h.snapshot(report)
	require.Equal(t, 2, h.sqlCount(db, `SELECT count(*) FROM advisory_groups WHERE kind='category-split-updates'`))
}
