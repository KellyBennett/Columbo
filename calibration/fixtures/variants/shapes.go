package variants

type Kind int

const (
	Circle Kind = iota
	Square
	Triangle
)

func Label(k Kind) string {
	switch k {
	case Circle:
		return "circle"
	case Square:
		return "square"
	case Triangle:
		return "triangle"
	}
	return "unknown"
}

type Shape interface {
	Name() string
}

type Disc struct{ R float64 }

type Box struct{ W float64 }

func (d *Disc) Name() string { return "disc" }

func (b *Box) Name() string { return "box" }

func Area(s Shape) float64 {
	switch v := s.(type) {
	case *Disc:
		return 3 * v.R * v.R
	case *Box:
		return v.W * v.W
	}
	return 0
}

func Perimeter(s Shape, scale float64) float64 {
	switch v := s.(type) {
	case *Disc:
		return 6 * v.R * scale
	case *Box:
		return 4 * v.W * scale
	default:
		return -1
	}
}
