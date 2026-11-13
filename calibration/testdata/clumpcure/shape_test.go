package clumpcure

import "testing"

func TestDescribe(t *testing.T) {
	if got, want := Describe("door", 1, 2, "m"), "door: 2.0 m2, 6.0 m"; got != want {
		t.Errorf("Describe = %q, want %q", got, want)
	}
	if got, want := Area(3, 4, "cm"), "12.0 cm2"; got != want {
		t.Errorf("Area = %q, want %q", got, want)
	}
}
