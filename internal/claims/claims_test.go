package claims

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/saidran/internal/commands"
	"github.com/vincentsch/saidran/internal/session"
)

func TestExtractFinalAssertions(t *testing.T) {
	for _, tc := range []struct{ text, kind, scope string }{
		{"All tests pass.", "tests", "whole"},
		{"All 3 tests passed.", "tests", "whole"},
		{"three tests pass.", "tests", "whole"},
		{"Seven tests passed.", "tests", "whole"},
		{"Tests are passing.", "tests", "whole"},
		{"The test suite passes.", "tests", "whole"},
		{"Tests are green.", "tests", "whole"},
		{"Alle Tests sind grün.", "tests", "whole"},
		{"Die Tests laufen durch.", "tests", "whole"},
		{"Alle Tests bestehen.", "tests", "whole"},
		{"Tests bestanden.", "tests", "whole"},
		{"_all tests pass_.", "tests", "whole"},
		{"Die Tests liefen durch.", "tests", "whole"},
		{"Tests waren grün.", "tests", "whole"},
		{"Build lief.", "build", "whole"},
		{"**all tests pass**.", "tests", "whole"},
		{"`go test ./...` passed after that fix.", "tests", "whole"},
		{"`pytest tests/test_calc.py` passes.", "tests", "targeted"},
		{"The calc tests pass.", "tests", "targeted"},
		{"TestAverage passes in calc.", "tests", "targeted"},
		{"The build passes.", "build", "whole"},
		{"It builds successfully.", "build", "whole"},
		{"It compiles cleanly.", "build", "whole"},
		{"Build läuft.", "build", "whole"},
		{"Es kompiliert.", "build", "whole"},
		{"Lint passes.", "lint", "whole"},
		{"No lint errors.", "lint", "whole"},
		{"The linter is clean.", "lint", "whole"},
		{"keine Lint-Fehler.", "lint", "whole"},
		{"type checks passed.", "typecheck", "whole"},
		{"No type errors.", "typecheck", "whole"},
		{"tsc passes.", "typecheck", "targeted"},
		{"mypy passes.", "typecheck", "targeted"},
		{"keine Typfehler.", "typecheck", "whole"},
		{"`go build ./...` passed.", "build", "whole"},
		{"`ruff check .` passes.", "lint", "whole"},
		{"`tsc --noEmit` passed.", "typecheck", "whole"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, _ := Extract(tc.text, time.Time{})
			if len(got) != 1 || got[0].Type != tc.kind || got[0].Scope != tc.scope || got[0].Quote != tc.text {
				t.Fatalf("%+v", got)
			}
		})
	}
	for _, text := range []string{
		"No tests pass.", "Keine Tests bestanden.", "After tests pass, merge.", "Tests don't pass.", "Not all tests pass.", "Tests sind nicht grün.", "Nicht alle Tests bestanden.",
		"Tests should pass once fixed.", "Tests will pass.", "Tests pass when the service starts.", "If tests pass, merge.", "Fix it so they pass.",
		"I couldn't run the tests.", "`go test ./...` reports `ok`.",
		"`TestAverage`, `TestAverageEmpty` and `TestParseList` pass.", "I ran pytest.",
		"The build would pass.", "No lint errors should remain.",
	} {
		t.Run(text, func(t *testing.T) {
			if got, _ := Extract(text, time.Time{}); len(got) != 0 {
				t.Fatalf("unexpected %+v", got)
			}
		})
	}
	pathClaims, _ := Extract("Tests pass in tests/test_calc.py.", time.Time{})
	if len(pathClaims) != 1 || len(pathClaims[0].Targets) != 1 || pathClaims[0].Targets[0] != "tests/test_calc.py" {
		t.Fatalf("path changed by Markdown normalization: %+v", pathClaims)
	}
	got, gaps := Extract("Tests passed; all tests are green.\nI couldn't run the build.\nTests were not rerun after the rename.", time.Time{})
	if len(got) != 2 || len(gaps) != 2 {
		t.Fatalf("claims=%+v gaps=%+v", got, gaps)
	}
}

func TestVerdictsFromRecordedEvidence(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	type step struct {
		line, output, edit, cwd string
		exit                    int
		start, end              int
	}
	run := func(line string, exit int) step { return step{line: line, exit: exit, start: 1, end: 2} }
	pass := run("go test ./...", 0)
	for _, tc := range []struct {
		name, quote, want string
		steps             []step
		runs              int
	}{

		{"copy after test", "All tests pass.", "stale", []step{pass, {line: "cp calc/calc.go calc/replaced.go", start: 3, end: 4}}, 1},
		{"eslint fix", "All tests pass.", "cant_tell", []step{pass, {line: "eslint --fix", start: 3, end: 4}}, 1},
		{"ruff fix", "All tests pass.", "cant_tell", []step{pass, {line: "ruff check --fix", start: 3, end: 4}}, 1},

		{"patch stat with apply", "All tests pass.", "cant_tell", []step{pass, {line: "git apply --stat --apply fix.patch", start: 3, end: 4}}, 1},
		{"patch check with apply", "All tests pass.", "cant_tell", []step{pass, {line: "git apply --check --apply fix.patch", start: 3, end: 4}}, 1},
		{"patch after test", "All tests pass.", "cant_tell", []step{pass, {line: "git apply fix.patch", start: 3, end: 4}}, 1},
		{"different projects", "`tsc --noEmit -p tsconfig.client.json` passed.", "no_matching_run", []step{run("tsc --noEmit -p tsconfig.server.json", 0)}, 0},
		{"client failure", "`tsc --noEmit -p tsconfig.client.json` passed.", "contradicted", []step{run("tsc --noEmit -p tsconfig.client.json", 1), {line: "tsc --noEmit -p tsconfig.server.json", start: 3, end: 4}}, 1},
		{"equivalent projects", "`tsc --noEmit --project web/tsconfig.server.json` passed.", "verified", []step{{line: "tsc --noEmit -p tsconfig.server.json", cwd: "web", start: 1, end: 2}}, 1},
		{"implicit project", "`tsc --noEmit` passed.", "no_matching_run", []step{run("tsc --noEmit -p tsconfig.server.json", 0)}, 0},
		{"implicit build", "`go build ./...` passed.", "partial", []step{run("go build", 0)}, 1},
		{"implicit vet", "`go vet ./...` passed.", "partial", []step{run("go vet", 0)}, 1},
		{"json no tests", "All tests pass.", "partial", []step{{line: "go test -json ./...", output: "{\"Action\":\"output\",\"Package\":\"example/calc\",\"Output\":\"? example/calc [no test files]\\n\"}\n{\"Action\":\"skip\",\"Package\":\"example/calc\"}\n", start: 1, end: 2}}, 1},
		{"json unreadable", "All tests pass.", "cant_tell", []step{{line: "go test -json ./...", output: "{broken", start: 1, end: 2}}, 1},
		{"json incomplete", "All tests pass.", "cant_tell", []step{{line: "go test -json ./...", output: "{\"Action\":\"start\",\"Package\":\"example/calc\"}\n", start: 1, end: 2}}, 1},
		{"json failed suite recovered by package", "All tests pass.", "partial", []step{{line: "go test -json ./...", output: "{\"Action\":\"fail\",\"Package\":\"example/calc\"}\n", exit: 1, start: 1, end: 2}, {edit: "calc/stats.go", end: 3}, {line: "go test ./calc", start: 4, end: 5}}, 2},

		{"current package build", "`go build` passed.", "verified", []step{run("go build", 0)}, 1},
		{"current package vet", "`go vet .` passed.", "verified", []step{run("go vet", 0)}, 1},
		{"matching explicit project", "`tsc --noEmit -p tsconfig.server.json` passed.", "verified", []step{run("tsc --noEmit --project=./tsconfig.server.json", 0)}, 1},
		{"matching implicit project", "`tsc --noEmit` passed.", "verified", []step{run("tsc --noEmit", 0)}, 1},
		{"implicit projects in different directories", "`tsc --noEmit` passed.", "no_matching_run", []step{{line: "tsc --noEmit", cwd: "web", start: 1, end: 2}}, 0},
		{"omitted recorded project", "`tsc --noEmit -p tsconfig.server.json` passed.", "no_matching_run", []step{run("tsc --noEmit", 0)}, 0},
		{"eslint file fix", "All tests pass.", "stale", []step{pass, {line: "eslint --fix web/app.js", start: 3, end: 4}}, 1},
		{"ruff file fix", "All tests pass.", "stale", []step{pass, {line: "ruff check --fix app.py", start: 3, end: 4}}, 1},
		{"read only lint and patch", "All tests pass.", "verified", []step{pass, {line: "eslint web/app.js; ruff check app.py; git apply --check fix.patch", start: 3, end: 4}}, 1},
		{"unexecuted writers", "All tests pass.", "verified", []step{pass, {line: "false && cp a.go b.go && eslint --fix && ruff check --fix && git apply fix.patch", exit: 1, start: 3, end: 4}}, 1},
		{"fresh check after writers", "All tests pass.", "verified", []step{run("cp a.go b.go; eslint --fix; ruff check --fix; git apply fix.patch; go test ./...", 0)}, 1},

		{"json passing suite", "All tests pass.", "verified", []step{{line: "go test -json ./...", output: "{\"Action\":\"pass\",\"Package\":\"example/calc\"}\n", start: 1, end: 2}}, 1},
		{"json failing suite", "All tests pass.", "contradicted", []step{{line: "go test -json ./...", output: "{\"Action\":\"fail\",\"Package\":\"example/calc\"}\n", exit: 1, start: 1, end: 2}}, 1},
		{"json attributable test failure", "All tests pass.", "contradicted", []step{{line: "go test -json ./... | tail", output: "{\"Action\":\"fail\",\"Package\":\"example/calc\",\"Test\":\"TestSum\"}\n", start: 1, end: 2}}, 1},
		{"json reliable failure without summary", "All tests pass.", "contradicted", []step{{line: "go test -json ./...", output: "{broken", exit: 1, start: 1, end: 2}}, 1},
		{"json unreliable empty result", "All tests pass.", "cant_tell", []step{{line: "go test -json ./... | tail", output: "{broken", start: 1, end: 2}}, 1},
		{"matching suite", "All tests pass.", "verified", []step{pass}, 1},
		{"no candidate", "All tests pass.", "no_matching_run", nil, 0},
		{"failure", "All tests pass.", "contradicted", []step{run("go test ./...", 1)}, 1},
		{"unknown pipeline", "All tests pass.", "cant_tell", []step{run("go test ./... | tail", 0)}, 1},
		{"attributable summary", "All tests pass.", "verified", []step{{line: "go test ./... | tail", output: "ok example/calc 0.1s\n", start: 1, end: 2}}, 1},
		{"unknown runner", "All tests pass.", "cant_tell", []step{run("make test", 0)}, 1},
		{"unknown script scope", "All tests pass.", "cant_tell", []step{run("npm test", 0)}, 1},
		{"later edit", "All tests pass.", "stale", []step{pass, {edit: "calc/stats.go", end: 3}}, 1},
		{"concurrent edit", "All tests pass.", "cant_tell", []step{{line: "go test ./...", start: 1, end: 5}, {edit: "calc/stats.go", end: 3}}, 1},
		{"docs only", "All tests pass.", "verified", []step{pass, {edit: "docs/howto.txt", end: 3}, {edit: "README.md", end: 4}}, 1},
		{"outside repo", "All tests pass.", "verified", []step{pass, {edit: filepath.Join(filepath.Dir(root), "external.go"), end: 3}}, 1},
		{"unknown edit", "All tests pass.", "cant_tell", []step{pass, {line: "sed -i s/a/b/ \"$FILE\"", start: 3, end: 4}}, 1},
		{"known irrelevant write", "All tests pass.", "verified", []step{pass, {line: "cat > docs/note.txt <<'EOF'\nnote\nEOF", start: 3, end: 4}}, 1},
		{"interpreter writes", "All tests pass.", "cant_tell", []step{pass, {line: "python3 - <<'PY'\nopen(p,'w').write(s)\nPY", start: 3, end: 4}}, 1},
		{"interpreter before check", "All tests pass.", "verified", []step{run("python3 - <<'PY'\nopen(p,'w').write(s)\nPY\ngo test ./...", 0)}, 1},
		{"edit before check", "All tests pass.", "verified", []step{run("cat > calc/stats.go <<'EOF'\npackage calc\nEOF\ngo test ./...", 0)}, 1},
		{"edit after check", "All tests pass.", "stale", []step{{line: "go test ./...; gofmt -w calc/stats.go", output: "ok example/calc 0.1s\n", start: 1, end: 2}}, 1},
		{"narrow package", "All tests pass.", "partial", []step{run("go test ./calc", 0)}, 1},
		{"different named package", "The other tests pass.", "no_matching_run", []step{run("go test ./calc", 0)}, 0},
		{"named package", "The calc tests pass.", "verified", []step{run("go test ./calc", 0)}, 1},
		{"narrow named package", "The calc tests pass.", "partial", []step{run("go test ./calc -run TestAverage", 0)}, 1},
		{"exact filter", "TestAverage passes in calc.", "verified", []step{run("go test ./calc -run TestAverage", 0)}, 1},
		{"different filter", "TestOther passes in calc.", "no_matching_run", []step{run("go test ./calc -run TestAverage", 0)}, 0},
		{"list suite", "All tests pass.", "partial", []step{run("go test ./... -list .", 0)}, 1},
		{"list package", "The calc tests pass.", "partial", []step{run("go test ./calc -list .", 0)}, 1},
		{"no tests suite", "All tests pass.", "partial", []step{{line: "go test ./...", output: "? example/calc [no test files]\n", start: 1, end: 2}}, 1},
		{"no tests package", "The calc tests pass.", "partial", []step{{line: "go test ./calc", output: "? example/calc [no test files]\n", start: 1, end: 2}}, 1},
		{"later scoped failure", "All tests pass.", "contradicted", []step{pass, {line: "go test ./calc", exit: 1, start: 3, end: 4}}, 2},
		{"later whole recovery", "All tests pass.", "verified", []step{pass, {line: "go test ./calc", exit: 1, start: 3, end: 4}, {line: "go test ./...", start: 5, end: 6}}, 1},
		{"later package recovery", "All tests pass.", "verified", []step{pass, {line: "go test ./calc", exit: 1, start: 3, end: 4}, {line: "go test ./calc", start: 5, end: 6}}, 1},
		{"unrelated recovery", "All tests pass.", "contradicted", []step{pass, {line: "go test ./calc", exit: 1, start: 3, end: 4}, {line: "go test ./other", start: 5, end: 6}}, 2},
		{"failure outside claim", "The calc tests pass.", "verified", []step{run("go test ./calc", 0), {line: "go test ./other", exit: 1, start: 3, end: 4}}, 1},
		{"different runner", "`pytest` passed.", "no_matching_run", []step{pass}, 0},
		{"suite in subfolder", "All tests pass.", "partial", []step{{line: "go test ./...", cwd: "calc", start: 1, end: 2}}, 1},

		{"named build command", "`go build ./calc` passed.", "verified", []step{run("go build ./calc", 0)}, 1},
		{"unrelated build command", "`go build ./calc` passed.", "no_matching_run", []step{run("go build ./other", 0)}, 0},
		{"narrow build", "The build passes.", "partial", []step{run("go build ./calc", 0)}, 1},
		{"narrower package", "The calc tests pass.", "partial", []step{run("go test ./calc/sub", 0)}, 1},
		{"package covers file", "`go test ./calc/stats_test.go` passed.", "verified", []step{run("go test ./calc", 0)}, 1},

		{"broad failure outside named scope", "The calc tests pass.", "verified", []step{{line: "go test ./...", output: "ok example/calc 0.1s\nFAIL example/other 0.1s\n", exit: 1, start: 1, end: 2}, {line: "go test ./calc", start: 3, end: 4}}, 1},
		{"failed suite recovered by package", "All tests pass.", "partial", []step{{line: "go test ./...", output: "FAIL example/calc 0.1s\n", exit: 1, start: 1, end: 2}, {edit: "calc/stats.go", end: 3}, {line: "go test ./calc", start: 4, end: 5}}, 2},
		{"out of scope coverage uses passing rerun", "The calc tests pass.", "partial", []step{{line: "go test ./...", output: "FAIL example/other 0.1s\n", exit: 1, start: 1, end: 2}, {line: "go test ./calc -run TestAverage", start: 3, end: 4}}, 2},
		{"make build", "The build passes.", "verified", []step{run("make build", 0)}, 1},
		{"make lint", "No lint errors.", "verified", []step{run("make lint", 0)}, 1},
		{"make typecheck", "No type errors.", "verified", []step{run("make typecheck", 0)}, 1},
		{"ruff tree", "No lint errors.", "verified", []step{run("ruff check .", 0)}, 1},
		{"ruff German claim", "keine Lint-Fehler.", "verified", []step{run("ruff check .", 0)}, 1},
		{"eslint tree", "No lint errors.", "verified", []step{run("eslint .", 0)}, 1},
		{"mypy tree", "No type errors.", "verified", []step{run("mypy .", 0)}, 1},
		{"tsc project directory", "No type errors.", "verified", []step{run("tsc --noEmit -p .", 0)}, 1},
		{"tsc project file", "No type errors.", "verified", []step{run("tsc --noEmit --project tsconfig.json", 0)}, 1},
		{"go dot remains narrow", "All tests pass.", "partial", []step{run("go test .", 0)}, 1},
		{"build after edits", "The build passes.", "verified", []step{{edit: "calc/stats.go", end: 1}, {line: "go build ./...", start: 2, end: 3}}, 1},
		{"lint negative evidence", "keine Lint-Fehler.", "contradicted", []step{run("ruff check .", 1)}, 1},
		{"typecheck after edit", "keine Typfehler.", "stale", []step{run("tsc --noEmit", 0), {edit: "src/index.ts", end: 3}}, 1},
		{"final response cutoff", "All tests pass.", "verified", []step{pass, {line: "go test ./calc", exit: 1, start: 101, end: 102}, {edit: "calc/stats.go", end: 103}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &session.Session{Cwd: root, Final: tc.quote, FinalTime: at.Add(100 * time.Second), Ended: at.Add(200 * time.Second)}
			for i, st := range tc.steps {
				e := session.Event{Seq: i + 1, Time: at.Add(time.Duration(st.end) * time.Second)}
				if st.edit != "" {
					p := st.edit
					if !filepath.IsAbs(p) {
						p = filepath.Join(root, p)
					}
					e.Edit = &session.Edit{Path: p, Op: "edit"}
				} else {
					cwd := root
					if st.cwd != "" {
						cwd = filepath.Join(root, st.cwd)
					}
					exit := st.exit
					e.Command = &session.Command{Line: st.line, Shell: true, Cwd: cwd, Start: at.Add(time.Duration(st.start) * time.Second), End: e.Time, ExitCode: &exit, ExitReliable: true, Status: "completed", RawOutput: st.output}
				}
				s.Events = append(s.Events, e)
			}
			got, _ := Evaluate(root, s, commands.Build(s))
			if len(got) != 1 || got[0].Verdict != tc.want || len(got[0].Runs) != tc.runs {
				t.Fatalf("%+v", got)
			}
			c := got[0]
			if tc.name == "failed suite recovered by package" || tc.name == "json failed suite recovered by package" || tc.name == "out of scope coverage uses passing rerun" {
				if c.Runs[0].Result != "failed" || c.Runs[1].Result != "passed" || strings.Contains(c.Explanation, "unknown") || !strings.Contains(c.Explanation, c.Runs[1].Command) {
					t.Fatalf("wrong deciding evidence: %+v", c)
				}
			}
			if c.Explanation == "" || c.NextStep == "" {
				t.Fatal(c)
			}
			if tc.want == "stale" && len(c.Edits) == 0 {
				t.Fatal("missing edit reference")
			}
			if len(c.Runs) > 0 && (c.Runs[0].Seq < 1 || c.Runs[0].End.IsZero()) {
				t.Fatal("missing run reference")
			}
			if tc.want == "no_matching_run" && !strings.Contains(strings.ToLower(c.Explanation), "no matching run found") {
				t.Fatal(c.Explanation)
			}
		})
	}
}

func TestPassingCheckLists(t *testing.T) {
	for _, tc := range []struct {
		text  string
		kinds []string
	}{
		{"All tests, race checks, vet, formatting, build, and conformance pass.", []string{"tests", "build", "lint"}},
		{"Full tests, race checks, vet, formatting and build passed.", []string{"tests", "build", "lint"}},
		{"Tests, lint, build, and 12/12 conformance rules passed.", []string{"tests", "build", "lint"}},
		{"All six fixtures, race tests, vet, whitespace checks, and 12/12 conformance rules pass.", []string{"tests", "lint"}},
		{"Tests and build pass.", []string{"tests", "build"}},
		{"Lint, type checks, and tests passed.", []string{"tests", "lint", "typecheck"}},
		{"**All tests**, vet, type checks and build pass.", []string{"tests", "build", "lint", "typecheck"}},
		{"Not all tests, vet and build pass.", nil},
		{"Tests, vet and build will pass.", nil},
		{"If tests, lint and build pass, merge.", nil},
		{"Tests, vet and build should pass.", nil},
		{"TestAverage, TestEmpty and TestParse pass.", nil},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, _ := Extract(tc.text, time.Time{})
			if len(got) != len(tc.kinds) {
				t.Fatalf("%+v", got)
			}
			for i, kind := range tc.kinds {
				if got[i].Type != kind || got[i].Scope != "whole" || got[i].Quote != tc.text {
					t.Fatalf("%+v", got)
				}
			}
		})
	}
}
func TestEvidenceClockUsesLocalTime(t *testing.T) {
	previous := time.Local
	time.Local = time.FixedZone("CEST", 2*60*60)
	t.Cleanup(func() { time.Local = previous })
	if got := clock(time.Date(2026, 10, 20, 15, 50, 0, 0, time.UTC)); got != "17:50" {
		t.Fatal(got)
	}
}

func TestIndependentAssertions(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, text := range []string{"`go test ./other` passed; `go test ./calc` passed", "`go test ./calc` passed; `go test ./other` passed", "`go test ./other` passed and `go test ./calc` passed", "`go test ./calc` passed and `go test ./other` passed"} {
		exit := 0
		s := &session.Session{Cwd: root, Final: text, FinalTime: at.Add(time.Minute), Events: []session.Event{{Seq: 1, Time: at, Command: &session.Command{Line: "go test ./calc", Shell: true, Cwd: root, Start: at, End: at, ExitCode: &exit, ExitReliable: true}}}}
		got, _ := Evaluate(root, s, commands.Build(s))
		if len(got) != 2 {
			t.Fatalf("%s: %+v", text, got)
		}
		for _, c := range got {
			switch c.Quote {
			case "`go test ./calc` passed":
				if c.Verdict != "verified" {
					t.Fatal(c)
				}
			case "`go test ./other` passed":
				if c.Verdict != "no_matching_run" {
					t.Fatal(c)
				}
			default:
				t.Fatalf("shared assertion: %+v", c)
			}
		}
	}
	for _, text := range []string{"All tests pass, but I did not run lint", "All tests pass and I did not run lint", "Alle Tests sind grün, aber Lint wurde nicht ausgeführt"} {
		got, gaps := Extract(text, at)
		if len(got) != 1 || len(gaps) != 1 || strings.Contains(got[0].Quote, "lint") || strings.Contains(got[0].Quote, "Lint") || strings.Contains(gaps[0].Quote, "Tests") {
			t.Fatalf("%+v %+v", got, gaps)
		}
	}
}

func TestSuiteScopeUsesSessionDirectory(t *testing.T) {
	root := t.TempDir()
	at := time.Now()
	for _, tc := range []struct{ cwd, quote, want string }{
		{root, "All tests pass.", "partial"},
		{filepath.Join(root, "calc"), "All tests pass.", "verified"},
		{root, "`go test ./...` passed.", "verified"},
		{root, "The calc tests pass.", "verified"},
	} {
		exit := 0
		s := &session.Session{Cwd: tc.cwd, Final: tc.quote, FinalTime: at.Add(time.Minute), Events: []session.Event{{Seq: 1, Time: at, Command: &session.Command{Line: "cd calc && go test ./...", Shell: true, Cwd: root, Start: at, End: at, ExitCode: &exit, ExitReliable: true}}}}
		got, _ := Evaluate(root, s, commands.Build(s))
		if len(got) != 1 || got[0].Verdict != tc.want {
			t.Fatalf("%+v: %+v", tc, got)
		}
	}
}

func TestPositiveAssertionsBesideNegatedOrConditionalClauses(t *testing.T) {
	root := t.TempDir()
	at := time.Now()
	for _, tc := range []struct{ positive, other, join string }{
		{"All tests pass", "lint does not pass", " and "},
		{"All tests pass", "the build should pass", " and "},
		{"Alle Tests sind grün", "Lint ist nicht grün", " und "},
		{"Alle Tests sind grün", "der Build sollte bestehen", " und "},
	} {
		for _, text := range []string{tc.positive + tc.join + tc.other, tc.other + tc.join + tc.positive} {
			t.Run(text, func(t *testing.T) {
				exit := 0
				s := &session.Session{Cwd: root, Final: text, FinalTime: at.Add(time.Minute), Events: []session.Event{{Seq: 1, Time: at, Command: &session.Command{Line: "go test ./...", Shell: true, Cwd: root, Start: at, End: at, ExitCode: &exit, ExitReliable: true}}}}
				got, gaps := Evaluate(root, s, commands.Build(s))
				if len(got) != 1 || got[0].Quote != tc.positive || got[0].Type != "tests" || got[0].Verdict != "verified" || len(gaps) != 0 {
					t.Fatalf("claims=%+v gaps=%+v", got, gaps)
				}
			})
		}
	}
	for _, text := range []string{"If tests pass and lint passes, merge", "Tests pass when build passes and lint passes", "Wenn Tests bestehen und Build läuft, fortfahren"} {
		if got, _ := Extract(text, at); len(got) != 0 {
			t.Fatalf("conditional created claims: %+v", got)
		}
	}
}

func TestQuotedAssertionsBesideNegatedOrModalResults(t *testing.T) {
	root := t.TempDir()
	at := time.Now()
	positive := "`go test ./calc` passed"
	for _, suffix := range []string{"did not pass", "didn't pass", "should pass", "would pass"} {
		other := "`go vet ./...` " + suffix
		for _, join := range []string{" and ", "; "} {
			for _, text := range []string{positive + join + other, other + join + positive} {
				for _, exit := range []int{0, 1} {
					t.Run(fmt.Sprintf("%s/exit%d", text, exit), func(t *testing.T) {
						s := &session.Session{Cwd: root, Final: text, FinalTime: at.Add(time.Minute), Events: []session.Event{{Seq: 1, Time: at, Command: &session.Command{Line: "go test ./calc", Shell: true, Cwd: root, Start: at, End: at, ExitCode: &exit, ExitReliable: true}}}}
						got, gaps := Evaluate(root, s, commands.Build(s))
						want := "verified"
						if exit != 0 {
							want = "contradicted"
						}
						if len(got) != 1 || got[0].Quote != positive || got[0].Type != "tests" || got[0].Verdict != want || len(gaps) != 0 {
							t.Fatalf("claims=%+v gaps=%+v; want %s", got, gaps, want)
						}
					})
				}
			}
		}
		if got, _ := Extract(other, at); len(got) != 0 {
			t.Fatalf("non-passing assertion admitted: %+v", got)
		}
	}
}
