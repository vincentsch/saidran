package agents_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/vincentsch/saidran/internal/agents"
)

func TestSymlinkedLogRootsAndProjectNames(t *testing.T) {
	for _, name := range []string{"plain-repo", "my_repo", "My Repo", "repo.with.dots"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			repo := filepath.Join(base, name)
			t.Setenv("HOME", filepath.Join(base, "home"))
			t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(base, "profile"))
			t.Setenv("CODEX_HOME", filepath.Join(base, "codex-profile"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(base, "xdg"))
			t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(base, "pi"))
			roots := agents.Roots()
			for i, root := range roots {
				target := filepath.Join(base, []string{"claude-logs", "codex-logs", "opencode-logs", "pi-logs"}[i])
				if err := os.MkdirAll(target, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, root); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			cwd := filepath.Join(repo, "child_dir")
			encoded := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(cwd, "-")
			b, _ := json.Marshal(map[string]any{"type": "user", "sessionId": "claude-session", "cwd": cwd, "message": map[string]any{"content": "Hello"}})
			writeLog(t, filepath.Join(roots[0], encoded, "session.jsonl"), string(b))
			b, _ = json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]any{"id": "codex-session", "cwd": repo}})
			writeLog(t, filepath.Join(roots[1], "2026", "10", "20", "rollout.jsonl"), string(b), `{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"Reasoning","id":"reason"}}}`)
			// A similar encoded folder is still rejected when its cwd belongs elsewhere.
			b, _ = json.Marshal(map[string]any{"type": "user", "sessionId": "unrelated", "cwd": repo + "-other", "message": map[string]any{"content": "Hello"}})
			writeLog(t, filepath.Join(roots[0], encoded+"-other", "session.jsonl"), string(b))
			d, err := agents.Discover(repo)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Sessions) != 2 || len(d.Notes) != 0 {
				t.Fatalf("sessions=%d notes=%v", len(d.Sessions), d.Notes)
			}
			if d.Roots[0] != roots[0] || d.Roots[1] != roots[1] {
				t.Fatal("configured roots lost")
			}
			seen := map[string]bool{}
			for _, s := range d.Sessions {
				seen[s.Agent] = true
			}
			if !seen["claude-code"] || !seen["codex"] {
				t.Fatal("symlinked agent root skipped")
			}
		})
	}
}

func TestSessionCwdThroughSymlink(t *testing.T) {
	base, repo, transcript := fixture(t, "claude-stale")
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(transcript)
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.ReplaceAll(string(b), repo, alias))
	encoded := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(alias, "-")
	path := filepath.Join(agents.Roots()[0], encoded, "session.jsonl")
	writeLog(t, path, string(b))
	d, err := agents.Discover(repo)
	if err != nil || len(d.Sessions) != 1 {
		t.Fatalf("discover: %+v %v", d, err)
	}
	other := filepath.Join(base, "other")
	if err := os.Mkdir(other, 0700); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []string{"", ".", other} {
		if agents.Within(repo, cwd) {
			t.Fatalf("unrelated cwd accepted: %q", cwd)
		}
	}
	if !agents.Within(repo, alias) {
		t.Fatal("symlinked cwd rejected")
	}
}
