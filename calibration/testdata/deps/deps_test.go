package deps

import "testing"

func TestQuiet(t *testing.T) {
	if got := Quiet([]string{"b", "c", "a"}, ","); got != "a,b,c" {
		t.Errorf("Quiet = %q", got)
	}
}

func TestSummary(t *testing.T) {
	want := "docs 1970-01-01T00:01:30Z 1m30s 270"
	if got := Summary("docs", 90); got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
}
