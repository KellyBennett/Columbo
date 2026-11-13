package neutral

import "fmt"

type Ledger struct {
	entries []string
	total   float64
}

func (l *Ledger) record(sku string, qty int, price float64) {
	line := fmt.Sprintf("%s x%d @ %.2f", sku, qty, price)
	l.entries = append(l.entries, line)
	l.total += float64(qty) * price
}

func quote(sku string, qty int, price float64) string {
	amount := float64(qty) * price
	return fmt.Sprintf("%s: %.2f", sku, amount)
}

func summarize(values []float64) (float64, float64, float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	lo, hi, sum := values[0], values[0], 0.0
	for _, v := range values {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
		sum += v
	}
	return lo, hi, sum / float64(len(values))
}

const currency = "EUR"
