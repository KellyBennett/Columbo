package fixture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func LongFunction() {
	fmt.Println(1)
	fmt.Println(2)
	fmt.Println(3)
	fmt.Println(4)
	fmt.Println(5)
	fmt.Println(6)
	fmt.Println(7)
	fmt.Println(8)
	fmt.Println(9)
	fmt.Println(10)
	fmt.Println(11)
}
func Parameters(a, b, c, d, e int) {}
func Complex(a, b bool) {
	if a {
		if b {
			for a {
				if b {
					break
				}
			}
		}
	}
}
func Dependencies(b bytes.Buffer, r strings.Reader, t time.Time, u url.URL, d json.Decoder, re regexp.Regexp) {
}

type Own struct{ N int }
type Foreign struct{ N int }

func (o Own) Envy(f Foreign) int {
	return f.N + f.N + f.N + f.N + f.N
}
func ClumpA(a int, b string, c bool) {}
func ClumpB(a int, b string, c bool) {}
func ClumpC(a int, b string, c bool) {}
func Parent(a, b int) {
	stepOne(a, b)
	stepTwo(a, b)
}
func stepOne(a, b int) {
	fmt.Println(a)
	fmt.Println(b)
	fmt.Println(a)
	fmt.Println(b)
	fmt.Println(a)
	fmt.Println(b)
}
func stepTwo(a, b int) {
	fmt.Println(a)
	fmt.Println(b)
	fmt.Println(a)
	fmt.Println(b)
	fmt.Println(a)
	fmt.Println(b)
}
