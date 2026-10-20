package render

import (
	"fmt"
	"html"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/vincentsch/gaugestone/internal/commands"
	"github.com/vincentsch/gaugestone/internal/report"
)

const markdownLimit = 60000

// Markdown shortens values before assembling markup, then measures the escaped
// document. It never cuts a completed table, code span or details block.
func Markdown(w io.Writer, r report.Report) error {
	commands := []commands.Command{}
	for _, c := range r.Commands {
		failed, _ := commandResult(c)
		if failed || hasCheck(c) {
			commands = append(commands, c)
		}
	}
	n := min(30, len(commands))
	cap := 0
	lists := max(len(r.Claims), len(r.Findings), len(r.AdmittedGaps), len(r.Completeness.Notes))
	for _, g := range groupOrder {
		lists = max(lists, len(r.Groups[g]))
	}
	for {
		doc := markdownDocument(r, commands, n, cap, lists)
		if len(doc) <= markdownLimit {
			_, err := io.WriteString(w, doc)
			return err
		}
		if n > 0 {
			n = 0
			continue
		}
		if cap == 0 {
			cap = 800
			continue
		}
		if cap > 80 {
			cap /= 2
			continue
		}
		// Halving list sizes keeps very large sessions cheap to shorten.
		lists /= 2
	}
}
func shorten(s string, limit int) string {
	if limit == 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit]) + "… [truncated]"
}
func mdText(s string, limit int) string {
	s = strings.Join(strings.Fields(shorten(s, limit)), " ")
	s = strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "~", "\\~", "#", "\\#", "+", "\\+", "-", "\\-", "!", "\\!", "(", "\\(", ")", "\\)").Replace(s)
	return strings.ReplaceAll(html.EscapeString(s), "|", "&#124;")
}

// Inline HTML decodes escaped evidence consistently inside and outside tables.
// Backtick spans are kept for values that need no angle-bracket or pipe escaping.
func mdCode(s string, limit int) string {
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
	s = strings.ReplaceAll(s, "\n", " ⏎ ")
	s = strings.Join(strings.Fields(shorten(s, limit)), " ")
	// Entities and backslash escapes stay literal in backtick spans. Encode
	// these values as HTML instead, including marker comments and shell regexes.
	if strings.ContainsAny(s, "<>|") {
		value := html.EscapeString(s)
		value = strings.NewReplacer("\\", "&#92;", "|", "&#124;", "`", "&#96;", "*", "&#42;", "_", "&#95;", "~", "&#126;", "[", "&#91;", "]", "&#93;").Replace(value)
		return "<code>" + value + "</code>"
	}
	longest, run := 0, 0
	for _, c := range s {
		if c == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	ticks := strings.Repeat("`", longest+1)
	return ticks + " " + s + " " + ticks
}
func markdownDocument(r report.Report, selected []commands.Command, n, cap, lists int) string {
	var b strings.Builder
	text := func(s string) string { return mdText(s, cap) }
	code := func(s string) string { return mdCode(s, cap) }
	omitted := func(count int, what string) {
		if count > 0 {
			fmt.Fprintf(&b, "\n%d %s omitted. HTML/JSON contain the complete evidence.\n", count, what)
		}
	}
	b.WriteString("### Gaugestone\n\n")
	fmt.Fprintf(&b, "**Verdicts**: %s\n\n", verdictTotals(r))
	b.WriteString("| Claim | Verdict | Evidence |\n| --- | --- | --- |\n")
	for _, c := range r.Claims[:min(lists, len(r.Claims))] {
		evidence := text(c.Explanation)
		for _, run := range c.Runs {
			evidence += " · " + code(run.Command) + " at " + clock(run.End) + " (" + text(run.Result) + ")"
			if run.ExitNote != "" {
				evidence += " · " + text(run.ExitNote)
			}
		}
		for _, edit := range c.Edits {
			evidence += " · " + code(edit.Path) + " changed " + clock(edit.Time)
		}
		if c.NextStep != "" {
			evidence += " · " + text(c.NextStep)
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", code(c.Quote), labels[c.Verdict], evidence)
	}
	if len(r.Claims) == 0 {
		b.WriteString("\nClaims: none found in the agent's final message.\n")
	}
	omitted(len(r.Claims)-min(lists, len(r.Claims)), "claims")
	for _, g := range r.AdmittedGaps[:min(lists, len(r.AdmittedGaps))] {
		fmt.Fprintf(&b, "\nAdmitted gap: %s\n", text(g.Quote))
	}
	omitted(len(r.AdmittedGaps)-min(lists, len(r.AdmittedGaps)), "admitted gaps")
	fmt.Fprintf(&b, "\n%s\n", completeness(r))
	for _, note := range r.Completeness.Notes[:min(lists, len(r.Completeness.Notes))] {
		fmt.Fprintf(&b, "\n%s\n", text(note))
	}
	omitted(len(r.Completeness.Notes)-min(lists, len(r.Completeness.Notes)), "completeness notes")
	failed, unknown := 0, 0
	for _, c := range r.Commands {
		f, u := commandResult(c)
		if f {
			failed++
		}
		if u {
			unknown++
		}
	}
	fmt.Fprintf(&b, "\n**Commands**: %d run · %d failed · %d result unreliable\n\n<details><summary>Checks and failed commands (%d shown)</summary>\n\n", len(r.Commands), failed, unknown, n)
	b.WriteString("| Time | Command | Exit / result |\n| --- | --- | --- |\n")
	for _, c := range selected[:n] {
		row := commandRow(c, r)
		fmt.Fprintf(&b, "| %s | %s | %s |\n", row.Time, code(row.Line), text(row.Exit))
	}
	b.WriteString("\n</details>\n")
	omitted(len(selected)-n, "check or failed command lines")
	b.WriteString("\n**Changes**\n")
	total := 0
	for _, g := range groupOrder {
		files := r.Groups[g]
		total += len(files)
		if len(files) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s (%d):\n\n", groupLabel(g), len(files))
		for _, f := range files[:min(lists, len(files))] {
			fmt.Fprintf(&b, "- %s +%d/−%d", code(f.Path), f.Additions, f.Deletions)
			if f.ChangedAfterSession {
				b.WriteString(" · changed after the session ended")
			}
			b.WriteByte('\n')
		}
		omitted(len(files)-min(lists, len(files)), g+" files")
	}
	fmt.Fprintf(&b, "\n%d changed files in total.\n\n**Findings** (%d)\n\n", total, len(r.Findings))
	for _, f := range r.Findings[:min(lists, len(r.Findings))] {
		reason, dismissed := dismissal(r, f.ID)
		if dismissed {
			fmt.Fprintf(&b, "<details><summary>Dismissed: %s</summary>\n\n", mdCode(f.ID, 0))
		}
		fmt.Fprintf(&b, "- %s %s in %s: %s", mdCode(f.ID, 0), code(f.Code), code(fmt.Sprintf("%s:%d", f.Path, f.Line)), text(f.Explanation))
		if f.Old != nil || f.New != nil {
			fmt.Fprintf(&b, " · %s → %s", code(side(f.Old)), code(side(f.New)))
		}
		if dismissed {
			fmt.Fprintf(&b, " · Dismissed: %s", text(reason))
		}
		b.WriteByte('\n')
		if dismissed {
			b.WriteString("\n</details>\n\n")
		}
	}
	omitted(len(r.Findings)-min(lists, len(r.Findings)), "findings")
	fmt.Fprintf(&b, "\nGaugestone %s · generated %s", text(r.Version), stamp(r.Generated))
	if r.Session != nil {
		fmt.Fprintf(&b, " · %s session %s", text(r.Session.Agent), code(r.Session.ID))
	}
	fmt.Fprintf(&b, " · compared with %s\n\n%s\n\nHTML/JSON contain the complete evidence within privacy limits.\n", code(r.Repo.Baseline.Ref), disclaimer)
	return b.String()
}
func side(s *string) string {
	if s == nil {
		return "(absent)"
	}
	return *s
}
func groupLabel(g string) string {
	if g == "ci" {
		return "CI"
	}
	return strings.ToUpper(g[:1]) + g[1:]
}
