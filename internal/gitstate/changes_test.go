package gitstate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s %v", args, b, e)
	}
	return string(b)
}
func write(t *testing.T, root, p, content string) {
	t.Helper()
	p = filepath.Join(root, p)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}
func repoTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", "main")
	write(t, root, "main.go", "package main\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "Initial source")
	return root
}
func TestBaselineFallbacks(t *testing.T) {
	for _, remote := range []string{"origin/HEAD", "origin/main", "origin/master", ""} {
		t.Run(remote, func(t *testing.T) {
			root := repoTest(t)
			head, _ := value(root, "rev-parse", "HEAD")
			if remote != "" {
				gitTest(t, root, "update-ref", "refs/remotes/"+remote, head)
			}
			write(t, root, "main.go", "package changed\n")
			gitTest(t, root, "commit", "-qam", "Change source")
			b, e := SelectBaseline(root, "")
			if e != nil {
				t.Fatal(e)
			}
			want := remote
			if want == "" {
				want = "HEAD"
				if b.Reason != "uncommitted changes only" {
					t.Fatal(b)
				}
			} else if b.Commit != head {
				t.Fatal(b)
			}
			if b.Ref != want {
				t.Fatal(b)
			}
			explicit, e := SelectBaseline(root, "HEAD~1")
			if e != nil || explicit.Commit != head || explicit.Reason != "explicit --base" {
				t.Fatal(explicit, e)
			}
			if _, e = SelectBaseline(root, "missing-revision"); e == nil {
				t.Fatal("invalid base accepted")
			}
		})
	}
	root := repoTest(t)
	head, _ := value(root, "rev-parse", "HEAD")
	for _, name := range []string{"main", "master", "HEAD"} {
		gitTest(t, root, "update-ref", "refs/remotes/origin/"+name, head)
	}
	write(t, root, "main.go", "package changed\n")
	gitTest(t, root, "commit", "-qam", "Next")
	for _, want := range []string{"origin/HEAD", "origin/main", "origin/master"} {
		b, e := SelectBaseline(root, "")
		if e != nil || b.Ref != want {
			t.Fatal(b, e)
		}
		gitTest(t, root, "update-ref", "-d", "refs/remotes/"+want)
	}
}
func TestTrackedFeatureAndWorkingChanges(t *testing.T) {
	root := repoTest(t)
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitTest(t, root, "init", "--bare", "-q", origin)
	gitTest(t, root, "remote", "add", "origin", origin)
	gitTest(t, root, "push", "-q", "-u", "origin", "main")
	gitTest(t, root, "remote", "set-head", "origin", "main")
	gitTest(t, root, "checkout", "-qb", "feature")
	write(t, root, "committed.go", "package committed\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "Add source")
	gitTest(t, root, "push", "-q", "-u", "origin", "feature")
	write(t, root, "staged.go", "package staged\n")
	gitTest(t, root, "add", "staged.go")
	write(t, root, "main.go", "package unstaged\n")
	write(t, root, "tests/new_test.go", "package tests\n")
	write(t, root, ".gitignore", "ignored\n")
	write(t, root, "ignored", "private\n")
	state, e := Inspect(root, "", time.Now().Add(-time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if state.Baseline.Ref != "origin/HEAD" || !state.Dirty {
		t.Fatal(state)
	}
	paths := map[string]File{}
	for _, f := range state.Files {
		paths[f.Path] = f
		if !f.ChangedAfterSession {
			t.Fatal("missing current mtime", f.Path)
		}
	}
	for _, p := range []string{"committed.go", "staged.go", "main.go", "tests/new_test.go", ".gitignore"} {
		if _, ok := paths[p]; !ok {
			t.Fatal("missing", p)
		}
	}
	if len(paths) != 5 || paths["tests/new_test.go"].Group != "tests" {
		t.Fatal(paths)
	}
	worktree := filepath.Join(t.TempDir(), "worktree")
	gitTest(t, root, "worktree", "add", "-q", "-b", "review", worktree)
	linked, e := Inspect(worktree, "", time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if linked.GitDir == state.GitDir || linked.GitDir == filepath.Join(worktree, ".git") {
		t.Fatal("worktree git dir", linked.GitDir)
	}
	for _, f := range linked.Files {
		if f.ChangedAfterSession {
			t.Fatal("future session", f)
		}
	}
}
func TestRenameAndLiteralPaths(t *testing.T) {
	root := repoTest(t)
	write(t, root, "tests/old_test.go", "package test\n// original test\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "Add test")
	gitTest(t, root, "mv", "tests/old_test.go", "tests/new_test.go")
	write(t, root, "odd [name]\t.go", "package odd\n")
	state, e := Inspect(root, "", time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	files := map[string]File{}
	for _, f := range state.Files {
		files[f.Path] = f
	}
	renamed := files["tests/new_test.go"]
	if len(files) != 2 || renamed.Status != "R" || renamed.OldPath != "tests/old_test.go" || len(renamed.Hunks) != 0 {
		t.Fatal(state.Files)
	}
	if literal := files["odd [name]\t.go"]; literal.Status != "?" || len(literal.Hunks) != 1 {
		t.Fatal(state.Files)
	}
}
func TestFileGroups(t *testing.T) {
	cases := map[string]string{"calc/a_test.go": "tests", "test_calc.py": "tests", "calc_test.py": "tests", "tests/FooTest.php": "tests", "__tests__/a.ts": "tests", "spec/a.rb": "tests", "src/a.spec.ts": "tests", "a.test.js": "tests", ".github/workflows/check.yml": "ci", ".gitlab-ci.yml": "ci", ".circleci/config.yml": "ci", "go.sum": "lockfiles", "composer.lock": "lockfiles", "pnpm-lock.yaml": "lockfiles", "package.json": "config", "nested/.eslintrc.js": "config", "phpunit.xml.dist": "config", "vitest.config.ts": "config", "docs/a.txt": "docs", "README.md": "docs", "calc/a.go": "source", "nested/data.json": "source"}
	for p, want := range cases {
		if got := Group(p, true); got != want {
			t.Errorf("%s: %s != %s", p, got, want)
		}
	}
	if Group("image.png", false) != "other" {
		t.Fatal("binary group")
	}
}

func TestConfiguredDiffProgramsAreDisabled(t *testing.T) {
	root := repoTest(t)
	write(t, root, ".gitattributes", "*.go diff=custom\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "Set diff attributes")
	write(t, root, "main.go", "package changed\n")
	t.Setenv("GIT_EXTERNAL_DIFF", "missing-external-diff-command")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "diff.custom.textconv")
	t.Setenv("GIT_CONFIG_VALUE_0", "missing-textconv-command")
	state, e := Inspect(root, "", time.Time{})
	if e != nil || len(state.Files) != 1 || len(state.Files[0].Hunks) != 1 {
		t.Fatal(state, e)
	}
}

func TestChangedSubmodules(t *testing.T) {
	root := repoTest(t)
	source := repoTest(t)
	write(t, source, "library.go", "package library\n")
	gitTest(t, source, "add", ".")
	gitTest(t, source, "commit", "-qm", "Add library source")
	sub := filepath.Join(root, "vendor", "lib")
	gitTest(t, root, "clone", "-q", source, sub)
	head := strings.TrimSpace(gitTest(t, sub, "rev-parse", "HEAD"))
	gitTest(t, root, "update-index", "--add", "--cacheinfo", "160000,"+head+",vendor/lib")
	gitTest(t, root, "commit", "-qm", "Add library")
	write(t, sub, "main.go", "package modified\n")
	for _, pointerChanged := range []bool{false, true} {
		if pointerChanged {
			gitTest(t, sub, "commit", "-qam", "Change library")
		}
		state, err := Inspect(root, "", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Files) != 1 || state.Files[0].Path != "vendor/lib" || state.Files[0].Group != "other" || len(state.Files[0].Hunks) != 0 {
			t.Fatal(state.Files)
		}
	}
}
