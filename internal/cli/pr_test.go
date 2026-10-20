package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/rungrad/testutil"
)

type prFixture struct{ repo, transcript, ghDir, parent, target string }

func prSetup(t *testing.T) prFixture {
	t.Helper()
	base := isolated(t)
	f := prFixture{repo: filepath.Join(base, "repo"), transcript: filepath.Join(base, "session.jsonl"), ghDir: filepath.Join(base, "fake-gh")}
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
	os.MkdirAll(f.ghDir, 0700)
	t.Setenv("FAKE_GH_DIR", f.ghDir)
	t.Setenv("PATH", f.ghDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := `#!/bin/sh
printf '%s\n' "$@" >> "$FAKE_GH_DIR/args"
case "$1 $2" in
 'pr view')
  if [ -f "$FAKE_GH_DIR/auth-required" ]; then exit 4; fi
  if [ -f "$FAKE_GH_DIR/view-fail" ]; then cat "$FAKE_GH_DIR/view-fail" >&2; exit 1; fi
  cat "$FAKE_GH_DIR/pr.json" ;;
 'auth status') [ ! -f "$FAKE_GH_DIR/auth-fail" ] ;;
 'pr edit')
  if [ -f "$FAKE_GH_DIR/edit-fail" ]; then exit 1; fi
  [ "$4" = '--body-file' ] || exit 9
  cp "$5" "$FAKE_GH_DIR/posted"
  printf '%s' "$5" > "$FAKE_GH_DIR/body-path" ;;
 *) exit 8 ;;
esac
`
	if err := os.WriteFile(filepath.Join(f.ghDir, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	gitPR(t, f.repo, "init", "-q", f.repo)
	os.WriteFile(filepath.Join(f.repo, "initial.txt"), []byte("initial\n"), 0600)
	gitPR(t, f.repo, "add", ".")
	gitPR(t, f.repo, "commit", "-qm", "Initial")
	f.parent = gitPR(t, f.repo, "rev-parse", "HEAD")
	gitPR(t, f.repo, "checkout", "-qb", "release")
	os.WriteFile(filepath.Join(f.repo, "target.txt"), []byte("target\n"), 0600)
	gitPR(t, f.repo, "add", ".")
	gitPR(t, f.repo, "commit", "-qm", "Target change")
	f.target = gitPR(t, f.repo, "rev-parse", "HEAD")
	gitPR(t, f.repo, "checkout", "-qb", "feature", f.parent)
	os.WriteFile(filepath.Join(f.repo, "feature.txt"), []byte("feature\n"), 0600)
	gitPR(t, f.repo, "add", ".")
	gitPR(t, f.repo, "commit", "-qm", "Feature change")
	gitPR(t, f.repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	log := []map[string]any{
		{"type": "user", "sessionId": "pr-session", "cwd": f.repo, "timestamp": "2026-10-20T12:00:00Z", "message": map[string]any{"content": "Review changes"}},
		{"type": "assistant", "cwd": f.repo, "timestamp": "2026-10-20T12:01:00Z", "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": "cmd", "name": "Bash", "input": map[string]any{"command": "go test ./... # | `untrusted` <!-- gaugestone:start --> TOKEN_KEY=private " + strings.Repeat("界", 1000) + "\ntouch " + filepath.Join(f.repo, "executed")}}}}},
		{"type": "user", "cwd": f.repo, "timestamp": "2026-10-20T12:02:00Z", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "cmd", "content": "ok example 0.1s"}}}},
	}
	var data bytes.Buffer
	for _, row := range log {
		json.NewEncoder(&data).Encode(row)
	}
	os.WriteFile(f.transcript, data.Bytes(), 0600)
	f.body(t, "Description\r\n界 | `outside`\n", f.target)
	return f
}
func gitPR(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if _, err := os.Stat(repo); err == nil {
		cmd.Dir = repo
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, b, err)
	}
	return strings.TrimSpace(string(b))
}
func (f prFixture) body(t *testing.T, body, oid string) {
	t.Helper()
	b, _ := json.Marshal(pullRequest{Number: 42, Body: body, URL: "https://example.invalid/pull/42", BaseRefName: "release", BaseRefOID: oid})
	if err := os.WriteFile(filepath.Join(f.ghDir, "pr.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}
func (f prFixture) read(name string) string {
	b, _ := os.ReadFile(filepath.Join(f.ghDir, name))
	return string(b)
}
func (f prFixture) run(args ...string) testutil.Result {
	return testutil.Run(New("test", Options{Dir: f.repo, Now: func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }}), append([]string{"pr", "--transcript", f.transcript}, args...)...)
}
func assertNoReports(t *testing.T, f prFixture) {
	t.Helper()
	for _, p := range []string{filepath.Join(f.repo, ".git", "gaugestone"), filepath.Join(f.repo, "executed"), filepath.Join(f.repo, "report.html")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("unexpected output %s: %v", p, err)
		}
	}
	if path := f.read("body-path"); path != "" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("body temp file remains")
		}
	}
}
func decodePR(t *testing.T, stdout string) prResult {
	t.Helper()
	var result prResult
	d := json.NewDecoder(strings.NewReader(stdout))
	if err := d.Decode(&result); err != nil {
		t.Fatalf("JSON: %v: %s", err, stdout)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		t.Fatalf("extra stdout: %v", err)
	}
	return result
}
func TestPRPublicationAndPreview(t *testing.T) {
	f := prSetup(t)
	outside := "Description\r\n界 | `outside`\n"
	dry := f.run("--dry-run", "--confirm", "--json")
	if dry.Exit != 0 {
		t.Fatalf("%+v", dry)
	}
	preview := decodePR(t, dry.Stdout)
	if preview.Status != "previewed" || f.read("posted") != "" {
		t.Fatal("dry run wrote")
	}
	if !strings.Contains(preview.Section, "feature.txt") || strings.Contains(preview.Section, "target.txt") {
		t.Fatal("did not use target merge base", preview.Section)
	}
	for _, s := range []string{"Run `gaugestone check` locally", "&#124;", "&#96;", "&lt;!-- gaugestone:start --&gt;", "&#91;redacted&#93;"} {
		if !strings.Contains(preview.Section, s) {
			t.Fatalf("missing %q: %s", s, preview.Section)
		}
	}
	human := f.run("--confirm")
	if human.Exit != 0 {
		t.Fatalf("%+v", human)
	}
	posted := f.read("posted")
	if posted != outside+"\n\n"+preview.Section || !strings.HasPrefix(human.Stdout, preview.Section+"\n") {
		t.Fatal("preview differs from publication")
	}
	// A second publication replaces exactly one section, preserving both sides.
	f.body(t, posted+"\r\nTrailing bytes 界\n", f.target)
	updated := f.run("--json", "--confirm")
	result := decodePR(t, updated.Stdout)
	if updated.Exit != 0 || result.Status != "updated" || result.Section != preview.Section || f.read("posted") != posted+"\r\nTrailing bytes 界\n" {
		t.Fatalf("replacement: %+v", updated)
	}
	if strings.Count(f.read("posted"), sectionStart) != 1 {
		t.Fatal("duplicate section")
	}
	explicit := f.run("--dry-run", "--json", "--base", f.target)
	if explicit.Exit != 0 || !strings.Contains(decodePR(t, explicit.Stdout).Section, "target.txt") {
		t.Fatalf("direct base ignored: %+v", explicit)
	}
	assertNoReports(t, f)
}
func TestPRRefusalsAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body, flag, marker, wantText string
		code                               int
	}{
		{name: "no prompt", flag: "--no-prompt", code: 1, wantText: "--confirm"},
		{name: "JSON requires confirmation", flag: "--json", code: 1, wantText: "--confirm"},
		{name: "unbalanced", body: sectionStart, code: 2, wantText: "markers"},
		{name: "reversed", body: sectionEnd + sectionStart, code: 2, wantText: "markers"},
		{name: "duplicate", body: sectionStart + sectionEnd + sectionStart + sectionEnd, code: 2, wantText: "markers"},
		{name: "malformed", body: "<!-- gaugestone:start --> x <!-- gaugestone:end-->", code: 2, wantText: "markers"},
		{name: "unterminated", body: "<!-- gaugestone:start", code: 2, wantText: "markers"},
		{name: "authentication exit code", marker: "auth-required", code: 3, wantText: "gh auth login"},
		{name: "runtime failure", marker: "runtime-fail", code: 2, wantText: "Could not read"},
		{name: "authentication", marker: "auth-fail", code: 3, wantText: "gh auth login"},
		{name: "no PR", marker: "view-fail", code: 5, wantText: "gh pr create"},
		{name: "edit failure", marker: "edit-fail", code: 2, wantText: "Could not update"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := prSetup(t)
			f.body(t, tc.body, f.target)
			if tc.marker != "" {
				os.WriteFile(filepath.Join(f.ghDir, tc.marker), []byte("no pull requests found for branch"), 0600)
			}
			if tc.marker == "runtime-fail" {
				os.WriteFile(filepath.Join(f.ghDir, "view-fail"), []byte("network unavailable"), 0600)
			}
			if tc.marker == "auth-fail" {
				os.WriteFile(filepath.Join(f.ghDir, "view-fail"), []byte("login required"), 0600)
			}
			args := []string{"--json"}
			if tc.flag != "--json" {
				args = append(args, "--confirm")
			}
			if tc.flag != "" {
				args = []string{tc.flag}
			}
			result := f.run(args...)
			if result.Exit != tc.code || !strings.Contains(result.Stderr, tc.wantText) {
				t.Fatalf("%+v", result)
			}
			if strings.Contains(strings.Join(args, " "), "--json") {
				if result.Stdout != "" {
					t.Fatal("partial JSON success", result.Stdout)
				}
				var failure struct {
					Error    string `json:"error"`
					ExitCode int    `json:"exit_code"`
				}
				if err := json.Unmarshal([]byte(result.Stderr), &failure); err != nil || failure.Error == "" || failure.ExitCode != tc.code {
					t.Fatalf("invalid JSON error: %+v", result)
				}
			}
			if tc.marker != "edit-fail" && strings.Contains(f.read("args"), "\nedit\n") {
				t.Fatal("attempted an edit on refusal")
			}
			if f.read("posted") != "" {
				t.Fatal("published on failure")
			}
			assertNoReports(t, f)
		})
	}
}
func TestPRMissingGHAndBase(t *testing.T) {
	f := prSetup(t)
	f.body(t, "", strings.Repeat("f", 40))
	result := f.run("--dry-run", "--json")
	if result.Exit != 2 || !strings.Contains(result.Stderr, "target repository's base branch") {
		t.Fatalf("%+v", result)
	}
	result = f.run("--dry-run", "--json", "--base", "HEAD")
	if result.Exit != 0 {
		t.Fatalf("override should skip unavailable target: %+v", result)
	}
	result = f.run("--session", "another", "--json")
	if result.Exit != 1 {
		t.Fatalf("selection conflict: %+v", result)
	}
	git, _ := exec.LookPath("git")
	onlyGit := t.TempDir()
	os.Symlink(git, filepath.Join(onlyGit, "git"))
	t.Setenv("PATH", onlyGit)
	result = f.run("--json", "--confirm")
	if result.Exit != 3 || !strings.Contains(result.Stderr, "Install gh") {
		t.Fatalf("%+v", result)
	}
	assertNoReports(t, f)
}
func TestPRLongDescriptions(t *testing.T) {
	f := prSetup(t)
	for _, size := range []int{64000, 65500} {
		outside := strings.Repeat("界", size/3) + strings.Repeat("x", size%3)
		f.body(t, outside, f.target)
		os.WriteFile(filepath.Join(f.ghDir, "args"), nil, 0600)
		result := f.run("--dry-run", "--json")
		if strings.Contains(f.read("args"), "\nedit\n") {
			t.Fatal("dry run attempted an edit")
		}
		if size == 65500 {
			if result.Exit != 2 || result.Stdout != "" || !strings.Contains(result.Stderr, "shortest Markdown summary") {
				t.Fatalf("%+v", result)
			}
			continue
		}
		if result.Exit != 0 {
			t.Fatalf("%+v", result)
		}
		section := decodePR(t, result.Stdout).Section
		if len(outside)+2+len(section) > prBodyLimit || !strings.Contains(section, "omitted") || strings.Contains(section, "HTML/JSON contain") {
			t.Fatal("incorrect shortened section", section)
		}
		posted := f.run("--json", "--confirm")
		if posted.Exit != 0 || f.read("posted") != outside+"\n\n"+section {
			t.Fatalf("long publication: %+v", posted)
		}
	}
	assertNoReports(t, f)
}
func TestPRBuiltBinary(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "gaugestone")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/gaugestone")
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", b, err)
	}
	f := prSetup(t)
	for _, args := range [][]string{{"--dry-run", "--json"}, {"--confirm", "--json"}} {
		cmd := exec.Command(binary, append([]string{"pr", "--transcript", f.transcript}, args...)...)
		cmd.Dir = f.repo
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("binary: %s %v", stderr.String(), err)
		}
		result := decodePR(t, string(b))
		if result.Section == "" {
			t.Fatal("missing preview")
		}
		if result.Status == "updated" && !strings.HasSuffix(f.read("posted"), result.Section) {
			t.Fatal("binary posted different section")
		}
	}
	assertNoReports(t, f)
}

func TestPRInteractiveConfirmation(t *testing.T) {
	for _, answer := range []string{"yes\n", "no\n"} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			f := prSetup(t)
			app := New("test", Options{Dir: f.repo})
			result := testutil.RunWith(app, testutil.Options{Stdin: strings.NewReader(answer), Terminal: true, TerminalSet: true}, "pr", "--transcript", f.transcript)
			end := strings.Index(result.Stdout, sectionEnd)
			if end < 0 || !strings.HasPrefix(result.Stdout, sectionStart) || !strings.Contains(result.Stderr, "About to update") {
				t.Fatalf("preview/prompt: %+v", result)
			}
			section := result.Stdout[:end+len(sectionEnd)]
			if answer == "yes\n" {
				if result.Exit != 0 || !strings.HasSuffix(f.read("posted"), section) {
					t.Fatalf("interactive edit: %+v", result)
				}
			} else if result.Exit != 1 || f.read("posted") != "" {
				t.Fatalf("decline: %+v", result)
			}
			assertNoReports(t, f)
		})
	}
}

func TestPRSessionSelection(t *testing.T) {
	f := prSetup(t)
	dir := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "projects", "test")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(f.transcript)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "session.jsonl"), log, 0600); err != nil {
		t.Fatal(err)
	}
	result := testutil.Run(New("test", Options{Dir: f.repo}), "pr", "--session", "claude:pr-session", "--dry-run", "--json")
	if result.Exit != 0 || !strings.Contains(decodePR(t, result.Stdout).Section, "pr-session") {
		t.Fatalf("session: %+v", result)
	}
	assertNoReports(t, f)
}

func TestPRForkTargetCommit(t *testing.T) {
	f := prSetup(t)
	var pr pullRequest
	if err := json.Unmarshal([]byte(f.read("pr.json")), &pr); err != nil {
		t.Fatal(err)
	}
	pr.BaseRefName = "main"
	body, err := json.Marshal(pr)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(f.ghDir, "pr.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	// origin/main already contains the feature commit; using it would hide the work.
	result := f.run("--dry-run", "--json")
	if result.Exit != 0 {
		t.Fatalf("%+v", result)
	}
	section := decodePR(t, result.Stdout).Section
	if !strings.Contains(section, "feature.txt") || strings.Contains(section, "target.txt") || !strings.Contains(section, "compared with ` main `") {
		t.Fatal("wrong fork comparison", section)
	}
	assertNoReports(t, f)
}
