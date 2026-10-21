// Package claims checks final-response assertions against recorded evidence.
package claims

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/vincentsch/saidran/internal/commands"
	"github.com/vincentsch/saidran/internal/session"
)

type Run struct {
	Seq       int       `json:"seq"`
	Statement int       `json:"statement"`
	Command   string    `json:"command"`
	Cwd       string    `json:"cwd"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Result    string    `json:"result"`
	ExitNote  string    `json:"exit_note,omitempty"`
}
type Edit struct {
	Path string    `json:"path"`
	Time time.Time `json:"time"`
	Seq  int       `json:"seq"`
}
type Claim struct {
	Type        string    `json:"type"`
	Quote       string    `json:"quote"`
	Time        time.Time `json:"time"`
	Scope       string    `json:"scope"`
	Runner      string    `json:"runner,omitempty"`
	Targets     []string  `json:"targets"`
	Verdict     string    `json:"verdict"`
	Explanation string    `json:"explanation"`
	NextStep    string    `json:"next_step"`
	Runs        []Run     `json:"runs"`
	Edits       []Edit    `json:"edits"`
	invocation  *commands.Statement
}
type Gap struct {
	Quote string    `json:"quote"`
	Time  time.Time `json:"time"`
}

var underscoreEmphasis = regexp.MustCompile(`(^|[ \t])_([^_\n]+)_([ \t.,!?;:]|$)`)
var inline = regexp.MustCompile("`([^`\n]+)`")
var blocked = regexp.MustCompile(`(?i)\b(not all|no tests|keine tests|nicht alle|not|never|don't|doesn't|didn't|isn't|aren't|weren't|wasn't|cannot|can't|couldn't|should|would|will|might|may|could|if|when|once|unless|(?:after|before|until) (?:the )?tests pass|so they pass|nicht|nie|sollten?|würde\w*|werden|wird|wenn|falls|sobald)\b`)
var gapPhrase = regexp.MustCompile(`(?i)(could(n't| not) run|can(not|'t) run|unable to run|did(n't| not) (run|rerun)|not (run|rerun)|were not rerun|konnte\w* .{0,45}nicht|nicht .{0,25}(ausgeführt|erneut|laufen lassen))`)
var rules = []struct {
	kind   string
	phrase *regexp.Regexp
}{
	{"tests", regexp.MustCompile(`(?i)\b(tests? (are |were |have |have been |had )?(pass(es|ed|ing)?|green|successful)|test suite (pass(es|ed)?|is passing)|Test[A-Z][a-zA-Z0-9_]* passes in [a-zA-Z0-9_./-]+|tests? (sind |waren )?(grün|bestehen|bestanden|durchgelaufen)|tests? (laufen|liefen) durch|tests? (haben )?bestanden)\b`)},
	{"build", regexp.MustCompile(`(?i)\b(build (pass(es|ed)?|works|worked|succeed(s|ed)?|is successful)|builds( successfully)?|built successfully|compiles?( cleanly)?|compiled( cleanly)?|build (läuft|lief|ist durchgelaufen)|kompiliert(e)?)\b`)},
	{"lint", regexp.MustCompile(`(?i)\b(lint (pass(es|ed)?|is clean)|linter is clean|no lint errors|keine lint[- ]fehler)\b`)},
	{"typecheck", regexp.MustCompile(`(?i)\b(type ?checks? (pass(es|ed)?|is passing)|no type errors|(tsc|mypy|phpstan|pyright) pass(es|ed)?|keine typfehler)\b`)},
}

// List summaries share a passing assertion across the named checks.
const checkNoun = `(?:(?:[0-9]+(?:/[0-9]+)?|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve) )?(?:tests|race checks|race tests|vet|lint|lint checks|formatting|build|type checks|typechecks|fixtures|[a-z]+ checks|conformance(?: rules)?)`

var passingList = regexp.MustCompile(`(?i)\b(?:all |full )?` + checkNoun + `(?:\s*(?:,\s*(?:and\s+)?|and\s+)` + checkNoun + `)+\s+pass(?:ed)?\b`)
var listKinds = map[string]*regexp.Regexp{
	"tests":     regexp.MustCompile(`(?i)\btests\b`),
	"build":     regexp.MustCompile(`(?i)\bbuild\b`),
	"lint":      regexp.MustCompile(`(?i)\b(vet|lint)\b`),
	"typecheck": regexp.MustCompile(`(?i)\b(type checks|typechecks)\b`),
}

var toolName = regexp.MustCompile(`(?i)\b(pytest|vitest|jest|phpunit|pest|tsc|mypy|phpstan|pyright|eslint|ruff|golangci-lint)\b`)
var namedPackage = regexp.MustCompile(`(?i)\b(?:the |die )?([a-zA-Z0-9_./-]+) (?:tests|package|Paket)\b`)
var namedTest = regexp.MustCompile(`\bTest[A-Z][a-zA-Z0-9_]*\b`)
var inPackage = regexp.MustCompile(`(?i)\bin ([a-zA-Z0-9_./-]+)`)
var pathName = regexp.MustCompile(`(?:\./|[a-zA-Z0-9_-]+/)[a-zA-Z0-9_./*-]+|\b[a-zA-Z0-9_-]+\.(?:go|py|ts|js|php|rs)\b`)
var wholeWords = regexp.MustCompile(`(?i)\b(all|alle|whole suite|entire suite)\b`)
var passed = regexp.MustCompile(`(?i)^\s*(?:\*\*|__)?\s*pass(?:es|ed)?\b`)

// commandList joins inline commands that share one result, as in "`a`, `b` and `c` passed".
var commandList = regexp.MustCompile(`(?i)^\s*(?:,\s*(?:(?:and|und)\s+)?|(?:and|und|&)\s+)`)

// sentences keeps original Markdown and avoids splitting paths or command text.
func sentences(text string) []string {
	var out []string
	start, ticks := 0, false
	for i, r := range text {
		if r == '`' {
			ticks = !ticks
		}
		end := i + len(string(r))
		boundary := r == '\n' || (!ticks && strings.ContainsRune(".!?", r) && (end == len(text) || unicode.IsSpace(rune(text[end]))))
		if boundary {
			if s := strings.TrimSpace(text[start:end]); s != "" {
				out = append(out, s)
			}
			start = end
		}
	}
	if s := strings.TrimSpace(text[start:]); s != "" {
		out = append(out, s)
	}
	return out
}

// Split independent assertions without changing existing single-assertion quotes.
// Commas inside shared passing lists and punctuation inside commands stay intact.
// These recognize clause structure, including non-passing assertions. Claim
// admission still applies negation and modality to each original clause.
var conditionalContext = regexp.MustCompile(`(?i)\b(if|when|once|unless|wenn|falls|sobald|(?:after|before|until) (?:the )?tests pass|so they pass)\b`)

const resultPredicate = `(?:(?:are|were|is|was|do|does|did|not|don't|doesn't|didn't|should|would|will|might|may|could|sind|waren|ist|war|nicht|sollte[n]?|würde[n]?)\s+)*(?:pass(?:es|ed|ing)?|green|grün|bestehen|bestanden|läuft|lief)\b`

var checkPredicate = regexp.MustCompile(`(?i)\b(?:tests?|test suite|build|lint|linter|type ?checks?)\s+` + resultPredicate)
var quotedPredicate = regexp.MustCompile(`(?i)^\s*(?:\*\*|__)?\s*` + resultPredicate)
var clauseBoundary = regexp.MustCompile(`(?i)^(?:;\s*|,\s*(?:but|aber)\s+|\s+(?:and|und)\s+)`)

func assertionClauses(text string) []string {
	var out []string
	for _, sentence := range sentences(text) {
		start, ticks := 0, false
		for i := 0; i < len(sentence); i++ {
			if sentence[i] == '`' {
				ticks = !ticks
			}
			if ticks {
				continue
			}
			loc := clauseBoundary.FindStringIndex(sentence[i:])
			if loc == nil || loc[0] != 0 {
				continue
			}
			delimiter := strings.ToLower(strings.TrimSpace(sentence[i : i+loc[1]]))
			if (delimiter == "and" || delimiter == "und") && (conditionalContext.MatchString(sentence[start:i]) || !assertion(sentence[start:i]) || !assertion(sentence[i+loc[1]:])) {
				continue
			}
			if clause := strings.TrimSpace(sentence[start:i]); clause != "" {
				out = append(out, clause)
			}
			i += loc[1] - 1
			start = i + 1
		}
		if clause := strings.TrimSpace(sentence[start:]); clause != "" {
			out = append(out, clause)
		}
	}
	return out
}

// A conjunction separates assertions only when each side states its own
// result (positive, negated or conditional) or admitted gap. Shared passing
// lists keep their original quote.
func assertion(text string) bool {
	plain := strings.NewReplacer("`", "", "*", "", "’", "'").Replace(text)
	if gapPhrase.MatchString(plain) {
		return true
	}
	if checkPredicate.MatchString(plain) {
		return true
	}
	for _, rule := range rules {
		if rule.phrase.MatchString(plain) {
			return true
		}
	}
	for _, m := range inline.FindAllStringIndex(text, -1) {
		if quotedPredicate.MatchString(text[m[1]:]) {
			return true
		}
	}
	return false
}

func Extract(final string, at time.Time) ([]Claim, []Gap) {
	claims, gaps := []Claim{}, []Gap{}
	for _, quote := range assertionClauses(final) {
		plain := underscoreEmphasis.ReplaceAllString(strings.NewReplacer("`", "", "*", "", "__", "", "’", "'").Replace(quote), "$1$2$3")
		if gapPhrase.MatchString(plain) {
			gaps = append(gaps, Gap{quote, at})
		}
		if blocked.MatchString(plain) {
			continue
		}
		// Every asserted command keeps its own claim, so one passing command
		// cannot stand in for another named in the same sentence.
		inlines := inline.FindAllStringSubmatchIndex(quote, -1)
		shared := make([]bool, len(inlines))
		for i := len(inlines) - 1; i >= 0; i-- {
			rest := quote[inlines[i][1]:]
			if passed.MatchString(rest) {
				shared[i] = true
			} else if i+1 < len(inlines) {
				if sep := commandList.FindString(rest); sep != "" && inlines[i][1]+len(sep) == inlines[i+1][0] {
					shared[i] = shared[i+1]
				}
			}
		}
		asserted := map[string][]*commands.Statement{}
		seen := map[string]bool{}
		for i, m := range inlines {
			if !shared[i] {
				continue
			}
			line := quote[m[2]:m[3]]
			analysis := commands.Analyze(&session.Command{Line: line, Shell: true})
			if len(analysis.Statements) != 1 {
				continue
			}
			st := analysis.Statements[0]
			kind := st.Class
			if kind == "test" {
				kind = "tests"
			}
			if (kind == "tests" || kind == "build" || kind == "lint" || kind == "typecheck") && !seen[st.Line] {
				seen[st.Line] = true
				asserted[kind] = append(asserted[kind], &st)
			}
		}
		listed := map[string]bool{}
		for _, list := range passingList.FindAllString(plain, -1) {
			for kind, noun := range listKinds {
				if noun.MatchString(list) {
					listed[kind] = true
				}
			}
		}
		for _, rule := range rules {
			statements := asserted[rule.kind]
			if len(statements) == 0 && !listed[rule.kind] && !rule.phrase.MatchString(plain) {
				continue
			}
			if len(statements) == 0 || listed[rule.kind] {
				statements = []*commands.Statement{nil}
			}
			for _, st := range statements {
				claims = append(claims, newClaim(rule.kind, quote, plain, at, st, listed[rule.kind]))
			}
		}
	}
	return claims, gaps
}

// newClaim builds one claim; st is the asserted command, or nil for prose.
func newClaim(kind, quote, plain string, at time.Time, st *commands.Statement, listed bool) Claim {
	c := Claim{Type: kind, Quote: quote, Time: at, Scope: "whole", Targets: []string{}, Runs: []Run{}, Edits: []Edit{}}
	if tool := toolName.FindString(plain); tool != "" {
		c.Runner = strings.ToLower(tool)
		c.Scope = "targeted"
	}
	if st != nil {
		c.Runner, c.invocation = st.Runner, st
		c.Targets = append(c.Targets, st.Targets...)
		c.Scope = st.Scope
		if st.Class != "test" {
			c.Targets = invocationTargets(*st)
			c.Scope = "whole"
			for _, target := range c.Targets {
				if target != "." && target != "./..." {
					c.Scope = "targeted"
				}
			}
		}
	} else {
		if m := namedPackage.FindStringSubmatch(plain); len(m) > 1 {
			if !scopeWord(m[1]) {
				c.Targets = append(c.Targets, strings.TrimRight(m[1], ".,:;"))
			}
		}
		for _, path := range pathName.FindAllString(plain, -1) {
			if !strings.HasSuffix(path, "/...") {
				path = strings.TrimRight(path, ".,:;")
			}
			c.Targets = append(c.Targets, path)
		}
		if test := namedTest.FindString(plain); test != "" {
			c.Targets = append(c.Targets, test)
			if m := inPackage.FindStringSubmatch(plain); len(m) > 1 {
				c.Targets = append(c.Targets, strings.TrimRight(m[1], ".,:;"))
			}
		}
		if len(c.Targets) > 0 {
			c.Scope = "targeted"
		}
	}
	if listed {
		c.Runner = ""
		c.Targets = []string{}
		c.Scope = "whole"
		c.invocation = nil
	}
	if wholeWords.MatchString(plain) {
		c.Scope = "whole"
	}
	return c
}

func scopeWord(s string) bool {
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	switch strings.ToLower(s) {
	case "in", "for", "with", "these", "those", "our", "my", "all", "alle", "the", "die", "whole", "entire", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "hundred", "ein", "eine", "zwei", "drei", "vier", "fünf", "sechs", "sieben", "acht", "neun", "zehn":
		return true
	}
	return false
}
