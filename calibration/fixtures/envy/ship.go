package envy

func Ship(inv *Invoice, c Customer) string {
	return inv.Address(&c) + "\n" + shippingLabel(c, 1) + inv.Label()
}
