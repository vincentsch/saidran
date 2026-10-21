package agents_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltCIReports(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct {
		name, policy string
		status       int
	}{
		{"claude-stale", "stale", 10}, {"claude-verified", "contradicted", 0}, {"codex-verified", "verified", 10}, {"opencode-verified", "verified", 10}, {"pi-stale", "stale", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, repo, transcript := fixture(t, tc.name)
			code, out, stderr := runBinary(t, binary, repo, "ci", "--transcript", transcript, "--fail-on", tc.policy, "--json")
			if code != tc.status {
				t.Fatalf("%d %s %s", code, out, stderr)
			}
			model := decodeReport(t, out)
			if model.Session == nil || len(model.Claims) == 0 {
				t.Fatal("missing transcript evidence")
			}
			for _, name := range []string{"report.html", "report.md", "report.json"} {
				if _, err := os.Stat(filepath.Join(repo, ".git", "saidran", "ci", name)); err != nil {
					t.Fatal(err)
				}
			}
			code, out, stderr = runBinary(t, binary, repo, "ci", "--json")
			if code != 0 {
				t.Fatalf("diff-only %d %s", code, stderr)
			}
			model = decodeReport(t, out)
			if model.Session != nil || len(model.Claims) != 0 || len(model.Commands) != 0 {
				t.Fatal("diff-only discovered a log")
			}
			for _, format := range []string{"text", "markdown", "html", "json"} {
				code, out, stderr = runBinary(t, binary, repo, "ci", "--format", format)
				if code != 0 {
					t.Fatalf("%s: %d %s", format, code, stderr)
				}
				if format == "json" {
					if decodeReport(t, out).Session != nil {
						t.Fatal("unexpected session")
					}
				} else if !strings.Contains(out, "Diff-only report") {
					t.Fatalf("%s: %s", format, out)
				}
			}
		})
	}
}
