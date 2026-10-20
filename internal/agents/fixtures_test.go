package agents_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/gaugestone/internal/agents"
	"github.com/vincentsch/gaugestone/internal/cli"
	"github.com/vincentsch/gaugestone/internal/session"
	"github.com/vincentsch/rungrad/testutil"
)

// Expectations are read from the tool calls and results, independently of expected.json's verdict labels.
func TestRecordedSessions(t *testing.T) {
	cases := []struct {
		name, agent     string
		commands, edits int
		exits           []int
		final           string
		start, end      string
	}{
		{"claude-contradicted", "claude-code", 2, 1, []int{0, 1}, "All tests pass now.", "15:48:36.127", "15:48:50.745"},
		{"claude-honest-failure", "claude-code", 2, 1, []int{0, 1}, "but not all tests pass", "15:48:36.127", "15:48:50.745"},
		{"claude-pytest", "claude-code", 2, 1, []int{0, 0}, "Yes, all 3 tests pass now.", "15:48:36.221", "15:48:49.457"},
		{"claude-skip", "claude-code", 2, 2, []int{0, 0}, "three tests pass and", "15:50:38.357", "15:50:56.719"},
		{"claude-stale", "claude-code", 2, 2, []int{0, 0}, "did not run the tests again", "15:50:35.506", "15:50:53.515"},
		{"claude-verified", "claude-code", 4, 0, []int{0, 0, 0, 0}, "Yes, all tests pass now.", "15:48:36.095", "15:48:51.376"},
		{"codex-stale", "codex", 3, 2, []int{0, 0, 0}, "Tests were not rerun after the rename", "15:50:35.468", "15:50:56.361"},
		{"codex-verified", "codex", 5, 1, []int{0, 0, 0, 1, 0}, "**all tests pass**", "15:50:35.484", "15:50:57.022"},
	}
	binary := filepath.Join(t.TempDir(), "gaugestone")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/gaugestone")
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %s: %v", b, e)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, repo, transcript := fixture(t, tc.name)
			s, err := agents.Read(transcript)
			if err != nil {
				t.Fatal(err)
			}
			if s.Agent != tc.agent || s.Cwd != repo || s.Unsupported != 0 {
				t.Fatalf("metadata: %s %s unsupported=%d", s.Agent, s.Cwd, s.Unsupported)
			}
			metaBytes, err := os.ReadFile(filepath.Join("../../testdata/sessions", tc.name, "meta.json"))
			if err != nil {
				t.Fatal(err)
			}
			var meta struct {
				Version string `json:"agent_version"`
				Model   string `json:"model"`
				Prompt  string `json:"prompt"`
			}
			if err = json.Unmarshal(metaBytes, &meta); err != nil {
				t.Fatal(err)
			}
			if s.AgentVersion != meta.Version || len(s.Models) != 1 || s.Models[0] != meta.Model || s.Branch != "main" {
				t.Fatal("version, model or branch lost")
			}
			if len(s.Prompts) != 1 || s.Prompts[0].Text != meta.Prompt {
				t.Fatal("recorded prompt differs from metadata")
			}
			var commands []*session.Command
			edits := 0
			for i, e := range s.Events {
				if e.Seq != i+1 || e.Source.Line < 1 || e.Source.File != transcript {
					t.Fatal("event references lost")
				}
				if i > 0 && e.Time.Before(s.Events[i-1].Time) {
					t.Fatal("event order")
				}
				if e.Command != nil {
					commands = append(commands, e.Command)
					if !agents.Within(repo, e.Command.Cwd) {
						t.Errorf("command cwd %s", e.Command.Cwd)
					}
					if e.Command.End.Before(e.Command.Start) {
						t.Fatal("duration")
					}
				}
				if e.Edit != nil {
					edits++
					if !agents.Within(repo, e.Edit.Path) {
						t.Fatal("edit path")
					}
				}
			}
			if len(commands) != tc.commands || edits != tc.edits {
				t.Fatalf("commands=%d edits=%d", len(commands), edits)
			}
			for i, c := range commands {
				if c.ExitCode == nil || *c.ExitCode != tc.exits[i] || !c.ExitReliable {
					t.Errorf("command %d exit=%v", i, c.ExitCode)
				}
			}
			if !strings.Contains(s.Final, tc.final) || s.Final == "" || len(s.Prompts) != 1 {
				t.Fatalf("final or prompt missing: %q", s.Final)
			}
			if tc.start != "" {
				if s.Started != session.Time("2026-10-20T"+tc.start+"Z") || s.Ended != session.Time("2026-10-20T"+tc.end+"Z") {
					t.Errorf("session times: %s %s", s.Started, s.Ended)
				}
			}
			if tc.agent == "codex" {
				for _, c := range commands {
					if !c.Shell || len(c.Argv) != 3 || c.End.Equal(c.Start) {
						t.Fatal("argv or duration lost")
					}
				}
			}
			if tc.name == "claude-verified" {
				if commands[1].Cwd != repo || commands[2].Cwd != filepath.Join(repo, "calc") {
					t.Fatal("command cwd inherited from another entry")
				}
			}
			// Place the transcript beside the extracted repository, then copy it to isolated discovery roots.
			var discovered string
			if tc.agent == "codex" {
				discovered = filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "20", "rollout-fixture.jsonl")
			} else {
				encoded := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(repo, "-")
				discovered = filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "projects", encoded, s.ID+".jsonl")
			}
			copyFile(t, transcript, discovered)
			d, err := agents.Discover(repo)
			if err != nil || len(d.Sessions) != 1 {
				t.Fatalf("discovery: %v %+v", err, d)
			}
			app := cli.New("test", cli.Options{Dir: repo, Now: func() time.Time { return s.Ended.Add(time.Hour) }})
			result := testutil.Run(app, "sessions", "--json")
			if result.Exit != 0 {
				t.Fatalf("listing: %+v", result)
			}
			var listing cli.Listing
			if err := json.Unmarshal([]byte(result.Stdout), &listing); err != nil {
				t.Fatal(err)
			}
			if len(listing.Sessions) != 1 || !listing.Sessions[0].AutomaticPick || listing.Sessions[0].Commands != tc.commands {
				t.Fatalf("listing: %+v", listing)
			}
			cmd := exec.Command(binary, "sessions", "--json")
			cmd.Dir = repo
			b, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("binary: %s %v", b, e)
			}
			if !strings.Contains(string(b), s.ID[:8]) {
				t.Fatal("binary omitted fixture")
			}
			if _, e := os.Stat(filepath.Join(base, "repo", "session.jsonl")); !os.IsNotExist(e) {
				t.Fatal("transcript inside repo")
			}
			for _, selection := range [][]string{{"--session", s.ID[:8]}} {
				args := append([]string{"check", "--no-write", "--json"}, selection...)
				code, out, errout := runBinary(t, binary, repo, args...)
				if code != 0 || errout != "" {
					t.Fatalf("session selection: %d %s %s", code, out, errout)
				}
				if decodeReport(t, out).Session.ID != s.ID {
					t.Fatal("selected another session")
				}
			}
			automatic := testutil.Run(cli.New("test", cli.Options{Dir: repo, Now: func() time.Time { return s.Ended.Add(time.Hour) }}), "check", "--no-write", "--json")
			if automatic.Exit != 0 || automatic.Stderr != "" || decodeReport(t, automatic.Stdout).Session.ID != s.ID {
				t.Fatalf("automatic report: %+v", automatic)
			}
		})
	}
}
func fixture(t *testing.T, name string) (string, string, string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(base, "claude"))
	t.Setenv("CODEX_HOME", filepath.Join(base, "codex"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "xdg"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(base, "pi"))
	repo := filepath.Join(base, "repo")
	os.MkdirAll(repo, 0700)
	f, err := os.Open(filepath.Join("../../testdata/sessions", name, "repo.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(base, h.Name)
		if !agents.Within(repo, path) {
			t.Fatal("archive path")
		}
		switch h.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(path), 0755)
			out, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, os.FileMode(h.Mode))
			if e != nil {
				t.Fatal(e)
			}
			_, e = io.Copy(out, tr)
			out.Close()
			if e != nil {
				t.Fatal(e)
			}
			os.Chtimes(path, h.ModTime, h.ModTime)
		}
	}
	source := filepath.Join("../../testdata/sessions", name)
	metaBytes, e := os.ReadFile(filepath.Join(source, "meta.json"))
	if e != nil {
		t.Fatal(e)
	}
	var meta struct {
		Transcript string `json:"transcript"`
	}
	if e = json.Unmarshal(metaBytes, &meta); e != nil {
		t.Fatal(e)
	}
	if meta.Transcript == "" {
		meta.Transcript = "session.jsonl"
	}
	transcript := filepath.Join(base, filepath.FromSlash(meta.Transcript))
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		if rel != meta.Transcript && !strings.HasPrefix(rel, "storage"+string(filepath.Separator)) {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		replacement, e := json.Marshal(repo)
		if e != nil {
			return e
		}
		b = []byte(strings.ReplaceAll(string(b), "/fixtures/"+name+"/repo", string(replacement[1:len(replacement)-1])))
		dest := filepath.Join(base, rel)
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		return os.WriteFile(dest, b, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return base, repo, transcript
}
func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, e := os.ReadFile(from)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(to), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(to, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestSelection(t *testing.T) {
	now := time.Date(2026, 10, 20, 17, 0, 0, 0, time.UTC)
	old := &session.Session{Agent: "codex", ID: "01a15a01-old", Ended: now.Add(-8 * 24 * time.Hour), Events: []session.Event{{Command: &session.Command{}}}}
	newer := &session.Session{Agent: "codex", ID: "01a15a01-new", Ended: now, Events: old.Events}
	other := &session.Session{Agent: "claude-code", ID: "other-agent", Ended: now.Add(-3 * time.Second), Events: old.Events}
	readOnly := &session.Session{Agent: "codex", ID: "read-only", Ended: now.Add(time.Hour)}
	all := []*session.Session{old, newer, other, readOnly}
	if got := agents.Automatic(all, now); !reflect.DeepEqual(got, []*session.Session{newer, other}) {
		t.Fatalf("automatic %+v", got)
	}
	for _, ref := range []string{old.ID, "codex:" + old.ID} {
		pick, e := agents.Select(all, ref, "", now, nil)
		if e != nil || pick.Session != old {
			t.Fatal("old explicit selection", e)
		}
	}
	if _, e := agents.Select(all, "01a15a01", "", now, nil); e == nil {
		t.Fatal("ambiguous prefix accepted")
	}
	if agents.DisplayID(old, all) == "01a15a01" {
		t.Fatal("prefix not extended")
	}
	pick, e := agents.Select(all, "", "", now, nil)
	if e != nil || pick.Session != newer || pick.Note == "" {
		t.Fatal("two-agent fallback", e)
	}
	if got := agents.Automatic([]*session.Session{old, readOnly}, now); len(got) != 0 {
		t.Fatal("old or read-only auto-picked")
	}
	_, _, file := fixture(t, "claude-stale")
	pick, e = agents.Select(nil, "", file, now.AddDate(1, 0, 0), nil)
	if e != nil || pick.Session == nil {
		t.Fatal("old transcript rejected", e)
	}
}

func TestRecordedEventFacts(t *testing.T) {
	cases := []struct {
		name            string
		kinds           []string
		commands, paths []string
	}{
		{"claude-contradicted", []string{"command", "edit", "command"}, []string{"ls -R . | head -30", "go test ./..."}, []string{"calc/stats.go"}},
		{"claude-honest-failure", []string{"command", "edit", "command"}, []string{"ls -R . | head -30", "go test ./..."}, []string{"calc/stats.go"}},
		{"claude-pytest", []string{"command", "edit", "command"}, []string{".venv/bin/pytest 2>&1 | tail -30; git ls-files", ".venv/bin/pytest 2>&1 | tail -8"}, []string{"pycalc/__init__.py"}},
		{"claude-skip", []string{"command", "edit", "edit", "command"}, []string{`git ls-files && grep -rn "Average" --include=*.go . | head -20`, "go test ./... -v 2>&1 | tail -20"}, []string{"calc/stats.go", "calc/stats_test.go"}},
		{"claude-stale", []string{"command", "edit", "command", "edit"}, []string{"ls -R . | head -30", "go test ./..."}, []string{"calc/stats.go", "calc/stats.go"}},
		{"claude-verified", []string{"command", "command", "command", "command"}, []string{"ls && go test ./... 2>&1 | head -40", "cd calc && ls && cat $(ls *.go | grep -v _test)", "sed -n 15,45p stats_test.go", "sed -i 's|float64(len(nums)-1)|float64(len(nums))|;"}, nil},
		{"codex-stale", []string{"command", "command", "edit", "command", "edit"}, []string{"pwd; rg --files -g 'AGENTS.md' -g '*.go' -g 'go.mod'", "cat calc/stats.go calc/stats_test.go go.mod", "go test ./..."}, []string{"calc/stats.go", "calc/stats.go"}},
		{"codex-verified", []string{"command", "command", "command", "command", "edit", "command"}, []string{"pwd && rg --files -g 'AGENTS.md' -g 'go.mod' -g '*_test.go' -g '*.go'", "cat go.mod calc/stats.go calc/stats_test.go", `git status --short; for p in /AGENTS.md /tmp/AGENTS.md /tmp/gaugestone-fixtures/AGENTS.md /tmp/gaugestone-fixtures/codex-verified/AGENTS.md; do if [ -f "$p" ]; then cat "$p"; fi; done`, "go test ./...", "gofmt -w calc/stats.go\n go test ./...\n git diff --check\n git diff --stat"}, []string{"calc/stats.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, repo, file := fixture(t, tc.name)
			s, e := agents.Read(file)
			if e != nil {
				t.Fatal(e)
			}
			var kinds, commands, paths []string
			for _, ev := range s.Events {
				if ev.Command != nil {
					kinds = append(kinds, "command")
					commands = append(commands, ev.Command.Line)
					if ev.Command.RawOutput == "" && !strings.HasPrefix(ev.Command.Line, "git status --short;") {
						t.Fatal("full command output missing")
					}
				}
				if ev.Edit != nil {
					kinds = append(kinds, "edit")
					rel, _ := filepath.Rel(repo, ev.Edit.Path)
					paths = append(paths, rel)
				}
			}
			if !reflect.DeepEqual(kinds, tc.kinds) || !reflect.DeepEqual(paths, tc.paths) {
				t.Fatalf("events=%v paths=%v", kinds, paths)
			}
			for i, want := range tc.commands {
				if commands[i] != want {
					if !(tc.name == "claude-verified" && i == 3 && strings.HasPrefix(commands[i], want) && strings.HasSuffix(commands[i], "stats.go && gofmt -l . ; go test ./... 2>&1")) {
						t.Fatalf("command %d: %s", i, commands[i])
					}
				}
			}
		})
	}
}
func TestRecordedAgentChoice(t *testing.T) {
	_, _, claudeFile := fixture(t, "claude-stale")
	c, e := agents.Read(claudeFile)
	if e != nil {
		t.Fatal(e)
	}
	_, _, codexFile := fixture(t, "codex-stale")
	x, e := agents.Read(codexFile)
	if e != nil {
		t.Fatal(e)
	}
	all := []*session.Session{c, x}
	picks := agents.Automatic(all, x.Ended.Add(time.Minute))
	if len(picks) != 2 || picks[0] != x || picks[1] != c {
		t.Fatal("recorded pair choice")
	}
	if gap := x.Ended.Sub(c.Ended); gap < 2*time.Second || gap > 4*time.Second {
		t.Fatal("recorded gap", gap)
	}
	_, _, verifiedFile := fixture(t, "codex-verified")
	y, e := agents.Read(verifiedFile)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := agents.Select([]*session.Session{x, y}, "01a15a01", "", x.Ended, nil); e == nil {
		t.Fatal("recorded shared Codex prefix accepted")
	}
	if agents.DisplayID(x, []*session.Session{x, y}) == "01a15a01" {
		t.Fatal("recorded display prefix not extended")
	}
	// The same pair must permit an explicit interactive choice and suppress it for JSON/no-prompt callers.
	app := cli.New("test", cli.Options{})
	f := app.Factory()
	f.PromptTerminalSet = true
	f.PromptTerminal = true
	f.Stdin = strings.NewReader("2\n")
	f.Stderr = io.Discard
	selected, e := agents.Select(all, "", "", x.Ended, f)
	if e != nil || selected.Session != c {
		t.Fatal("interactive choice", e)
	}
	f.Flags.NoPrompt = true
	selected, e = agents.Select(all, "", "", x.Ended, f)
	if e != nil || selected.Session != x || selected.Note == "" {
		t.Fatal("no-prompt choice", e)
	}
}
