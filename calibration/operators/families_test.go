package operators_test

import (
	"fmt"
	"testing"
)

type want struct {
	operator, symbol string
	dose             int
	status           string
}

func expectRows(t *testing.T, rows []row, wants []want) {
	t.Helper()
	got := map[string]row{}
	for _, r := range rows {
		got[fmt.Sprintf("%s %s %d", r.Operator, r.Symbol, r.Dose)] = r
		if r.Class == "inject" && (r.Predicted == nil || *r.Predicted != (r.Status == "killed")) {
			t.Errorf("%s %s dose %d: status %s disagrees with prediction (%s)", r.Operator, r.Symbol, r.Dose, r.Status, r.Reason)
		}
	}
	for _, w := range wants {
		key := fmt.Sprintf("%s %s %d", w.operator, w.symbol, w.dose)
		r, ok := got[key]
		if !ok {
			t.Errorf("%s: no mutant", key)
			continue
		}
		if r.Status != w.status {
			t.Errorf("%s: %s (%s), want %s", key, r.Status, r.Reason, w.status)
		}
	}
	if len(rows) != len(wants) {
		t.Errorf("%d mutants, want %d", len(rows), len(wants))
	}
}

func TestCollaboratorsFireAboveTheLimitAndAFacadeHidesThem(t *testing.T) {
	rows := runFixture(t, "deps", "add-collaborators", "route-through-facade", "split-by-collaborators")
	expectRows(t, rows, []want{
		{"add-collaborators", "example.com/deps.Quiet", 3, "equivalent"},
		{"add-collaborators", "example.com/deps.Quiet", 4, "killed"},
		{"route-through-facade", "example.com/deps.Quiet", 1, "survived"},
		{"split-by-collaborators", "example.com/deps.Summary", 1, "killed"},
	})
}

func TestProseIsCaughtAStringConstantHidesItAndDeletionCures(t *testing.T) {
	rows := runFixture(t, "prose", "add-prose-comment", "prose-to-string-constant", "delete-prose-comments")
	const clause = "example.com/prose.<package-clause>"
	expectRows(t, rows, []want{
		{"add-prose-comment", clause, 1, "killed"},
		{"prose-to-string-constant", clause, 1, "survived"},
		{"delete-prose-comments", clause, 1, "killed"},
	})
}

func TestDuplicateSurvivesRenamesAndDelegationCuresExactCopies(t *testing.T) {
	rows := runFixture(t, "dup", "duplicate-block", "rename-and-relabel", "delegate-duplicate")
	expectRows(t, rows, []want{
		{"duplicate-block", "example.com/dup.Score", 1, "killed"},
		{"duplicate-block", "example.com/dup.Sign", 1, "equivalent"},
		{"duplicate-block", "example.com/dup.Average", 1, "equivalent"},
		{"rename-and-relabel", "example.com/dup.scoreDup", 1, "killed"},
		{"delegate-duplicate", "example.com/dup.Mean", 1, "killed"},
	})
}
