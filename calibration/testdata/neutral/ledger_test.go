package neutral

import "testing"

func TestLedger(t *testing.T) {
	var l Ledger
	l.record("a", 2, 1.5)
	if l.total != 3 || quote("a", 2, 1.5) != "a: 3.00" || !reserve("a", 1, 0) {
		t.Fatal("ledger arithmetic changed")
	}
	if lo, hi, avg := summarize([]float64{1, 3}); lo != 1 || hi != 3 || avg != 2 {
		t.Fatal("summarize changed")
	}
	if lo, hi, avg := spread([]float64{1, 3}); lo != 1 || hi != 3 || avg != 2 {
		t.Fatal("spread changed")
	}
}
