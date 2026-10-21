package main

import (
	"github.com/vincentsch/saidran/internal/cli"
	"github.com/vincentsch/saidran/internal/version"
	"os"
)

func main() { os.Exit(cli.Run(version.Version, cli.Options{}, os.Args[1:], os.Stdout, os.Stderr)) }
