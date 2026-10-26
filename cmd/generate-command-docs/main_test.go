package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceDriftCheckDoesNotWrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reference")
	docs := map[string]string{"saidran.md": "current reference\n", "index.md": "index\n"}
	if err := syncDocs(dir, docs, false); err != nil {
		t.Fatal(err)
	}
	if err := syncDocs(dir, docs, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "index.md")); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"saidran.md": "stale\n", "orphan.md": "orphan\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := syncDocs(dir, docs, true)
	if err == nil || !strings.Contains(err.Error(), "missing=[index.md] stale=[saidran.md] orphaned=[orphan.md]") {
		t.Fatalf("expected all drift classes, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "saidran.md"))
	if err != nil || string(got) != "stale\n" {
		t.Fatalf("check changed the stale file: %q, %v", got, err)
	}
	if err := syncDocs(dir, docs, false); err != nil {
		t.Fatal(err)
	}
	if err := syncDocs(dir, docs, true); err != nil {
		t.Fatal(err)
	}
}
