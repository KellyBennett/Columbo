package envy

import (
	"fmt"
	"strconv"
)

type Line struct {
	Qty, Price int
}

type Customer struct {
	Name, Street, City, Zip, Country string
}

type Invoice struct {
	Lines    []Line
	Discount int
	TaxRate  int
	Shipping int
	Currency string
}

func (inv *Invoice) Total() int {
	sum := 0
	for _, l := range inv.Lines {
		sum += l.Qty * l.Price
	}
	if inv.Discount > sum {
		sum = 0
	} else {
		sum -= inv.Discount
	}
	sum += sum * inv.TaxRate / 100
	return sum + inv.Shipping
}

func (inv *Invoice) Label() string {
	return fmt.Sprintf("%d %s, %d lines, %d off", inv.Total(), inv.Currency, len(inv.Lines), inv.Discount)
}

func (inv Invoice) String() string {
	return fmt.Sprintf("%v %d %d %d %s", inv.Lines, inv.Discount, inv.TaxRate, inv.Shipping, inv.Currency)
}

func (inv *Invoice) Address(c *Customer) string {
	return inv.Currency + ": " + c.Name + ", " + c.Street + ", " + c.City + " " + c.Zip + ", " + c.Country
}

func shippingLabel(c Customer, copies int) string {
	out := ""
	for i := 0; i < copies; i++ {
		out += c.Name + "\n" + c.Street + "\n" + c.City + " " + c.Zip + "\n" + c.Country + "\n"
	}
	return out
}

func (inv *Invoice) Remind(c Customer) string {
	return c.Name + ": " + inv.Currency + " " + strconv.Itoa(inv.Shipping) + "/" + strconv.Itoa(inv.TaxRate) + "/" + strconv.Itoa(inv.Discount) + " x" + strconv.Itoa(len(inv.Lines))
}

func (inv *Invoice) Audit(log func(string)) {
	log(inv.Currency + strconv.Itoa(inv.Discount) + strconv.Itoa(inv.TaxRate) + strconv.Itoa(inv.Shipping) + strconv.Itoa(len(inv.Lines)))
	again := func() {
		log(inv.Currency + strconv.Itoa(len(inv.Lines)))
	}
	again()
}
