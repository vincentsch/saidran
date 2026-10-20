package render

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/vincentsch/assaylight/internal/claims"
	"github.com/vincentsch/assaylight/internal/commands"
	"github.com/vincentsch/assaylight/internal/findings"
	"github.com/vincentsch/assaylight/internal/gitstate"
	"github.com/vincentsch/assaylight/internal/report"
	"github.com/vincentsch/assaylight/internal/session"
	"github.com/vincentsch/rungrad/output"
)

func TestTextEvidenceAndWrapping(t *testing.T) {
	code := 0
	command := commands.Analyze(&session.Command{Line: "go test ./...; go test ./...", Shell: true, Cwd: "/repo", ExitCode: &code, ExitReliable: true, RawOutput: "FAIL example/calc 0.1s\n"})
	// Repeated runner output cannot identify which test statement failed. The
	// first statement stays unknown; the line's exit verifies only the second.
	r := report.Report{Session: &report.Session{Agent: "codex", ID: "123456789", Started: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}, Commands: []commands.Command{command}, Groups: map[string][]gitstate.File{"tests": {{Path: "calc/stats_test.go", ChangedAfterSession: true}}}, Findings: []findings.Finding{{ID: "F-0123456789ab", Code: "test_skipped", Path: "calc/stats_test.go", Line: 26}}, AdmittedGaps: []claims.Gap{{Quote: "I couldn't run lint."}}, Completeness: report.Completeness{Unsupported: 1, MissingExits: 1, Notes: []string{"The recording is incomplete."}}}
	for _, width := range []int{20, 60, 80, 100, 150} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			var b bytes.Buffer
			Text(&b, r, r.Session.ID, width, output.TerminalMode{})
			compact := strings.Join(strings.Fields(b.String()), " ")
			for _, want := range []string{"Claims none found in the agent's final message", "1 run · 0 failed · 1 result unreliable", "F-0123456789ab", "test skipped in calc/stats_test.go:26", "changed after the session ended", "I couldn't run lint.", "Incomplete evidence", "The recording is incomplete."} {
				if !strings.Contains(compact, want) {
					t.Fatalf("missing %q in %s", want, &b)
				}
			}
			max := width
			if max < 60 {
				max = 60
			}
			if max > 100 {
				max = 100
			}
			for _, line := range strings.Split(b.String(), "\n") {
				if utf8.RuneCountInString(line) > max {
					t.Fatalf("line exceeds width %d: %s", max, line)
				}
			}
		})
	}
}
func TestLineCountsAndVerdictColor(t *testing.T) {
	exit := 0
	command := commands.Analyze(&session.Command{Line: "go test ./...; go test ./calc", Shell: true, Cwd: "/repo", ExitCode: &exit, ExitReliable: true, RawOutput: "FAIL example/calc 0.1s\n"})
	// Set independent attributable summaries to exercise per-line de-duplication.
	for i := range command.Statements {
		command.Statements[i].Tests = &commands.TestSummary{Outcome: "failed"}
	}
	r := report.Report{Session: &report.Session{}, Commands: []commands.Command{command}, Claims: []claims.Claim{{Verdict: "contradicted", Quote: strings.Repeat("ü", 140), Explanation: "The check failed.", NextStep: "Run it again."}}}
	for _, mode := range []output.TerminalMode{{}, {ANSI: true}, {Color: true}, {ANSI: true, Color: true}} {
		var b bytes.Buffer
		Text(&b, r, r.Session.ID, 60, mode)
		got := b.String()
		if !strings.Contains(got, "1 run · 1 failed · 0 result unreliable") {
			t.Fatal(got)
		}
		if strings.Contains(got, "\x1b[") != (mode.ANSI && mode.Color) || !strings.Contains(got, "Contradicted") {
			t.Fatal(got)
		}
		plain := string(output.SanitizeControlBytes([]byte(got)))
		for _, line := range strings.Split(plain, "\n") {
			if utf8.RuneCountInString(line) > 60 {
				t.Fatal(line)
			}
		}
	}
}

func TestHeaderDisplayIDAndLocalTime(t *testing.T) {
	previous := time.Local
	time.Local = time.FixedZone("CEST", 2*60*60)
	t.Cleanup(func() { time.Local = previous })
	r := report.Report{Session: &report.Session{ID: "01a15ab5-full-session-id", Agent: "codex", Started: time.Date(2026, 10, 20, 18, 30, 0, 0, time.UTC), Ended: time.Date(2026, 10, 20, 19, 6, 0, 0, time.UTC)}, Groups: map[string][]gitstate.File{"tests": {{Path: "calc/stats_test.go"}}}}
	var b bytes.Buffer
	Text(&b, r, "01a15ab5a", 100, output.TerminalMode{})
	for _, want := range []string{"session 01a15ab5a ", "2026-10-20 20:30–21:06", "1 file · 1 test"} {
		if !strings.Contains(b.String(), want) {
			t.Fatalf("missing %q: %s", want, &b)
		}
	}
}
