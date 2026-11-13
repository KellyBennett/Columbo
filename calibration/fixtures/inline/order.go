package inline

type Order struct {
	Items    []int
	Discount int
	Tax      int
	Shipping int
}

func Total(o Order) int {
	sum := subtotal(o.Items)
	sum = discounted(sum, o.Discount)
	sum = taxed(sum, o.Tax)
	return shipped(sum, o.Shipping)
}

func subtotal(items []int) int {
	sum := 0
	for _, it := range items {
		sum += it
	}
	return sum
}

func discounted(sum, discount int) int {
	if discount > sum {
		discount = sum
	}
	return sum - discount
}

func taxed(sum, tax int) int {
	extra := sum * tax
	extra /= 100
	return sum + extra
}

func shipped(sum, shipping int) int {
	if sum > 1000 {
		shipping = 0
	}
	return sum + shipping
}
