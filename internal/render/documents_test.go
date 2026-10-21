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
	"unicode/utf8"

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
	r.AdmittedGaps = []claims.Gap{{Quote: "**<img src=https://example.org onerror=alert(1)>** `<script>alert(1)</script>`"}}
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
	if !strings.Contains(h, "<strong>&lt;img src=https://example.org onerror=alert(1)&gt;</strong>") || !strings.Contains(h, "<code>&lt;script&gt;alert(1)&lt;/script&gt;</code>") {
		t.Fatal("admitted gap formatting or escaping missing")
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

func TestMarkdownByteBudgets(t *testing.T) {
	r := documentReport()
	r.Claims[0].Quote = strings.Repeat("界 | ` <!-- gaugestone:start -->\n", 200)
	var complete bytes.Buffer
	opts := MarkdownOptions{ByteBudget: 60000, LocalEvidence: true}
	if err := MarkdownWithOptions(&complete, r, opts); err != nil {
		t.Fatal(err)
	}
	for _, budget := range []int{complete.Len(), complete.Len() - 1, 1500, 1, -1} {
		t.Run(fmt.Sprint(budget), func(t *testing.T) {
			var b bytes.Buffer
			opts.ByteBudget = budget
			err := MarkdownWithOptions(&b, r, opts)
			if budget <= 1 {
				if err == nil || b.Len() != 0 {
					t.Fatal("tiny budget must fail without partial output")
				}
				return
			}
			if err != nil || b.Len() > budget || !utf8.Valid(b.Bytes()) {
				t.Fatalf("budget %d, bytes %d: %v", budget, b.Len(), err)
			}
			if strings.Contains(b.String(), "<!-- gaugestone:start -->") || strings.Contains(b.String(), "HTML/JSON contain") {
				t.Fatal("unsafe marker or misleading notice")
			}
			for _, required := range []string{"**Verdicts**", completeness(r), disclaimer, "Gaugestone test", "Run `gaugestone check` locally"} {
				if !strings.Contains(b.String(), required) {
					t.Fatal("missing", required)
				}
			}
			if budget == complete.Len() && b.String() != complete.String() {
				t.Fatal("exact byte budget shortened unnecessarily")
			}
			pandoc(t, b.String())
		})
	}
}

func TestHTMLQuotationFormatting(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"Ran `go test ./...`: **all tests pass**.", "Ran <code>go test ./...</code>: <strong>all tests pass</strong>."},
		{"__Alles grün__ mit `**literal** __literal__ <img src=https://example.org>`.", "<strong>Alles grün</strong> mit <code>**literal** __literal__ &lt;img src=https://example.org&gt;</code>."},
		{"**Passed `go test`**", "<strong>Passed <code>go test</code></strong>"},
		{"**A `**` B**", "<strong>A <code>**</code> B</strong>"},
		{"unmatched **bold and `code", "unmatched **bold and `code"},
		{"*italic* [link](https://example.org) ~~strike~~", "*italic* [link](https://example.org) ~~strike~~"},
		{"```literal``` ***literal*** ___literal___", "```literal``` ***literal*** ___literal___"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			if got := string(quoteView(tc.source).Full); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
			r := documentReport()
			r.Claims[0].Quote = tc.source
			r.AdmittedGaps = []claims.Gap{{Quote: tc.source}}
			h, _ := documents(t, r)
			if strings.Count(h, "<blockquote>"+tc.want+"</blockquote>") != 2 {
				t.Fatal("claims and admitted gaps must share quotation formatting")
			}
		})
	}
}

func TestHTMLQuotationPreview(t *testing.T) {
	for _, letter := range []string{"a", "ü", "界"} {
		for _, length := range []int{200, 201} {
			r := documentReport()
			source := strings.Repeat(letter, length)
			r.Claims[0].Quote = source
			r.AdmittedGaps = []claims.Gap{{Quote: source}}
			h, _ := documents(t, r)
			q := quoteView(source)
			if q.Long != (length > 200) || strings.Count(h, `class="quote-toggle"`) != 2*(length-200) {
				t.Fatalf("wrong controls for %d characters of %s", length, letter)
			}
			if length == 201 && string(q.Preview) != strings.Repeat(letter, 200)+"…" {
				t.Fatal("preview did not count source characters")
			}
		}
	}
	for _, delimiter := range []string{"`", "**", "__"} {
		tag := "strong"
		if delimiter == "`" {
			tag = "code"
		}
		source := strings.Repeat("ä", 190) + delimiter + strings.Repeat("ö", 20) + delimiter
		q := quoteView(source)
		want := strings.Repeat("ä", 190) + "<" + tag + ">" + strings.Repeat("ö", 10-len(delimiter)) + "</" + tag + ">…"
		if string(q.Preview) != want {
			t.Fatalf("unbalanced or incorrectly limited preview: %s", q.Preview)
		}
	}
}

func TestHTMLDurationAndHeader(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		elapsed time.Duration
		want    string
	}{
		{104900 * time.Microsecond, "104 ms"},
		{999999999, "999 ms"}, {time.Second, "1 s"},
		{18900 * time.Millisecond, "18 s"},
		{time.Minute - time.Nanosecond, "59 s"}, {time.Minute, "1 min 0 s"},
		{21*time.Minute + 3900*time.Millisecond, "21 min 3 s"},
		{time.Hour - time.Nanosecond, "59 min 59 s"}, {time.Hour, "1 h 0 min"},
		{time.Hour + 4*time.Minute + 59*time.Second, "1 h 4 min"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			r := documentReport()
			r.Session.Started, r.Session.Ended = start, start.Add(tc.elapsed)
			r.Commands[0].Original.Start, r.Commands[0].Original.End = start, start.Add(tc.elapsed)
			h, _ := documents(t, r)
			if !strings.Contains(h, " · "+tc.want+"<br>Models:") || !strings.Contains(h, `data-label="Duration">`+tc.want+"</td>") {
				t.Fatalf("duration missing: %s", tc.want)
			}
		})
	}
	for _, times := range [][2]time.Time{{{}, start}, {start, {}}, {{}, {}}} {
		r := documentReport()
		r.Session.Started, r.Session.Ended = times[0], times[1]
		r.Commands[0].Original.Start, r.Commands[0].Original.End = times[0], times[1]
		h, _ := documents(t, r)
		if !strings.Contains(h, " · unknown<br>Models:") || !strings.Contains(h, `data-label="Duration">unknown</td>`) {
			t.Fatal("missing timestamps should show unknown")
		}
	}
	r := documentReport()
	r.Repo.Root = "~/projects/sample-folder"
	r.Repo.Baseline.Commit = "3890c7b1234567890"
	r.Session.ID = "full-session-123456789"
	h, _ := documents(t, r)
	for _, want := range []string{"<strong>sample-folder</strong>", "<code>~/projects/sample-folder</code>", "<code>3890c7b</code> (HEAD, uncommitted changes only)", "<code>full-session-123456789</code>", `<summary>Output</summary>`} {
		if !strings.Contains(h, want) {
			t.Fatal("missing header or command context", want)
		}
	}
	if strings.Contains(h, r.Repo.Baseline.Commit) {
		t.Fatal("HTML should abbreviate baseline")
	}
}

func TestWritePreservesFormattedQuotes(t *testing.T) {
	r := documentReport()
	r.Repo.Baseline.Commit = "3890c7b1234567890"
	r.Session.ID = "full-session-123456789"
	r.Claims[0].Quote = "Ran `go test ./...`: **all tests pass**."
	r.AdmittedGaps = []claims.Gap{{Quote: "Could not run `pytest` with __this setup__."}}
	before, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := Write(dir, r); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("rendering mutated the shared report")
	}
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved report.Report
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Repo.Baseline.Commit != r.Repo.Baseline.Commit || saved.Session.ID != r.Session.ID {
		t.Fatal("saved JSON shortened header identifiers")
	}
	if saved.Claims[0].Quote != r.Claims[0].Quote || saved.AdmittedGaps[0].Quote != r.AdmittedGaps[0].Quote {
		t.Fatal("saved JSON changed quote strings")
	}
	data, err = os.ReadFile(filepath.Join(dir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, escaped := range []string{mdCode(r.Claims[0].Quote, 0), mdText(r.AdmittedGaps[0].Quote, 0)} {
		if !strings.Contains(string(data), escaped) {
			t.Fatal("saved Markdown lost original quote delimiters")
		}
	}
}
