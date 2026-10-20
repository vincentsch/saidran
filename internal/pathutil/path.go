// Package pathutil compares recorded filesystem paths without losing deleted files.
package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// Resolve joins relative paths to their recorded cwd and resolves existing parents.
// An absent cwd never falls back to the process's current directory.
func Resolve(cwd, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		if cwd == "" || !filepath.IsAbs(cwd) {
			return ""
		}
		path = filepath.Join(cwd, path)
	}
	path = filepath.Clean(path)
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved
		}
		if !os.IsNotExist(err) {
			return ""
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		tail = append(tail, filepath.Base(path))
		path = parent
	}
}

func Within(parent, child string) bool {
	parent, child = Resolve("", parent), Resolve("", child)
	if parent == "" || child == "" {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
