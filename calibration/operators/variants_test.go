package operators_test

import "testing"

func TestRepeatVariantDecisionKillsFromTwoVariantsAndMapLookupEvades(t *testing.T) {
	rows := runFixture(t, "variants", "repeat-variant-decision", "decision-to-map-lookup")
	statuses := map[string]map[int]string{"inject": {}, "evade": {}}
	for _, r := range rows {
		statuses[r.Class][r.Dose] = r.Status
		if r.Class == "inject" && (r.Predicted == nil || *r.Predicted != (r.Status == "killed")) {
			t.Errorf("inject dose %d: status %s disagrees with prediction (%s)", r.Dose, r.Status, r.Reason)
		}
	}
	want := map[string]map[int]string{
		"inject": {1: "equivalent", 2: "killed", 3: "killed"},
		"evade":  {1: "survived"},
	}
	for class, doses := range want {
		for dose, status := range doses {
			if got := statuses[class][dose]; got != status {
				t.Errorf("%s dose %d: %q, want %q", class, dose, got, status)
			}
		}
	}
}

func TestReplaceConditionalWithPolymorphismCuresTypeSwitchDomain(t *testing.T) {
	rows := runFixture(t, "variants", "replace-conditional-with-polymorphism")
	if len(rows) != 2 {
		t.Fatalf("%d cure mutants, want one per curable decision", len(rows))
	}
	for _, r := range rows {
		if r.Status != "killed" {
			t.Errorf("dose %d: %s (%s)", r.Dose, r.Status, r.Reason)
		}
	}
}
