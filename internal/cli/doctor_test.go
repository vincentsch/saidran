package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/vincentsch/rungrad/testutil"
)

func TestDoctorLogRootsAndFormats(t *testing.T) {
	base := isolated(t)
	repo := filepath.Join(base, "repo")
	if b, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("init: %s %v", b, err)
	}
	result := testutil.Run(New("test", Options{Dir: repo}), "doctor", "--json")
	var empty health
	if result.Exit != 0 || result.JSON(&empty) != nil || len(empty.Logs) != 4 || empty.Logs[0].Found || empty.Logs[1].Sessions != 0 {
		t.Fatalf("empty: %+v", result)
	}
	claude := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "projects", regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(repo, "-"))
	codex := filepath.Join(os.Getenv("CODEX_HOME"), "sessions")
	for _, dir := range []string{claude, codex} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	user, _ := json.Marshal(map[string]any{"type": "user", "cwd": repo, "sessionId": "test-recording", "version": "2.1.293", "timestamp": "2026-10-20T15:00:00Z", "message": map[string]any{"content": "PRIVATE MESSAGE"}})
	if err := os.WriteFile(filepath.Join(claude, "session.jsonl"), append(user, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(map[string]any{"type": "session_meta", "timestamp": "2026-10-20T16:00:00Z", "payload": map[string]any{"cwd": repo, "id": "old", "cli_version": "0.100"}})
	if err := os.WriteFile(filepath.Join(codex, "old.jsonl"), append(old, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"doctor"}, {"doctor", "--json"}} {
		result := testutil.Run(New("test", Options{Dir: repo}), args...)
		if result.Exit != 0 || strings.Contains(result.Stdout, "PRIVATE MESSAGE") || !strings.Contains(result.Stdout, "Update Codex") || !strings.Contains(result.Stdout, "0.100") {
			t.Fatalf("doctor: %+v", result)
		}
		if len(args) > 1 {
			var v health
			if result.JSON(&v) != nil || v.Logs[0].Sessions != 1 || v.Logs[0].NewestVersion != "2.1.293" || v.Logs[1].UnsupportedFormats != 1 {
				t.Fatalf("metadata: %+v", v)
			}
		}
	}
	outside := testutil.Run(New("test", Options{Dir: base}), "doctor", "--json")
	if outside.Exit != 5 || outside.Stdout != "" || !strings.Contains(outside.Stderr, "inside your repository") {
		t.Fatalf("outside: %+v", outside)
	}
	t.Setenv("PATH", t.TempDir())
	missing := testutil.Run(New("test", Options{Dir: repo}), "doctor", "--json")
	if missing.Exit != 2 || !strings.Contains(missing.Stderr, "Git is missing. Install it with") {
		t.Fatalf("missing git: %+v", missing)
	}
}
