package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/gaugestone/internal/claims"
	"github.com/vincentsch/gaugestone/internal/commands"
	"github.com/vincentsch/gaugestone/internal/findings"
	"github.com/vincentsch/gaugestone/internal/gitstate"
	"github.com/vincentsch/gaugestone/internal/report"
	"github.com/vincentsch/gaugestone/internal/session"
)

func documentReport() report.Report {
	now := time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)
	s := &session.Session{ID: "example", Agent: "codex", Started: now.Add(-time.Minute), Ended: now, Final: "All tests pass.", FinalTime: now}
	zero := 0
	s.Events = []session.Event{{Seq: 1, Time: now.Add(-time.Second), Command: &session.Command{Line: "gofmt -w . && go test ./...", Cwd: "/example", Shell: true, Start: now.Add(-time.Second), End: now, ExitCode: &zero, ExitReliable: true, Output: "ok example 0.1s"}}}
	return report.Build("test-version", now, gitstate.State{Root: "example", Branch: "main", Baseline: gitstate.Baseline{Ref: "HEAD", Reason: "uncommitted changes only"}}, s, nil)
}
func documents(t *testing.T, r report.Report) (string, string) {
	t.Helper()
	var h, m bytes.Buffer
	if err := HTML(&h, r); err != nil {
		t.Fatal(err)
	}
	if err := Markdown(&m, r); err != nil {
		t.Fatal(err)
	}
	return h.String(), m.String()
}
func pandoc(t *testing.T, md string) string {
	t.Helper()
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc is not installed")
	}
	cmd := exec.Command("pandoc", "-f", "gfm", "-t", "html")
	cmd.Stdin = strings.NewReader(md)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pandoc: %v %s", err, b)
	}
	html := string(b)
	if strings.Count(html, "<details>") < 1 || strings.Count(html, "<details>") != strings.Count(html, "</details>") || strings.Count(html, "<table>") != strings.Count(html, "</table>") {
		t.Fatal("unbalanced Markdown HTML")
	}
	if strings.Count(html, "<h3") != 1 {
		t.Fatal("unexpected report sections")
	}
	return html
}
func TestDocumentsEscapeUntrustedEvidence(t *testing.T) {
	r := documentReport()
	poison := "a | b\n``` ` <script>alert(1)</script> <!-- gaugestone:start --> <!-- gaugestone:end --> <!-- gaugestone -->"
	r.Claims[0].Quote = poison
	r.Claims[0].Explanation = poison
	r.Commands[0].Original.Line = poison
	r.Commands[0].Output = poison
	r.Groups["source"] = []gitstate.File{{Path: poison, ChangedAfterSession: true}}
	r.Findings = []findings.Finding{{ID: "F-0123456789ab", Path: poison, Code: "check_disabled", Explanation: poison}}
	r.Dismissals = []report.Dismissal{{ID: r.Findings[0].ID, Reason: poison}}
	h, m := documents(t, r)
	for _, marker := range []string{"<!-- gaugestone:start -->", "<!-- gaugestone:end -->", "<!-- gaugestone -->", "<script>alert(1)</script>"} {
		if strings.Contains(h, marker) || strings.Contains(m, marker) {
			t.Fatalf("unescaped %q", marker)
		}
	}
	if !strings.Contains(m, "<code>") || !strings.Contains(m, "&#124;") || !strings.Contains(h, "&lt;script&gt;") {
		t.Fatal("escaping missing")
	}
	for _, text := range []string{"F-0123456789ab", "changed after the session ended", disclaimer, "Dismissed:"} {
		if !strings.Contains(m, text) {
			t.Fatalf("missing %s", text)
		}
	}
	if !strings.Contains(h, `gaugestone dismiss F-0123456789ab --reason`) {
		t.Fatal("missing copy command")
	}
	rendered := pandoc(t, m)
	if strings.Contains(rendered, "<script>") || strings.Contains(rendered, "<!-- gaugestone") {
		t.Fatal("unsafe rendered Markdown")
	}
	// Network attributes and CSS must remain local. Untrusted text has no way to
	// create an attribute, even when it contains a URL.
	external := regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["']https?://|url\(\s*["']?https?://|@import`)
	if external.MatchString(h) {
		t.Fatal("external resource")
	}
}
func TestMarkdownOverflow(t *testing.T) {
	for _, kind := range []string{"claims", "findings", "dismissals", "paths", "commands"} {
		t.Run(kind, func(t *testing.T) {
			r := documentReport()
			r.Commands = nil
			long := strings.Repeat("|`<script>界", 300)
			switch kind {
			case "claims":
				c := r.Claims[0]
				c.Quote = long
				c.Explanation = long
				c.NextStep = long
				for range 1500 {
					r.Claims = append(r.Claims, c)
				}
			case "findings", "dismissals":
				for i := range 1500 {
					id := fmt.Sprintf("F-%012d", i)
					f := findings.Finding{ID: id, Code: "test_skipped", Path: "test.go", Explanation: "Added skip"}
					if kind == "findings" {
						f.Explanation = long
					}
					r.Findings = append(r.Findings, f)
					if kind == "dismissals" {
						r.Dismissals = append(r.Dismissals, report.Dismissal{ID: id, Reason: long})
					}
				}
			case "paths":
				for range 1500 {
					r.Groups["source"] = append(r.Groups["source"], gitstate.File{Path: long})
				}
			case "commands":
				for range 40 {
					c := documentReport().Commands[0]
					c.Original.Line = long
					r.Commands = append(r.Commands, c)
				}
			}
			var m bytes.Buffer
			if err := Markdown(&m, r); err != nil {
				t.Fatal(err)
			}
			if m.Len() > markdownLimit {
				t.Fatalf("oversize %d", m.Len())
			}
			for _, text := range []string{"omitted", "HTML/JSON contain the complete evidence", "**Verdicts**", completeness(r), disclaimer} {
				if !strings.Contains(m.String(), text) {
					t.Fatalf("missing %s", text)
				}
			}
			pandoc(t, m.String())
			if kind == "dismissals" && !strings.Contains(m.String(), "truncated") {
				t.Fatal("no visible reason truncation")
			}
			// Rendering shortened Markdown must leave the source model intact.
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(b, []byte("script")) {
				t.Fatal("model changed")
			}

			var complete bytes.Buffer
			if err := HTML(&complete, r); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(complete.String(), html.EscapeString(long)) {
				t.Fatal("HTML shortened evidence")
			}
			if kind == "findings" || kind == "dismissals" {
				for _, f := range r.Findings {
					if !bytes.Contains(complete.Bytes(), []byte(f.ID)) {
						t.Fatal("HTML omitted finding", f.ID)
					}
				}
			}
		})
	}
}
func TestMarkdownNotesAndCheckSelection(t *testing.T) {
	r := documentReport()
	r.Claims = nil
	r.AdmittedGaps = []claims.Gap{{Quote: "I couldn't run the tests."}}
	r.Completeness.Unsupported = 2
	r.Completeness.MissingExits = 1
	r.Completeness.Notes = []string{"An incomplete final line was skipped."}
	r.Groups["tests"] = []gitstate.File{{Path: "test.go", ChangedAfterSession: true}}
	// A formatter before the check must not hide that original command line.
	r.Commands = append(r.Commands, commands.Analyze(&session.Command{Line: "echo hello", Shell: true}))
	_, m := documents(t, r)
	for _, text := range []string{"none found in the agent&#39;s final message", "I couldn&#39;t run the tests.", "2 unsupported entries, 1 missing exit codes", "incomplete final line", "gofmt -w . && go test", "changed after the session ended", disclaimer} {
		if text == "none found in the agent&#39;s final message" {
			text = "none found in the agent's final message"
		}
		if !strings.Contains(m, text) {
			t.Fatalf("missing %s in %s", text, m)
		}
	}
	if strings.Contains(m, "echo hello") {
		t.Fatal("included non-check successful command")
	}
	pandoc(t, m)
}
func TestWriteKeepsCompleteHTMLAndJSON(t *testing.T) {
	r := documentReport()
	r.Claims[0].Quote = strings.Repeat("long evidence ", 10000)
	dir := t.TempDir()
	if err := Write(dir, r); err != nil {
		t.Fatal(err)
	}
	h, _ := os.ReadFile(filepath.Join(dir, "report.html"))
	j, _ := os.ReadFile(filepath.Join(dir, "report.json"))
	m, _ := os.ReadFile(filepath.Join(dir, "report.md"))
	if !bytes.Contains(h, []byte(r.Claims[0].Quote)) || !bytes.Contains(j, []byte(r.Claims[0].Quote)) {
		t.Fatal("complete formats lost evidence")
	}
	if len(m) > markdownLimit {
		t.Fatal("oversize Markdown")
	}
}

func TestSavedFindingValuesSurviveRedactionRoundTrip(t *testing.T) {
	r := documentReport()
	value := "go test ./..."
	r.Findings = []findings.Finding{{ID: "F-0123456789ab", Code: "test_command_changed", Path: "package.json", New: &value, Values: true}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded report.Report
	if err = json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = Write(dir, decoded); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte(`"old": null`)) || !bytes.Contains(saved, []byte(value)) {
		t.Fatal("saved JSON lost absent command side")
	}
	h, m := documents(t, decoded)
	if !strings.Contains(h, "(absent)") || !strings.Contains(m, "(absent)") {
		t.Fatal("report lost absent command side")
	}
}

func TestMarkdownShellRegexPipeStaysInOneCell(t *testing.T) {
	r := documentReport()
	r.Commands[0].Original.Line = "go test ./... | grep 'a\\|b' # `quoted`"
	_, m := documents(t, r)
	rendered := pandoc(t, m)
	if !strings.Contains(rendered, `a\|b`) || !strings.Contains(rendered, "`quoted`") {
		t.Fatal("shell regex or backticks changed", rendered)
	}
	if strings.Count(rendered, "<td>") != 6 {
		t.Fatal("pipe split a table cell", rendered)
	}
}

func TestMarkdownCodePreservesShellTextInTablesAndLists(t *testing.T) {
	for _, value := range []string{"go test ./... 2>&1 | tail", "jest || true", "value < limit > minimum"} {
		t.Run(value, func(t *testing.T) {
			r := documentReport()
			r.Commands[0].Original.Line = value
			r.Findings = []findings.Finding{{ID: "F-0123456789ab", Code: "test_command_changed", Path: "package.json", Old: &value, New: &value, Values: true}}
			_, md := documents(t, r)
			rendered := strings.Join(strings.Fields(pandoc(t, md)), " ")
			code := "<code>" + html.EscapeString(value) + "</code>"
			if !strings.Contains(rendered, "<td>"+code+"</td>") {
				t.Fatalf("command text changed in table: %s", rendered)
			}
			if !strings.Contains(rendered, code+" → "+code+"</li>") {
				t.Fatalf("finding values changed in list: %s", rendered)
			}
		})
	}
}

func TestMarkdownKeepsCommandLineBreaksVisible(t *testing.T) {
	r := documentReport()
	r.Commands[0].Original.Line = "gofmt -w .\r\ngo test ./...\ngit diff --check"
	r.Claims[0].Runs = []claims.Run{{Command: "go test ./...", ExitNote: "a later statement determines the line exit"}}
	r.Claims[0].NextStep = "No rerun needed."
	_, md := documents(t, r)
	rendered := strings.Join(strings.Fields(pandoc(t, md)), " ")
	if !strings.Contains(rendered, "<code>gofmt -w . ⏎ go test ./... ⏎ git diff --check</code>") {
		t.Fatal("command line breaks disappeared", rendered)
	}
	if !strings.Contains(rendered, "a later statement determines the line exit · No rerun needed.") {
		t.Fatal("next step runs into exit note", rendered)
	}
}

func TestMarkdownBacktickDelimitersPreserveQuotes(t *testing.T) {
	r := documentReport()
	r.Claims[0].Quote = "a ``` b"
	_, md := documents(t, r)
	if !strings.Contains(md, "```` a ``` b ````") {
		t.Fatal("code delimiter does not exceed embedded backticks")
	}
	if !strings.Contains(pandoc(t, md), "<code>a ``` b</code>") {
		t.Fatal("backticks changed in rendered quote")
	}
}
