package cosmetic

import "testing"

func TestTotal(t *testing.T) {
	cases := []struct {
		o    Order
		want int
	}{
		{Order{Items: []int{100, 200}, Discount: 50, Tax: 10, Shipping: 7}, 282},
		{Order{Items: []int{900, 300}, Discount: 0, Tax: 0, Shipping: 7}, 1200},
		{Order{Items: []int{10}, Discount: 99, Tax: 50, Shipping: 1}, 1},
	}
	for _, c := range cases {
		if got := Total(c.o); got != c.want {
			t.Errorf("Total(%+v) = %d, want %d", c.o, got, c.want)
		}
	}
}
