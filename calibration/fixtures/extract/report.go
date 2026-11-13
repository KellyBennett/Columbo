package extract

import "fmt"

type Line struct {
	Name  string
	Price int
	Qty   int
}

func Summary(lines []Line) string {
	total := 0
	count := 0
	for _, l := range lines {
		total += l.Price * l.Qty
		count += l.Qty
	}
	header := "items: "
	header += fmt.Sprint(count)
	header += "\n"
	footer := "total: "
	footer += fmt.Sprint(total)
	footer += "\n"
	out := header + footer
	return out
}
