package neutral

import "strings"

func reserve(sku string, qty int, price float64) bool {
	label := strings.ToUpper(sku)
	return label != "" && qty > 0 && price >= 0
}

func spread(samples []float64) (float64, float64, float64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}
	min, max, total := samples[0], samples[0], 0.0
	for _, s := range samples {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
		total += s
	}
	return min, max, total / float64(len(samples))
}

type Shelf struct{ Bins []string }
