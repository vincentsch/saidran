package gitstate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vincentsch/rungrad"
)

type Baseline struct {
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
	Reason string `json:"reason"`
}
type Line struct {
	Kind     byte
	Text     string
	Old, New int
}
type Hunk struct{ Lines []Line }

func (h Hunk) Normalized() string {
	var b strings.Builder
	for _, l := range h.Lines {
		b.WriteByte(l.Kind)
		b.WriteString(strings.TrimRight(l.Text, " \t\r"))
		b.WriteByte('\n')
	}
	return b.String()
}

type File struct {
	Additions           int    `json:"additions"`
	Deletions           int    `json:"deletions"`
	Path                string `json:"path"`
	OldPath             string `json:"old_path,omitempty"`
	Status              string `json:"status"`
	Group               string `json:"group"`
	ChangedAfterSession bool   `json:"changed_after_session"`
	Hunks               []Hunk `json:"-"`
	Old, New            []byte `json:"-"`
	submodule           bool
}
type State struct {
	Root     string   `json:"root"`
	Branch   string   `json:"branch"`
	Head     string   `json:"head"`
	Baseline Baseline `json:"baseline"`
	Dirty    bool     `json:"dirty"`
	GitDir   string   `json:"-"`
	Files    []File   `json:"-"`
}

// git never invokes configured diff drivers or text conversion programs.
func git(root string, args ...string) ([]byte, error) {
	return gitEnv(root, nil, args...)
}
func gitEnv(root string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.quotepath=off", "-c", "diff.external=", "-c", "diff.suppressBlankEmpty=false", "--no-pager"}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1")
	cmd.Env = append(cmd.Env, env...)
	b, err := cmd.Output()
	if err != nil {
		return nil, rungrad.NewError(rungrad.ExitAPI, fmt.Sprintf("Git %s failed; check the repository and revision.", args[0]))
	}
	return b, nil
}
func value(root string, args ...string) (string, error) {
	b, e := git(root, args...)
	return strings.TrimSpace(string(b)), e
}
func SelectBaseline(root, ref string) (Baseline, error) {
	if ref != "" {
		commit, err := value(root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
		return Baseline{ref, commit, "explicit --base"}, err
	}
	for _, candidate := range []string{"origin/HEAD", "origin/main", "origin/master"} {
		if commit, err := value(root, "merge-base", "HEAD", candidate); err == nil {
			return Baseline{candidate, commit, "merge base with " + candidate}, nil
		}
	}
	head, err := value(root, "rev-parse", "--verify", "HEAD")
	return Baseline{"HEAD", head, "uncommitted changes only"}, err
}
func Inspect(root, base string, ended time.Time) (State, error) {
	s := State{Root: root, Files: []File{}}
	var err error
	if s.Baseline, err = SelectBaseline(root, base); err != nil {
		return s, err
	}
	if s.Head, err = value(root, "rev-parse", "HEAD"); err != nil {
		return s, err
	}
	if s.GitDir, err = value(root, "rev-parse", "--absolute-git-dir"); err != nil {
		return s, err
	}
	if s.Branch, err = value(root, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil {
		s.Branch = "(detached HEAD)"
	}
	status, err := git(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return s, err
	}
	s.Dirty = len(status) > 0
	untracked, err := git(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return s, err
	}
	untrackedPaths := map[string]bool{}
	for _, p := range strings.Split(string(untracked), "\x00") {
		if p != "" {
			untrackedPaths[p] = true
		}
	}
	var env []string
	if len(untrackedPaths) > 0 {
		dir, err := os.MkdirTemp("", "assaylight-index-")
		if err != nil {
			return s, err
		}
		defer os.RemoveAll(dir)
		indexPath, err := value(root, "rev-parse", "--path-format=absolute", "--git-path", "index")
		if err != nil {
			return s, err
		}
		index, err := os.ReadFile(indexPath)
		if err != nil && !os.IsNotExist(err) {
			return s, err
		}
		env = []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}
		if err == nil {
			if err = os.WriteFile(filepath.Join(dir, "index"), index, 0600); err != nil {
				return s, err
			}
		} else if _, err = gitEnv(root, env, "read-tree", "--empty"); err != nil {
			return s, err
		}
		paths := filepath.Join(dir, "paths")
		if err = os.WriteFile(paths, untracked, 0600); err != nil {
			return s, err
		}
		// Intent-to-add lets Git match ordinary file moves without touching the real
		// index or staging working-file contents. The temporary copy is discarded.
		if _, err = gitEnv(root, env, "add", "--intent-to-add", "--pathspec-from-file="+paths, "--pathspec-file-nul"); err != nil {
			return s, err
		}
	}
	names, err := gitEnv(root, env, "diff", "--no-color", "--no-ext-diff", "--no-textconv", "--find-renames", "--raw", "--no-abbrev", "-z", s.Baseline.Commit, "--")
	if err != nil {
		return s, err
	}
	fields := strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00")
	for i := 0; i+1 < len(fields); {
		header, p := strings.Fields(fields[i]), fields[i+1]
		i += 2
		if len(header) != 5 || len(header[4]) == 0 {
			return s, fmt.Errorf("incomplete Git change record")
		}
		status := header[4]
		f := File{Path: p, Status: status[:1], submodule: header[0] == ":160000" || header[1] == "160000"}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if i >= len(fields) {
				return s, fmt.Errorf("incomplete Git rename record")
			}
			f.OldPath = p
			f.Path = fields[i]
			i++
		} else if f.Status == "A" && untrackedPaths[p] {
			f.Status = "?"
		}
		s.Files = append(s.Files, f)
	}
	for i := range s.Files {
		f := &s.Files[i]
		if f.submodule {
			f.Group = "other"
			continue
		}
		oldPath := f.Path
		if f.OldPath != "" {
			oldPath = f.OldPath
		}
		if f.Status != "?" && f.Status != "A" {
			f.Old, err = git(root, "show", s.Baseline.Commit+":"+oldPath)
			if err != nil {
				return s, err
			}
		}
		if f.Status != "D" {
			info, e := os.Lstat(filepath.Join(root, filepath.FromSlash(f.Path)))
			if e != nil {
				return s, e
			}
			f.ChangedAfterSession = !ended.IsZero() && info.ModTime().After(ended)
			if info.Mode()&os.ModeSymlink != 0 {
				link, e := os.Readlink(filepath.Join(root, filepath.FromSlash(f.Path)))
				if e != nil {
					return s, e
				}
				f.New = []byte(link)
			} else if info.Mode().IsRegular() {
				f.New, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
				if err != nil {
					return s, err
				}
			}
		}
		f.Group = Group(f.Path, isText(f.New) && isText(f.Old))
		args := []string{"diff", "--no-color", "--no-ext-diff", "--no-textconv", "--find-renames", "--unified=3", s.Baseline.Commit, "--", oldPath}
		if f.OldPath != "" {
			args = append(args, f.Path)
		}
		diff, e := gitEnv(root, env, args...)
		if e != nil {
			return s, e
		}
		f.Hunks = ParseHunks(string(diff))
	}
	return s, nil
}
func isText(b []byte) bool { return !bytes.ContainsRune(b, 0) && utf8.Valid(b) }
func Group(p string, text bool) string {
	b := path.Base(p)
	lower := strings.ToLower(b)
	parts := "/" + p + "/"
	if strings.Contains(parts, "/tests/") || strings.Contains(parts, "/__tests__/") || strings.Contains(parts, "/spec/") || strings.HasSuffix(b, "_test.go") || strings.HasPrefix(b, "test_") && strings.HasSuffix(b, ".py") || strings.HasSuffix(b, "_test.py") || strings.Contains(b, ".test.") || strings.Contains(b, ".spec.") {
		return "tests"
	}
	if strings.HasPrefix(p, ".github/workflows/") || p == ".gitlab-ci.yml" || strings.HasPrefix(p, ".circleci/") {
		return "ci"
	}
	switch lower {
	case "package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "pnpm-lock.yaml", "bun.lock", "bun.lockb", "composer.lock", "go.sum", "cargo.lock", "poetry.lock", "uv.lock", "pipfile.lock", "gemfile.lock":
		return "lockfiles"
	}
	ext := path.Ext(b)
	if !strings.Contains(p, "/") && (ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml" || ext == ".ini") || strings.HasPrefix(b, ".eslintrc") || strings.HasPrefix(b, "phpunit.xml") || b == "pyproject.toml" || strings.HasPrefix(b, "jest.config.") || strings.HasPrefix(b, "vitest.config.") {
		return "config"
	}
	if strings.HasSuffix(b, ".md") || strings.HasPrefix(p, "docs/") {
		return "docs"
	}
	if text {
		return "source"
	}
	return "other"
}
func ParseHunks(diff string) []Hunk {
	var hunks []Hunk
	old, newLine := 0, 0
	active := false
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "@@ ") {
			fields := strings.Fields(l)
			if len(fields) < 3 {
				continue
			}
			old, _ = strconv.Atoi(strings.Split(strings.TrimPrefix(fields[1], "-"), ",")[0])
			newLine, _ = strconv.Atoi(strings.Split(strings.TrimPrefix(fields[2], "+"), ",")[0])
			hunks = append(hunks, Hunk{})
			active = true
			continue
		}
		if strings.HasPrefix(l, "diff --git ") {
			active = false
		}
		if !active || len(l) == 0 || !strings.ContainsRune(" +-", rune(l[0])) {
			continue
		}
		line := Line{Kind: l[0], Text: l[1:], Old: old, New: newLine}
		h := &hunks[len(hunks)-1]
		h.Lines = append(h.Lines, line)
		if l[0] != '+' {
			old++
		}
		if l[0] != '-' {
			newLine++
		}
	}
	return hunks
}
