package operators_test

import "testing"

func TestInlineParameterObjectKillsAtOccurrences(t *testing.T) {
	rows := runFixture(t, "clump", "inline-parameter-object", "inline-parameter-object-lpl", "pack-anonymous-struct", "pack-anonymous-struct-lpl", "reorder-params")
	firstKill := 0
	byOp := map[string][]row{}
	for _, r := range rows {
		byOp[r.Operator] = append(byOp[r.Operator], r)
		if r.Class != "inject" {
			continue
		}
		if r.Predicted == nil || *r.Predicted != (r.Status == "killed") {
			t.Errorf("%s dose %d: status %s disagrees with prediction (%s)", r.Operator, r.Dose, r.Status, r.Reason)
		}
		if r.Operator == "inline-parameter-object" && r.Status == "killed" && firstKill == 0 {
			firstKill = r.Dose
		}
	}
	if firstKill != 3 {
		t.Errorf("data-clump first fired at dose %d, want 3 (data-clump-occurrences)", firstKill)
	}
	if lpl := byOp["inline-parameter-object-lpl"]; len(lpl) != 1 || lpl[0].Status != "killed" {
		t.Errorf("long-parameter-list inject rows = %+v, want one kill", lpl)
	}
	for _, r := range byOp["reorder-params"] {
		if r.Status != "killed" {
			t.Errorf("reorder-params dose %d %s: %s", r.Dose, r.Status, r.Reason)
		}
	}
	// Columbo compares parameter types, so one anonymous struct parameter hides both the
	// clump and the long list. This pins the blind spot until a rule notices it.
	for _, op := range []string{"pack-anonymous-struct", "pack-anonymous-struct-lpl"} {
		if len(byOp[op]) == 0 {
			t.Errorf("%s never ran on a killed inject", op)
		}
		for _, r := range byOp[op] {
			if r.Status != "survived" {
				t.Errorf("%s dose %d: %s (%s), want survived", op, r.Dose, r.Status, r.Reason)
			}
		}
	}
}

func TestIntroduceParameterObjectReachesGreen(t *testing.T) {
	rows := runFixture(t, "clumpcure", "introduce-parameter-object")
	if len(rows) != 1 {
		t.Fatalf("got %d cure rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].Status != "killed" {
		t.Errorf("cure %s: %s", rows[0].Status, rows[0].Reason)
	}
}
