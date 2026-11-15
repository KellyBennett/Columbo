package main

import (
	"os"

	"github.com/KellyBennett/Columbo/internal/feedback"
)

func main() {
	os.Exit(feedback.Main(os.Args[1:], os.Stdout, os.Stderr, feedback.DefaultEnvironment()))
}
