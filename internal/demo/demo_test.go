package demo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedExampleIsCurrent(t *testing.T) {
	for _, name := range []string{"repo.tar.gz", "session.jsonl"} {
		embedded, err := data.ReadFile("data/" + name)
		if err != nil {
			t.Fatal(err)
		}
		fixture, err := os.ReadFile(filepath.Join("../../testdata/sessions/claude-stale", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(embedded, fixture) {
			t.Fatalf("%s differs; run go generate ./internal/demo", name)
		}
	}
}
