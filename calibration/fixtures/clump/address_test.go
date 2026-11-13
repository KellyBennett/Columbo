package clump

import "testing"

func TestShip(t *testing.T) {
	cases := []struct {
		o    Order
		want string
	}{
		{Order{ID: 1, To: Address{Street: "1 Main", City: "Oslo", Zip: 150}}, "1 Main, OSLO via 1/7/OSLO/150"},
		{Order{ID: 2, To: Address{City: "Oslo", Zip: 150}}, "invalid"},
		{Order{ID: 3, To: Address{Street: "9 Elm", City: "rome"}}, "invalid"},
	}
	for _, c := range cases {
		if got := Ship(c.o); got != c.want {
			t.Errorf("Ship(%+v) = %q, want %q", c.o, got, c.want)
		}
	}
	if !valid(Address{Street: "x", Zip: 1}) {
		t.Error("valid rejected a complete address")
	}
}
