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
	"github.com/vincentsch/gaugestone/internal/commands"
	"github.com/vincentsch/gaugestone/internal/session"
	"github.com/vincentsch/rungrad/testutil"
)

func TestPiBashResults(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		isError    bool
		exit       *int
		status     string
	}{
		{"success without details", "ok\nCommand exited with code 9", false, intPointer(0), "completed"},
		{"missing error flag", "output", false, nil, "pending"},
		{"truncation details", "ok", false, intPointer(0), "completed"},
		{"error trailer", "failure output\n\nCommand exited with code 7", true, intPointer(7), "failed"},
		{"ordinary error text", "Command exited with code 2\nmore output", true, nil, "failed"},
		{"aborted", "partial output\n\nCommand aborted", true, nil, "interrupted"},
		{"timeout", "partial output\n\nCommand timed out after 10 seconds", true, nil, "interrupted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "arbitrary.log")
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			enc.Encode(map[string]any{"type": "session", "version": 3, "id": "session-id", "cwd": "/repo", "timestamp": "2026-10-20T16:00:00Z"})
			enc.Encode(map[string]any{"type": "message", "id": "call-record", "timestamp": "2026-10-20T16:00:01Z", "message": map[string]any{"role": "assistant", "stopReason": "toolUse", "content": []any{map[string]any{"type": "toolCall", "id": "call", "name": "bash", "arguments": map[string]any{"command": "go test ./..."}}, map[string]any{"type": "toolCall", "id": "pending", "name": "bash", "arguments": map[string]any{"command": "go test ./..."}}}}})
			resultMessage := map[string]any{"role": "toolResult", "toolCallId": "call", "toolName": "bash", "content": []any{map[string]any{"type": "text", "text": tc.text}}}
			if tc.name != "missing error flag" {
				resultMessage["isError"] = tc.isError
			}
			if tc.name == "truncation details" {
				resultMessage["details"] = map[string]any{"truncation": map[string]any{"truncated": true}, "exitCode": 9}
			}
			enc.Encode(map[string]any{"type": "message", "id": "result-record", "timestamp": "2026-10-20T16:00:02Z", "message": resultMessage})
			if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			s, err := agents.Read(path)
			if err != nil {
				t.Fatal(err)
			}
			if s.Agent != "pi" || len(s.Events) != 2 {
				t.Fatalf("%+v", s)
			}
			c := s.Events[0].Command
			if c == nil || c.RawOutput != tc.text || c.Status != tc.status || c.Start != session.Time("2026-10-20T16:00:01Z") || c.End != session.Time("2026-10-20T16:00:02Z") || c.Cwd != "/repo" {
				t.Fatalf("command %+v", c)
			}
			if (c.ExitCode == nil) != (tc.exit == nil) || (tc.exit != nil && *c.ExitCode != *tc.exit) || c.ExitReliable != (tc.exit != nil) {
				t.Fatalf("exit %+v", c)
			}
			if commands.Analyze(c).OutputTruncatedByAgent != (tc.name == "truncation details") {
				t.Fatal("truncation metadata lost")
			}
			pending := s.Events[1].Command
			if pending.ExitCode != nil || pending.ExitReliable || pending.Status != "pending" {
				t.Fatalf("pending %+v", pending)
			}
		})
	}
}

func TestOpenCodeBashStates(t *testing.T) {
	for i, tc := range []struct {
		status string
		exit   *int
		want   string
	}{
		{"completed", intPointer(0), "completed"}, {"completed", intPointer(4), "failed"}, {"completed", nil, "completed"}, {"running", nil, "pending"}, {"error", intPointer(0), "failed"},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.status, i), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "session", "project", "session.json")
			putJSON(t, path, map[string]any{"id": "session", "directory": "/repo", "version": "1.1.4", "time": map[string]any{"created": 1000, "updated": 4000}})
			putJSON(t, filepath.Join(root, "message", "session", "msg.json"), map[string]any{"id": "msg", "sessionID": "session", "role": "assistant", "path": map[string]any{"cwd": "/repo/sub"}, "time": map[string]any{"created": 2000, "completed": 4000}, "finish": "stop"})
			state := map[string]any{"status": tc.status, "input": map[string]any{"command": "go test ./..."}, "output": "ok\n" + strings.Repeat("line\n", 80), "time": map[string]any{"start": 2000, "end": 3000}}
			if tc.exit != nil {
				state["metadata"] = map[string]any{"exit": *tc.exit}
			}
			putJSON(t, filepath.Join(root, "part", "msg", "tool.json"), map[string]any{"id": "part", "sessionID": "session", "messageID": "msg", "type": "tool", "callID": "call", "tool": "bash", "state": state})
			putJSON(t, filepath.Join(root, "part", "msg", "text.json"), map[string]any{"id": "text", "sessionID": "session", "messageID": "msg", "type": "text", "text": "All tests pass."})
			s, e := agents.Read(path)
			if e != nil {
				t.Fatal(e)
			}
			if s.Final != "All tests pass." || s.FinalTime.UnixMilli() != 4000 {
				t.Fatalf("final %+v", s)
			}
			var c *session.Command
			for _, ev := range s.Events {
				if ev.Command != nil {
					if c != nil {
						t.Fatal("duplicate command")
					}
					c = ev.Command
					if ev.Source.File != filepath.Join(root, "part", "msg", "tool.json") || ev.Source.Line != 1 {
						t.Fatal("source")
					}
				}
			}
			if c == nil || c.Status != tc.want || c.Cwd != "/repo/sub" || c.Start.UnixMilli() != 2000 || c.End.UnixMilli() != 3000 || len(c.RawOutput) <= len(c.Output) {
				t.Fatalf("command %+v", c)
			}
			reliable := tc.status == "completed" && tc.exit != nil
			if c.ExitReliable != reliable || (!reliable && c.ExitCode != nil) {
				t.Fatalf("exit %+v", c)
			}
		})
	}
}
func putJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}

// installRecording preserves OpenCode's sibling storage files and rewrites every
// JSON file, including absolute paths nested in command output and diff keys.
func installRecording(t *testing.T, name, repo string) string {
	t.Helper()
	source := filepath.Join("../../testdata/sessions", name)
	b, e := os.ReadFile(filepath.Join(source, "meta.json"))
	if e != nil {
		t.Fatal(e)
	}
	var meta struct{ Agent, Transcript string }
	if e = json.Unmarshal(b, &meta); e != nil {
		t.Fatal(e)
	}
	if meta.Transcript == "" {
		meta.Transcript = "session.jsonl"
	}
	var transcript string
	switch meta.Agent {
	case "claude-code":
		transcript = filepath.Join(agents.Roots()[0], projectFolder(repo), name+".jsonl")
	case "codex":
		transcript = filepath.Join(agents.Roots()[1], "2026", "10", "20", name+".jsonl")
	case "pi":
		transcript = filepath.Join(agents.Roots()[3], "encoded-repo", name+".jsonl")
	case "opencode":
		transcript = filepath.Join(agents.Roots()[2], strings.TrimPrefix(meta.Transcript, "storage/"))
	}
	e = filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(source, path)
		if rel != meta.Transcript && !strings.HasPrefix(rel, "storage/") {
			return nil
		}
		dest := transcript
		if meta.Agent == "opencode" {
			dest = filepath.Join(agents.Roots()[2], strings.TrimPrefix(rel, "storage/"))
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		b = []byte(strings.ReplaceAll(string(b), "/fixtures/"+name+"/repo", repo))
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		return os.WriteFile(dest, b, 0600)
	})
	if e != nil {
		t.Fatal(e)
	}
	return transcript
}
func projectFolder(path string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, path)
}
func TestAllAgentRecordingsTogether(t *testing.T) {
	binary := buildBinary(t)
	_, repo, _ := fixture(t, "claude-verified")
	if agents.Roots()[2] != filepath.Join(os.Getenv("XDG_DATA_HOME"), "opencode", "storage") || agents.Roots()[3] != filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "sessions") {
		t.Fatal("log root overrides ignored")
	}
	names := []string{"claude-verified", "codex-verified", "opencode-verified", "pi-verified"}
	for _, name := range names {
		installRecording(t, name, repo)
	}
	// Unrelated repositories can have unfinished or missing transcript bodies;
	// metadata membership must exclude them before loading those bodies.
	putJSON(t, filepath.Join(agents.Roots()[2], "session", "other", "unrelated.json"), map[string]any{"id": "unrelated", "directory": repo + "-other", "version": "1.1.4"})
	otherHeader, _ := json.Marshal(map[string]any{"type": "session", "version": 3, "id": "unrelated", "cwd": repo + "-other"})
	otherPi := filepath.Join(agents.Roots()[3], "other", "unrelated.jsonl")
	if err := os.MkdirAll(filepath.Dir(otherPi), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPi, append(otherHeader, []byte("\n{incomplete unrelated journal\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	d, e := agents.Discover(repo)
	if e != nil || len(d.Sessions) != 4 || len(d.Notes) != 0 {
		t.Fatalf("discovery %+v %v", d, e)
	}
	newest := d.Sessions[0].Ended
	for _, s := range d.Sessions {
		sel, e := agents.Select(d.Sessions, s.Agent+":"+s.ID, "", newest.Add(time.Hour), nil)
		if e != nil || sel.Session != s {
			t.Fatalf("selection %s: %v", s.Agent, e)
		}
		for _, format := range []string{"text", "markdown", "html"} {
			code, out, errout := runBinary(t, binary, repo, "--session", s.Agent+":"+s.ID, "--no-write", "--format", format)
			if code != 0 || errout != "" || !strings.Contains(out, session.AgentLabel(s.Agent)) {
				t.Fatalf("%s label: %d %s %s", format, code, out, errout)
			}
		}
	}
	for _, args := range [][]string{{"sessions", "--json"}, {"doctor", "--json"}} {
		code, out, errout := runBinary(t, binary, repo, args...)
		if code != 0 || errout != "" {
			t.Fatalf("binary %d %s %s", code, out, errout)
		}
		for _, agent := range []string{"claude-code", "codex", "opencode", "pi"} {
			if !strings.Contains(out, `"agent": "`+agent+`"`) {
				t.Fatal("missing agent", agent, out)
			}
		}
		if args[0] == "doctor" {
			var health struct {
				Logs []struct {
					Agent, Root string
					Sessions    int
				}
			}
			if e = json.Unmarshal([]byte(out), &health); e != nil {
				t.Fatal(e)
			}
			if len(health.Logs) != 4 {
				t.Fatal(out)
			}
			for _, log := range health.Logs {
				if log.Sessions != 1 {
					t.Fatal(out)
				}
			}
			if !strings.Contains(out, "No change needed") {
				t.Fatal(out)
			}
		}
	}
	listing := testutil.Run(cli.New("test", cli.Options{Dir: repo, Now: func() time.Time { return newest.Add(time.Hour) }}), "sessions", "--json")
	var rows cli.Listing
	if listing.Exit != 0 || listing.JSON(&rows) != nil {
		t.Fatal(listing)
	}
	picked := 0
	for _, row := range rows.Sessions {
		if row.AutomaticPick {
			picked++
		}
	}
	if picked != 1 {
		t.Fatal(rows)
	}
}

func TestUnsupportedAdditionalFormats(t *testing.T) {
	for _, agent := range []string{"pi", "opencode"} {
		t.Run(agent, func(t *testing.T) {
			_, repo, _ := fixture(t, "claude-verified")
			file := installRecording(t, agent+"-verified", repo)
			if agent == "opencode" {
				b, _ := os.ReadFile(file)
				b = bytes.ReplaceAll(b, []byte(`"version": "1.1.4"`), []byte(`"version": "99.0"`))
				os.WriteFile(file, b, 0600)
			} else {
				b, _ := os.ReadFile(file)
				b = bytes.Replace(b, []byte(`"version": 3`), []byte(`"version": 99`), 1)
				os.WriteFile(file, b, 0600)
			}
			if _, e := agents.Read(file); e == nil || !strings.Contains(e.Error(), "unsupported") {
				t.Fatal("unsupported accepted", e)
			}
			result := testutil.Run(cli.New("test", cli.Options{Dir: repo}), "check", "--transcript", file, "--no-write", "--json")
			if result.Exit == 0 || strings.Contains(result.Stdout, `"verdict": "verified"`) {
				t.Fatal(result)
			}
			doctor := testutil.Run(cli.New("test", cli.Options{Dir: repo}), "doctor", "--json")
			if doctor.Exit != 0 || !strings.Contains(doctor.Stdout, `"unsupported_formats": 1`) || !strings.Contains(doctor.Stdout, "Record a supported session") {
				t.Fatal(doctor)
			}
		})
	}
}

func TestAdditionalRecordedFacts(t *testing.T) {
	for _, tc := range []struct {
		name, agent, kinds string
		commands, edits    int
		final              string
	}{
		{"pi-verified", "pi", "command,command,edit,command", 3, 1, "All tests pass now."},
		{"pi-stale", "pi", "command,edit,command,edit", 2, 2, "Summary: All tests pass."},
		{"opencode-verified", "opencode", "command,edit,edit,edit,command", 2, 3, "All tests pass."},
		{"opencode-stale", "opencode", "command,edit,command,edit", 2, 2, "All tests pass (`go test ./...` returned `ok`)."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, repo, path := fixture(t, tc.name)
			s, e := agents.Read(path)
			if e != nil {
				t.Fatal(e)
			}
			if s.Agent != tc.agent || s.Cwd != repo || s.Unsupported != 0 || len(s.Models) != 1 || s.Models[0] != "deepseek/deepseek-v4-flash-0731" || len(s.Prompts) != 1 || !strings.HasPrefix(s.Final, tc.final) || s.FinalTime.IsZero() || s.Started.IsZero() || s.Ended.Before(s.FinalTime) {
				t.Fatalf("session %+v", s)
			}
			metaBytes, e := os.ReadFile(filepath.Join("../../testdata/sessions", tc.name, "meta.json"))
			if e != nil {
				t.Fatal(e)
			}
			var meta struct{ Prompt string }
			json.Unmarshal(metaBytes, &meta)
			wantPrompt := meta.Prompt
			if tc.agent == "opencode" {
				wantPrompt = fmt.Sprintf("%q\n", meta.Prompt)
			}
			if s.Prompts[0].Text != wantPrompt {
				t.Fatal("prompt changed")
			}
			if tc.agent == "opencode" && s.AgentVersion != "1.1.4" {
				t.Fatal("version missing")
			}
			var kinds []string
			commands, edits := 0, 0
			for i, ev := range s.Events {
				if ev.Seq != i+1 || ev.Source.Line < 1 || !agents.Within(base, ev.Source.File) || (i > 0 && ev.Time.Before(s.Events[i-1].Time)) {
					t.Fatal("order/source", ev)
				}
				if c := ev.Command; c != nil {
					kinds = append(kinds, "command")
					commands++
					if !agents.Within(repo, c.Cwd) || c.End.Before(c.Start) || c.Start.IsZero() || c.End.IsZero() || c.ExitCode == nil || *c.ExitCode != 0 || !c.ExitReliable || c.RawOutput == "" {
						t.Fatalf("command %+v", c)
					}
				}
				if ev.Edit != nil {
					kinds = append(kinds, "edit")
					edits++
					if ev.Edit.Path != filepath.Join(repo, "calc", "stats.go") {
						t.Fatal("edit path", ev)
					}
				}
			}
			if strings.Join(kinds, ",") != tc.kinds || commands != tc.commands || edits != tc.edits {
				t.Fatalf("events %v commands=%d edits=%d", kinds, commands, edits)
			}
		})
	}
}
