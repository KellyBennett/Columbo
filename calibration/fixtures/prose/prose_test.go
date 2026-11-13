package prose

import "testing"

func TestClamp(t *testing.T) {
	for _, c := range []struct{ v, want int }{{-3, 0}, {4, 4}, {12, 10}} {
		if got := Clamp(c.v, 0, 10); got != c.want {
			t.Errorf("Clamp(%d) = %d, want %d", c.v, got, c.want)
		}
	}
}

func TestLabel(t *testing.T) {
	if got := Label(3); got != "3 items" {
		t.Errorf("Label = %q", got)
	}
}
