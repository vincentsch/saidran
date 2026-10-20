package commands

import (
	"mvdan.cc/sh/v3/syntax"
	"regexp"
	"strconv"
	"strings"
)

var ansi = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
var outputCut = regexp.MustCompile(`(?i)(output truncated|truncated (?:output|by)|original token count|\[.{0,30}truncat.{0,30}\]|\.\.\. \d+ (?:lines|characters) (?:omitted|truncated))`)
var goFailure = regexp.MustCompile(`(?m)^FAIL(?:\s|$)`)
var pytestSummary = regexp.MustCompile(`^\s*(?:=+\s*)?\d+\s+(?:passed|failed|errors?|skipped|deselected|xfailed)\b`)
var goPackage = regexp.MustCompile(`(?m)^(ok|FAIL|\?)[ \t]+([^\s]+)([^\n]*)$`)
var countStatus = regexp.MustCompile(`\b(\d+)\s+(passed|failed|errors?|skipped|pending|todo|deselected|risky|incomplete|xfailed)\b`)
var phpOK = regexp.MustCompile(`OK(?:, but[^\n]*)?\s*\((\d+) tests?\b`)
var phpTotals = regexp.MustCompile(`\b(Tests|Failures|Errors|Skipped|Incomplete|Risky):\s*(\d+)`)

func parseOutput(runner, raw string, flags *TestSummary) *TestSummary {
	t := *flags
	raw = ansi.ReplaceAllString(raw, "")
	raw = strings.ReplaceAll(raw, "\r", "")
	if t.ListOnly {
		return &t
	}
	switch runner {
	case "go":
		packages := map[string]string{}
		for _, m := range goPackage.FindAllStringSubmatch(raw, -1) {
			packages[m[2]] = m[1] + m[3]
		}
		if len(packages) > 0 {
			t.Packages = intp(len(packages))
			t.Outcome = "passed"
			noTests := 0
			for _, status := range packages {
				if strings.HasPrefix(status, "FAIL") {
					t.Outcome = "failed"
				}
				if strings.Contains(status, "no test files") || strings.Contains(status, "no tests to run") {
					noTests++
				}
			}
			t.NoTests = noTests == len(packages)
		}
		if strings.Contains(raw, "--- FAIL:") || goFailure.MatchString(raw) {
			t.Outcome = "failed"
		}
		skips := strings.Count(raw, "--- SKIP:")
		if skips > 0 {
			t.Skipped = intp(skips)
		}
	case "pytest":
		for _, line := range strings.Split(raw, "\n") {
			if strings.Contains(line, "no tests ran") || strings.Contains(line, "collected 0 items") {
				t.NoTests = true
			}
			if !pytestSummary.MatchString(line) || !strings.Contains(line, " in ") {
				continue
			}
			if countStatus.MatchString(line) {
				counts(&t, line)
			}
		}
	case "jest", "vitest", "pest":
		for _, line := range strings.Split(raw, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Tests:") || strings.HasPrefix(trimmed, "Tests ") {
				counts(&t, trimmed)
			}
			if strings.Contains(line, "No tests found") || strings.Contains(line, "No test files found") {
				t.NoTests = true
			}
		}
	case "phpunit":
		if m := phpOK.FindStringSubmatch(raw); m != nil {
			n, _ := strconv.Atoi(m[1])
			t.Passed = intp(n)
			t.Outcome = "passed"
		}
		if strings.Contains(raw, "FAILURES!") || strings.Contains(raw, "ERRORS!") {
			t.Outcome = "failed"
		}
		if strings.Contains(raw, "No tests executed!") {
			t.NoTests = true
		}
		for _, line := range strings.Split(raw, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "Tests:") && !strings.Contains(line, "Tests: ") {
				continue
			}
			total, failed, skipped := 0, 0, 0
			found := false
			for _, m := range phpTotals.FindAllStringSubmatch(line, -1) {
				n, _ := strconv.Atoi(m[2])
				switch m[1] {
				case "Tests":
					total = n
					found = true
				case "Failures", "Errors":
					failed += n
				case "Skipped", "Incomplete", "Risky":
					skipped += n
				}
			}
			if found {
				t.Failed = intp(failed)
				t.Skipped = intp(skipped)
				if total >= failed+skipped {
					t.Passed = intp(total - failed - skipped)
				}
				if failed > 0 {
					t.Outcome = "failed"
				} else if total > skipped {
					t.Outcome = "passed"
				}
			}
		}
	}
	if t.Outcome == "" {
		t.Outcome = "unknown"
	}
	return &t
}
func counts(t *TestSummary, line string) {
	// A final summary supersedes an earlier watch-mode summary rather than
	// counting the same tests twice.
	t.Passed = nil
	t.Failed = nil
	t.Skipped = nil
	t.Outcome = "unknown"
	for _, m := range countStatus.FindAllStringSubmatch(line, -1) {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "passed":
			t.Passed = intp(n)
		case "failed", "error", "errors":
			if t.Failed == nil {
				t.Failed = intp(n)
			} else {
				*t.Failed += n
			}
		case "skipped", "pending", "todo", "incomplete", "risky", "xfailed":
			if t.Skipped == nil {
				t.Skipped = intp(n)
			} else {
				*t.Skipped += n
			}
		case "deselected":
			t.Filtered = true
		}
	}
	if t.Failed != nil && *t.Failed > 0 {
		t.Outcome = "failed"
	} else if t.Passed != nil && *t.Passed > 0 {
		t.Outcome = "passed"
	}
}

var runnerBanner = regexp.MustCompile(`(?m)^(?:platform [^\n]*pytest-[^\n]*|PHPUnit [^\n]*|[ \t]*(?:RUN|DEV)[ \t]+v\d[^\n]*)$`)

// bannerOutput isolates distinct runners' output. Repeated runners cannot be
// matched to individual invocations from a single aggregate tool result.
func bannerOutput(runner, raw string) (string, bool) {
	raw = ansi.ReplaceAllString(raw, "")
	matches := runnerBanner.FindAllStringIndex(raw, -1)
	start, end, count := 0, 0, 0
	for i, m := range matches {
		line := strings.TrimSpace(raw[m[0]:m[1]])
		name := "vitest"
		if strings.HasPrefix(line, "platform ") {
			name = "pytest"
		} else if strings.HasPrefix(line, "PHPUnit ") {
			name = "phpunit"
		}
		if name != runner {
			continue
		}
		count++
		start = m[0]
		end = len(raw)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
	}
	if count != 1 {
		return "", false
	}
	return raw[start:end], true
}

// Go prints per-package results rather than a final suite total. A head limit
// can hide later failures even when the visible packages passed.
func guardHeadSummaries(n *node, c *Command, raw string) {
	if n.index >= 0 {
		return
	}
	guardHeadSummaries(n.left, c, raw)
	guardHeadSummaries(n.right, c, raw)
	if n.op == syntax.AndStmt || n.op == syntax.OrStmt || !headMayCut(n.right, c, raw) {
		return
	}
	var guard func(*node)
	guard = func(part *node) {
		if part.index < 0 {
			guard(part.left)
			guard(part.right)
			return
		}
		st := &c.Statements[part.index]
		if st.Runner == "go" && st.Tests != nil && st.Tests.Outcome == "passed" {
			st.Tests.Outcome = "unknown"
			st.ExitNote += "; head may have cut later package results"
		}
	}
	guard(n.left)
}

func headMayCut(n *node, c *Command, raw string) bool {
	if n.index < 0 {
		return headMayCut(n.left, c, raw) || headMayCut(n.right, c, raw)
	}
	a := executable(c.Statements[n.index].Argv)
	if len(a) == 0 || a[0] != "head" {
		return false
	}
	limit := 10
	for i := 1; i < len(a); i++ {
		arg := a[i]
		if arg == "-n" || arg == "--lines" {
			if i+1 == len(a) {
				return true
			}
			i++
			arg = a[i]
		} else if strings.HasPrefix(arg, "--lines=") {
			arg = strings.TrimPrefix(arg, "--lines=")
		} else if strings.HasPrefix(arg, "-") {
			arg = arg[1:]
		} else {
			return true
		}
		value, err := strconv.Atoi(arg)
		if err != nil || value < 0 {
			return true
		}
		limit = value
	}
	lines := strings.Count(raw, "\n")
	if raw != "" && !strings.HasSuffix(raw, "\n") {
		lines++
	}
	return lines >= limit
}
