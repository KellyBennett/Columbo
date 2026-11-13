package variants

import "testing"

type blob struct{}

func (blob) Name() string { return "blob" }

func TestLabel(t *testing.T) {
	for k, want := range map[Kind]string{Circle: "circle", Square: "square", Triangle: "triangle", Kind(9): "unknown"} {
		if got := Label(k); got != want {
			t.Errorf("Label(%d) = %q, want %q", k, got, want)
		}
	}
}

func TestMeasures(t *testing.T) {
	cases := []struct {
		s               Shape
		area, perimeter float64
	}{
		{&Disc{R: 2}, 12, 24},
		{&Box{W: 3}, 9, 24},
		{blob{}, 0, -1},
		{nil, 0, -1},
	}
	for _, c := range cases {
		if got := Area(c.s); got != c.area {
			t.Errorf("Area(%v) = %v, want %v", c.s, got, c.area)
		}
		if got := Perimeter(c.s, 2); got != c.perimeter {
			t.Errorf("Perimeter(%v) = %v, want %v", c.s, got, c.perimeter)
		}
	}
}
