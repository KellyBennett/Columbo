package cosmetic

type Order struct {
	Items    []int
	Discount int
	Tax      int
	Shipping int
}

func Total(o Order) int {
	sum := 0
	for _, it := range o.Items {
		sum += it
	}
	discount := o.Discount
	if discount > sum {
		discount = sum
	}
	sum -= discount
	extra := sum * o.Tax
	extra /= 100
	sum += extra
	shipping := o.Shipping
	if sum > 1000 {
		shipping = 0
	}
	return sum + shipping
}
