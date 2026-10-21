package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vincentsch/saidran/internal/session"
)

func analyze(line, output string, exit int) Command {
	return Analyze(&session.Command{Line: line, Shell: true, Cwd: "/repo", RawOutput: output, ExitCode: intp(exit), ExitReliable: true, Status: "completed"})
}
func TestStatementExitAttribution(t *testing.T) {
	cases := []struct {
		line              string
		exit              int
		reliable, skipped []bool
	}{
		{"go test ./...", 0, []bool{true}, nil},
		{"go test ./... && git diff --check", 0, []bool{true, true}, nil},
		{"go test ./... && git diff --check", 1, []bool{false, false}, nil},
		{"false && go test ./...", 1, []bool{false, false}, []bool{false, true}},
		{"go test ./...; git diff", 0, []bool{false, true}, nil},
		{"gofmt -w x.go\ngo test ./...", 0, []bool{false, true}, nil},
		{"go test ./... | tail -20", 0, []bool{false, false}, nil},
		{"go test ./... | head -40", 0, []bool{false, false}, nil},
		{"go test ./... | grep ok", 0, []bool{false, false}, nil},
		{"go test ./... | tee result.log", 0, []bool{false, false}, nil},
		{"go test ./... || true", 0, []bool{false, false}, nil},
		{"go test ./... || :", 0, []bool{false, false}, nil},
		{"go test ./... &", 0, []bool{false}, nil},
		{"set -e; go test ./...", 0, []bool{false, false}, nil},
		{"set -o pipefail; go test ./... | tail", 1, []bool{false, false, false}, nil},
		{"sed -i 's/a/b/' x.go && gofmt -l .; go test ./...", 0, []bool{false, false, true}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			c := analyze(tc.line, "", tc.exit)
			if len(c.Statements) != len(tc.reliable) {
				t.Fatalf("statements: %+v", c.Statements)
			}
			for i, s := range c.Statements {
				if s.ExitReliable != tc.reliable[i] {
					t.Errorf("statement %d reliable=%v: %s", i, s.ExitReliable, s.ExitNote)
				}
				if !s.ExitReliable && (s.ExitCode != nil || s.ExitNote == "") {
					t.Errorf("unreliable exit lacks explanation: %+v", s)
				}
				if tc.skipped != nil && (s.Execution == "skipped") != tc.skipped[i] {
					t.Errorf("statement %d execution=%s", i, s.Execution)
				}
			}
		})
	}
}

func TestClassesAndLaunchers(t *testing.T) {
	cases := []struct{ line, class, runner, scope string }{
		{"env X=1 go test ./...", "test", "go", "whole"},
		{"env -u X go test ./...", "test", "go", "whole"},
		{"X=1 go test ./...", "test", "go", "whole"},
		{"npx vitest run", "test", "vitest", "whole"},
		{"bunx jest", "test", "jest", "whole"},
		{"pnpm exec vitest run", "test", "vitest", "whole"},
		{"uv run pytest", "test", "pytest", "whole"},
		{"poetry run pytest", "test", "pytest", "whole"},
		{".venv/bin/pytest", "test", "pytest", "whole"},
		{"./vendor/bin/pest", "test", "pest", "whole"},
		{"python -m pytest", "test", "pytest", "whole"},
		{"php artisan test", "test", "artisan", "unknown"},
		{"cargo test", "test", "cargo", "unknown"},
		{"rspec", "test", "rspec", "unknown"},
		{"go build ./...", "build", "go", "unknown"},
		{"cargo build", "build", "cargo", "unknown"},
		{"go vet ./...", "lint", "go", "unknown"},
		{"golangci-lint run", "lint", "golangci-lint", "unknown"},
		{"eslint .", "lint", "eslint", "unknown"},
		{"ruff check .", "lint", "ruff", "unknown"},
		{"biome check .", "lint", "biome", "unknown"},
		{"pint --test", "lint", "pint", "unknown"},
		{"tsc --noEmit", "typecheck", "tsc", "unknown"},
		{"tsc --build --noEmit", "build", "tsc", "unknown"},
		{"tsc", "build", "tsc", "unknown"},
		{"vue-tsc", "typecheck", "vue-tsc", "unknown"},
		{"mypy .", "typecheck", "mypy", "unknown"},
		{"pyright", "typecheck", "pyright", "unknown"},
		{"phpstan analyse", "typecheck", "phpstan", "unknown"},
		{"psalm", "typecheck", "psalm", "unknown"},
		{"gofmt -l .", "format", "gofmt", "unknown"},
		{"prettier --check .", "format", "prettier", "unknown"},
		{"black --diff .", "format", "black", "unknown"},
		{"ruff format .", "format", "ruff", "unknown"},
		{"pint", "format", "pint", "unknown"},
		{"composer install", "install", "composer", "unknown"},
		{"npm ci", "install", "npm", "unknown"},
		{"pnpm install", "install", "pnpm", "unknown"},
		{"yarn add jest", "install", "yarn", "unknown"},
		{"pip install pytest", "install", "pip", "unknown"},
		{"git diff", "git", "git", "unknown"},
		{"npm test", "test", "npm-script:test", "unknown"},
		{"npm run test:unit", "test", "npm-script:test:unit", "unknown"},
		{"pnpm test", "test", "pnpm-script:test", "unknown"},
		{"yarn run build", "build", "yarn-script:build", "unknown"},
		{"bun test", "test", "bun-script:test", "unknown"},
		{"npm run lint:all", "lint", "npm-script:lint:all", "unknown"},
		{"npm run typecheck", "typecheck", "npm-script:typecheck", "unknown"},
		{"make test", "test", "make", "unknown"},
		{"make", "build", "make", "unknown"},
		{"make build", "build", "make", "unknown"},
		{"make lint", "lint", "make", "unknown"},
		{"make check", "typecheck", "make", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			s := analyze(tc.line, "", 0).Statements[0]
			if s.Class != tc.class || s.Runner != tc.runner || s.Scope != tc.scope {
				t.Fatalf("got %s/%s/%s, want %s/%s/%s", s.Class, s.Runner, s.Scope, tc.class, tc.runner, tc.scope)
			}
		})
	}
}
func TestChecksSurviveFormattingPrefix(t *testing.T) {
	c := analyze("gofmt -w x.go && go test ./...; tsc --noEmit", "", 0)
	if !c.HasChecks() || !reflect.DeepEqual(c.CheckClasses, []string{"test", "typecheck"}) {
		t.Fatalf("classes: %+v", c.CheckClasses)
	}
	if c.Statements[0].Class != "format" || c.Statements[1].Class != "test" {
		t.Fatal(c.Statements)
	}
}
func TestScriptHintIsOnlyCurrentDefinition(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"vitest run"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"npm test", "pnpm run test", "yarn run test", "bun run test"} {
		c := Analyze(&session.Command{Line: line, Cwd: dir, Shell: true})
		s := c.Statements[0]
		if s.CurrentDefinition != "current definition: vitest run" || s.Scope != "unknown" || s.Tests != nil || s.Runner == "vitest" {
			t.Fatalf("hint changes historical evidence: %+v", s)
		}
	}
}
func TestDirectArgvAndUnreadableSource(t *testing.T) {
	c := Analyze(&session.Command{Argv: []string{"go", "test", "./...", "; rm x.go"}, Line: `go test ./... '; rm x.go'`, Cwd: "/repo"})
	if len(c.Statements) != 1 || c.Statements[0].Class != "test" || len(c.Statements[0].Edits) != 0 {
		t.Fatal(c)
	}
	c = analyze("go test 'unterminated", "", 0)
	if len(c.Statements) != 1 || c.Statements[0].Class != "other" || c.Statements[0].ExitNote != "could not read this command" {
		t.Fatal(c)
	}
}
func TestInterruptedAndPendingCommands(t *testing.T) {
	for _, status := range []string{"interrupted", "background", "pending"} {
		c := Analyze(&session.Command{Line: "go test ./...", Shell: true, Status: status, ExitCode: intp(0), ExitReliable: true})
		s := c.Statements[0]
		if s.ExitReliable || s.Result() != "unknown" || s.ExitNote == "" {
			t.Fatal(s)
		}
	}
}
func TestShellEditsAndOrdering(t *testing.T) {
	cases := []struct {
		line      string
		paths     []string
		uncertain bool
	}{
		{"sed -i 's/a/b/' x.go", []string{"/repo/x.go"}, false},
		{"sed -i -e 's/a/b/' x.go", []string{"/repo/x.go"}, false},
		{"gofmt -w x.go", []string{"/repo/x.go"}, false},
		{"prettier --write 'src/app.js'", []string{"/repo/src/app.js"}, false},
		{"black .", []string{"/repo"}, false},
		{"prettier --write 'src/*.js'", nil, true},
		{"black --check .", nil, false},
		{"black --diff .", nil, false},
		{"black --check", nil, false},
		{"echo hi > x.go", []string{"/repo/x.go"}, false},
		{"echo hi >> x.go", []string{"/repo/x.go"}, false},
		{"echo hi > /dev/null 2>&1", nil, false},
		{"mv old.go new.go", []string{"/repo/old.go", "/repo/new.go"}, false},
		{"rm -f x.go", []string{"/repo/x.go"}, false},
		{"git checkout -- x.go", []string{"/repo/x.go"}, false},
		{"git restore --source HEAD -- x.go", []string{"/repo/x.go"}, false},
		{"sed -i 's/a/b/' \"$FILE\"", nil, true},
		{"gofmt -w *.go", nil, true},
		{"git diff", nil, false},
		{"gofmt -l .", nil, false},
		{"ls", nil, false}, {"cat x.go", nil, false}, {"rg hello", nil, false}, {"go test ./...", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			s := analyze(tc.line, "", 0).Statements[0]
			var paths []string
			for _, e := range s.Edits {
				paths = append(paths, e.Path)
			}
			if !reflect.DeepEqual(paths, tc.paths) || s.MayEditFiles != tc.uncertain {
				t.Fatalf("paths=%v uncertain=%v", paths, s.MayEditFiles)
			}
		})
	}
	c := &session.Command{Line: "cd calc && sed -i 's/a/b/' stats.go; go test ./...", Shell: true, Cwd: "/repo", ExitCode: intp(0), ExitReliable: true}
	s := &session.Session{Events: []session.Event{{Seq: 7, Command: c}}}
	l := Build(s)
	var evidence []string
	for _, e := range l.Entries {
		if e.Command != nil {
			evidence = append(evidence, e.Command.Class)
		}
		if e.Edit != nil {
			evidence = append(evidence, e.Edit.Path)
		}
		if e.Seq != 7 {
			t.Fatal("event reference lost")
		}
	}
	if !reflect.DeepEqual(evidence, []string{"other", "other", "/repo/calc/stats.go", "test"}) {
		t.Fatal(evidence)
	}
	if l.Commands[0].Statements[2].Cwd != "/repo/calc" {
		t.Fatal("cd not applied")
	}
	if len(s.Events) != 1 {
		t.Fatal("reader events modified")
	}
	for _, line := range []string{"false && sed -i 's/a/b/' x.go", "go test ./... && black ."} {
		c := analyze(line, "FAIL\tsample\t0.001s\n", 1)
		if s := c.Statements[1]; s.Execution != "skipped" || len(s.Edits) != 0 || s.MayEditFiles {
			t.Fatalf("unexecuted edit: %+v", s)
		}
	}
	c2 := analyze("cd /tmp | cat; echo hi > result", "", 0)
	if c2.Statements[2].Edits[0].Path != "/repo/result" {
		t.Fatal("pipeline cd leaked")
	}
}
func TestSummaryBeforeDisplayTailAndRedaction(t *testing.T) {
	output := "ok\tsample\t0.001s\n" + strings.Repeat("detail\n", 80) + "API_TOKEN=private\n"
	c := analyze("go test ./...; git diff", output, 0)
	s := c.Statements[0]
	if s.ExitReliable || s.Result() != "passed" || s.Tests.Packages == nil || *s.Tests.Packages != 1 {
		t.Fatalf("summary lost: %+v", s)
	}
	if strings.Contains(c.Output, "sample") || strings.Contains(c.Output, "private") || len(strings.Split(c.Output, "\n")) != 40 {
		t.Fatalf("display tail: %s", c.Output)
	}
	if c.OutputTruncatedByAgent {
		t.Fatal("display tail confused with agent cutting")
	}
	c = analyze("pytest | tail -8", "=== 3 passed in 0.03s ===", 0)
	if !c.OutputTruncatedByAgent || c.Statements[0].Result() != "passed" {
		t.Fatal(c)
	}
	c = analyze("pytest", "Output truncated\n=== 3 passed in 0.03s ===", 0)
	if !c.OutputTruncatedByAgent {
		t.Fatal("tool truncation lost")
	}
}
func TestAmbiguousMultipleRuns(t *testing.T) {
	for _, line := range []string{"go test ./a; go test ./b", "pytest; pytest", "npm test; pytest"} {
		c := analyze(line, "ok\tsample\t0.01s\n=== 3 passed in 0.03s ===", 0)
		if c.Statements[0].Tests != nil && c.Statements[0].Tests.Outcome == "passed" {
			t.Fatal("summary assigned to ambiguous run")
		}
		if c.Statements[1].Tests != nil && c.Statements[1].Tests.Passed != nil {
			t.Fatal("summary assigned to ambiguous run")
		}
	}
}

func TestDistinctRunnerBannersAttributeSummaries(t *testing.T) {
	output := "platform linux -- Python 3.12.3, pytest-9.1.1, pluggy-1.7.0\n=== 2 passed in 0.01s ===\nPHPUnit 11.5.56 by Sebastian Bergmann and contributors.\nFAILURES!\nTests: 3, Assertions: 2, Failures: 1, Skipped: 1.\n"
	c := analyze("pytest; phpunit", output, 1)
	if c.Statements[0].Result() != "passed" || c.Statements[0].Tests.Passed == nil || *c.Statements[0].Tests.Passed != 2 {
		t.Fatal(c.Statements[0])
	}
	if c.Statements[1].Tests.Outcome != "failed" || *c.Statements[1].Tests.Failed != 1 {
		t.Fatal(c.Statements[1])
	}
	c = analyze("pytest; pytest", output, 0)
	for _, s := range c.Statements {
		if s.Tests.Passed != nil {
			t.Fatal("runner banner cannot disambiguate repeated invocations")
		}
	}
}

func TestFailureSummaryDoesNotProveAnotherStatementsExit(t *testing.T) {
	c := analyze("pytest && git diff --check", "=== 3 passed in 0.01s ===", 1)
	for _, s := range c.Statements {
		if s.ExitReliable {
			t.Fatal("test counts promoted to shell exit attribution")
		}
	}
	if c.Statements[0].Result() != "passed" {
		t.Fatal("attributable summary lost")
	}
}
func TestShellStopsDoNotCreateEdits(t *testing.T) {
	for _, line := range []string{"set -e; false; black .", "exit 1; sed -i 's/a/b/' x.go"} {
		c := analyze(line, "", 1)
		s := c.Statements[len(c.Statements)-1]
		if s.Execution != "skipped" || len(s.Edits) > 0 || s.MayEditFiles {
			t.Fatal(s)
		}
	}
	c := analyze("set -e; set +e; go test ./...", "", 0)
	if !c.Statements[2].ExitReliable {
		t.Fatal("disabled errexit still masks exit")
	}
}

func TestSelectorsRetainTheirValues(t *testing.T) {
	for _, tc := range []struct {
		line    string
		targets []string
	}{
		{"go test ./calc -run TestAverage", []string{"./calc", "TestAverage"}},
		{"go test ./... -run=TestAverage -timeout 30s", []string{"TestAverage"}},
		{"pytest -k average", []string{"average"}},
		{"pest --filter=average", []string{"average"}},
	} {
		s := analyze(tc.line, "", 0).Statements[0]
		if !reflect.DeepEqual(s.Targets, tc.targets) {
			t.Errorf("%s targets=%v want %v", tc.line, s.Targets, tc.targets)
		}
	}
}

func TestBackgroundChainsKeepTheirStatements(t *testing.T) {
	c := analyze("cd calc && go test ./... & echo done", "", 0)
	if len(c.Statements) != 3 || c.Statements[1].Class != "test" || c.Statements[1].ExitReliable || c.Statements[2].Cwd != "/repo" {
		t.Fatalf("background statements: %+v", c.Statements)
	}
	c = analyze("set -e; false & black .", "", 0)
	if s := c.Statements[2]; s.Execution == "skipped" || len(s.Edits) != 1 {
		t.Fatal("background failure stopped its parent shell")
	}
	c = analyze("! go test ./... | tail", "", 0)
	if !c.HasChecks() || c.Statements[0].ExitReliable {
		t.Fatal("negation hid a check or supplied its exit")
	}
}

func TestMoveAndDeleteOperations(t *testing.T) {
	c := analyze("mv old.go new.go; rm obsolete.go", "", 0)
	if c.Statements[0].Edits[0].Op != "delete" || c.Statements[0].Edits[1].Op != "write" || c.Statements[1].Edits[0].Op != "delete" {
		t.Fatal("move/delete operation lost")
	}
}

func TestNonFinalChainExitNote(t *testing.T) {
	c := analyze("gofmt -w x.go && go test ./...; git diff", "", 0)
	for _, s := range c.Statements[:2] {
		if !strings.Contains(s.ExitNote, "a later statement determines the line exit") {
			t.Fatalf("misleading exit note: %s", s.ExitNote)
		}
	}
}

func TestConditionalShellExit(t *testing.T) {
	for _, line := range []string{"false || exit 1; black .", "true && exit 1; black ."} {
		c := analyze(line, "", 1)
		s := c.Statements[2]
		if s.Execution != "skipped" || s.ExitReliable || len(s.Edits) > 0 || s.MayEditFiles {
			t.Fatalf("command after shell exit: %+v", s)
		}
	}
	c := analyze("go test ./... || exit 1; git diff", "", 1)
	if s := c.Statements[2]; s.ExitReliable || s.Result() != "unknown" || !strings.Contains(s.ExitNote, "conditional exit") {
		t.Fatalf("conditional exit attributed to following command: %+v", s)
	}
	c = analyze("true || exit 1; go test ./...", "", 0)
	if !c.Statements[2].ExitReliable {
		t.Fatal("skipped exit stopped shell")
	}
	c = analyze("exit 1 & go test ./...", "", 0)
	if !c.Statements[1].ExitReliable {
		t.Fatal("background exit stopped parent shell")
	}
	c = analyze("exit 1 | cat; go test ./...", "", 0)
	if !c.Statements[2].ExitReliable {
		t.Fatal("pipeline exit stopped parent shell")
	}
}

func TestBSDInPlaceSedTargets(t *testing.T) {
	c := analyze("sed -i '' 's/a/b/' x.go", "", 0)
	s := c.Statements[0]
	if len(s.Edits) != 1 || s.Edits[0].Path != "/repo/x.go" || s.MayEditFiles {
		t.Fatalf("sed targets: %+v", s)
	}
}

func TestOpaqueCompoundEditEvidence(t *testing.T) {
	for _, line := range []string{
		"for f in *.go; do gofmt -w \"$f\"; done",
		"if test -f x.go; then sed -i 's/a/b/' x.go; fi",
		"(cd calc; black .)", "{ rm x.go; }", "time gofmt -w x.go",
		"if true; then { cat x; } > output; fi",
	} {
		s := analyze(line, "", 0).Statements[0]
		if !s.MayEditFiles {
			t.Errorf("compound editor disappeared: %s", line)
		}
	}
	for _, line := range []string{"{ git diff; }", "(black --check .)", "if true; then black --diff .; fi", "format_later() { black .; }", "false && { black .; }"} {
		c := analyze(line, "", 1)
		for _, s := range c.Statements {
			if s.MayEditFiles || len(s.Edits) > 0 {
				t.Errorf("read-only or unexecuted compound marked as editing: %s", line)
			}
		}
	}
}

func TestHeadCannotProveCompleteGoSuccess(t *testing.T) {
	for _, head := range []string{"head -1", "head -n 1", "head --lines=1"} {
		c := analyze("go test ./... | "+head, "ok\tsample/pass\t0.002s\n", 0)
		s := c.Statements[0]
		if s.Result() != "unknown" || s.Tests.Packages == nil || *s.Tests.Packages != 1 || !strings.Contains(s.ExitNote, "head may have cut") {
			t.Fatalf("cut summary: %+v", s)
		}
	}
	c := analyze("go test ./... | head -40", "ok\tsample/pass\t0.002s\n", 0)
	if c.Statements[0].Result() != "passed" {
		t.Fatal("head below limit hid complete summary")
	}
	c = analyze("go test ./... | head -1", "FAIL\tsample/fail\t0.002s\n", 0)
	if c.Statements[0].Result() != "failed" {
		t.Fatal("cut output hid visible failure")
	}
}

func TestInlineInterpreterWrites(t *testing.T) {
	for _, tc := range []struct {
		line    string
		mayEdit bool
	}{
		{"python3 - <<'PY'\nopen(p,'w').write(s)\nPY", true},
		{"python3 -c 'from pathlib import Path; Path(p).write_text(s)'", true},
		{"node -e 'fs.writeFileSync(path, data)'", true},
		{"python3 - <<'PY'\nprint(open(p).read())\nPY", false},
	} {
		got := Analyze(&session.Command{Line: tc.line, Shell: true, Cwd: "/repo"})
		if len(got.Statements) != 1 || got.Statements[0].MayEditFiles != tc.mayEdit {
			t.Fatalf("%s: %+v", tc.line, got.Statements)
		}
	}
}
