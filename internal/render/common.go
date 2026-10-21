package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/vincentsch/saidran/internal/commands"
	"github.com/vincentsch/saidran/internal/report"
)

const disclaimer = "This report summarises the agent's session log and your git state. Session logs are local files and can be edited. It is not a certificate."

var groupOrder = []string{"source", "tests", "config", "ci", "lockfiles", "docs", "other"}
var verdictOrder = []string{"verified", "stale", "contradicted", "partial", "no_matching_run", "cant_tell"}

func commandResult(c commands.Command) (failed, unreliable bool) {
	if c.Original != nil {
		o := c.Original
		failed = o.ExitReliable && o.ExitCode != nil && *o.ExitCode != 0
	}
	for _, st := range c.Statements {
		if st.Execution == "skipped" {
			continue
		}
		failed = failed || st.Result() == "failed"
		if isCheck(st.Class) && st.Result() == "unknown" {
			unreliable = true
		}
	}
	return
}
func isCheck(class string) bool {
	return class == "test" || class == "build" || class == "lint" || class == "typecheck"
}
func hasCheck(c commands.Command) bool {
	for _, st := range c.Statements {
		if isCheck(st.Class) {
			return true
		}
	}
	return false
}
func clock(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Local().Format("15:04:05")
}
func stamp(t time.Time) string { return t.Local().Format("2006-01-02 15:04:05 MST") }
func verdictTotals(r report.Report) string {
	counts := map[string]int{}
	for _, c := range r.Claims {
		counts[c.Verdict]++
	}
	var totals []string
	for _, v := range verdictOrder {
		totals = append(totals, fmt.Sprintf("%d %s", counts[v], labels[v]))
	}
	return strings.Join(totals, " · ")
}
func completeness(r report.Report) string {
	return fmt.Sprintf("Completeness: %d unsupported entries, %d missing exit codes.", r.Completeness.Unsupported, r.Completeness.MissingExits)
}
func dismissal(r report.Report, id string) (string, bool) {
	for _, d := range r.Dismissals {
		if d.ID == id {
			return d.Reason, true
		}
	}
	return "", false
}
