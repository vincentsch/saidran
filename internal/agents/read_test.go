package agents_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/assaylight/internal/agents"
	"github.com/vincentsch/assaylight/internal/session"
)

func writeLog(t *testing.T, file string, records ...string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Join(records, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}
func TestClaudeToolEvidence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "parent.jsonl")
	writeLog(t, file,
		`{"type":"user","sessionId":"parent","cwd":"/repo","timestamp":"2026-10-20T10:00:00Z","message":{"content":"Fix it"}}`,
		`{"type":"assistant","uuid":"a","cwd":"/repo/child","timestamp":"2026-10-20T10:00:01Z","message":{"id":"m","content":[{"type":"text","text":"First"},{"type":"tool_use","id":"bash","name":"Bash","input":{"command":"go test ./..."}},{"type":"tool_use","id":"bad","name":"Edit","input":{"file_path":"/repo/bad.go"}},{"type":"tool_use","id":"wait","name":"Write","input":{"file_path":"/repo/wait.go"}},{"type":"tool_use","id":"bg","name":"Bash","input":{"command":"sleep 90","run_in_background":true}},{"type":"tool_use","id":"task","name":"Agent","input":{}},{"type":"tool_use","id":"pending","name":"Bash","input":{"command":"go vet ./..."}}]}}`,
		`{"type":"user","timestamp":"2026-10-20T10:00:02Z","message":{"content":[{"type":"tool_result","tool_use_id":"bash","content":"Exit code 3\nfailed","is_error":true},{"type":"tool_result","tool_use_id":"bad","content":"failed","is_error":true},{"type":"tool_result","tool_use_id":"bg","content":"started"}]}}`,
		`{"type":"user","toolUseResult":{"agentId":"missing"},"message":{"content":[{"type":"tool_result","tool_use_id":"task","content":"Agent finished"}]}}`,
		`{"type":"assistant","uuid":"split","timestamp":"2026-10-20T10:00:03Z","message":{"id":"m","content":[{"type":"text","text":"Second"}]}}`,
		`{"type":"attachment","timestamp":"2026-10-20T10:00:04Z","message":{"content":"attachment"}}`,
		`{"type":"assistant","isSidechain":true,"uuid":"side","timestamp":"2026-10-20T10:00:05Z","message":{"content":[{"type":"text","text":"Child response"}]}}`)
	child := filepath.Join(strings.TrimSuffix(file, ".jsonl"), "subagents", "agent-child.jsonl")
	writeLog(t, child,
		`{"type":"assistant","uuid":"split","message":{"content":[{"type":"text","text":"duplicate"}]}}`,
		`{"type":"assistant","uuid":"c","timestamp":"2026-10-20T10:00:02Z","cwd":"/repo","message":{"content":[{"type":"tool_use","id":"edit","name":"NotebookEdit","input":{"notebook_path":"/repo/book.ipynb"}},{"type":"tool_use","id":"bash","name":"Bash","input":{"command":"child"}}]}}`,
		`{"type":"user","timestamp":"2026-10-20T10:00:03Z","toolUseResult":{"interrupted":true},"message":{"content":[{"type":"tool_result","tool_use_id":"bash","content":"stopped"}]}}`,
		`{"type":"user","timestamp":"2026-10-20T10:00:04Z","message":{"content":[{"type":"tool_result","tool_use_id":"edit","content":"saved"}]}}`)
	s, e := agents.Read(file)
	if e != nil {
		t.Fatal(e)
	}
	if s.Final != "First\nSecond" || s.Unsupported != 0 || len(s.Notes) != 1 {
		t.Fatalf("final=%q unsupported=%d notes=%v", s.Final, s.Unsupported, s.Notes)
	}
	counts := map[string]int{}
	commands := map[string]*session.Command{}
	for _, ev := range s.Events {
		counts[ev.Kind]++
		if ev.Command != nil {
			commands[ev.Command.Line] = ev.Command
		}
		if ev.Edit != nil && (ev.Edit.Path != "/repo/book.ipynb" || ev.Source.File != child) {
			t.Fatal("failed edit confirmed or child source lost")
		}
	}
	if counts["edit"] != 1 {
		t.Fatal(counts)
	}
	if c := commands["go test ./..."]; c.Cwd != "/repo/child" || c.ExitCode == nil || *c.ExitCode != 3 {
		t.Fatal("Bash facts lost")
	}
	if !commands["sleep 90"].End.IsZero() {
		t.Fatal("background notice treated as command completion")
	}
	for _, line := range []string{"sleep 90", "go vet ./...", "child"} {
		c := commands[line]
		if c == nil || c.ExitCode != nil || c.ExitReliable {
			t.Fatalf("unreliable command %s %+v", line, c)
		}
	}
}
func TestLargeAndIncompleteRecords(t *testing.T) {
	file := filepath.Join(t.TempDir(), "session.jsonl")
	content := strings.Repeat("output\n", 20000)
	b, _ := json.Marshal(map[string]any{"type": "user", "sessionId": "large", "message": map[string]any{"content": content}})
	writeLog(t, file, string(b), `{"type":"unknown"}`)
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString(`{"type":`)
	f.Close()
	s, e := agents.Read(file)
	if e != nil || len(s.Prompts) != 1 || s.Prompts[0].Text != content || len(s.Notes) != 1 || s.Unsupported != 1 {
		t.Fatalf("large/partial: %v %+v", e, s)
	}
	writeLog(t, file, string(b), `{"type":`, `{"type":"system"}`)
	if _, e = agents.Read(file); e == nil || !strings.Contains(e.Error(), ":2:") {
		t.Fatal("malformed interior line accepted", e)
	}
}
func TestCodexDirectCommands(t *testing.T) {
	file := filepath.Join(t.TempDir(), "session.jsonl")
	writeLog(t, file,
		`{"type":"session_meta","timestamp":"2026-10-20T10:00:00Z","payload":{"id":"direct","cwd":"file:///repo/with%20space"}}`,
		`{"type":"event_msg","timestamp":"2026-10-20T10:00:05Z","payload":{"type":"item_completed","item":{"id":"a","type":"CommandExecution","command":["go","test","a; echo wrong"],"cwd":"file:///repo/child%20dir","duration":{"secs":2,"nanos":100},"status":"failed","exit_code":2,"aggregated_output":"whole output"}}}`,
		`{"type":"event_msg","timestamp":"2026-10-20T10:00:06Z","payload":{"type":"item_completed","item":{"id":"b","type":"CommandExecution","command":["/usr/bin/python","-c","print(1)"],"status":"running","exit_code":0}}}`,
		`{"type":"event_msg","timestamp":"2026-10-20T10:00:07Z","payload":{"type":"item_completed","item":{"id":"c","type":"CommandExecution","command":["/bin/zsh","-lc","go test ./..."],"status":"completed","exit_code":0}}}`,
		`{"type":"event_msg","timestamp":"2026-10-20T10:00:08Z","payload":{"type":"item_completed","item":{"id":"m","type":"AgentMessage","content":[{"type":"Text","text":"Done"}]}}}`,
		`{"type":"event_msg","timestamp":"2026-10-20T10:00:09Z","payload":{"type":"task_complete","last_agent_message":"Done"}}`,
		`{"type":"response_item","payload":{"type":"function_call"}}`,
		`{"type":"event_msg","payload":{"type":"unknown"}}`)
	s, e := agents.Read(file)
	if e != nil {
		t.Fatal(e)
	}
	if s.Cwd != "/repo/with space" || s.Final != "Done" || len(s.Events) != 4 || s.Unsupported != 1 {
		t.Fatalf("metadata or dedup: %+v", s)
	}
	c := s.Events[0].Command
	if c.Shell || len(c.Argv) != 3 || c.Cwd != "/repo/child dir" || c.End.Sub(c.Start) != 2*time.Second+100 || c.Output != "whole output" {
		t.Fatalf("command %+v", c)
	}
	if s.Events[1].Command.Shell || s.Events[1].Command.ExitCode != nil || !s.Events[2].Command.Shell {
		t.Fatal("shell or status handling")
	}
	writeLog(t, file, `{"type":"session_meta","payload":{"id":"old"}}`, `{"type":"response_item","payload":{"type":"function_call"}}`)
	if _, e = agents.Read(file); e == nil || !strings.Contains(e.Error(), "update Codex") {
		t.Fatal("old format accepted", e)
	}
}

func TestClaudeEditVariantsAndFullOutput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "session.jsonl")
	writeLog(t, file,
		`{"type":"user","sessionId":"edits","cwd":"/repo","message":{"content":"Update files"}}`,
		`{"type":"assistant","cwd":"/repo","timestamp":"2026-10-20T10:00:00Z","message":{"content":[{"type":"tool_use","id":"write","name":"Write","input":{"file_path":"/repo/new.go"}},{"type":"tool_use","id":"multi","name":"MultiEdit","input":{"file_path":"/repo/existing.go"}},{"type":"tool_use","id":"run","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"user","timestamp":"2026-10-20T10:00:01Z","message":{"content":[{"type":"tool_result","tool_use_id":"write","content":"saved"},{"type":"tool_result","tool_use_id":"multi","content":"saved"}]}}`)
	raw := strings.Repeat("old output\n", 10000) + "PASSWORD=private\n"
	result := map[string]any{"type": "user", "timestamp": "2026-10-20T10:00:02Z", "toolUseResult": map[string]any{"stdout": raw, "stderr": ""}, "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "run", "content": "display was cut"}}}}
	b, _ := json.Marshal(result)
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	f.Write(b)
	f.WriteString("\n")
	f.Close()
	s, e := agents.Read(file)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Events) != 3 || s.Events[1].Edit.Op != "write" || s.Events[2].Edit.Op != "edit" {
		t.Fatalf("edit variants %+v", s.Events)
	}
	c := s.Events[0].Command
	encoded, _ := json.Marshal(c)
	if strings.Contains(string(encoded), "private") {
		t.Fatal("raw output included in serialized command")
	}
	if c == nil || c.RawOutput != raw || len(strings.Split(c.Output, "\n")) != 40 || strings.Contains(c.Output, "private") {
		t.Fatal("full output or redacted display tail lost")
	}
}

func TestClaudeExitTextAndAgentMentions(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		isError      bool
		exit         *int
		status       string
	}{
		{"success printing exit text", "Printed log\nExit code 1\nagentId: missing", false, intPointer(0), "completed"},
		{"success starting with exit text", "Exit code 2\nagentId: missing", false, intPointer(0), "completed"},
		{"tool error prefix", "Exit code 3\nfailed", true, intPointer(3), "failed"},
		{"tool error without prefix", "Command failed\nExit code 4", true, nil, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "session.jsonl")
			writeLog(t, file, `{"type":"user","sessionId":"exit-text","cwd":"/repo","message":{"content":"Check"}}`,
				`{"type":"assistant","cwd":"/repo","message":{"content":[{"type":"tool_use","id":"bash","name":"Bash","input":{"command":"cat log.txt"}},{"type":"tool_use","id":"read","name":"Read","input":{"file_path":"/repo/log.txt"}}]}}`)
			result := map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "bash", "content": tc.output, "is_error": tc.isError}, map[string]any{"type": "tool_result", "tool_use_id": "read", "content": "agentId: missing"}}}}
			b, _ := json.Marshal(result)
			f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.Write(append(b, '\n'))
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			s, err := agents.Read(file)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Notes) != 0 || len(s.Events) != 1 {
				t.Fatal("printed text created sub-agent evidence")
			}
			c := s.Events[0].Command
			if c.Status != tc.status || c.ExitReliable != (tc.exit != nil) {
				t.Fatalf("status=%s reliable=%t", c.Status, c.ExitReliable)
			}
			if tc.exit == nil {
				if c.ExitCode != nil {
					t.Fatal("invented exit")
				}
			} else if c.ExitCode == nil || *c.ExitCode != *tc.exit {
				t.Fatalf("exit=%v want=%d", c.ExitCode, *tc.exit)
			}
		})
	}
}
func intPointer(n int) *int { return &n }
