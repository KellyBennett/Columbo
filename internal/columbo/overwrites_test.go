package columbo

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWitnessedConditionalOverwrite(t *testing.T) {
	tests := []struct {
		name, body string
		count      int
	}{
		{"intervening unconditional write", `if x>y{s="a"};s="b";if x>=y{s="b"}`, 0},
		{"intervening conditional write", `if x>y{s="a"};if x>=y{s="b"};if x>y{s="b"}`, 1},
		{"extra local assignment", `if x>y{n:=1;_ = n;s="a"};if x>=y{s="b"}`, 1},
		{"assignment tuple unsupported", `a,b:=0,0;_ = a;_ = b;if x>y{s="a"};if x>=y{s="b"}`, 0},
		{"blank assignment preserves destination history", `if x>y{s="a"};_ = s;if x>=y{s="b"}`, 1},
		{"sample lower bound", `if x == -2{s="a"};if x <= -2{s="b"}`, 1},
		{"sample upper bound", `if x == 8{s="a"};if x >= 8{s="b"}`, 1},
		{"same decision excluded", `if x>y{s="a";s="b"}`, 0},
		{"last writer in earlier body", `if x>y{s="a";s="b"};if x>=y{s="c"}`, 1},
		{"overlap", `if x>y&&y>=3{s="advantage"};if x>=4&&y>=0&&x-y>=2{s="win"}`, 1},
		{"deliberate priority policy remains ambiguous", `if x>y&&y>=3{s="standard"};if x>=4&&y>=0&&x-y>=2{s="premium"}`, 1},
		{"exclusive", `if x>y{s="a"};if x<=y{s="b"}`, 0},
		{"same result", `if x>y{s="a"};if x>=y{s="a"}`, 0},
		{"accumulation", `if x>y{s+="a"};if x>=y{s+="b"}`, 0},
		{"local changing guard", `z:=x;if z>y{s="a"};z=8;if z>y{s="b"}`, 0},
		{"input mutation", `if x>y{s="a"};x++;if x>=y{s="b"}`, 0},
		{"input mutation followed by local write", `x=5;s="reset";if x>y{s="a"};if x>=y{s="b"}`, 0},
		{"input assignment", `if x>y{s="a"};x=5;if x>=y{s="b"}`, 0},
		{"else", `if x>y{s="a"}else if x>=y{s="b"}`, 0},
		{"early return", `if x>y{return "a"};if x>y{s="a"};if x>=y{s="b"}`, 0},
		{"unknown call", `if x>y{s="a"};println(x);if x>=y{s="b"}`, 0},
		{"escape", `p:=&s;_ = p;if x>y{s="a"};if x>=y{s="b"}`, 0},
		{"shadowed destination", `if x>y{s:="a";_ = s};if x>=y{s="b"}`, 0},
		{"no small witness is inconclusive", `if x>100{s="a"};if x>101{s="b"}`, 0},
		{"unsupported or", `if x>y||x==y{s="a"};if x>=y{s="b"}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture("package fixture\nfunc score(x,y int)string{s:=\"\";"+test.body+";return s}"), quiet())
			require.Len(t, report.ConditionalOverwrites, test.count)
		})
	}
}
func TestOverwriteSnapshotAndStages(t *testing.T) {
	h := &testHarness{T: t}
	source := `package fixture
 func score(x,y int)string{s:="";if x>y&&y>=3{s="advantage"};if x>=4&&y>=0&&x-y>=2{s="win"};return s}`
	report := h.investigate(h.fixture(source), quiet())
	require.Len(t, report.ConditionalOverwrites, 1)
	db := h.snapshot(report)
	require.Equal(t, 1, h.sqlCount(db, "SELECT count(*) FROM advisory_groups WHERE kind='witnessed-conditional-overwrite'"))
	require.Equal(t, 0, h.sqlCount(db, "SELECT count(*) FROM cases"))
	text, code, err := RenderSnapshot(testDatabaseQueries(db), "saved.sqlite")
	require.NoError(t, err)
	require.Zero(t, code)
	require.Contains(t, string(text), "Deliberate override policies are legitimate")
	require.Contains(t, string(text), "witness-input: x (int) 5")
	require.Contains(t, string(text), "witness-input: y (int) 3")
	require.Contains(t, string(text), "witness-path-condition")
	require.Contains(t, string(text), `x>y&&y>=3`)
	require.Contains(t, string(text), `s="advantage"`)
	require.NotEmpty(t, h.sqlStrings(db, "SELECT spelling FROM advisory_receipts WHERE kind='overlap-guard'"))
	cfg := quiet()
	cfg.Staged = true
	staged := h.investigate(h.fixture(source), cfg)
	require.Len(t, staged.ConditionalOverwrites, 1)
	for _, stage := range staged.Stages {
		require.Zero(t, stage.issueCount())
	}
}
func TestOverwriteReceiverAndTypes(t *testing.T) {
	tests := []struct {
		name, source string
		count        int
	}{
		{"receiver", `type Game struct{A,B int};func(g *Game)score()string{s:="";if g.A>g.B&&g.B>=3{s="a"};if g.A>=4&&g.A-g.B>=2{s="b"};return s}`, 1},
		{"receiver input mutation", `type Game struct{A,B int};func(g *Game)score()string{s:="";if g.A>g.B{s="a"};g.A=7;if g.A>=g.B{s="b"};return s}`, 0},
		{"alias mutation", `type Game struct{A,B int};func(g *Game)score()string{s:="";p:=g;if g.A>g.B{s="a"};p.A=7;if g.A>=g.B{s="b"};return s}`, 0},
		{"unsigned unsupported", `func score(x,y uint)string{s:="";if x>y{s="a"};if x>=y{s="b"};return s}`, 0},
		{"named signed", `type Point int8;func score(x,y Point)string{s:="";if x>y{s="a"};if x>=y{s="b"};return s}`, 1},
		{"overflow rejected", `func score(x,y int8)string{s:="";if x<=-2{s="a"};if x-127<y{s="b"};return s}`, 0},
		{"promoted receiver field unsupported", `type Inner struct{A int};type Game struct{Inner};func(g *Game)score()string{s:="";if g.A>0{s="a"};if g.A>=0{s="b"};return s}`, 0},
		{"unrelated receiver constant write", `type Game struct{A,B int};func(g *Game)score()string{s:="";if g.A>0{s="a"};g.B=7;if g.A>=0{s="b"};return s}`, 1},
		{"unrelated receiver dynamic write unsupported", `type Game struct{A,B int};func(g *Game)score()string{s:="";if g.A>0{s="a"};g.B=g.A;if g.A>=0{s="b"};return s}`, 0},
		{"different roots unsupported", `type Game struct{A int};func score(g,h *Game)string{s:="";if g.A>0{s="a"};if h.A>0{s="b"};return s}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture("package fixture\n"+test.source), quiet())
			require.Len(t, report.ConditionalOverwrites, test.count)
		})
	}
}
