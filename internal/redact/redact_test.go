package redact

import (
	"strings"
	"testing"
)

func TestSharedRedaction(t *testing.T) {
	r := Redactor{Home: "/home/example"}
	for _, s := range []string{
		`API_KEY=secret COMMAND_TOKEN="two words" SOME_SECRET='private' PASSWORD: hidden`,
		`/home/example/project ghp_example12345678901234567890 sk-example12345678901234567890 github_pat_example123 xoxb-example12345678901234567890 AKIA1234567890123456`,
		`abcdefgh123.ijklmnop456.qrstuvwx789 digest=0123456789abcdef0123456789abcdef0123456789`,
	} {
		out := r.Text(s)
		for _, secret := range []string{"secret", "two words", "private", "hidden", "/home/example", "ghp_example", "sk-example", "github_pat_example", "xoxb-example12345678901234567890", "AKIA123", "abcdefgh123", "0123456789abcdef"} {
			if strings.Contains(out, secret) {
				t.Errorf("leaked %s in %s", secret, out)
			}
		}
	}
	if len([]rune(r.Prompt(strings.Repeat("ä", 300)))) != 200 {
		t.Fatal("prompt limit")
	}
	tail := r.Tail(strings.Repeat("old\n", 80) + "PASSWORD=secret\n")
	if len(strings.Split(tail, "\n")) != 40 || strings.Contains(tail, "secret") {
		t.Fatal("tail redaction")
	}
}

func TestTokenBoundariesAndPasswordVariables(t *testing.T) {
	r := Redactor{}
	plain := "Fix the task-runner so ask-me works in the risk-free path; sk-short ghp_short xoxb-short"
	if got := r.Text(plain); got != plain {
		t.Fatalf("ordinary text changed: %s", got)
	}
	assignments := `DB_PASSWORD=hunter2 POSTGRES_PASSWORD=hunter3 password='two words' MY_PASSWORD="three words"`
	want := `DB_PASSWORD=[redacted] POSTGRES_PASSWORD=[redacted] password=[redacted] MY_PASSWORD=[redacted]`
	if got := r.Text(assignments); got != want {
		t.Fatalf("password redaction: %s", got)
	}
	for _, prefix := range []string{"sk-", "ghp_", "xoxb-"} {
		token := prefix + strings.Repeat("a", 24)
		if got := r.Text("token " + token); got != "token [redacted]" {
			t.Fatalf("token visible: %s", got)
		}
		embedded := "word" + token
		if got := r.Text(embedded); got != embedded {
			t.Fatal("token boundary ignored")
		}
	}
}

func TestRedactedAssignmentsKeepSourcePunctuation(t *testing.T) {
	r := Redactor{}
	for _, source := range []string{`t.Skipf("API_TOKEN='private words'")`, `API_KEY=private; t.SkipNow()`, `PASSWORD="private" // explanation`} {
		once := r.Text(source)
		if twice := r.Text(once); twice != once {
			t.Fatalf("second redaction changed source: %q -> %q", once, twice)
		}
	}
}
