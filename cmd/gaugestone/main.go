package main

import (
	"github.com/vincentsch/gaugestone/internal/cli"
	"github.com/vincentsch/gaugestone/internal/version"
	"os"
)

func main() { os.Exit(cli.Run(version.Version, cli.Options{}, os.Args[1:], os.Stdout, os.Stderr)) }
