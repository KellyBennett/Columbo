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
