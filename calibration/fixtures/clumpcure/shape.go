package clumpcure

import "fmt"

func Area(width, height float64, unit string) string {
	return fmt.Sprintf("%.1f %s2", width*height, unit)
}

func Perimeter(width, height float64, unit string) string {
	return fmt.Sprintf("%.1f %s", 2*(width+height), unit)
}

func Describe(name string, width, height float64, unit string) string {
	return name + ": " + Area(width, height, unit) + ", " + Perimeter(width, height, unit)
}
