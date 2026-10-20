// Command generate-command-docs writes the command reference from the CLI definition.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vincentsch/gaugestone/internal/cli"
	"github.com/vincentsch/rungrad/docsgen"
)

func main() {
	check := flag.Bool("check", false, "check generated reference without writing files")
	flag.Parse()
	app := cli.New("dev", cli.Options{})
	const dir = "docs/reference"
	if *check {
		result, err := docsgen.Check(app, dir)
		if err != nil {
			fail(err)
		}
		if !result.OK() {
			fail(fmt.Errorf("generated command docs differ: missing=%v stale=%v orphaned=%v; run make docs", result.Missing, result.Stale, result.Orphaned))
		}
		return
	}
	if err := docsgen.Write(app, dir); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
