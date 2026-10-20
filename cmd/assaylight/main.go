package main

import (
	"github.com/vincentsch/assaylight/internal/cli"
	"os"
)

var version = "dev"

func main() { os.Exit(cli.New(version, cli.Options{}).Run(os.Args[1:], os.Stdout, os.Stderr)) }
