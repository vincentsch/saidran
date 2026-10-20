package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/assaylight/internal/gitstate"
	"github.com/vincentsch/assaylight/internal/redact"
	"github.com/vincentsch/assaylight/internal/session"
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
