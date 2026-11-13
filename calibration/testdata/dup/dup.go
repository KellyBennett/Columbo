package dup

type rect struct{ w, h int }

func Score(xs []int) int {
	total := 0
	for i, x := range xs {
		if x < 0 {
			continue
		}
		if i%2 == 0 {
			total += x * 2
		} else {
			total += x
		}
	}
	if total > 100 {
		total = 100
	}
	return total
}

func Sign(x int) int {
	if x < 0 {
		return -1
	}
	return 1
}

func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		if x > 0 {
			sum += x
		}
	}
	if sum == 0 {
		return 0
	}
	return sum / float64(len(xs))
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	acc := 0.0
	for _, v := range values {
		if v > 0 {
			acc += v
		}
	}
	if acc == 0 {
		return 0
	}
	return acc / float64(len(values))
}

func Widths(rs []rect) int {
	n := 0
	for _, r := range rs {
		if r.w > 0 && r.h > 0 {
			n += r.w * 2
		}
	}
	if n > 50 {
		n = 50
	}
	return n + len(rs)
}

func Heights(rs []rect) int {
	n := 0
	for _, r := range rs {
		if r.h > 0 && r.w > 0 {
			n += r.h * 3
		}
	}
	if n > 70 {
		n = 70
	}
	return n + len(rs)
}

func Average(values []float64) float64 { return average(values) }
