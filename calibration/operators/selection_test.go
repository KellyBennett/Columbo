package operators_test

import "testing"

func TestSelectThenUseKillsLaunderingEvadesAndExtractFactoryCures(t *testing.T) {
	rows := runFixture(t, "selection", "select-then-use", "launder-selection-through-role", "extract-factory")
	got := map[string]map[string]string{}
	for _, r := range rows {
		if got[r.Class] == nil {
			got[r.Class] = map[string]string{}
		}
		got[r.Class][r.Symbol] = r.Status
	}
	want := map[string]map[string]string{
		"inject": {"example.com/selection.Notify": "killed", "example.com/selection.Alert": "killed"},
		"evade":  {"example.com/selection.Notify": "survived", "example.com/selection.Alert": "survived"},
		"cure":   {"example.com/selection.Broadcast": "killed"},
	}
	for class, bySymbol := range want {
		for symbol, status := range bySymbol {
			if got[class][symbol] != status {
				t.Errorf("%s %s: %q, want %q", class, symbol, got[class][symbol], status)
			}
		}
	}
	for _, r := range rows {
		if r.Status == "invalid" {
			t.Errorf("%s %s invalid: %s", r.Operator, r.Symbol, r.Reason)
		}
	}
}
