package dup

import "testing"

func TestScore(t *testing.T) {
	if got := Score([]int{3, 4, -1, 5}); got != 15 {
		t.Errorf("Score = %d", got)
	}
}

func TestSign(t *testing.T) {
	if Sign(-4) != -1 || Sign(4) != 1 {
		t.Error("Sign")
	}
}

func TestMeans(t *testing.T) {
	xs := []float64{2, -1, 4, 6}
	if Mean(xs) != 3 || Average(xs) != 3 || Mean(nil) != 0 || Average(nil) != 0 {
		t.Errorf("Mean = %v, Average = %v", Mean(xs), Average(xs))
	}
}

func TestSizes(t *testing.T) {
	rs := []rect{{2, 3}, {0, 5}}
	if Widths(rs) != 6 || Heights(rs) != 11 {
		t.Errorf("Widths = %d, Heights = %d", Widths(rs), Heights(rs))
	}
}
