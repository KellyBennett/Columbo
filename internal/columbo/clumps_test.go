package columbo

import (
	"fmt"
	"strings"
)

// clumpSource gives each parameter its own named type, shared by three functions.
func clumpSource(size int) string {
	var source strings.Builder
	source.WriteString("package fixture\n")
	params := clumpParameters(&source, size)
	for _, name := range []string{"One", "Two", "Three"} {
		fmt.Fprintf(&source, "func %s(%s){}\n", name, strings.Join(params, ","))
	}
	return source.String()
}
func clumpParameters(source *strings.Builder, size int) []string {
	params := []string{}
	for i := 0; i < size; i++ {
		name := fmt.Sprintf("A%02d", i)
		fmt.Fprintf(source, "type %s struct{}\n", name)
		params = append(params, fmt.Sprintf("p%d %s", i, name))
	}
	return params
}
