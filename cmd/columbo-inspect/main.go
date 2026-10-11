package main

import (
	"flag"
	"fmt"
	"github.com/KellyBennett/Columbo/internal/columbo"
	"os"
)

func main() {
	root := flag.String("root", ".", "source module root")
	config := flag.String("config", "", "explicit Columbo configuration")
	flag.Parse()
	if err := columbo.WriteInspectionFacts(*root, *config, flag.Args(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
