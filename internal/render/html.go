package render

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/vincentsch/assaylight/internal/commands"
	"github.com/vincentsch/assaylight/internal/findings"
	"github.com/vincentsch/assaylight/internal/report"
)

//go:embed report.html
var htmlSource string
var htmlTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"label": func(v string) string { return labels[v] }, "clock": clock, "stamp": stamp, "join": strings.Join,
	"preview": func(s string) string { return shorten(s, 200) }, "side": side,
}).Parse(htmlSource))

type commandView struct {
	Time, Line, Exit, Duration, Classes   string
	Failed, Check, Unreliable, EditsAfter bool
	Command                               commands.Command
}
type findingView struct {
	findings.Finding
	Reason    string
	Dismissed bool
	Copy      string
}
type fileView struct {
	Path, Status         string
	Additions, Deletions int
	After                bool
	Findings             []findingView
}
type groupView struct {
	Label string
	Files []fileView
}
type verdictCount struct {
	Label, Verdict string
	Count          int
}

type htmlView struct {
	report.Report
	Verdicts                                      []verdictCount
	Disclaimer, Completeness                      string
	Rows                                          []commandView
	Files                                         []groupView
	FindingRows                                   []findingView
	Failed, Unreliable, FileCount, DismissedCount int
}

func commandRow(c commands.Command, r report.Report) commandView {
	v := commandView{Command: c, Check: hasCheck(c)}
	v.Failed, v.Unreliable = commandResult(c)
	if c.Original != nil {
		o := c.Original
		v.Time = clock(o.Start)
		v.Line = o.Line
		v.Exit = "unknown"
		if o.ExitCode != nil {
			v.Exit = fmt.Sprint(*o.ExitCode)
		}
		if !o.ExitReliable {
			v.Exit += " · line exit unreliable"
		}
		if v.Failed {
			v.Exit += " · failed"
		}
		if v.Unreliable {
			v.Exit += " · result unreliable"
		}
		if !o.Start.IsZero() && !o.End.IsZero() {
			v.Duration = o.End.Sub(o.Start).Round(time.Millisecond).String()
		}
		for _, e := range r.Edits {
			if e.Time.After(o.End) {
				v.EditsAfter = true
				break
			}
		}
	}
	classes := []string{}
	for _, st := range c.Statements {
		if !strings.Contains(" "+strings.Join(classes, " ")+" ", " "+st.Class+" ") {
			classes = append(classes, st.Class)
		}
	}
	v.Classes = strings.Join(classes, ", ")
	return v
}
func HTML(w io.Writer, r report.Report) error {
	v := htmlView{Report: r, Disclaimer: disclaimer, Completeness: completeness(r)}
	counts := map[string]int{}
	for _, c := range r.Claims {
		counts[c.Verdict]++
	}
	for _, verdict := range verdictOrder {
		v.Verdicts = append(v.Verdicts, verdictCount{Label: labels[verdict], Verdict: verdict, Count: counts[verdict]})
	}
	for _, c := range r.Commands {
		row := commandRow(c, r)
		v.Rows = append(v.Rows, row)
		if row.Failed {
			v.Failed++
		}
		if row.Unreliable {
			v.Unreliable++
		}
	}
	for _, f := range r.Findings {
		reason, ok := dismissal(r, f.ID)
		if ok {
			v.DismissedCount++
		}
		v.FindingRows = append(v.FindingRows, findingView{Finding: f, Reason: reason, Dismissed: ok, Copy: fmt.Sprintf("assaylight dismiss %s --reason \"…\"", f.ID)})
	}
	for _, g := range groupOrder {
		group := groupView{Label: groupLabel(g)}
		for _, file := range r.Groups[g] {
			fv := fileView{Path: file.Path, Status: file.Status, Additions: file.Additions, Deletions: file.Deletions, After: file.ChangedAfterSession}
			for _, f := range v.FindingRows {
				if f.Path == file.Path {
					fv.Findings = append(fv.Findings, f)
				}
			}
			group.Files = append(group.Files, fv)
			v.FileCount++
		}
		if len(group.Files) > 0 {
			v.Files = append(v.Files, group)
		}
	}
	return htmlTemplate.Execute(w, v)
}
