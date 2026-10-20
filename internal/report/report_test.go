package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/gaugestone/internal/gitstate"
	"github.com/vincentsch/gaugestone/internal/redact"
	"github.com/vincentsch/gaugestone/internal/session"
)

func TestSharedReportRedaction(t *testing.T) {
	t.Setenv("HOME", "/home/fixture")
	secret := "sk-private12345678901234567890"
	s := &session.Session{Agent: "codex", ID: "session-id", Prompts: []session.Prompt{{Text: strings.Repeat("a", 210) + secret}}, Events: []session.Event{{Seq: 1, Command: &session.Command{Line: "TOKEN_KEY=" + secret + " cat /home/fixture/file", Cwd: "/home/fixture/repo", RawOutput: strings.Repeat("private earlier output\n", 45) + "DB_PASSWORD=hidden\n"}}}, Final: "full transcript content must stay private"}
	state := gitstate.State{Root: "/home/fixture/repo", Head: strings.Repeat("a", 40)}
	r := Build("test", time.Now(), state, s, []string{"/home/fixture/session"})
	value, e := r.SafeValue(redact.Redactor{Home: "/home/fixture"})
	if e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	text := string(b)
	for _, private := range []string{secret, "/home/fixture", "hidden", s.Final} {
		if strings.Contains(text, private) {
			t.Fatal("private value leaked", private)
		}
	}
	if r.Completeness.MissingExits != 1 || len(r.Session.Prompts[0]) != 200 || len(r.Commands) != 1 || strings.Count(r.Commands[0].Original.Output, "\n") != 39 {
		t.Fatal("summary or output tail incorrect", r.Completeness, r.Commands)
	}
	if !strings.Contains(text, state.Head) || !strings.Contains(text, "~/repo") || !strings.Contains(text, "[redacted]") {
		t.Fatal("redaction corrupted structure", text)
	}
}
func TestReportWithoutSession(t *testing.T) {
	r := Build("test", time.Now(), gitstate.State{}, nil, nil)
	b, e := json.Marshal(r)
	if e != nil || !strings.Contains(string(b), `"session":null`) {
		t.Fatal(string(b), e)
	}
}

func TestClaimEvidenceRedaction(t *testing.T) {
	t.Setenv("HOME", "/home/fixture")
	secret := "sk-private12345678901234567890"
	exit := 0
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	s := &session.Session{Agent: "codex", Cwd: "/home/fixture/repo", Final: "All tests pass with TOKEN_KEY=" + secret + ".", FinalTime: at.Add(time.Minute), Events: []session.Event{{Seq: 1, Time: at, Command: &session.Command{Line: "go test ./...", Cwd: "/home/fixture/repo", Shell: true, Start: at.Add(-time.Second), End: at, ExitCode: &exit, ExitReliable: true}}}}
	r := Build("test", at, gitstate.State{Root: s.Cwd}, s, nil)
	if len(r.Claims) != 1 {
		t.Fatal("claim missing")
	}
	value, err := r.SafeValue(redact.Redactor{Home: "/home/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), secret) || strings.Contains(string(b), "/home/fixture") || !strings.Contains(string(b), "[redacted]") {
		t.Fatal("claim evidence was not redacted")
	}
}
