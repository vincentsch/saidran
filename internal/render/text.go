// Package render presents the shared report evidence.
package render

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vincentsch/gaugestone/internal/report"
	"github.com/vincentsch/gaugestone/internal/session"
	"github.com/vincentsch/rungrad/output"
)

var labels = map[string]string{"verified": "Verified", "stale": "Stale", "contradicted": "Contradicted", "partial": "Partial", "no_matching_run": "No matching run", "cant_tell": "Can't tell"}

// Text uses the caller's terminal width and rungrad mode. Input must be the
// redacted report value, just as for saved JSON.
func Text(w io.Writer, r report.Report, displayID string, width int, mode output.TerminalMode) {
	if width < 60 {
		width = 60
	}
	if width > 100 {
		width = 100
	}
	line := func(prefix, text string) { wrap(w, prefix, text, width) }
	agent := session.AgentLabel(r.Session.Agent)
	startLayout, endLayout := "2006-01-02 15:04", "15:04"
	if r.Session.Started.Truncate(time.Minute).Equal(r.Session.Ended.Truncate(time.Minute)) {
		startLayout, endLayout = "2006-01-02 15:04:05", "15:04:05"
	}
	line("Gaugestone  ", fmt.Sprintf("%s session %s  %s–%s  %s vs %s", agent, displayID, r.Session.Started.Local().Format(startLayout), r.Session.Ended.Local().Format(endLayout), r.Repo.Branch, r.Repo.Baseline.Ref))
	line("Baseline  ", shortCommit(r.Repo.Baseline.Commit)+" ("+r.Repo.Baseline.Ref+", "+r.Repo.Baseline.Reason+")")
	fmt.Fprintln(w)
	if len(r.Claims) == 0 {
		line("Claims  ", "none found in the agent's final message")
	} else {
		fmt.Fprintln(w, "Claims")
		for _, c := range r.Claims {
			label := labels[c.Verdict]
			prefix := fmt.Sprintf("  %-15s", label)
			if mode.ANSI && mode.Color {
				color := "90"
				switch c.Verdict {
				case "verified":
					color = "32"
				case "stale", "partial":
					color = "33"
				case "contradicted":
					color = "31"
				}
				prefix = "  \x1b[" + color + "m" + label + "\x1b[0m" + strings.Repeat(" ", 15-utf8.RuneCountInString(label))
			}
			wrapPrefix(w, prefix, "                 ", "\""+c.Quote+"\"", width)
			line("                 ", c.Explanation)
			for _, run := range c.Runs {
				if run.ExitNote != "" {
					line("                 ", "Exit note: "+run.ExitNote+".")
				}
			}
			line("                 ", c.NextStep)
		}
	}
	for _, gap := range r.AdmittedGaps {
		line("Gap  ", gap.Quote)
	}
	failed, unreliable := 0, 0
	for _, c := range r.Commands {
		bad, unknown := commandResult(c)
		if bad {
			failed++
		}
		if unknown {
			unreliable++
		}
	}
	fmt.Fprintln(w)
	line("Commands  ", fmt.Sprintf("%d run · %d failed · %d result unreliable", len(r.Commands), failed, unreliable))
	total := 0
	groups := []string{}
	for _, group := range []string{"source", "tests", "config", "ci", "lockfiles", "docs", "other"} {
		n := len(r.Groups[group])
		total += n
		if n > 0 {
			label := group
			if n == 1 {
				switch group {
				case "tests":
					label = "test"
				case "lockfiles":
					label = "lockfile"
				}
			}
			groups = append(groups, fmt.Sprintf("%d %s", n, label))
		}
	}
	fileLabel := "files"
	if total == 1 {
		fileLabel = "file"
	}
	changes := fmt.Sprintf("%d %s", total, fileLabel)
	if len(groups) > 0 {
		changes += " · " + strings.Join(groups, " · ")
	}
	line("Changes   ", changes)
	if len(r.Findings) == 0 {
		line("Findings  ", "0")
	}
	for i, f := range r.Findings {
		label := strings.ReplaceAll(f.Code, "_", " ")
		text := fmt.Sprintf("%s in %s:%d · %s", label, f.Path, f.Line, f.ID)
		if i == 0 {
			text = fmt.Sprintf("%d · %s", len(r.Findings), text)
		}
		line("Findings  ", text)
		line("          ", f.Explanation)
		if reason, ok := dismissal(r, f.ID); ok {
			line("          ", "Dismissed: "+reason)
		}
		if f.Values {
			side := func(v *string) string {
				if v == nil {
					return "(absent)"
				}
				return *v
			}
			line("          ", side(f.Old)+" → "+side(f.New))
		}
	}
	for _, group := range []string{"source", "tests", "config", "ci", "lockfiles", "docs", "other"} {
		for _, file := range r.Groups[group] {
			if file.ChangedAfterSession {
				line("Note  ", file.Path+": changed after the session ended")
			}
		}
	}
	for _, note := range r.Completeness.Notes {
		line("Note  ", note)
	}
	if r.Completeness.Unsupported > 0 || r.Completeness.MissingExits > 0 {
		line("Incomplete evidence  ", fmt.Sprintf("%d unsupported entries, %d missing exit codes.", r.Completeness.Unsupported, r.Completeness.MissingExits))
	}
}
func wrap(w io.Writer, prefix, text string, width int) {
	wrapPrefix(w, prefix, strings.Repeat(" ", utf8.RuneCountInString(prefix)), text, width)
}
func wrapPrefix(w io.Writer, prefix, continuation, text string, width int) {
	remaining := width - utf8.RuneCountInString(continuation)
	words := strings.Fields(text)
	if len(words) == 0 {
		fmt.Fprintln(w, prefix)
		return
	}
	current := ""
	flush := func() { fmt.Fprintln(w, prefix+current); prefix = continuation; current = "" }
	for _, word := range words {
		if current != "" && utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) > remaining {
			flush()
		}
		// Split long tokens so a path or quote cannot push the summary off screen.
		for utf8.RuneCountInString(word) > remaining {
			if current != "" {
				flush()
			}
			r := []rune(word)
			current = string(r[:remaining])
			flush()
			word = string(r[remaining:])
		}
		if current != "" {
			current += " "
		}
		current += word
	}
	if current != "" {
		flush()
	}
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
