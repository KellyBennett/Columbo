package harness

import (
	"strings"
	"testing"
)

const pkg = "example.com/m."

func declared(smell, file, symbol string) Finding {
	return Finding{Smell: smell, File: file, Symbol: pkg + symbol, CaseID: caseID(smell, file, pkg+symbol, ""),
		Support: []Decl{{File: file, Symbol: pkg + symbol}}}
}

func clump(id string, anchor Decl, support ...Decl) Finding {
	return Finding{Smell: "data-clump", File: anchor.File, Symbol: anchor.Symbol, CaseID: id, Support: append([]Decl{anchor}, support...)}
}

func neutral(exp Expectation, before, after []Finding) Outcome {
	o := Outcome{Class: Neutral}
	Score(&o, exp, before, after)
	return o
}

func TestCaseIDMatchesColumbo(t *testing.T) {
	got := caseID("feature-envy", "internal/columbo/advisory_snapshot.go", "github.com/KellyBennett/Columbo/internal/columbo.advisoryReceipt", "")
	if got != "C-075853fe3e" {
		t.Fatalf("caseID = %s, want Columbo's C-075853fe3e", got)
	}
}

func TestNeutralAcceptsPredictedRenameAndMove(t *testing.T) {
	before := []Finding{declared("long-function", "a.go", "run"), declared("feature-envy", "a.go", "load")}
	after := []Finding{declared("long-function", "a.go", "runRenamed"), declared("feature-envy", "moved.go", "load")}
	o := neutral(Expectation{Renames: map[string]string{pkg + "run": pkg + "runRenamed"}, Moves: map[string]string{pkg + "load": "moved.go"}}, before, after)
	if o.Status != Equivalent {
		t.Fatalf("status %s: %s", o.Status, o.Reason)
	}
}

func TestNeutralFlagsUnpredictedIDChange(t *testing.T) {
	before := []Finding{declared("long-function", "a.go", "run")}
	after := []Finding{declared("long-function", "b.go", "run")}
	o := neutral(Expectation{}, before, after)
	if o.Status != IDDrift || !strings.Contains(o.Reason, "changed ID") {
		t.Fatalf("status %s: %s", o.Status, o.Reason)
	}
}

func TestNeutralFlagsRenameToTheWrongSymbol(t *testing.T) {
	before := []Finding{declared("long-function", "a.go", "run")}
	after := []Finding{declared("long-function", "a.go", "other")}
	o := neutral(Expectation{Renames: map[string]string{pkg + "run": pkg + "runRenamed"}}, before, after)
	if o.Status != IDDrift {
		t.Fatalf("status %s: %s", o.Status, o.Reason)
	}
}

func TestNeutralLetsAnAnchorMoveToTheNewEarliestDeclaration(t *testing.T) {
	a, b := Decl{File: "a.go", Symbol: pkg + "f", Offset: 10}, Decl{File: "b.go", Symbol: pkg + "g", Offset: 10}
	before := []Finding{clump("C-1", a, b)}
	movedA := Decl{File: "z.go", Symbol: a.Symbol, Offset: 10}
	after := []Finding{clump("C-2", b, movedA)}
	o := neutral(Expectation{Moves: map[string]string{a.Symbol: "z.go"}}, before, after)
	if o.Status != Equivalent {
		t.Fatalf("status %s: %s", o.Status, o.Reason)
	}
}

func TestNeutralFlagsAnAnchorThatIsNotEarliest(t *testing.T) {
	a, b := Decl{File: "a.go", Symbol: pkg + "f", Offset: 10}, Decl{File: "a.go", Symbol: pkg + "g", Offset: 90}
	before := []Finding{clump("C-1", a, b)}
	after := []Finding{clump("C-2", Decl{File: "a.go", Symbol: b.Symbol, Offset: 90}, Decl{File: "a.go", Symbol: a.Symbol, Offset: 10})}
	o := neutral(Expectation{}, before, after)
	if o.Status != IDDrift || !strings.Contains(o.Reason, "not its earliest") {
		t.Fatalf("status %s: %s", o.Status, o.Reason)
	}
}

func TestNeutralFlagsAKeptAnchorWithANewID(t *testing.T) {
	a, b := Decl{File: "a.go", Symbol: pkg + "f", Offset: 10}, Decl{File: "b.go", Symbol: pkg + "g", Offset: 10}
	o := neutral(Expectation{}, []Finding{clump("C-1", a, b)}, []Finding{clump("C-2", a, b)})
	if o.Status != IDDrift || !strings.Contains(o.Reason, "kept anchor") {
		t.Fatalf("status %s: %s", o.Status, o.Reason)
	}
}

func TestNeutralFlagsAnAddedCase(t *testing.T) {
	before := []Finding{declared("long-function", "a.go", "run")}
	after := append(before, declared("long-function", "a.go", "walk"))
	o := neutral(Expectation{}, before, after)
	if o.Status != IDDrift || !strings.Contains(o.Reason, "unexplained") {
		t.Fatalf("status %s: %s", o.Status, o.Reason)
	}
}
