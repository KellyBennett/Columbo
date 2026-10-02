package main

import (
	"github.com/KellyBennett/Columbo/internal/columbo"
	"os"
)

var version = "dev"

func main() { os.Exit(columbo.Main(version)) }
