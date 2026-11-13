package operators_test

import (
	"strings"
	"testing"
)

func outcomes(rows []row) map[string]row {
	out := map[string]row{}
	for _, r := range rows {
		out[r.Operator+" "+r.Symbol[strings.LastIndex(r.Symbol, ".")+1:]] = r
	}
	return out
}

func TestMoveMethodInjectsEnvyAndEvadesAreMeasured(t *testing.T) {
	rows := runFixture(t, "envy", "move-method-to-function", "move-method-to-collaborator", "copy-fields-to-locals", "hide-behind-getters")
	for _, r := range rows {
		if r.Predicted == nil || *r.Predicted != (r.Status == "killed") {
			t.Errorf("%s %s: status %s disagrees with prediction (%s)", r.Operator, r.Symbol, r.Status, r.Reason)
		}
	}
	got := outcomes(rows)
	want := map[string]string{
		"move-method-to-function Total":       "killed",
		"move-method-to-function Remind":      "killed",
		"move-method-to-function Label":       "equivalent",
		"move-method-to-collaborator Remind":  "killed",
		"copy-fields-to-locals invoiceTotal":  "survived",
		"copy-fields-to-locals invoiceRemind": "killed",
		"hide-behind-getters invoiceTotal":    "killed",
		"hide-behind-getters invoiceRemind":   "killed",
		"move-method-to-function Audit":       "killed",
		"copy-fields-to-locals invoiceAudit":  "killed",
		"hide-behind-getters invoiceAudit":    "killed",
	}
	for key, status := range want {
		if got[key].Status != status {
			t.Errorf("%s: got %q (%s), want %s", key, got[key].Status, got[key].Reason, status)
		}
	}
	if _, ok := got["move-method-to-function String"]; ok {
		t.Error("String satisfies fmt.Stringer and must not be moved")
	}
	if len(rows) != len(want) {
		t.Errorf("%d mutants, want %d", len(rows), len(want))
	}
}

func TestMoveFunctionCuresEnvyOntoTheEnviedType(t *testing.T) {
	rows := runFixture(t, "envy", "move-function")
	got := outcomes(rows)
	for _, key := range []string{"move-function Address", "move-function shippingLabel"} {
		if r := got[key]; r.Status != "killed" || r.Predicted == nil || !*r.Predicted {
			t.Errorf("%s: got %q (%s), want a predicted, clean cure", key, r.Status, r.Reason)
		}
	}
	if len(rows) != 2 {
		t.Errorf("%d mutants, want 2", len(rows))
	}
}
