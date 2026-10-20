package main

import (
	"github.com/vincentsch/assaylight/internal/cli"
	"os"
)

var version = "dev"

func main() { os.Exit(cli.Run(version, cli.Options{}, os.Args[1:], os.Stdout, os.Stderr)) }
