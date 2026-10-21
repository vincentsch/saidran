package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vincentsch/rungrad/testutil"
	"github.com/vincentsch/saidran/internal/report"
)

func TestCIDiffReportsAndPolicy(t *testing.T) {
	f := prSetup(t)
	if err := os.WriteFile(filepath.Join(f.repo, "sample_test.go"), []byte("package sample\nfunc TestSample(t *testing.T) { t.Skipf(\"intentional %s\", \"skip\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitDir := gitPR(t, f.repo, "rev-parse", "--absolute-git-dir")
	out := filepath.Join(gitDir, "saidran", "ci")
	run := func(args ...string) testutil.Result {
		return testutil.Run(New("test", Options{Dir: f.repo}), append([]string{"ci"}, args...)...)
	}
	r := run("--dry-run", "--json")
	if r.Exit != 0 {
		t.Fatal(r)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote output: %v", err)
	}
	r = run("--json", "--fail-on", "test_skipped")
	if r.Exit != 10 {
		t.Fatal(r)
	}
	var model report.Report
	if err := json.Unmarshal([]byte(r.Stdout), &model); err != nil {
		t.Fatal(err)
	}
	if model.Session != nil || len(model.Claims) != 0 || len(model.Findings) != 1 {
		t.Fatalf("diff model: %+v", model)
	}
	for _, name := range []string{"report.html", "report.md", "report.json"} {
		path := filepath.Join(out, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if name != "report.json" && (!strings.Contains(string(data), "Diff-only report") || !strings.Contains(string(data), "No session log was available") || strings.Contains(string(data), "agent's final message")) {
			t.Fatalf("misleading %s", name)
		}
		if err := os.WriteFile(path, []byte("old output"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r = run()
	if r.Exit != 0 || !strings.Contains(r.Stdout, "Diff-only report") || strings.Contains(r.Stdout, "agent's final message") {
		t.Fatal(r)
	}
	for _, name := range []string{"report.html", "report.md", "report.json"} {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil || string(data) == "old output" {
			t.Fatalf("not replaced %s: %v", name, err)
		}
	}
	dismissed := testutil.Run(New("test", Options{Dir: f.repo}), "dismiss", model.Findings[0].ID, "--reason", "Intentional test skip")
	if dismissed.Exit != 0 {
		t.Fatal(dismissed)
	}
	r = run("--fail-on", "test_skipped", "--json")
	if r.Exit != 0 || !strings.Contains(r.Stdout, "Intentional test skip") {
		t.Fatal(r)
	}
	for _, format := range []string{"text", "markdown", "html"} {
		r := run("--format", format, "--out", filepath.Join(f.repo, ".git", "explicit"))
		if r.Exit != 0 || !strings.Contains(r.Stdout, "Diff-only report") {
			t.Fatal(r)
		}
	}
}

func TestCITranscriptAndFailures(t *testing.T) {
	f := prSetup(t)
	out := filepath.Join(t.TempDir(), "reports")
	for _, tc := range []struct {
		args    []string
		exit    int
		message string
	}{
		{[]string{"--transcript", f.transcript, "--base", f.parent, "--json"}, 0, "pr-session"},
		{[]string{"--base", "unavailable-commit"}, 2, ""},
		{[]string{"--fail-on", "invalid"}, 1, ""},
		{[]string{"--fail-on", ""}, 1, ""},
		{[]string{"--transcript", filepath.Join(f.repo, "missing.jsonl")}, 2, ""},
		{[]string{"--format", "invalid"}, 1, ""},
		{[]string{"--session", "other"}, 1, ""},
	} {
		r := testutil.Run(New("test", Options{Dir: f.repo}), append([]string{"ci", "--out", out}, tc.args...)...)
		if r.Exit != tc.exit || !strings.Contains(r.Stdout+r.Stderr, tc.message) {
			t.Fatalf("%v: %+v", tc.args, r)
		}
	}
	data, err := os.ReadFile(f.transcript)
	if err != nil {
		t.Fatal(err)
	}
	foreignRepo := t.TempDir()
	gitPR(t, foreignRepo, "init", "-q")
	foreign := filepath.Join(t.TempDir(), "foreign.jsonl")
	if err := os.WriteFile(foreign, []byte(strings.ReplaceAll(string(data), f.repo, foreignRepo)), 0600); err != nil {
		t.Fatal(err)
	}
	r := testutil.Run(New("test", Options{Dir: f.repo}), "ci", "--transcript", foreign)
	if r.Exit != 5 || !strings.Contains(r.Stderr, "inside this checkout") {
		t.Fatal(r)
	}
	broken := filepath.Join(t.TempDir(), "broken.jsonl")
	os.WriteFile(broken, []byte("{invalid}\n{}\n"), 0600)
	r = testutil.Run(New("test", Options{Dir: f.repo}), "ci", "--transcript", broken)
	if r.Exit != 2 {
		t.Fatal(r)
	}
	r = testutil.Run(New("test", Options{Dir: t.TempDir()}), "ci")
	if r.Exit != 5 {
		t.Fatal(r)
	}
}

func TestCIUsesWorktreeGitDirectory(t *testing.T) {
	f := prSetup(t)
	worktree := filepath.Join(t.TempDir(), "worktree")
	gitPR(t, f.repo, "worktree", "add", "-q", "--detach", worktree)
	gitDir := gitPR(t, worktree, "rev-parse", "--absolute-git-dir")
	r := testutil.Run(New("test", Options{Dir: worktree}), "ci", "--dry-run")
	if r.Exit != 0 {
		t.Fatal(r)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "saidran", "ci")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote: %v", err)
	}
	r = testutil.Run(New("test", Options{Dir: worktree}), "ci")
	if r.Exit != 0 {
		t.Fatal(r)
	}
	for _, name := range []string{"report.html", "report.md", "report.json"} {
		if _, err := os.Stat(filepath.Join(gitDir, "saidran", "ci", name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git", "saidran", "ci")); !os.IsNotExist(err) {
		t.Fatalf("worktree wrote to main git directory: %v", err)
	}
}

func TestCINeedsFirstCommit(t *testing.T) {
	repo := t.TempDir()
	gitPR(t, repo, "init")
	for _, args := range [][]string{{"ci"}, {"ci", "--json"}} {
		r := testutil.Run(New("test", Options{Dir: repo}), args...)
		if r.Exit != 2 || !strings.Contains(r.Stdout+r.Stderr, "first commit") {
			t.Fatal(r)
		}
		if len(args) > 1 {
			var doc struct {
				Error    string `json:"error"`
				ExitCode int    `json:"exit_code"`
			}
			if err := json.Unmarshal([]byte(r.Stderr), &doc); err != nil || doc.ExitCode != 2 || doc.Error == "" || r.Stdout != "" {
				t.Fatalf("invalid JSON error: %+v", r)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "saidran")); !os.IsNotExist(err) {
		t.Fatalf("unexpected reports: %v", err)
	}
	gitPR(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "Initial commit")
	r := testutil.Run(New("test", Options{Dir: repo}), "ci", "--json")
	if r.Exit != 0 {
		t.Fatal(r)
	}
}
