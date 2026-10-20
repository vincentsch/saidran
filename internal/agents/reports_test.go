package agents_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vincentsch/gaugestone/internal/agents"
	"github.com/vincentsch/gaugestone/internal/report"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "gaugestone")
	cmd := exec.Command("go", "build", "-o", binary, "../../cmd/gaugestone")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build: %s %v", b, e)
	}
	return binary
}
func runBinary(t *testing.T, binary, repo string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = repo
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	code := 0
	if e := cmd.Run(); e != nil {
		if exit, ok := e.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(e)
		}
	}
	return code, out.String(), errout.String()
}
func decodeReport(t *testing.T, s string) report.Report {
	t.Helper()
	var r report.Report
	decoder := json.NewDecoder(strings.NewReader(s))
	if e := decoder.Decode(&r); e != nil {
		t.Fatalf("report: %v %s", e, s)
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		t.Fatalf("multiple output documents: %v", e)
	}
	return r
}
func TestRecordedReports(t *testing.T) {
	binary := buildBinary(t)
	for _, name := range []string{"claude-contradicted", "claude-honest-failure", "claude-stale", "claude-verified", "claude-skip", "claude-pytest", "codex-stale", "codex-verified", "opencode-verified", "opencode-stale", "pi-verified", "pi-stale"} {
		t.Run(name, func(t *testing.T) {
			base, repo, transcript := fixture(t, name)
			s, e := agents.Read(transcript)
			if e != nil {
				t.Fatal(e)
			}
			for _, form := range [][]string{{"--json"}, {"--format", "json"}, {"--format=json"}} {
				args := append([]string{"--transcript", transcript, "--no-write"}, form...)
				code, out, errout := runBinary(t, binary, repo, args...)
				if code != 0 || errout != "" {
					t.Fatalf("check: %d %s %s", code, out, errout)
				}
				r := decodeReport(t, out)
				expectedBytes, err := os.ReadFile(filepath.Join("../../testdata/sessions", name, "expected.json"))
				if err != nil {
					t.Fatal(err)
				}
				var expected struct {
					Claims []struct {
						Type, Verdict string
						QuoteContains string `json:"quote_contains"`
					}
					Findings []struct{ Code, Path string }
				}
				if err := json.Unmarshal(expectedBytes, &expected); err != nil {
					t.Fatal(err)
				}
				if len(r.Claims) != len(expected.Claims) {
					t.Fatalf("claims: %+v; expected %+v", r.Claims, expected.Claims)
				}
				for i, want := range expected.Claims {
					got := r.Claims[i]
					verdict := strings.ReplaceAll(strings.ToUpper(got.Verdict), "_", " ")
					if got.Type != want.Type || verdict != want.Verdict || !strings.Contains(got.Quote, want.QuoteContains) || !strings.Contains(s.Final, got.Quote) {
						t.Fatalf("claim: %+v; expected %+v", got, want)
					}
				}
				if len(r.Findings) != len(expected.Findings) {
					t.Fatalf("findings: %+v", r.Findings)
				}
				for i, want := range expected.Findings {
					if got := r.Findings[i]; got.Code != want.Code || got.Path != want.Path {
						t.Fatalf("finding: %+v; expected %+v", got, want)
					}
				}

				if r.Type != "gaugestone.report/v1" || r.Session == nil || r.Session.ID != s.ID || r.Repo.Baseline.Ref != "HEAD" || r.Repo.Baseline.Reason != "uncommitted changes only" || len(r.Commands) == 0 || len(r.Ledger) == 0 {
					t.Fatalf("report facts: %+v", r)
				}
				if r.Repo.Baseline.Commit != r.Repo.Head || len(r.Repo.Head) != 40 {
					t.Fatal("commit hash lost", r.Repo)
				}
				count := 0
				for group, files := range r.Groups {
					for _, f := range files {
						count++
						wantPath, wantGroup := "calc/stats.go", "source"
						if name == "claude-pytest" {
							wantPath = "pycalc/__init__.py"
						}
						if f.Path == "calc/stats_test.go" && name == "claude-skip" {
							wantPath = f.Path
							wantGroup = "tests"
						}
						if f.Path != wantPath || group != wantGroup || f.ChangedAfterSession {
							t.Fatalf("unexpected file %s %+v", group, f)
						}
					}
				}
				wantFiles := 1
				wantFindings := 0
				if name == "claude-skip" {
					wantFiles = 2
					wantFindings = 1
				}
				if count != wantFiles || len(r.Findings) != wantFindings {
					t.Fatalf("files=%d findings=%+v", count, r.Findings)
				}
				if wantFindings == 1 && (r.Findings[0].Code != "test_skipped" || r.Findings[0].Path != "calc/stats_test.go" || r.Findings[0].Line != 26) {
					t.Fatal(r.Findings)
				}
			}
			if _, e = os.Stat(filepath.Join(repo, ".git", "gaugestone")); !os.IsNotExist(e) {
				t.Fatal("no-write created report storage", e)
			}
			outDir := filepath.Join(base, "reports")
			code, out, errout := runBinary(t, binary, repo, "check", "--transcript", transcript, "--out", outDir, "--json")
			if code != 0 || errout != "" {
				t.Fatalf("write %d %s %s", code, out, errout)
			}
			saved, e := os.ReadFile(filepath.Join(outDir, "report.json"))
			if e != nil {
				t.Fatal(e)
			}
			decodeReport(t, string(saved))
			entries, e := os.ReadDir(outDir)
			if e != nil || len(entries) != 3 {
				t.Fatal("out is not direct", entries, e)
			}
			dryDir := filepath.Join(base, "preview")
			code, out, errout = runBinary(t, binary, repo, "check", "--transcript", transcript, "--out", dryDir, "--dry-run", "--json")
			if code != 0 || errout != "" || !strings.Contains(out, `"dry_run": true`) {
				t.Fatalf("dry run %d %s %s", code, out, errout)
			}
			if _, e = os.Stat(dryDir); !os.IsNotExist(e) {
				t.Fatal("dry run created directory", e)
			}
			code, out, errout = runBinary(t, binary, repo, "--transcript", transcript, "--no-write")
			if code != 0 || errout != "" || strings.Contains(out, "Report:") {
				t.Fatalf("text no-write %d %s %s", code, out, errout)
			}
		})
	}
}
func TestJSONErrorStreams(t *testing.T) {
	binary := buildBinary(t)
	_, repo, transcript := fixture(t, "claude-verified")
	for _, form := range [][]string{{"--json"}, {"--format", "json"}, {"--format=json"}} {
		for _, tc := range []struct {
			args []string
			exit int
		}{
			{[]string{"unknown-word"}, 1},
			{[]string{"check", "--unknown"}, 1},
			{[]string{"check", "extra"}, 1},
			{[]string{"check", "--format", "text"}, 1},
			{[]string{"check", "--format", "html", "--format", "markdown"}, 1},
			{[]string{"--format=markdown", "--format=text"}, 1},
			{[]string{"check", "--transcript", transcript, "--base", "missing-ref", "--no-write"}, 2},
			{[]string{"check", "--transcript", filepath.Join(t.TempDir(), "absent")}, 5},
			{[]string{"check", "--session", "absent"}, 5},
		} {
			args := append(append([]string{}, tc.args...), form...)
			code, out, errout := runBinary(t, binary, repo, args...)
			var body struct {
				Error string `json:"error"`
				Exit  int    `json:"exit_code"`
			}
			if code != tc.exit || out != "" || json.Unmarshal([]byte(errout), &body) != nil || body.Exit != tc.exit || body.Error == "" {
				t.Fatalf("%v: %d stdout=%q stderr=%q", args, code, out, errout)
			}
		}
	}
	// Root positional validation happens before repository or session selection.
	code, out, errout := runBinary(t, binary, t.TempDir(), "unknown-word")
	if code != 1 || out != "" || errout == "" {
		t.Fatalf("root word %d %s %s", code, out, errout)
	}
	for _, form := range [][]string{{"--json"}, {"--format", "json"}} {
		code, out, errout := runBinary(t, binary, t.TempDir(), form...)
		if code != 5 || out != "" || !json.Valid([]byte(errout)) {
			t.Fatalf("non-repo %d %s %s", code, out, errout)
		}
	}
}
func TestFullSessionReportPaths(t *testing.T) {
	binary := buildBinary(t)
	base, repo, first := fixture(t, "codex-stale")
	raw, e := os.ReadFile("../../testdata/sessions/codex-verified/session.jsonl")
	if e != nil {
		t.Fatal(e)
	}
	second := filepath.Join(base, "second.jsonl")
	if e = os.WriteFile(second, []byte(strings.ReplaceAll(string(raw), "/fixtures/codex-verified/repo", repo)), 0600); e != nil {
		t.Fatal(e)
	}
	ids := map[string]bool{}
	for _, transcript := range []string{first, second} {
		code, out, errout := runBinary(t, binary, repo, "--transcript", transcript, "--json")
		if code != 0 || errout != "" {
			t.Fatalf("%d %s %s", code, out, errout)
		}
		r := decodeReport(t, out)
		ids[r.Session.ID] = true
		saved, e := os.ReadFile(filepath.Join(repo, ".git", "gaugestone", "codex-"+r.Session.ID, "report.json"))
		if e != nil {
			t.Fatal(e)
		}
		if decodeReport(t, string(saved)).Session.ID != r.Session.ID {
			t.Fatal("report overwritten")
		}
	}
	if len(ids) != 2 {
		t.Fatal(ids)
	}
	entries, e := os.ReadDir(filepath.Join(repo, ".git", "gaugestone"))
	if e != nil || len(entries) != 2 {
		t.Fatal(entries, e)
	}
}

func TestUnstagedTestRename(t *testing.T) {
	binary := buildBinary(t)
	_, repo, transcript := fixture(t, "claude-verified")
	indexPath := filepath.Join(repo, ".git", "index")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(filepath.Join(repo, "calc", "stats_test.go"), filepath.Join(repo, "calc", "statistics_test.go")); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(repo, "calc", "statistics_test.go"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString("\nfunc TestMoved(t *testing.T) { t.Skip(\"moved\") }\n")
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	code, out, errout := runBinary(t, binary, repo, "check", "--transcript", transcript, "--no-write", "--json")
	if code != 0 || errout != "" {
		t.Fatalf("%d %s %s", code, out, errout)
	}
	r := decodeReport(t, out)
	if len(r.Findings) != 1 || r.Findings[0].Code != "test_skipped" || r.Findings[0].Path != "calc/statistics_test.go" {
		t.Fatal(r.Findings)
	}
	tests := r.Groups["tests"]
	if len(tests) != 1 || tests[0].Status != "R" || tests[0].OldPath != "calc/stats_test.go" || tests[0].Path != "calc/statistics_test.go" {
		t.Fatal(tests)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("real index changed", err)
	}
}

func TestConfiguredDiffFormatting(t *testing.T) {
	binary := buildBinary(t)
	for _, setting := range [][2]string{{"color.ui", "always"}, {"color.diff", "always"}, {"diff.suppressBlankEmpty", "true"}} {
		t.Run(setting[0], func(t *testing.T) {
			_, repo, transcript := fixture(t, "claude-skip")
			t.Setenv("GIT_CONFIG_COUNT", "1")
			t.Setenv("GIT_CONFIG_KEY_0", setting[0])
			t.Setenv("GIT_CONFIG_VALUE_0", setting[1])
			code, out, errout := runBinary(t, binary, repo, "--transcript", transcript, "--no-write", "--json")
			if code != 0 || errout != "" {
				t.Fatalf("%d %s %s", code, out, errout)
			}
			r := decodeReport(t, out)
			if len(r.Findings) != 1 || r.Findings[0].Code != "test_skipped" || r.Findings[0].Line != 26 {
				t.Fatal(r.Findings)
			}
		})
	}
}

func TestSelectedSessionCompleteness(t *testing.T) {
	binary := buildBinary(t)
	_, repo, transcript := fixture(t, "codex-verified")
	s, err := agents.Read(transcript)
	if err != nil {
		t.Fatal(err)
	}
	logs := filepath.Join(os.Getenv("CODEX_HOME"), "sessions")
	selected := filepath.Join(logs, "selected.jsonl")
	copyFile(t, transcript, selected)
	f, err := os.OpenFile(selected, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("{incomplete")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]any{"id": "old", "cwd": repo}})
		if err = os.WriteFile(filepath.Join(logs, fmt.Sprintf("old-%d.jsonl", i)), append(meta, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, form := range [][]string{{}, {"--json"}} {
		args := append([]string{"check", "--session", s.ID, "--no-write"}, form...)
		code, out, errout := runBinary(t, binary, repo, args...)
		if code != 0 || errout != "" {
			t.Fatalf("%d %s %s", code, out, errout)
		}
		if strings.Contains(out, "unsupported Codex log format") || strings.Contains(out, "update Codex") || strings.Contains(out, "old-0.jsonl") {
			t.Fatal("unrelated recording note in report", out)
		}
		if len(form) > 0 {
			r := decodeReport(t, out)
			if len(r.Completeness.Notes) != 1 || !strings.Contains(r.Completeness.Notes[0], "incomplete final line skipped") {
				t.Fatal(r.Completeness)
			}
		} else if strings.Count(strings.Join(strings.Fields(out), " "), "incomplete final line skipped") != 1 {
			t.Fatal("selected session note missing", out)
		}
	}
}
