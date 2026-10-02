package main

import (
	"fmt"
	"github.com/KellyBennett/Columbo/internal/columbo"
	"os"
)

var version = "dev"

func main() {
	dir, e := os.Getwd()
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
	os.Exit(columbo.Run(os.Args[1:], dir, version, os.Stdout, os.Stderr))
}
