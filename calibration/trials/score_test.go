package trials

import (
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

var (
	oldCase     = harness.Finding{Smell: "long-function", Symbol: "example.com/p.Old", CaseID: "C-aaaaaaaaaa"}
	plantedCase = harness.Finding{Smell: "duplicate-code", Symbol: "example.com/p.New", CaseID: "C-bbbbbbbbbb"}
	sideCase    = harness.Finding{Smell: "long-function", Symbol: "example.com/p.New", CaseID: "C-cccccccccc"}
)

func scored(baseline, mutant, fixed []harness.Finding) Score {
	p := &Plant{Smell: plantedCase.Smell, Corpus: &harness.Corpus{Baseline: harness.Snapshot{Findings: baseline}},
		Mutant: harness.Snapshot{Findings: mutant}, Added: []harness.Finding{plantedCase, sideCase}}
	var s Score
	s.Analyzed = true
	s.compare(p, harness.Snapshot{Findings: fixed})
	return s
}

func TestAFixThatAlsoRemovesAnOutOfScopeCaseSaysSo(t *testing.T) {
	s := scored([]harness.Finding{oldCase}, []harness.Finding{oldCase, plantedCase, sideCase}, nil)
	if !s.Cleared() || len(s.OutOfScopeGone) != 1 || s.OutOfScopeGone[0].Symbol != oldCase.Symbol {
		t.Fatalf("score %+v, want cleared with the old case listed as out of scope and gone", s)
	}
}

func TestACaseTheMutantAlreadyRemovedIsNotChargedToTheAgent(t *testing.T) {
	s := scored([]harness.Finding{oldCase}, []harness.Finding{plantedCase, sideCase}, nil)
	if len(s.OutOfScopeGone) != 0 {
		t.Fatalf("out of scope gone %v, want none: the agent never saw that case", s.OutOfScopeGone)
	}
}

func TestRemainingCasesSplitIntoTargetSideAndIntroduced(t *testing.T) {
	introduced := harness.Finding{Smell: "feature-envy", Symbol: "example.com/p.Helper", CaseID: "C-dddddddddd"}
	s := scored(nil, []harness.Finding{plantedCase, sideCase}, []harness.Finding{plantedCase, sideCase, introduced})
	if s.Cleared() || s.TargetRemaining != 1 || s.OtherRemaining != 1 || len(s.Introduced) != 1 {
		t.Fatalf("score %+v, want target 1, other 1, introduced 1", s)
	}
}
