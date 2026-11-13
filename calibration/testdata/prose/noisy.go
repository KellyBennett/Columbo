//go:build go1.21

package prose

import "strconv"

// Label renders a count for display.
func Label(n int) string {
	s := strconv.Itoa(n) /* digits only */
	//nolint:gosec // the conversion cannot overflow
	return s + " items" // suffix is fixed
}
