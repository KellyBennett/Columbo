package envy

import "testing"

func invoice() Invoice {
	return Invoice{Lines: []Line{{2, 100}, {1, 50}}, Discount: 50, TaxRate: 10, Shipping: 7, Currency: "EUR"}
}

var ada = Customer{Name: "Ada", Street: "1 Loop", City: "Turin", Zip: "10100", Country: "IT"}

func TestTotal(t *testing.T) {
	inv := invoice()
	if got := inv.Total(); got != 227 {
		t.Errorf("Total = %d, want 227", got)
	}
}

func TestLabelAndString(t *testing.T) {
	inv := invoice()
	if got := inv.Label(); got != "227 EUR, 2 lines, 50 off" {
		t.Errorf("Label = %q", got)
	}
	if got := inv.String(); got != "[{2 100} {1 50}] 50 10 7 EUR" {
		t.Errorf("String = %q", got)
	}
}

func TestShip(t *testing.T) {
	inv := invoice()
	want := "EUR: Ada, 1 Loop, Turin 10100, IT\nAda\n1 Loop\nTurin 10100\nIT\n227 EUR, 2 lines, 50 off"
	if got := Ship(&inv, ada); got != want {
		t.Errorf("Ship = %q, want %q", got, want)
	}
	if got := shippingLabel(ada, 2); len(got) != 2*len("Ada\n1 Loop\nTurin 10100\nIT\n") {
		t.Errorf("shippingLabel copies = %q", got)
	}
}

func TestRemind(t *testing.T) {
	inv := invoice()
	if got := inv.Remind(ada); got != "Ada: EUR 7/10/50 x2" {
		t.Errorf("Remind = %q", got)
	}
}

func TestAudit(t *testing.T) {
	inv := invoice()
	var got []string
	inv.Audit(func(s string) { got = append(got, s) })
	if len(got) != 2 || got[0] != "EUR501072" || got[1] != "EUR2" {
		t.Errorf("Audit logged %q", got)
	}
}
