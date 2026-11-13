package deps

import (
	"bytes"
	"math/big"
	"net/url"
	"sort"
	"strings"
	"time"
)

func Quiet(words []string, sep string) string {
	sorted := append([]string{}, words...)
	sort.Strings(sorted)
	return strings.Join(sorted, sep)
}

func Summary(name string, n int) string {
	var u url.URL
	u.Path = name
	when := time.Unix(int64(n), 0).UTC()
	age := time.Duration(n) * time.Second
	stamp := u.String() + " " + when.Format(time.RFC3339) + " " + age.String()
	var b strings.Builder
	b.WriteString(stamp)
	var buf bytes.Buffer
	buf.WriteString(b.String())
	var z big.Int
	z.SetInt64(int64(n) * 3)
	return buf.String() + " " + z.String()
}
