package clump

import (
	"fmt"
	"strings"
)

type Address struct {
	Street string
	City   string
	Zip    int
}

type Order struct {
	ID int
	To Address
}

func Ship(o Order) string {
	a := o.To
	if !valid(a) {
		return "invalid"
	}
	a = normalize(a)
	return label(a) + " via " + route(o.ID, 7, &a)
}

func valid(a Address) bool { return a.Street != "" && a.Zip > 0 }

func normalize(a Address) Address {
	a.City = strings.ToUpper(a.City)
	return a
}

func label(a Address) string { return a.Street + ", " + a.City }

func route(id, hub int, a *Address) string {
	return fmt.Sprintf("%d/%d/%s/%d", id, hub, a.City, a.Zip)
}
