package trials

import (
	"os"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/operators"
)

func TestEveryNamedCureIsARegisteredCureOperator(t *testing.T) {
	for lead, ref := range leadRefactorings {
		if ref.Operator == "" {
			continue
		}
		ops, err := operators.Select([]string{ref.Operator})
		if err != nil {
			t.Errorf("%q: %v", lead, err)
			continue
		}
		if ops[0].Class() != harness.Cure {
			t.Errorf("%q maps to %s, a %s operator", lead, ref.Operator, ops[0].Class())
		}
		if ref.Name == "" {
			t.Errorf("%q has a cure but names no refactoring", lead)
		}
	}
	for lead, ref := range leadRefactorings {
		if (ref.Name == "") != (ref.Reading == "") {
			t.Errorf("%q maps to %q with reading %q; a refactoring needs a reading and none needs none", lead, ref.Name, ref.Reading)
		}
	}
}

func TestACureThatDeclinesTheCaseIsRefusedNotSiteless(t *testing.T) {
	outcomes := map[LeadOutcome]int{}
	for _, c := range checkFixture(t, "fixtures/dup") {
		outcomes[c.Outcome]++
		if c.Outcome == LeadRefused && !strings.Contains(c.Reason, "delegate-duplicate refused") {
			t.Errorf("refused lead on %s gives reason %q, want the operator's refusal", c.Case.Symbol, c.Reason)
		}
	}
	if outcomes[LeadRefused] == 0 || outcomes[LeadNoSite] == 0 {
		t.Fatalf("fixtures/dup lead outcomes %v, want both a refusal and a cure with no site", outcomes)
	}
}

func TestAProseCaseClearsEvenWhenASiblingFileSharesItsSymbol(t *testing.T) {
	clause := "example.com/p.<package-clause>"
	a := harness.Finding{Smell: "prose-comment", Symbol: clause, File: "a.go", CaseID: "C-aaaaaaaaaa"}
	b := harness.Finding{Smell: "prose-comment", Symbol: clause, File: "b.go", CaseID: "C-bbbbbbbbbb"}
	c := &harness.Corpus{Baseline: harness.Snapshot{Findings: []harness.Finding{a, b}}}
	cure := &application{
		applied: harness.Applied{Edit: harness.Edit{"a.go": []byte("package p\n")}, Expectation: harness.Expectation{Touched: []string{clause}}},
		probe:   harness.Probe{Snapshot: harness.Snapshot{Findings: []harness.Finding{b}}},
	}
	if got, why := cure.outcomeFor(c, a); got != LeadCleared {
		t.Fatalf("deleting a.go's prose left %s (%s), want cleared", got, why)
	}
	if got, _ := cure.outcomeFor(c, b); got != LeadSurvived {
		t.Fatalf("b.go's untouched prose case scored %s, want survived", got)
	}
}

func TestADataClumpThatOnlyChangedItsIDSurvives(t *testing.T) {
	before := harness.Finding{Smell: "data-clump", Symbol: "example.com/p.Ship", File: "p.go", CaseID: "C-aaaaaaaaaa"}
	after := before
	after.CaseID = "C-cccccccccc"
	c := &harness.Corpus{Baseline: harness.Snapshot{Findings: []harness.Finding{before}}}
	cure := &application{
		applied: harness.Applied{Edit: harness.Edit{"p.go": nil}, Expectation: harness.Expectation{Touched: []string{before.Symbol}}},
		probe:   harness.Probe{Snapshot: harness.Snapshot{Findings: []harness.Finding{after}}},
	}
	if got, _ := cure.outcomeFor(c, before); got != LeadSurvived {
		t.Fatalf("a clump that kept its anchor under a new ID scored %s, want survived", got)
	}
}

func checkFixture(t *testing.T, fixture string) []LeadCheck {
	t.Helper()
	env := fixtureEnv(t)
	src, err := Resolve(fixture, "..")
	if err != nil {
		t.Fatal(err)
	}
	c, err := env.PrepareCorpus(src.Dir, nil, src.Module, src.Version)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(c.Root)
	checks, err := LeadChecker{Env: env, Workers: 4}.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	return checks
}

func TestLeadsThatNameACureClearTheirOwnCases(t *testing.T) {
	want := map[string]map[string]LeadOutcome{
		"fixtures/envy":     {"Consider moving cohesive behavior toward the owner": LeadCleared, "Review each qualifying foreign value": LeadNoRefactoring},
		"fixtures/prose":    {"Remove obsolete explanations": LeadCleared, "write a regression test": LeadNoRefactoring},
		"fixtures/variants": {"interchangeable role players": LeadCleared, "evaluate a visitor": LeadNoOperator, "Move behavior toward the object or role": LeadNoSite},
		"fixtures/extract":  {"Look for cohesive responsibilities": LeadCleared, "Move behavior toward the object that owns": LeadNoSite},
	}
	for fixture, leads := range want {
		checks := checkFixture(t, fixture)
		for _, c := range checks {
			if c.Outcome == LeadUnclassified {
				t.Errorf("%s: unclassified lead %q", fixture, c.Lead)
			}
		}
		for fragment, outcome := range leads {
			found := false
			for _, c := range checks {
				if strings.Contains(c.Lead, fragment) {
					found = true
					if c.Outcome != outcome {
						t.Errorf("%s: %q on %s was %s (%s), want %s", fixture, fragment, c.Case.Symbol, c.Outcome, c.Reason, outcome)
					}
				}
			}
			if !found {
				t.Errorf("%s: no case carries a lead containing %q", fixture, fragment)
			}
		}
	}
}
