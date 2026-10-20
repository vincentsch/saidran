package agents_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/gaugestone/internal/agents"
	"github.com/vincentsch/gaugestone/internal/cli"
	"github.com/vincentsch/gaugestone/internal/session"
)

func TestSyntheticEvidenceReports(t *testing.T) {
	binary := buildBinary(t)
	b, err := os.ReadFile("../../testdata/synthetic/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Final string
		Commands    []struct {
			Line, Output string
			Exit         int
		}
		Verdicts []string
	}
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			base, repo, _ := fixture(t, "claude-verified")
			path := filepath.Join(base, "synthetic.jsonl")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			enc := json.NewEncoder(f)
			emit := func(at string, kind string, payload any) {
				t.Helper()
				if err := enc.Encode(map[string]any{"timestamp": at, "type": kind, "payload": payload}); err != nil {
					t.Fatal(err)
				}
			}
			emit("2026-10-20T16:00:00Z", "session_meta", map[string]any{"id": "synthetic", "cwd": repo, "cli_version": "0.163.0"})
			for i, c := range tc.Commands {
				emit(fmt.Sprintf("2026-10-20T16:00:%02dZ", i+2), "event_msg", map[string]any{"type": "item_completed", "item": map[string]any{"id": fmt.Sprint(i), "type": "CommandExecution", "command": []string{"bash", "-lc", c.Line}, "cwd": repo, "aggregated_output": c.Output, "exit_code": c.Exit, "status": "completed", "duration": map[string]any{"secs": 1, "nanos": 0}}})
			}
			emit("2026-10-20T16:01:00Z", "event_msg", map[string]any{"type": "item_completed", "item": map[string]any{"id": "final", "type": "AgentMessage", "content": []map[string]any{{"type": "text", "text": tc.Final}}}})
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
			code, out, errout := runBinary(t, binary, repo, "--transcript", path, "--no-write", "--json")
			if code != 0 || errout != "" {
				t.Fatalf("%d %s %s", code, out, errout)
			}
			r := decodeReport(t, out)
			if len(r.Claims) != len(tc.Verdicts) {
				t.Fatalf("%+v", r.Claims)
			}
			for i, want := range tc.Verdicts {
				if r.Claims[i].Verdict != want {
					t.Fatalf("%+v", r.Claims)
				}
			}
			code, out, errout = runBinary(t, binary, repo, "--transcript", path, "--no-write")
			if code != 0 || errout != "" || !strings.Contains(out, "1 run ·") {
				t.Fatalf("%d %s %s", code, out, errout)
			}
			if tc.Name == "mixed-results" && !strings.Contains(out, "1 failed · 0 result unreliable") {
				t.Fatal(out)
			}
			if tc.Name == "unreliable-result" && !strings.Contains(out, "0 failed · 1 result unreliable") {
				t.Fatal(out)
			}
		})
	}
}

func TestReportOutputGoldens(t *testing.T) {
	t.Setenv("TZ", "UTC")
	previous := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previous })
	t.Setenv("NO_COLOR", "1")
	for _, name := range []string{"claude-stale", "codex-verified"} {
		t.Run(name, func(t *testing.T) {
			base, repo, transcript := fixture(t, name)
			for _, format := range []string{"text", "json", "markdown"} {
				t.Run(format, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					args := []string{"--transcript", transcript, "--no-write", "--format", format}
					code := cli.Run("test-version", cli.Options{Dir: repo, Now: func() time.Time { return time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC) }}, args, &stdout, &stderr)
					if code != 0 || stderr.Len() != 0 {
						t.Fatalf("%d %s %s", code, &stdout, &stderr)
					}
					got := strings.ReplaceAll(stdout.String(), base, "/fixture")
					path := filepath.Join("testdata", "reports", name+"."+format)
					if os.Getenv("UPDATE_GOLDEN") == "1" {
						if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(got), 0644); err != nil {
							t.Fatal(err)
						}
					}
					want, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if got != string(want) {
						t.Fatalf("%s output differs from %s", format, path)
					}
				})
			}
		})
	}
}

func TestReportHeaderUsesUniqueSessionPrefix(t *testing.T) {
	_, repo, transcript := fixture(t, "codex-verified")
	original, err := agents.Read(transcript)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(transcript)
	if err != nil {
		t.Fatal(err)
	}
	peerID := original.ID[:8] + "-peer"
	logs := filepath.Join(os.Getenv("CODEX_HOME"), "sessions")
	if err = os.MkdirAll(logs, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(logs, "original.jsonl"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(logs, "peer.jsonl"), []byte(strings.ReplaceAll(string(raw), original.ID, peerID)), 0600); err != nil {
		t.Fatal(err)
	}
	peer, err := agents.Read(filepath.Join(logs, "peer.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	displayID := agents.DisplayID(original, []*session.Session{original, peer})
	if len(displayID) <= 8 {
		t.Fatal("test sessions must collide at eight characters")
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run("test", cli.Options{Dir: repo}, []string{"--session", original.ID, "--no-write"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "session "+displayID+" ") {
		t.Fatalf("%d %s %s", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	code = cli.Run("test", cli.Options{Dir: repo}, []string{"--session", displayID, "--no-write", "--json"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || decodeReport(t, stdout.String()).Session.ID != original.ID {
		t.Fatalf("prefix selection failed: %d %s %s", code, &stdout, &stderr)
	}
}

func TestBuiltReportUsesLocalTime(t *testing.T) {
	binary := buildBinary(t)
	_, repo, transcript := fixture(t, "claude-stale")
	t.Setenv("TZ", "Europe/Berlin")
	code, out, errout := runBinary(t, binary, repo, "--transcript", transcript, "--no-write")
	if code != 0 || errout != "" || !strings.Contains(out, "2026-10-20 17:50–17:50") || !strings.Contains(out, "passed at 17:50") {
		t.Fatalf("%d %s %s", code, out, errout)
	}
}
