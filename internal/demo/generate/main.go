// Command generate copies the recorded example without changing the fixtures.
package main

import (
	"os"
	"path/filepath"
)

func main() {
	for _, name := range []string{"repo.tar.gz", "session.jsonl"} {
		b, err := os.ReadFile(filepath.Join("../../testdata/sessions/claude-stale", name))
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(filepath.Join("data", name), b, 0644); err != nil {
			panic(err)
		}
	}
}
