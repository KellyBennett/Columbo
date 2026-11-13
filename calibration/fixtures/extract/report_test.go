package extract

import "testing"

func TestSummary(t *testing.T) {
	got := Summary([]Line{{"a", 3, 2}, {"b", 5, 1}})
	if got != "items: 3\ntotal: 11\n" {
		t.Fatalf("got %q", got)
	}
}
