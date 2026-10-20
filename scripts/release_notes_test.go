package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseNotesMatchTag(t *testing.T) {
	script, err := filepath.Abs("release-notes.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, tag, heading, want string
		failed                   bool
	}{
		{"released", "v0.1.0", "0.1.0 - 2026-10-21", "Current behavior.", false},
		{"unreleased", "v0.1.0", "0.1.0 (unreleased)", "unreleased", true},
		{"missing", "v0.2.0", "0.1.0 - 2026-10-21", "No changelog section", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			changelog := "# Changelog\n\n## " + tc.heading + "\n\nCurrent behavior.\n\n## 0.0.1\n\nOld behavior.\n"
			if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(changelog), 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("python3", script, tc.tag)
			command.Dir = dir
			output, err := command.CombinedOutput()
			if (err != nil) != tc.failed || !strings.Contains(string(output), tc.want) {
				t.Fatalf("release notes: %v %s", err, output)
			}
			if !tc.failed && strings.TrimSpace(string(output)) != tc.want {
				t.Fatalf("wrong release section: %s", output)
			}
		})
	}
}
