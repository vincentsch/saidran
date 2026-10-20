package agents_test

import (
	"encoding/json"
	"github.com/vincentsch/assaylight/internal/findings"
	"github.com/vincentsch/assaylight/internal/gitstate"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/assaylight/internal/report"
)

func TestFailurePolicy(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct {
		fixture, policy string
		want            int
	}{
		{"claude-contradicted", "contradicted", 10}, {"claude-verified", "contradicted", 0},
		{"claude-stale", " stale, test_skipped ", 10}, {"claude-skip", "test_skipped", 10},
		{"claude-verified", "verified", 10},
	} {
		t.Run(tc.fixture+tc.policy, func(t *testing.T) {
			base, repo, transcript := fixture(t, tc.fixture)
			for _, form := range [][]string{{"--json"}, {"--format", "json"}, {"--format", "markdown"}, {"--format", "html"}, {"--format", "text"}} {
				for _, noWrite := range []bool{false, true} {
					outDir := filepath.Join(base, "reports")
					args := append([]string{"--transcript", transcript, "--out", outDir, "--fail-on", tc.policy}, form...)
					if noWrite {
						args = append(args, "--no-write")
					}
					code, out, stderr := runBinary(t, binary, repo, args...)
					if code != tc.want || out == "" {
						t.Fatalf("%v: %d %s", args, code, stderr)
					}
					if form[0] == "--json" || form[len(form)-1] == "json" {
						decodeReport(t, out)
					}
					if code == 10 && (strings.Count(stderr, "\n") != 1 || !strings.HasPrefix(stderr, "Failure policy matched:")) {
						t.Fatal(stderr)
					}
					if code == 0 && stderr != "" {
						t.Fatal(stderr)
					}
					for _, name := range []string{"report.html", "report.md", "report.json"} {
						if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			blocked := filepath.Join(base, "blocked")
			if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
				t.Fatal(err)
			}
			code, out, stderr := runBinary(t, binary, repo, "--transcript", transcript, "--out", blocked, "--fail-on", tc.policy, "--json")
			if code != 2 || out != "" || !json.Valid([]byte(stderr)) {
				t.Fatalf("write failure: %d %s %s", code, out, stderr)
			}
		})
	}
	_, repo, transcript := fixture(t, "claude-stale")
	for _, policy := range []string{"", ",", "stale,", "unknown", "stale,,verified"} {
		code, out, stderr := runBinary(t, binary, repo, "--transcript", transcript, "--no-write", "--fail-on", policy, "--json")
		if code != 1 || out != "" || !json.Valid([]byte(stderr)) {
			t.Fatalf("invalid policy: %d %s %s", code, out, stderr)
		}
	}
	code, out, stderr := runBinary(t, binary, repo, "--transcript", transcript, "--no-write", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("default: %d %s", code, stderr)
	}
	decodeReport(t, out)
	if _, err := os.Stat(filepath.Join(repo, ".git", "assaylight")); !os.IsNotExist(err) {
		t.Fatal("no-write wrote reports")
	}
}

func TestDismissalLifecycle(t *testing.T) {
	binary := buildBinary(t)
	_, repo, transcript := fixture(t, "claude-skip")
	check := func(want int, extra ...string) report.Report {
		t.Helper()
		args := append([]string{"check", "--transcript", transcript, "--json", "--fail-on", "test_skipped"}, extra...)
		code, out, stderr := runBinary(t, binary, repo, args...)
		if code != want {
			t.Fatalf("check: %d %s", code, stderr)
		}
		return decodeReport(t, out)
	}
	initial := check(10, "--no-write")
	id := initial.Findings[0].ID
	storePath := filepath.Join(repo, ".git", "assaylight", "dismissals.json")
	for _, args := range [][]string{
		{"dismiss", id[:8], "--reason", "test", "--dry-run"},
		{"dismiss", id[:8], "--reason", "test", "--dry-run", "--json"},
	} {
		code, out, stderr := runBinary(t, binary, repo, args...)
		if code != 0 || stderr != "" || !strings.Contains(out, "would dismiss") {
			t.Fatalf("preview: %d %s %s", code, out, stderr)
		}
		for _, forbidden := range []string{"created", "updated", "deleted", "removed", "successfully"} {
			if strings.Contains(out, forbidden) {
				t.Fatal(out)
			}
		}
	}
	if _, err := os.Stat(filepath.Dir(storePath)); !os.IsNotExist(err) {
		t.Fatal("preview wrote files")
	}
	for _, tc := range []struct {
		id   string
		want int
	}{{"F-ffffff", 5}, {"verified", 1}, {"F-123", 1}} {
		code, _, _ := runBinary(t, binary, repo, "dismiss", tc.id, "--reason", "test")
		if code != tc.want {
			t.Fatalf("bad ID: %d", code)
		}
	}
	reason := "Intentional API_TOKEN=private-value"
	code, _, stderr := runBinary(t, binary, repo, "dismiss", id[:8], "--reason", reason, "--base", "HEAD")
	if code != 0 {
		t.Fatal(stderr)
	}
	raw, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-value") {
		t.Fatal("reason leaked")
	}
	var store report.DismissalStore
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatal(err)
	}
	if store.Version != 1 || len(store.Entries) != 1 || store.Entries[0].Context == "" || store.Entries[0].Time.IsZero() {
		t.Fatal("missing stored facts")
	}
	dismissed := check(0)
	if len(dismissed.Findings) != len(initial.Findings) || len(dismissed.Dismissals) != 1 {
		t.Fatal("dismissed finding lost")
	}
	for _, format := range []string{"text", "markdown", "html"} {
		code, out, stderr := runBinary(t, binary, repo, "--transcript", transcript, "--format", format, "--no-write")
		if code != 0 || !strings.Contains(out, "Dismissed:") || strings.Contains(out, "private-value") {
			t.Fatalf("%s: %d %s", format, code, stderr)
		}
	}
	code, out, stderr := runBinary(t, binary, repo, "dismissals", "--json")
	if code != 0 || !strings.Contains(out, id) {
		t.Fatalf("list: %d %s", code, stderr)
	}
	for _, command := range [][]string{{"dismissals", "--remove", id[:8], "--dry-run"}, {"dismiss", id, "--reason", "replacement", "--dry-run"}} {
		code, _, stderr := runBinary(t, binary, repo, command...)
		if code != 0 {
			t.Fatal(stderr)
		}
		after, _ := os.ReadFile(storePath)
		if string(after) != string(raw) {
			t.Fatal("preview changed dismissals")
		}
	}
	// Editing the skipped line changes its diff context and requires another dismissal.
	path := filepath.Join(repo, initial.Findings[0].Path)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(original), "t.Skip(", "t.Skip( /* changed context */", 1)
	if changed == string(original) {
		t.Fatal("fixture skip not found")
	}
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	current := check(10, "--no-write")
	if len(current.Dismissals) != 0 || current.Findings[0].ID == id {
		t.Fatal("changed context kept dismissal")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runBinary(t, binary, repo, "dismissals", "--remove", id[:8])
	if code != 0 {
		t.Fatal(stderr)
	}
	if len(check(10, "--no-write").Dismissals) != 0 {
		t.Fatal("dismissal not removed")
	}
}

func TestDemoFromEmptyHome(t *testing.T) {
	binary := buildBinary(t)
	base, _, _ := fixture(t, "claude-stale")
	t.Setenv("TMPDIR", base)
	code, out, stderr := runBinary(t, binary, base, "demo")
	if code != 0 || stderr != "" || !strings.HasPrefix(out, "Demo: a recorded Claude Code session") || !strings.Contains(out, "Stale") {
		t.Fatalf("demo: %d %s %s", code, out, stderr)
	}
	dirs, err := filepath.Glob(filepath.Join(base, "assaylight-demo-*"))
	if err != nil || len(dirs) != 1 {
		t.Fatal(dirs, err)
	}
	repo := filepath.Join(dirs[0], "repo")
	reports, err := filepath.Glob(filepath.Join(repo, ".git", "assaylight", "*", "report.json"))
	if err != nil || len(reports) != 1 {
		t.Fatal(reports, err)
	}
	raw, err := os.ReadFile(reports[0])
	if err != nil {
		t.Fatal(err)
	}
	model := decodeReport(t, string(raw))
	for _, group := range model.Groups {
		for _, file := range group {
			if strings.Contains(file.Path, "session.jsonl") || file.ChangedAfterSession {
				t.Fatalf("demo changes: %+v", file)
			}
		}
	}
	for _, name := range []string{"report.html", "report.md"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(reports[0]), name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dirs[0], "session.jsonl")); err != nil {
		t.Fatal(err)
	}
	code, out, stderr = runBinary(t, binary, base, "demo", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("json demo: %d %s", code, stderr)
	}
	decodeReport(t, out)
	before, _ := filepath.Glob(filepath.Join(base, "assaylight-demo-*"))
	code, _, stderr = runBinary(t, binary, base, "demo", "--dry-run")
	if code != 0 {
		t.Fatal(stderr)
	}
	after, _ := filepath.Glob(filepath.Join(base, "assaylight-demo-*"))
	if len(before) != len(after) {
		t.Fatal("dry-run extracted demo")
	}
}

func TestDismissalWorktreeStorage(t *testing.T) {
	binary := buildBinary(t)
	base, repo, _ := fixture(t, "claude-skip")
	worktree := filepath.Join(base, "worktree")
	if b, err := exec.Command("git", "-C", repo, "worktree", "add", "--detach", worktree, "HEAD").CombinedOutput(); err != nil {
		t.Fatalf("worktree: %s %v", b, err)
	}
	raw, err := os.ReadFile(filepath.Join(repo, "calc", "stats_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "calc", "stats_test.go"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	// The original transcript belongs to the main worktree; dismiss needs no transcript.
	code, out, stderr := runBinary(t, binary, worktree, "dismissals", "--json")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatal(stderr)
	}
	// Obtain the current finding directly from Git inspection in another test helper.
	cmd := exec.Command("git", "-C", worktree, "rev-parse", "--absolute-git-dir")
	dir, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	state, err := inspectFindings(worktree)
	if err != nil || len(state) == 0 {
		t.Fatal(state, err)
	}
	code, _, stderr = runBinary(t, binary, worktree, "dismiss", state[0], "--reason", "worktree only")
	if code != 0 {
		t.Fatal(stderr)
	}
	if _, err := os.Stat(filepath.Join(strings.TrimSpace(string(dir)), "assaylight", "dismissals.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "assaylight")); !os.IsNotExist(err) {
		t.Fatal("wrote into main worktree")
	}
}

func inspectFindings(root string) ([]string, error) {
	state, err := gitstate.Inspect(root, "", time.Time{})
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, f := range findings.Detect(state.Files) {
		ids = append(ids, f.ID)
	}
	return ids, nil
}

func TestStoredDismissalResolutionFailuresDoNotWrite(t *testing.T) {
	binary := buildBinary(t)
	_, repo, _ := fixture(t, "claude-skip")
	path := filepath.Join(repo, ".git", "assaylight", "dismissals.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	store := report.DismissalStore{Version: 1, Entries: []report.Dismissal{{ID: "F-123456abcdef", Reason: "one"}, {ID: "F-123456789abc", Reason: "two"}}}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ref  string
		want int
	}{{"F-123456", 1}, {"F-ffffff", 5}} {
		code, out, stderr := runBinary(t, binary, repo, "dismissals", "--remove", tc.ref, "--json")
		if code != tc.want || out != "" || !json.Valid([]byte(stderr)) {
			t.Fatalf("resolution: %d %s %s", code, out, stderr)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(raw) != string(after) {
			t.Fatal("failed resolution wrote dismissals", err)
		}
	}
}
