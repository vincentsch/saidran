package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// executable removes launcher prefixes but never reads or executes script bodies.
func executable(argv []string) []string {
	a := append([]string(nil), argv...)
	for len(a) > 0 {
		name := filepath.Base(a[0])
		switch name {
		case "env":
			a = a[1:]
			for len(a) > 0 {
				if a[0] == "-u" || a[0] == "--unset" || a[0] == "-C" || a[0] == "--chdir" {
					if len(a) < 2 {
						return nil
					}
					a = a[2:]
					continue
				}
				if strings.Contains(a[0], "=") || strings.HasPrefix(a[0], "-") {
					a = a[1:]
					continue
				}
				break
			}
		case "npx", "bunx":
			a = a[1:]
			for len(a) > 0 && strings.HasPrefix(a[0], "-") {
				a = a[1:]
			}
		case "pnpm", "uv", "poetry":
			if len(a) > 1 && ((name == "pnpm" && a[1] == "exec") || (name != "pnpm" && a[1] == "run")) {
				a = a[2:]
				for len(a) > 0 && strings.HasPrefix(a[0], "-") {
					a = a[1:]
				}
			} else {
				a[0] = name
				return a
			}
		default:
			a[0] = name
			return a
		}
	}
	return a
}
func has(a []string, flag string) bool {
	for _, v := range a {
		if v == flag || strings.HasPrefix(v, flag+"=") {
			return true
		}
	}
	return false
}
func classify(s *Statement) {
	s.Class = "other"
	s.Scope = "unknown"
	a := executable(s.Argv)
	if len(a) == 0 {
		return
	}
	name := a[0]
	s.Runner = name
	sub := ""
	if len(a) > 1 {
		sub = a[1]
	}
	switch name {
	case "go":
		switch sub {
		case "test":
			s.Class = "test"
		case "build":
			s.Class = "build"
		case "vet":
			s.Class = "lint"
		case "get", "install", "mod":
			s.Class = "install"
		}
	case "pytest", "jest", "phpunit", "pest", "rspec":
		s.Class = "test"
	case "vitest":
		s.Class = "test"
	case "python", "python3":
		if len(a) > 2 && sub == "-m" && a[2] == "pytest" {
			s.Class = "test"
			s.Runner = "pytest"
		}
	case "php":
		if len(a) > 2 && sub == "artisan" && a[2] == "test" {
			s.Class = "test"
			s.Runner = "artisan"
		}
	case "cargo":
		if sub == "test" {
			s.Class = "test"
		} else if sub == "build" {
			s.Class = "build"
		} else if sub == "install" {
			s.Class = "install"
		}
	case "golangci-lint", "eslint", "flake8", "phpcs":
		s.Class = "lint"
	case "ruff":
		if sub == "check" {
			s.Class = "lint"
		} else if sub == "format" {
			s.Class = "format"
		}
	case "biome":
		if sub == "check" || sub == "lint" {
			s.Class = "lint"
		} else if sub == "format" {
			s.Class = "format"
		}
	case "pint":
		s.Class = "format"
		if has(a, "--test") {
			s.Class = "lint"
		}
	case "tsc":
		s.Class = "build"
		if has(a, "--noEmit") && !has(a, "--build") {
			s.Class = "typecheck"
		}
	case "mypy", "pyright", "phpstan", "psalm", "vue-tsc":
		s.Class = "typecheck"
	case "gofmt", "prettier", "black":
		s.Class = "format"
	case "git":
		s.Class = "git"
	case "pip", "pip3", "composer":
		if sub == "install" || sub == "update" || sub == "require" {
			s.Class = "install"
		}
	case "npm", "pnpm", "yarn", "bun":
		if sub == "install" || sub == "add" || sub == "ci" || sub == "update" {
			s.Class = "install"
			break
		}
		script := sub
		if sub == "run" || sub == "run-script" {
			if len(a) < 3 {
				break
			}
			script = a[2]
		}
		s.Class = scriptClass(script)
		s.Runner = name + "-script:" + script
	case "make":
		target := sub
		if target == "" {
			s.Class = "build"
		} else {
			s.Class = scriptClass(target)
		}
	}
}
func scriptClass(s string) string {
	switch {
	case strings.HasPrefix(s, "test"):
		return "test"
	case strings.HasPrefix(s, "build"):
		return "build"
	case strings.HasPrefix(s, "lint"):
		return "lint"
	case strings.HasPrefix(s, "typecheck"), s == "check":
		return "typecheck"
	case strings.HasPrefix(s, "format"):
		return "format"
	}
	return "other"
}
func supported(r string) bool {
	switch r {
	case "go", "pytest", "jest", "vitest", "phpunit", "pest":
		return true
	}
	return false
}

func invocation(s *Statement) *TestSummary {
	t := &TestSummary{Outcome: "unknown"}
	a := executable(s.Argv)
	if len(a) == 0 {
		return t
	}
	args := a[1:]
	if s.Runner == "go" && len(args) > 0 {
		args = args[1:]
	}
	if s.Runner == "pytest" && (a[0] == "python" || a[0] == "python3") && len(args) >= 2 {
		args = args[2:]
	}
	skipNext := false
	selectorNext := false
	packageSeen := false
	for _, arg := range args {
		if skipNext {
			if selectorNext {
				s.Targets = append(s.Targets, arg)
			}
			selectorNext = false
			skipNext = false
			continue
		}
		switch s.Runner {
		case "go":
			if arg == "-list" || strings.HasPrefix(arg, "-list=") {
				t.ListOnly = true
			}
			if arg == "-run" || strings.HasPrefix(arg, "-run=") || arg == "-bench" || strings.HasPrefix(arg, "-bench=") {
				t.Filtered = true
				selectorNext = arg == "-run" || arg == "-bench"
			}
			if arg == "-run" || arg == "-list" || arg == "-count" || arg == "-timeout" || arg == "-parallel" || arg == "-bench" || arg == "-benchtime" || arg == "-coverprofile" || arg == "-coverpkg" {
				skipNext = true
			}
		case "pytest":
			if arg == "--collect-only" || arg == "--co" {
				t.ListOnly = true
			}
			if arg == "-k" || arg == "-m" || strings.HasPrefix(arg, "-k=") || strings.HasPrefix(arg, "-m=") {
				t.Filtered = true
				if !strings.Contains(arg, "=") {
					skipNext = true
				}
			}
			if arg == "--rootdir" || arg == "-c" || arg == "--confcutdir" {
				skipNext = true
			}
		case "jest", "vitest":
			if arg == "--listTests" || (s.Runner == "vitest" && arg == "list") {
				t.ListOnly = true
			}
			if arg == "-t" || strings.HasPrefix(arg, "-t=") || strings.HasPrefix(arg, "--testNamePattern") {
				t.Filtered = true
				if !strings.Contains(arg, "=") {
					skipNext = true
				}
			}
			if arg == "--config" || arg == "--reporter" || arg == "--maxWorkers" || arg == "--pool" {
				skipNext = true
			}
		case "phpunit", "pest":
			if arg == "--filter" || strings.HasPrefix(arg, "--filter=") {
				t.Filtered = true
				if !strings.Contains(arg, "=") {
					skipNext = true
				}
			}
			if arg == "--list-tests" || arg == "--list-tests-xml" {
				t.ListOnly = true
			}
			if arg == "--configuration" || arg == "-c" || arg == "--testsuite" {
				skipNext = true
				if arg == "--testsuite" {
					t.Filtered = true
				}
			}
		}
		if t.Filtered && strings.HasPrefix(arg, "-") {
			if value, ok := strings.CutPrefix(arg, "-run="); ok {
				s.Targets = append(s.Targets, value)
			}
			if strings.Contains(arg, "=") && (strings.HasPrefix(arg, "--filter=") || strings.HasPrefix(arg, "--testNamePattern=") || strings.HasPrefix(arg, "-k=") || strings.HasPrefix(arg, "-t=")) {
				_, value, _ := strings.Cut(arg, "=")
				s.Targets = append(s.Targets, value)
			}
			if skipNext && (arg == "-k" || arg == "-m" || arg == "-t" || arg == "--testNamePattern" || arg == "--filter" || arg == "--testsuite") {
				selectorNext = true
			}
		}
		if strings.HasPrefix(arg, "-") || skipNext {
			continue
		}
		if s.Runner == "go" {
			packageSeen = true
		}
		if (s.Runner == "go" && arg == "./...") || ((s.Runner == "vitest" || s.Runner == "jest") && (arg == "run" || arg == "list")) {
			continue
		}
		t.Filtered = true
		s.Targets = append(s.Targets, arg)
	}
	if s.Runner == "go" && !packageSeen && !t.ListOnly {
		t.Filtered = true
		s.Targets = append(s.Targets, ".")
	}
	return t
}

func scriptHint(s *Statement) {
	name, script, ok := strings.Cut(s.Runner, "-script:")
	if !ok || (name != "npm" && name != "pnpm" && name != "yarn" && name != "bun") || s.Cwd == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(s.Cwd, "package.json"))
	if err != nil {
		return
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) == nil && pkg.Scripts[script] != "" {
		s.CurrentDefinition = "current definition: " + pkg.Scripts[script]
	}
}
