package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/rungrad/testutil"
)

func isolated(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(base, "claude"))
	t.Setenv("CODEX_HOME", filepath.Join(base, "codex"))
	return base
}
func TestEmptyAndNonRepository(t *testing.T) {
	base := isolated(t)
	for _, args := range [][]string{{"sessions"}, {"sessions", "--json"}} {
		result := testutil.Run(New("test", Options{Dir: base}), args...)
		if result.Exit != 5 {
			t.Fatalf("non-repo %+v", result)
		}
	}
	if b, e := exec.Command("git", "init", "-q", base).CombinedOutput(); e != nil {
		t.Fatalf("init %s %v", b, e)
	}
	for _, args := range [][]string{{"sessions"}, {"sessions", "--json"}} {
		result := testutil.Run(New("test", Options{Dir: base}), args...)
		if result.Exit != 0 {
			t.Fatalf("empty %+v", result)
		}
		if len(args) == 2 {
			var v Listing
			if e := result.JSON(&v); e != nil || v.Sessions == nil || len(v.Sessions) != 0 {
				t.Fatalf("empty json %+v %v", v, e)
			}
		} else if !strings.Contains(result.Stdout, "No sessions found. Searched") {
			t.Fatal(result.Stdout)
		}
	}
}
func TestHelpVersionAndErrors(t *testing.T) {
	base := isolated(t)
	for _, args := range [][]string{{"--help"}, {"sessions", "--help"}, {"--version"}} {
		r := testutil.Run(New("test", Options{Dir: base}), args...)
		if r.Exit != 0 || r.Stdout == "" {
			t.Fatalf("help/version %+v", r)
		}
	}
	for _, args := range [][]string{{"sessions", "extra", "--json"}, {"--unknown=sk-private12345678901234567890", "--json"}} {
		r := testutil.Run(New("test", Options{}), args...)
		if r.Exit != 1 || strings.Contains(r.Stderr, "sk-private12345678901234567890") {
			t.Fatalf("usage %+v", r)
		}
		var v any
		if json.Unmarshal([]byte(r.Stderr), &v) != nil {
			t.Fatal("non-JSON error")
		}
	}
	t.Setenv("PATH", t.TempDir())
	r := testutil.Run(New("test", Options{Dir: base}), "sessions", "--json")
	if r.Exit != 2 || !strings.Contains(r.Stderr, "Git is missing") || !strings.Contains(r.Stderr, "install") {
		t.Fatalf("missing git %+v", r)
	}
}
func TestListingRedactionAndRecency(t *testing.T) {
	base := isolated(t)
	repo := filepath.Join(base, "repo.with.dots")
	if b, e := exec.Command("git", "init", "-q", repo).CombinedOutput(); e != nil {
		t.Fatalf("init %s %v", b, e)
	}
	dir := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "projects", regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(repo, "-")+"-child")
	os.MkdirAll(dir, 0700)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	records := []map[string]any{
		{"type": "user", "sessionId": "a123456789", "cwd": filepath.Join(repo, "child"), "timestamp": now.Add(-8 * 24 * time.Hour).Format(time.RFC3339), "message": map[string]any{"content": "TOKEN_KEY=private /home/private sk-private12345678901234567890"}},
		{"type": "assistant", "cwd": repo, "timestamp": now.Add(-8 * 24 * time.Hour).Format(time.RFC3339), "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": "bash", "name": "Bash", "input": map[string]any{"command": "pwd"}}}}},
	}
	var b strings.Builder
	for _, r := range records {
		v, _ := json.Marshal(r)
		b.Write(v)
		b.WriteByte('\n')
	}
	os.WriteFile(filepath.Join(dir, "log.jsonl"), []byte(b.String()), 0600)
	result := testutil.Run(New("test", Options{Dir: repo, Now: func() time.Time { return now }}), "sessions", "--json")
	var v Listing
	if e := result.JSON(&v); e != nil || len(v.Sessions) != 1 || v.Sessions[0].AutomaticPick {
		t.Fatalf("old listing %+v %v", result, e)
	}
	if strings.Contains(result.Stdout, "=private") || strings.Contains(result.Stdout, "sk-private12345678901234567890") {
		t.Fatal("secret in listing")
	}
	for _, tc := range []struct{ prompt, want string }{
		{"Fix the task-runner so ask-me works in the risk-free path", "Fix the task-runner so ask-me works in the risk-free path"},
		{"DB_PASSWORD=hunter2 POSTGRES_PASSWORD=hunter3", "DB_PASSWORD=[redacted] POSTGRES_PASSWORD=[redacted]"},
	} {
		records[0]["message"] = map[string]any{"content": tc.prompt}
		var log strings.Builder
		for _, record := range records {
			data, _ := json.Marshal(record)
			log.Write(data)
			log.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, "log.jsonl"), []byte(log.String()), 0600); err != nil {
			t.Fatal(err)
		}
		result := testutil.Run(New("test", Options{Dir: repo}), "sessions", "--json")
		var v Listing
		if err := result.JSON(&v); err != nil || result.Exit != 0 || len(v.Sessions) != 1 || v.Sessions[0].Prompt != tc.want {
			t.Fatalf("listing prompt changed or leaked: %+v %v", result, err)
		}
	}

}

func TestNonRepositoryWithGermanLocale(t *testing.T) {
	base := isolated(t)
	t.Setenv("LANG", "de_DE.UTF-8")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	for _, args := range [][]string{{"sessions"}, {"sessions", "--json"}} {
		result := testutil.Run(New("test", Options{Dir: base}), args...)
		if result.Exit != 5 {
			t.Fatalf("localized non-repository exit: %+v", result)
		}
	}
}

func TestDiscoveryNoteSummary(t *testing.T) {
	base := isolated(t)
	repo := filepath.Join(base, "repo")
	if b, e := exec.Command("git", "init", "-q", repo).CombinedOutput(); e != nil {
		t.Fatalf("init %s %v", b, e)
	}
	logs := filepath.Join(os.Getenv("CODEX_HOME"), "sessions")
	if err := os.MkdirAll(logs, 0700); err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]any{"id": "old", "cwd": repo}})
	for _, name := range []string{"old-a", "old-b", "old-c", "broken-a", "broken-b"} {
		data := string(meta) + "\n"
		if strings.HasPrefix(name, "broken") {
			data += "{invalid}\n{}\n"
		}
		if err := os.WriteFile(filepath.Join(logs, name+".jsonl"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	text := testutil.Run(New("test", Options{Dir: repo}), "sessions")
	if text.Exit != 0 {
		t.Fatal(text.Stderr)
	}
	if !strings.Contains(text.Stdout, "Older Codex recordings skipped: 3.") || !strings.Contains(text.Stdout, "Malformed recordings skipped: 2.") || strings.Contains(text.Stdout, "update Codex") || strings.Count(text.Stdout, "\n") != 4 {
		t.Fatalf("unbounded or misleading notes: %s", text.Stdout)
	}
	machine := testutil.Run(New("test", Options{Dir: repo}), "sessions", "--json")
	var result Listing
	if err := machine.JSON(&result); err != nil || machine.Exit != 0 || len(result.Notes) != 5 {
		t.Fatalf("individual notes lost: %+v %v", machine, err)
	}
}

func TestCompletenessNoteKinds(t *testing.T) {
	choice := "Two agents finished less than ten minutes apart. Interactive selection asks which to use; otherwise the newest session is picked."
	notes := []string{"a: incomplete final line skipped", "b: incomplete final line skipped", "Missing sub-agent transcript: child-a", "Missing sub-agent transcript: child-b", "codex:abcd1234 has 2 unsupported entries.", "codex:bcde2345 has 3 unsupported entries.", "open a: permission denied", "open b: permission denied", choice}
	summary := strings.Join(summarizeNotes(notes), "\n")
	for _, want := range []string{"Incomplete final lines skipped: 2.", "Missing sub-agent transcripts: 2.", "Sessions with unsupported entries: 2.", "Other discovery notes: 2.", choice} {
		if strings.Count(summary, want) != 1 {
			t.Fatalf("missing grouped note %q: %s", want, summary)
		}
	}
	if strings.Contains(summary, "child-a") || strings.Contains(summary, "permission denied") {
		t.Fatal("individual note details printed")
	}
}
