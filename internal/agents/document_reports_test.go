package agents_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vincentsch/saidran/internal/cli"
)

func TestDocumentOutputsAndOpen(t *testing.T) {
	_, repo, transcript := fixture(t, "claude-stale")
	for _, tc := range []struct {
		name    string
		args    []string
		written bool
		opens   int
		code    int
	}{
		{"default", nil, true, 0, 0},
		{"open", []string{"--open"}, true, 1, 0},
		{"html", []string{"--format", "html", "--open"}, true, 1, 0},
		{"markdown", []string{"--format", "markdown"}, true, 0, 0},
		{"suppressed", []string{"--no-write", "--open", "--format", "html"}, false, 0, 0},
		{"preview", []string{"--dry-run", "--open"}, false, 0, 0},
		{"preview-suppressed", []string{"--dry-run", "--no-write", "--open", "--format", "markdown"}, false, 0, 0},
		{"conflict", []string{"--format", "html", "--json", "--open"}, false, 0, 1},
		{"markdown-conflict", []string{"--format", "markdown", "--format=json"}, false, 0, 1},
		{"repeated-conflict", []string{"--format", "html", "--format", "markdown", "--open"}, false, 0, 1},
		{"repeated-equals-conflict", []string{"--format=markdown", "--format=text"}, false, 0, 1},
		{"repeated-same", []string{"--format", "markdown", "--format", "markdown"}, true, 0, 0},
		{"invalid", []string{"--format", "pdf"}, false, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "reports")
			calls := 0
			var out, stderr bytes.Buffer
			opts := cli.Options{Dir: repo, Open: func(path string) error {
				calls++
				if path != filepath.Join(dir, "report.html") {
					t.Fatal(path)
				}
				for _, name := range []string{"report.html", "report.md", "report.json"} {
					if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
						t.Fatal("opened before writing", err)
					}
				}
				return nil
			}}
			args := append([]string{"--transcript", transcript, "--out", dir}, tc.args...)
			code := cli.Run("test", opts, args, &out, &stderr)
			if code != tc.code || calls != tc.opens {
				t.Fatalf("code=%d opens=%d %s", code, calls, &stderr)
			}
			_, err := os.Stat(dir)
			if tc.written && err != nil || !tc.written && !os.IsNotExist(err) {
				t.Fatalf("write status: %v", err)
			}
			if tc.code != 0 {
				return
			}
			if strings.Contains(strings.Join(tc.args, " "), "--format html") {
				if !strings.HasPrefix(out.String(), "<!doctype html>") || strings.Contains(out.String(), "Report:") {
					t.Fatal("HTML stream has extra text")
				}
			} else if strings.Contains(strings.Join(tc.args, " "), "--format markdown") {
				if !strings.HasPrefix(out.String(), "### Saidran\n") || strings.Contains(out.String(), "Report:") {
					t.Fatal("Markdown stream has extra text")
				}
			} else if tc.written && !strings.Contains(out.String(), filepath.Join(dir, "report.html")) {
				t.Fatal("missing HTML path")
			}
		})
	}
	t.Run("write-failure", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "report.md"), 0700); err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		calls := 0
		code := cli.Run("test", cli.Options{Dir: repo, Open: func(string) error { calls++; return nil }}, []string{"--transcript", transcript, "--out", dir, "--open"}, &out, &stderr)
		if code == 0 || calls != 0 || strings.Contains(out.String(), "Report:") {
			t.Fatalf("%d %d %s %s", code, calls, &out, &stderr)
		}
	})
	t.Run("opener-failure", func(t *testing.T) {
		var out, stderr bytes.Buffer
		code := cli.Run("test", cli.Options{Dir: repo, Open: func(string) error { return errors.New("fake opener unavailable") }}, []string{"--transcript", transcript, "--out", t.TempDir(), "--open"}, &out, &stderr)
		if code == 0 || !strings.Contains(stderr.String(), "Reports saved, but the browser could not open") {
			t.Fatalf("%d %s %s", code, &out, &stderr)
		}
	})
}

func TestRecordedDocumentReports(t *testing.T) {
	binary := buildBinary(t)
	for _, name := range []string{"claude-stale", "codex-verified", "codex-stale", "claude-honest-failure", "claude-skip", "claude-contradicted", "claude-pytest", "claude-verified"} {
		t.Run(name, func(t *testing.T) {
			_, repo, transcript := fixture(t, name)
			for _, format := range []string{"html", "markdown"} {
				code, out, stderr := runBinary(t, binary, repo, "--transcript", transcript, "--format", format, "--no-write")
				if code != 0 || stderr != "" {
					t.Fatalf("%d %s %s", code, out, stderr)
				}
				if !strings.Contains(out, "not a certificate") || !strings.Contains(out, "Completeness:") {
					t.Fatal("missing evidence limits")
				}
				if name == "codex-stale" && !strings.Contains(out, "Admitted gap") {
					t.Fatal("missing admitted gap")
				}
				if format == "markdown" {
					if _, err := exec.LookPath("pandoc"); err == nil {
						cmd := exec.Command("pandoc", "-f", "gfm", "-t", "html")
						cmd.Stdin = strings.NewReader(out)
						b, err := cmd.CombinedOutput()
						if err != nil || bytes.Count(b, []byte("<details>")) != 1 || bytes.Count(b, []byte("</details>")) != 1 {
							t.Fatalf("pandoc: %v %s", err, b)
						}
					}
				}
			}
		})
	}
}
