package redact

import (
	"regexp"
	"strings"
	"unicode"
)

var assignments = regexp.MustCompile(`(?i)\b([a-z0-9_]*(?:_KEY|_TOKEN|_SECRET)|[a-z0-9_]*PASSWORD)(\s*[=:]\s*)(\[redacted\]|"[^"\n]*"|'[^'\n]*'|[^\s;,]+)`)
var tokens = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{20,}|ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]+|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[A-Z0-9]{16}|[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,})`)
var longAssignment = regexp.MustCompile(`([=:]\s*["']?)[A-Za-z0-9_+/=-]{33,}`)

type Redactor struct{ Home string }

func (r Redactor) Text(s string) string {
	s = assignments.ReplaceAllString(s, "${1}${2}[redacted]")
	s = tokens.ReplaceAllString(s, "[redacted]")
	s = longAssignment.ReplaceAllString(s, "${1}[redacted]")
	if r.Home != "" && r.Home != "/" {
		s = strings.ReplaceAll(s, r.Home, "~")
	}
	return strings.Map(func(c rune) rune {
		if unicode.IsControl(c) && c != '\n' && c != '\t' {
			return -1
		}
		return c
	}, s)
}
func Limit(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
func (r Redactor) Prompt(s string) string { return Limit(r.Text(s), 200) }
func (r Redactor) Tail(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return r.Text(strings.Join(lines, "\n"))
}
