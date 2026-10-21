// Package findings describes review points in a repository comparison.
package findings

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/vincentsch/saidran/internal/gitstate"
)

type Finding struct {
	Context     string  `json:"-"`
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Path        string  `json:"path"`
	Line        int     `json:"line"`
	Explanation string  `json:"explanation"`
	Old         *string `json:"old,omitempty"`
	New         *string `json:"new,omitempty"`
	// Values distinguishes a missing command side from findings without command values.
	Values bool `json:"-"`
}

// UnmarshalJSON preserves explicit absent command sides across the redacted model.
func (f *Finding) UnmarshalJSON(b []byte) error {
	type plain Finding
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	*f = Finding(v)
	_, old := fields["old"]
	_, new := fields["new"]
	f.Values = old || new
	return nil
}

// MarshalJSON keeps absent test entries explicit while other findings omit values.
func (f Finding) MarshalJSON() ([]byte, error) {
	type plain Finding
	if !f.Values {
		return json.Marshal(plain(f))
	}
	return json.Marshal(struct {
		plain
		Old *string `json:"old"`
		New *string `json:"new"`
	}{plain(f), f.Old, f.New})
}

var skipCalls = map[string]*regexp.Regexp{
	".go":  regexp.MustCompile(`\bt\.(?:Skip|Skipf|SkipNow)\s*\(`),
	".py":  regexp.MustCompile(`(?:@pytest\.mark\.(?:skip|xfail)\b|@unittest\.skip\b|\bpytest\.skip\s*\()`),
	".js":  regexp.MustCompile(`\b(?:(?:it|test|describe)\.(?:skip|only)|xit|xdescribe)\s*\(`),
	".php": regexp.MustCompile(`(?:\bmarkTestSkipped\s*\(|->skip\s*\(|#\[Skip\])`),
}
var disabled = regexp.MustCompile(`(?:\beslint-disable\b|@ts-ignore\b|@ts-nocheck\b|#\s*type:\s*ignore\b|#\s*noqa\b|//\s*nolint\b|@phpstan-ignore\b|@psalm-suppress\b)`)
var ciFlag = regexp.MustCompile(`\b(?:continue-on-error|allow_failure):\s*true\b`)
var ignored = regexp.MustCompile(`\|\|\s*true\b`)

func Detect(files []gitstate.File) []Finding {
	result := []Finding{}
	counts := map[string]int{}
	add := func(f gitstate.File, code string, line int, hunk, explanation string, old, newValue *string, values bool) {
		key := code + "\x00" + f.Path + "\x00" + hunk
		counts[key]++
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d", key, counts[key])))
		result = append(result, Finding{Context: hunk, ID: fmt.Sprintf("F-%x", sum[:6]), Code: code, Path: f.Path, Line: line, Explanation: explanation, Old: old, New: newValue, Values: values})
	}
	for _, f := range files {
		if f.Status == "D" && f.Group == "tests" {
			var h string
			for _, v := range f.Hunks {
				h += v.Normalized()
			}
			add(f, "test_deleted", 1, h, "A test file was deleted.", nil, nil, false)
		}
		ext := path.Ext(f.Path)
		if ext == ".ts" || ext == ".tsx" || ext == ".jsx" || ext == ".mjs" || ext == ".cjs" {
			ext = ".js"
		}
		codeLines := strings.Split(codeOnly(string(f.New), ext), "\n")
		suppressionLines := strings.Split(lexical(string(f.New), ext, false), "\n")
		ciRuns := runLines(string(f.New))
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if l.Kind != '+' {
					continue
				}
				if f.Group == "tests" && skipCalls[ext] != nil && l.New > 0 && l.New <= len(codeLines) {
					for range skipCalls[ext].FindAllStringIndex(codeLines[l.New-1], -1) {
						add(f, "test_skipped", l.New, h.Normalized(), "A skip or focus marker was added to a test.", nil, nil, false)
					}
				}
				if l.New > 0 && l.New <= len(suppressionLines) {
					for range disabled.FindAllStringIndex(suppressionLines[l.New-1], -1) {
						add(f, "check_disabled", l.New, h.Normalized(), "A check suppression was added.", nil, nil, false)
					}
				}
				if f.Group == "ci" && !strings.HasPrefix(strings.TrimSpace(l.Text), "#") {
					for range ciFlag.FindAllStringIndex(l.Text, -1) {
						add(f, "ci_ignores_failure", l.New, h.Normalized(), "A CI check was configured to allow failure.", nil, nil, false)
					}
					if ciRuns[l.New] {
						for range ignored.FindAllStringIndex(l.Text, -1) {
							add(f, "ci_ignores_failure", l.New, h.Normalized(), "A CI command was changed to ignore failure.", nil, nil, false)
						}
					}
				}
			}
		}
		name := path.Base(f.Path)
		if name == "package.json" || name == "composer.json" || name == "Makefile" {
			old, os, oe := testCommand(name, f.Old)
			newValue, ns, ne := testCommand(name, f.New)
			if equal(old, newValue) {
				continue
			}
			var normalized string
			line := ns
			if line == 0 {
				line = os
			}
			if line == 0 {
				line = 1
			}
			for _, h := range f.Hunks {
				hit := false
				for _, l := range h.Lines {
					if l.Kind == '+' && l.New >= ns && l.New <= ne || l.Kind == '-' && l.Old >= os && l.Old <= oe {
						hit = true
					}
				}
				if hit {
					normalized += h.Normalized()
				}
			}
			if normalized == "" {
				for _, h := range f.Hunks {
					normalized += h.Normalized()
				}
			}
			add(f, "test_command_changed", line, normalized, "The test command changed.", old, newValue, true)
		}
	}
	return result
}
func equal(a, b *string) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

// codeOnly masks comments and quoted text but preserves columns and line numbers.
// It is deliberately lexical: no source code or test commands are executed.
func codeOnly(s, ext string) string { return lexical(s, ext, true) }

// lexical can retain comments for suppression detection while masking literal
// examples inside strings. Quotes inside comments never start strings.
func lexical(s, ext string, maskComments bool) string {
	b := []byte(s)
	block := false
	var quote byte
	triple := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c == '\n' {
			if quote != 0 && !triple && ext != ".go" && quote != '`' {
				quote = 0
			}
			continue
		}
		if block {
			if c == '*' && i+1 < len(b) && b[i+1] == '/' {
				if maskComments {
					b[i] = ' '
					b[i+1] = ' '
				}
				i++
				block = false
			} else if maskComments {
				b[i] = ' '
			}
			continue
		}
		if quote != 0 {
			if c == '\\' && i+1 < len(b) {
				b[i] = ' '
				i++
				if b[i] != '\n' {
					b[i] = ' '
				}
				continue
			}
			if c == quote {
				if !triple {
					quote = 0
				} else if i+2 < len(b) && b[i+1] == quote && b[i+2] == quote {
					b[i+1] = ' '
					b[i+2] = ' '
					i += 2
					triple = false
					quote = 0
				}
			}
			b[i] = ' '
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote = c
			if ext == ".py" && i+2 < len(b) && b[i+1] == c && b[i+2] == c {
				triple = true
				b[i+1] = ' '
				b[i+2] = ' '
				b[i] = ' '
				i += 2
			} else {
				b[i] = ' '
			}
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '*' {
			block = true
			if maskComments {
				b[i] = ' '
				b[i+1] = ' '
			}
			i++
			continue
		}
		comment := c == '/' && i+1 < len(b) && b[i+1] == '/' || c == '#' && (ext == ".py" || ext == ".php") && !(ext == ".php" && i+1 < len(b) && b[i+1] == '[')
		if comment {
			for i < len(b) && b[i] != '\n' {
				if maskComments {
					b[i] = ' '
				}
				i++
			}
			i--
		}
	}
	return string(b)
}
func runLines(s string) map[int]bool {
	out := map[int]bool{}
	indent := -1
	for i, l := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(l)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if indent >= 0 && n <= indent {
			indent = -1
		}
		key := strings.TrimSpace(strings.TrimPrefix(trim, "- "))
		if strings.HasPrefix(key, "run:") || strings.HasPrefix(key, "script:") || strings.HasPrefix(key, "command:") {
			indent = n
			out[i+1] = true
		} else if indent >= 0 {
			out[i+1] = true
		}
	}
	return out
}
func testCommand(name string, b []byte) (*string, int, int) {
	lines := strings.Split(string(b), "\n")
	if name == "Makefile" {
		for i, l := range lines {
			if !strings.HasPrefix(l, "test:") {
				continue
			}
			body := []string{strings.TrimSpace(strings.TrimPrefix(l, "test:"))}
			end := i + 1
			for end < len(lines) {
				v := lines[end]
				if v != "" && !strings.HasPrefix(v, "\t") && !strings.HasPrefix(v, "#") {
					break
				}
				body = append(body, strings.TrimRight(v, " \t\r"))
				end++
			}
			v := strings.TrimSpace(strings.Join(body, "\n"))
			return &v, i + 1, end
		}
		return nil, 0, 0
	}
	rawScripts, scriptsStart, ok := objectField(b, "scripts")
	if !ok {
		return nil, 0, 0
	}
	raw, start, ok := objectField(rawScripts, "test")
	if !ok {
		return nil, 0, 0
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		var value any
		if json.Unmarshal(raw, &value) != nil {
			return nil, 0, 0
		}
		compact, _ := json.Marshal(value)
		text = string(compact)
	}
	offset := scriptsStart + start
	first := bytes.Count(b[:offset], []byte("\n")) + 1
	last := first + bytes.Count(raw, []byte("\n"))
	return &text, first, last
}

// objectField retains the exact source span of a JSON property without relying
// on indentation or accidentally matching another object's test property.
func objectField(b []byte, name string) (json.RawMessage, int, bool) {
	decoder := json.NewDecoder(bytes.NewReader(b))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, 0, false
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, 0, false
		}
		before := int(decoder.InputOffset())
		var raw json.RawMessage
		if decoder.Decode(&raw) != nil {
			return nil, 0, false
		}
		if key == name {
			start := before + bytes.Index(b[before:int(decoder.InputOffset())], raw)
			return raw, start, true
		}
	}
	return nil, 0, false
}
