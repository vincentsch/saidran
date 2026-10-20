// Package commands analyzes recorded commands without executing shell source or scripts.
package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vincentsch/gaugestone/internal/pathutil"
	"github.com/vincentsch/gaugestone/internal/redact"
	"github.com/vincentsch/gaugestone/internal/session"
	"mvdan.cc/sh/v3/syntax"
)

type TestSummary struct {
	// Outcome is independent of counts: Go reports package results, not test counts.
	Outcome  string `json:"outcome"`
	Passed   *int   `json:"passed"`
	Failed   *int   `json:"failed"`
	Skipped  *int   `json:"skipped"`
	Packages *int   `json:"packages"`
	Filtered bool   `json:"filtered"`
	NoTests  bool   `json:"no_tests"`
	ListOnly bool   `json:"list_only"`
}

type Statement struct {
	Line   string   `json:"line"`
	Cwd    string   `json:"cwd"`
	Class  string   `json:"class"`
	Runner string   `json:"runner"`
	Argv   []string `json:"argv"`
	// Scope is whole, targeted, or unknown. Targets retain the runner's selectors.
	Scope        string   `json:"scope"`
	Targets      []string `json:"targets"`
	ExitCode     *int     `json:"exit_code"`
	ExitReliable bool     `json:"exit_reliable"`
	ExitNote     string   `json:"exit_note"`
	// Execution is possible, confirmed, or skipped. Possible edits remain evidence.
	Execution         string         `json:"execution"`
	Tests             *TestSummary   `json:"tests"`
	CurrentDefinition string         `json:"current_definition"`
	MayEditFiles      bool           `json:"may_edit_files"`
	Edits             []session.Edit `json:"edits"`
}

type Command struct {
	// Original retains the recording, including its whole-line exit, unmodified.
	Original               *session.Command `json:"original"`
	Statements             []Statement      `json:"statements"`
	CheckClasses           []string         `json:"check_classes"`
	Output                 string           `json:"output"`
	OutputTruncatedByAgent bool             `json:"output_truncated_by_agent"`
}

func (c Command) HasChecks() bool { return len(c.CheckClasses) > 0 }

// Entry orders statement and edit evidence. Seq identifies the original event;
// Statement is its zero-based statement index, or -1 for a native edit event.
type Entry struct {
	Time      time.Time      `json:"time"`
	Seq       int            `json:"seq"`
	Statement int            `json:"statement"`
	Source    session.Source `json:"source"`
	Command   *Statement     `json:"command,omitempty"`
	Edit      *session.Edit  `json:"edit,omitempty"`
}

type Ledger struct {
	Commands []Command `json:"commands"`
	Entries  []Entry   `json:"entries"`
}

// Build keeps the session's native events intact and inserts shell edits before
// the following statement. Callers use Entries for within-command ordering and
// the original command's Start/End for timing across concurrent events.
func Build(s *session.Session) Ledger {
	var ledger Ledger
	for _, e := range s.Events {
		if e.Edit != nil {
			ledger.Entries = append(ledger.Entries, Entry{Time: e.Time, Seq: e.Seq, Statement: -1, Source: e.Source, Edit: &session.Edit{Path: pathutil.Resolve(s.Cwd, e.Edit.Path), Op: e.Edit.Op}})
		}
		if e.Command == nil {
			continue
		}
		recorded := *e.Command
		recorded.Cwd = pathutil.Resolve(s.Cwd, recorded.Cwd)
		c := Analyze(&recorded)
		c.Original = e.Command
		ledger.Commands = append(ledger.Commands, c)
		for i := range c.Statements {
			st := &c.Statements[i]
			ledger.Entries = append(ledger.Entries, Entry{Time: e.Time, Seq: e.Seq, Statement: i, Source: e.Source, Command: st})
			for j := range st.Edits {
				ledger.Entries = append(ledger.Entries, Entry{Time: e.Time, Seq: e.Seq, Statement: i, Source: e.Source, Edit: &st.Edits[j]})
			}
		}
	}
	return ledger
}

type node struct {
	stmt        *syntax.Stmt
	index       int
	op          syntax.BinCmdOperator
	left, right *node
}

func printed(n syntax.Node) string {
	var b bytes.Buffer
	_ = syntax.NewPrinter().Print(&b, n)
	return b.String()
}

// literal decodes quoting without evaluating variables, substitutions or globs.
func literal(w *syntax.Word) (string, bool) {
	var b strings.Builder
	var parts func([]syntax.WordPart, bool) bool
	parts = func(ps []syntax.WordPart, quoted bool) bool {
		for _, p := range ps {
			switch p := p.(type) {
			case *syntax.Lit:
				for i := 0; i < len(p.Value); i++ {
					ch := p.Value[i]
					if ch == '\\' && i+1 < len(p.Value) {
						i++
						b.WriteByte(p.Value[i])
						continue
					}
					if !quoted && strings.ContainsRune("*?[~", rune(ch)) {
						return false
					}
					b.WriteByte(ch)
				}
			case *syntax.SglQuoted:
				b.WriteString(p.Value)
			case *syntax.DblQuoted:
				if !parts(p.Parts, true) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	ok := parts(w.Parts, false)
	return b.String(), ok
}

func Analyze(c *session.Command) Command {
	result := Command{Original: c}
	home, _ := os.UserHomeDir()
	r := redact.Redactor{Home: home}
	raw := c.RawOutput
	if raw == "" {
		raw = c.Output
	}
	result.Output = r.Tail(raw)
	result.OutputTruncatedByAgent = c.OutputTruncatedByAgent || outputCut.MatchString(raw)
	var roots []*node
	var flatten func(*syntax.Stmt) *node
	flatten = func(s *syntax.Stmt) *node {
		n := &node{stmt: s, index: -1}
		if b, ok := s.Cmd.(*syntax.BinaryCmd); ok {
			n.op = b.Op
			n.left = flatten(b.X)
			n.right = flatten(b.Y)
			return n
		}
		st := Statement{Line: printed(s), Cwd: c.Cwd, Class: "other", Scope: "unknown", Execution: "possible"}
		if call, ok := s.Cmd.(*syntax.CallExpr); ok {
			for _, w := range call.Args {
				v, ok := literal(w)
				if !ok {
					v = printed(w)
				}
				st.Argv = append(st.Argv, v)
			}
		}
		n.index = len(result.Statements)
		result.Statements = append(result.Statements, st)
		return n
	}
	if c.Shell {
		lang := syntax.LangBash
		if len(c.Argv) > 0 && filepath.Base(c.Argv[0]) == "zsh" {
			lang = syntax.LangZsh
		}
		file, err := syntax.NewParser(syntax.Variant(lang)).Parse(strings.NewReader(c.Line), "")
		if err != nil || len(file.Stmts) == 0 {
			result.Statements = []Statement{{Line: c.Line, Cwd: c.Cwd, Class: "other", Scope: "unknown", Execution: "possible", ExitNote: "could not read this command"}}
			return result
		}
		for _, s := range file.Stmts {
			roots = append(roots, flatten(s))
		}
	} else {
		result.Statements = []Statement{{Line: c.Line, Cwd: c.Cwd, Argv: append([]string(nil), c.Argv...), Scope: "unknown", Execution: "possible"}}
		roots = []*node{{index: 0}}
	}
	tests := 0
	runnerUses := map[string]int{}
	for i := range result.Statements {
		st := &result.Statements[i]
		classify(st)
		if st.Class == "test" {
			tests++
			runnerUses[st.Runner]++
		}
		if st.Class == "test" || st.Class == "build" || st.Class == "lint" || st.Class == "typecheck" {
			found := false
			for _, v := range result.CheckClasses {
				if v == st.Class {
					found = true
				}
			}
			if !found {
				result.CheckClasses = append(result.CheckClasses, st.Class)
			}
		}
		if st.Runner == "head" || st.Runner == "tail" {
			result.OutputTruncatedByAgent = true
		}
	}
	// One run owns its output. Distinct runners need identifiable banner
	// segments; repeated runs of a runner remain ambiguous.
	for i := range result.Statements {
		st := &result.Statements[i]
		if st.Class == "test" && supported(st.Runner) {
			flags := invocation(st)
			if tests == 1 {
				st.Tests = parseOutput(st.Runner, raw, flags)
			} else if segment, ok := bannerOutput(st.Runner, raw); ok && runnerUses[st.Runner] == 1 {
				st.Tests = parseOutput(st.Runner, segment, flags)
			} else {
				st.Tests = flags
			}
			st.Scope = "whole"
			if st.Tests.Filtered {
				st.Scope = "targeted"
			}
		}
	}
	reliable := c.ExitReliable && c.ExitCode != nil && (c.Status == "" || c.Status == "completed" || c.Status == "failed")
	var exit *int
	if reliable {
		exit = c.ExitCode
	}
	errexit := false
	stopped := false
	mayStop := false
	for i, n := range roots {
		note := "a later statement determines the line exit"
		var code *int
		if i == len(roots)-1 && !errexit && !mayStop {
			code = exit
			note = "no completed exit status"
		}
		if mayStop {
			note = "an earlier conditional exit may have stopped the shell"
		}
		if errexit {
			note = "set -e can stop the line before this statement"
		}
		attribute(n, &result, code, note, stopped)
		definite, possible := shellExit(n, &result)
		stopped = stopped || definite
		mayStop = mayStop || possible
		if n.index >= 0 && !stopped {
			st := result.Statements[n.index]
			if errexit {
				if k := known(n, &result); k != nil && *k != 0 {
					stopped = true
				}
			}
			if value, ok := setsErrexit(st.Argv); ok {
				errexit = value
			}
		}
	}
	cwd := pathutil.Resolve("", c.Cwd)
	for _, n := range roots {
		resolve(n, &result, &cwd)
	}
	for _, n := range roots {
		guardHeadSummaries(n, &result, raw)
	}
	for i := range result.Statements {
		st := &result.Statements[i]
		if st.Execution == "skipped" {
			st.Tests = nil
		}
		st.CurrentDefinition = r.Text(st.CurrentDefinition)
	}
	return result
}

func intp(v int) *int { return &v }

func known(n *node, c *Command) *int {
	if n.stmt != nil && (n.stmt.Negated || n.stmt.Background || n.stmt.Coprocess) {
		return nil
	}
	if n.index >= 0 {
		st := &c.Statements[n.index]
		if st.ExitReliable {
			return st.ExitCode
		}
		if len(st.Argv) > 0 {
			switch st.Argv[0] {
			case "false":
				return intp(1)
			case "true", ":":
				return intp(0)
			}
		}
		if st.Tests != nil && !st.Tests.ListOnly {
			if st.Tests.Outcome == "failed" {
				return intp(1)
			}

		}
		return nil
	}
	left := known(n.left, c)
	if n.op == syntax.AndStmt && left != nil {
		if *left != 0 {
			return left
		}
		return known(n.right, c)
	}
	if n.op == syntax.OrStmt && left != nil {
		if *left == 0 {
			return left
		}
		return known(n.right, c)
	}
	return nil
}

func attribute(n *node, c *Command, exit *int, note string, skip bool) {
	if n.stmt != nil && (n.stmt.Background || n.stmt.Negated || n.stmt.Coprocess) {
		exit = nil
		note = "background or negated execution has no attributable exit"
	}
	if n.index >= 0 {
		st := &c.Statements[n.index]
		if skip {
			st.Execution = "skipped"
			st.ExitNote = "statement did not execute"
			return
		}
		if exit != nil {
			st.ExitCode = intp(*exit)
			st.ExitReliable = true
			st.Execution = "confirmed"
		} else {
			st.ExitNote = note
		}
		return
	}
	switch n.op {
	case syntax.AndStmt:
		if exit != nil && *exit == 0 {
			attribute(n.left, c, exit, "", skip)
			attribute(n.right, c, exit, "", skip)
		} else {
			chainNote := note
			if exit != nil || chainNote == "" {
				chainNote = "a failed or incomplete && chain does not identify each exit"
			}
			attribute(n.left, c, nil, chainNote, skip)
			k := known(n.left, c)
			if k != nil && *k == 0 {
				attribute(n.right, c, exit, note, skip)
			} else {
				attribute(n.right, c, nil, chainNote+"; an && predecessor may have stopped execution", skip || (k != nil && *k != 0))
			}
		}
	case syntax.OrStmt:
		attribute(n.left, c, nil, "an || fallback can mask failure", skip)
		k := known(n.left, c)
		attribute(n.right, c, nil, "an || fallback determines the line exit", skip || (k != nil && *k == 0))
	default:
		attribute(n.left, c, nil, "pipeline exit cannot be attributed to this statement", skip)
		attribute(n.right, c, nil, "pipeline exit cannot be attributed to this statement", skip)
	}
}

func resolve(n *node, c *Command, cwd *string) {
	if n.stmt != nil && n.stmt.Background {
		local := *cwd
		cwd = &local
	}
	if n.index < 0 {
		if n.op != syntax.AndStmt && n.op != syntax.OrStmt {
			left, right := *cwd, *cwd
			resolve(n.left, c, &left)
			resolve(n.right, c, &right)
			return
		}
		resolve(n.left, c, cwd)
		resolve(n.right, c, cwd)
		return
	}
	st := &c.Statements[n.index]
	st.Cwd = *cwd
	if st.Execution == "skipped" {
		return
	}
	scriptHint(st)
	edits(st, n.stmt)
	if n.stmt != nil && len(st.Argv) == 0 {
		st.MayEditFiles = st.MayEditFiles || compoundMayEdit(n.stmt)
	}
	if len(st.Argv) > 0 && st.Argv[0] == "cd" && (n.stmt == nil || !n.stmt.Background) {
		if len(st.Argv) == 2 && n.stmt != nil {
			call, _ := n.stmt.Cmd.(*syntax.CallExpr)
			if call != nil {
				v, ok := literal(call.Args[1])
				if ok && *cwd != "" {
					*cwd = absolute(*cwd, v)
					return
				}
			}
		}
		*cwd = ""
	}
}

func absolute(cwd, path string) string { return pathutil.Resolve(cwd, path) }

// Result lets attributable failure evidence take precedence over an unrelated
// successful line exit. Unknown counts never imply a successful test run.
func (s Statement) Result() string {
	if s.Execution == "skipped" {
		return "unknown"
	}
	if s.Tests != nil && s.Tests.Outcome == "failed" {
		return "failed"
	}
	if s.ExitReliable && s.ExitCode != nil {
		if *s.ExitCode == 0 {
			return "passed"
		}
		return "failed"
	}
	if s.Tests != nil && (s.Tests.Outcome == "passed" || s.Tests.Outcome == "failed") {
		return s.Tests.Outcome
	}
	return "unknown"
}

func setsErrexit(argv []string) (bool, bool) {
	if len(argv) == 0 || argv[0] != "set" {
		return false, false
	}
	value, found := false, false
	for i, arg := range argv[1:] {
		if arg == "--" {
			break
		}
		if (arg == "-o" || arg == "+o") && i+2 < len(argv) && argv[i+2] == "errexit" {
			value = arg == "-o"
			found = true
		}
		if len(arg) > 1 && (arg[0] == '-' || arg[0] == '+') && strings.Contains(arg[1:], "e") {
			value = arg[0] == '-'
			found = true
		}
	}
	return value, found
}

// A conditional exit can determine the line result before its last statement.
// Pipeline and background children run outside the parent shell.
func shellExit(n *node, c *Command) (definite, possible bool) {
	if n.stmt != nil && (n.stmt.Background || n.stmt.Coprocess) {
		return false, false
	}
	if n.index >= 0 {
		st := c.Statements[n.index]
		exits := st.Execution != "skipped" && len(st.Argv) > 0 && st.Argv[0] == "exit"
		return exits, exits
	}
	if n.op != syntax.AndStmt && n.op != syntax.OrStmt {
		return false, false
	}
	ld, lp := shellExit(n.left, c)
	rd, rp := shellExit(n.right, c)
	k := known(n.left, c)
	runsRight := k != nil && ((n.op == syntax.AndStmt && *k == 0) || (n.op == syntax.OrStmt && *k != 0))
	return ld || (runsRight && rd), lp || rp
}

// Compound control flow stays opaque. Recognized editors inside it still
// prevent a later consumer from assuming that no files changed.
func compoundMayEdit(root *syntax.Stmt) bool {
	mayEdit := false
	syntax.Walk(root, func(n syntax.Node) bool {
		if _, ok := n.(*syntax.FuncDecl); ok {
			return false
		}
		st, ok := n.(*syntax.Stmt)
		if !ok || st == root {
			return true
		}
		call, _ := st.Cmd.(*syntax.CallExpr)
		candidate := Statement{} // cwd and expanded targets are unknown in this control flow.
		var words []*syntax.Word
		if call != nil {
			words = call.Args
		}
		for _, word := range words {
			v, ok := literal(word)
			if !ok {
				v = printed(word)
			}
			candidate.Argv = append(candidate.Argv, v)
		}
		edits(&candidate, st)
		if candidate.MayEditFiles || len(candidate.Edits) > 0 {
			mayEdit = true
		}
		return true
	})
	return mayEdit
}
