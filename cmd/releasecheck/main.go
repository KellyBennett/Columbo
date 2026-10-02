// releasecheck is the public-release gate; ordinary dogfooding builds remain valid.
package main

import (
	"fmt"
	"github.com/KellyBennett/Columbo/internal/columbo"
	"os"
)

func main() {
	if e := columbo.ValidatePublicRelease(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
