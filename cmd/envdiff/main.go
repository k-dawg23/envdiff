package main

import (
	"github.com/k-dawg23/envdiff/internal/cli"
	"os"
)

// version may be set at build time with -ldflags '-X main.version=1.0.0'.
var version = "dev"

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, version)) }
