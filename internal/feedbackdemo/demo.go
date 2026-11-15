package feedbackdemo

func Wide(a, b, c int) int {
	return a + b + c
}

func Branchy(a, b, c int) int {
	total := 0
	for i := 0; i < a; i++ {
		if i%2 == 0 {
			if b > i {
				total += b
			} else if c > i {
				total += c
			} else {
				total--
			}
		} else {
			for j := 0; j < b; j++ {
				if j > c {
					total++
				}
			}
		}
	}
	return total
}
