package commands

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vincentsch/gaugestone/internal/session"
	"mvdan.cc/sh/v3/syntax"
)

var interpreterWrite = regexp.MustCompile(`(?i)(\.write(?:_text|_bytes|File(?:Sync)?)?\s*\(|(?:open|File\.open)\s*\([^\n]*(?:["'](?:w|a|x|r\+)[bt+]*["'])|(?:unlink|rename|remove|mkdir|rmdir|shutil\.copy)\s*\()`)

func edits(s *Statement, stmt *syntax.Stmt) {
	a := executable(s.Argv)
	if len(a) == 0 && stmt == nil {
		return
	}
	add := func(path, op string, ok bool) {
		if path == "/dev/null" {
			return
		}
		if !ok || path == "" || absolute(s.Cwd, path) == "" {
			s.MayEditFiles = true
			return
		}
		s.Edits = append(s.Edits, session.Edit{Path: absolute(s.Cwd, path), Op: op})
	}
	// Redirection duplication (2>&1) is not a file write.
	if stmt != nil {
		for _, rd := range stmt.Redirs {
			if rd.Op == syntax.RdrOut || rd.Op == syntax.AppOut || rd.Op == syntax.ClbOut || rd.Op == syntax.RdrAll || rd.Op == syntax.AppAll {
				path, ok := literal(rd.Word)
				add(path, "write", ok)
			}
		}
	}
	if len(a) == 0 {
		return
	}
	// Recover static target words from the AST; direct argv is already literal.
	static := func(arg string) bool {
		if stmt == nil {
			return true
		}
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok {
			return false
		}
		for _, w := range call.Args {
			v, ok := literal(w)
			if (ok && v == arg) || printed(w) == arg {
				return ok
			}
		}
		return false
	}
	args := a[1:]
	name := filepath.Base(a[0])
	var targets []string
	op := "edit"
	recognized := false
	skipNext := false
	switch name {
	case "python", "python3", "node", "ruby", "perl":
		script := strings.Join(args, " ")
		if stmt != nil {
			for _, rd := range stmt.Redirs {
				if rd.Hdoc != nil {
					script += "\n" + printed(rd.Hdoc)
				}
			}
		}
		s.MayEditFiles = s.MayEditFiles || interpreterWrite.MatchString(script)
		return
	case "gofmt":
		recognized = has(args, "-w")
	case "prettier":
		recognized = has(args, "--write")
	case "black":
		recognized = !has(args, "--check") && !has(args, "--diff")
	case "sed":
		for _, arg := range args {
			if arg == "-i" || strings.HasPrefix(arg, "-i") || arg == "--in-place" || strings.HasPrefix(arg, "--in-place=") {
				recognized = true
			}
		}
	case "mv", "rm":
		recognized = true
		op = "delete"
	case "git":
		if len(args) > 0 && args[0] == "checkout" && has(args, "--") {
			recognized = true
			for i, v := range args {
				if v == "--" {
					targets = args[i+1:]
					break
				}
			}
		}
		if len(args) > 0 && args[0] == "restore" {
			recognized = true
			args = args[1:]
		}
	}
	if !recognized {
		return
	}
	if targets == nil {
		sedScript := false
		for i, arg := range args {
			if skipNext {
				skipNext = false
				continue
			}
			if arg == "--" {
				continue
			}
			if strings.HasPrefix(arg, "-") {
				if name == "sed" && arg == "-i" && i+1 < len(args) && args[i+1] == "" {
					skipNext = true
				}
				if name == "sed" && (arg == "-e" || arg == "--expression" || arg == "-f" || arg == "--file") {
					sedScript = true
					skipNext = true
				}
				if (name == "black" && (arg == "--line-length" || arg == "-l" || arg == "--target-version" || arg == "-t" || arg == "--exclude" || arg == "--include" || arg == "--config")) || (name == "prettier" && (arg == "--config" || arg == "--ignore-path" || arg == "--parser" || arg == "--plugin")) || (name == "git" && (arg == "--source" || arg == "-s")) {
					skipNext = true
				}
				continue
			}
			if name == "sed" && !sedScript {
				sedScript = true
				continue
			}
			targets = append(targets, arg)
		}
	}
	if len(targets) == 0 {
		s.MayEditFiles = true
		return
	}
	for i, path := range targets {
		if path == "-" {
			continue
		}
		targetOp := op
		if name == "mv" && i == len(targets)-1 {
			targetOp = "write"
		}
		add(path, targetOp, static(path) && !(name == "prettier" && strings.ContainsAny(path, "*?[{")))
	}
}
