//go:generate true

package prose

import "strconv"

// Label renders a count for display.
func Label(n int) string {
	s := strconv.Itoa(n) /* digits only */
	//lint:ignore SA4006 the conversion cannot overflow
	return s + " items" // suffix is fixed
}
