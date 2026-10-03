// releasecheck is the public-release gate; ordinary dogfooding builds remain valid.
package main

import (
	"github.com/KellyBennett/Columbo/internal/columbo"
	"os"
)

func main() { os.Exit(columbo.ReleaseMain()) }
