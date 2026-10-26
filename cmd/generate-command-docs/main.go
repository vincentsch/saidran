// Command generate-command-docs writes the command reference from the CLI definition.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vincentsch/rungrad/docsgen"
	"github.com/vincentsch/saidran/internal/cli"
)

func main() {
	check := flag.Bool("check", false, "check generated reference without writing files")
	flag.Parse()
	app := cli.New("dev", cli.Options{})
	docs := docsgen.Generate(app)
	// Keep command metadata from rungrad and use concise reference prose.
	prose := strings.NewReplacer(
		"Shows text you can read in your terminal.", "Prints text output by default.",
		"Use --json to get JSON for scripts and other programs.", "Use `--json` for JSON output.",
		"This command makes changes. Use `--dry-run` to see what would change without changing anything.", "Use `--dry-run` to preview changes without applying them.",
	)
	for path, content := range docs {
		docs[path] = prose.Replace(content)
	}
	if err := syncDocs("docs/reference", docs, *check); err != nil {
		fail(err)
	}
}

func syncDocs(dir string, docs map[string]string, check bool) error {
	var result docsgen.CheckResult
	for path, content := range docs {
		full := filepath.Join(dir, path)
		if check {
			got, err := os.ReadFile(full)
			switch {
			case errors.Is(err, os.ErrNotExist):
				result.Missing = append(result.Missing, path)
			case err != nil:
				return err
			case string(got) != content:
				result.Stale = append(result.Stale, path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				return err
			}
		}
	}
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if _, ok := docs[filepath.ToSlash(rel)]; !ok {
			if check {
				result.Orphaned = append(result.Orphaned, filepath.ToSlash(rel))
			} else {
				return os.Remove(path)
			}
		}
		return nil
	}); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !result.OK() {
		sort.Strings(result.Missing)
		sort.Strings(result.Stale)
		sort.Strings(result.Orphaned)
		return fmt.Errorf("generated command docs differ: missing=%v stale=%v orphaned=%v; run make docs", result.Missing, result.Stale, result.Orphaned)
	}
	return nil
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
