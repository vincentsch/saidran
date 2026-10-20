package agents_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/gaugestone/internal/agents"
	"github.com/vincentsch/gaugestone/internal/cli"
	"github.com/vincentsch/gaugestone/internal/report"
)

func TestCompletionHookPayloads(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		for _, override := range []bool{false, true} {
			t.Run(agent+map[bool]string{false: "-transcript", true: "-override"}[override], func(t *testing.T) {
				name := "claude-stale"
				expectedAgent := "claude-code"
				if agent == "codex" {
					name = "codex-verified"
					expectedAgent = "codex"
				}
				base, repo, transcript := fixture(t, name)
				s, err := agents.Read(transcript)
				if err != nil {
					t.Fatal(err)
				}
				if agent == "codex" {
					logs := filepath.Join(base, "codex", "sessions", "2026", "10", "20")
					if err = os.MkdirAll(logs, 0700); err != nil {
						t.Fatal(err)
					}
					b, e := os.ReadFile(transcript)
					if e != nil {
						t.Fatal(e)
					}
					if err = os.WriteFile(filepath.Join(logs, "rollout-selected.jsonl"), b, 0600); err != nil {
						t.Fatal(err)
					}
					// A newer session must never replace the exact notified thread.
					other := strings.ReplaceAll(string(b), s.ID, "unrelated-thread")
					other = strings.ReplaceAll(other, "2026-10-20T", "2026-10-21T")
					if err = os.WriteFile(filepath.Join(logs, "rollout-newer.jsonl"), []byte(other), 0600); err != nil {
						t.Fatal(err)
					}
					t.Setenv("CODEX_HOME", filepath.Join(base, "codex"))
				}
				now := s.Ended.Add(time.Hour)
				payload := map[string]any{"cwd": repo, "transcript_path": transcript, "thread-id": s.ID}
				if override {
					field := "last_assistant_message"
					if agent == "codex" {
						field = "last-assistant-message"
					}
					payload[field] = "All tests pass."
				}
				b, _ := json.Marshal(payload)
				app := cli.New("test", cli.Options{Dir: repo, Now: func() time.Time { return now }})
				args := []string{"hook", agent}
				if agent == "codex" {
					args = append(args, string(b))
				}
				var stdout, stderr bytes.Buffer
				code := app.RunIO(args, strings.NewReader(string(b)), &stdout, &stderr)
				if code != 0 || stderr.Len() != 0 {
					t.Fatalf("%d %s %s", code, &stdout, &stderr)
				}
				if agent == "codex" {
					if stdout.Len() != 0 {
						t.Fatal(stdout.String())
					}
				} else {
					var response map[string]string
					if err = json.Unmarshal(stdout.Bytes(), &response); err != nil || !strings.Contains(response["systemMessage"], "Report:") || len(response) != 1 {
						t.Fatalf("%s %v", &stdout, err)
					}
				}
				dir := filepath.Join(repo, ".git", "gaugestone", expectedAgent+"-"+s.ID)
				for _, file := range []string{"report.html", "report.md", "report.json"} {
					if _, err = os.Stat(filepath.Join(dir, file)); err != nil {
						t.Fatal(err)
					}
				}
				b, err = os.ReadFile(filepath.Join(dir, "report.json"))
				if err != nil {
					t.Fatal(err)
				}
				var model report.Report
				if err = json.Unmarshal(b, &model); err != nil {
					t.Fatal(err)
				}
				if model.Session.ID != s.ID || len(model.Claims) == 0 {
					t.Fatalf("wrong session or claims %+v", model.Session)
				}
				want := s.FinalTime
				if override {
					want = now
				}
				for _, claim := range model.Claims {
					if !claim.Time.Equal(want) {
						t.Fatalf("claim time %s want %s", claim.Time, want)
					}
				}
				if !override && !model.Session.Ended.Equal(s.Ended) {
					t.Fatal("changed fallback time")
				}
				if override && agent == "claude" && model.Claims[0].Verdict != "stale" {
					t.Fatal(model.Claims)
				}
			})
		}
	}
}

func TestCompletionHooksStayNonblocking(t *testing.T) {
	base, repo, transcript := fixture(t, "claude-stale")
	for _, tc := range []struct {
		name, payload string
		silent        bool
	}{
		{"malformed", "{broken", false},
		{"missing-fields", `{}`, false},
		{"non-repository", `{"cwd":` + quote(base) + `,"transcript_path":` + quote(transcript) + `}`, true},
		{"foreign-session", `{"cwd":` + quote(repo) + `,"transcript_path":` + quote(filepath.Join(base, "foreign.jsonl")) + `}`, true},
		{"missing-transcript", `{"cwd":` + quote(repo) + `,"transcript_path":"missing"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "foreign-session" {
				b, _ := os.ReadFile(transcript)
				b = bytes.ReplaceAll(b, []byte(repo), []byte("/another/repository"))
				if err := os.WriteFile(filepath.Join(base, "foreign.jsonl"), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, agent := range []string{"claude", "codex"} {
				var out, stderr bytes.Buffer
				args := []string{"hook", agent}
				if agent == "codex" {
					args = append(args, tc.payload)
				}
				code := cli.New("test", cli.Options{Dir: repo}).RunIO(args, strings.NewReader(tc.payload), &out, &stderr)
				if code != 0 || stderr.Len() != 0 {
					t.Fatalf("%d %s", code, &stderr)
				}
				if agent == "codex" || tc.silent {
					if out.Len() != 0 {
						t.Fatal(out.String())
					}
				} else if !strings.Contains(out.String(), "could not create report") || strings.Contains(out.String(), "Report:") {
					t.Fatal(out.String())
				}
			}
		})
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "gaugestone")); !os.IsNotExist(err) {
		t.Fatal("invalid payload wrote reports")
	}
}
func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestInstalledHookWorksWithoutBinaryOnPath(t *testing.T) {
	binary := buildBinary(t)
	base, repo, transcript := fixture(t, "claude-stale")
	dir := filepath.Join(base, "binary path with spaces")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(dir, "gaugestone")
	b, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(installed, b, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(installed, "install", "claude")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	b, err = os.ReadFile(filepath.Join(repo, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err = json.Unmarshal(b, &settings); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"cwd": repo, "transcript_path": transcript})
	cmd = exec.Command("sh", "-c", settings.Hooks["Stop"][0].Hooks[0].Command)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin")
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("Report:")) {
		t.Fatalf("%v %s", err, output)
	}
	// A failed multi-file write must not announce a success path.
	s, err := agents.Read(transcript)
	if err != nil {
		t.Fatal(err)
	}
	reportDir := filepath.Join(repo, ".git", "gaugestone", s.Agent+"-"+s.ID)
	if err = os.Remove(filepath.Join(reportDir, "report.md")); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(reportDir, "report.md"), 0700); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(installed, "hook", "claude")
	cmd.Dir = repo
	cmd.Stdin = bytes.NewReader(payload)
	output, err = cmd.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("could not create report")) || bytes.Contains(output, []byte("Report:")) {
		t.Fatalf("%v %s", err, output)
	}
}
