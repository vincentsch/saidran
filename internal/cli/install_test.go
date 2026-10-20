package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vincentsch/rungrad/testutil"
)

func TestHookCommandOwnership(t *testing.T) {
	for _, path := range []string{"/opt/gaugestone", "/a path/gaugestone", `/a 'quoted' path/gaugestone`} {
		if !ownedHook(hookCommand(path)) {
			t.Fatal(hookCommand(path))
		}
	}
	for _, command := range []string{`"/old path/gaugestone" hook claude`, `/old\ path/gaugestone hook claude`, `"C:\Program Files\gaugestone.exe" hook claude`} {
		if !ownedHook(command) {
			t.Fatal(command)
		}
	}
	for _, command := range []string{`gaugestone hook claude`, `/opt/other hook claude`, `/opt/gaugestone hook claude extra`, `/opt/gaugestone hook claude && echo done`, `$(which gaugestone) hook claude`, `/opt/gaugestone hook claude > /tmp/output`, `/opt/gaugestone hook codex`} {
		if ownedHook(command) {
			t.Fatal(command)
		}
	}
}
func TestClaudeSettingsLifecycle(t *testing.T) {
	for _, user := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "user"}[user], func(t *testing.T) {
			base := isolated(t)
			repo := filepath.Join(base, "repo")
			if b, e := exec.Command("git", "init", "-q", repo).CombinedOutput(); e != nil {
				t.Fatalf("%v %s", e, b)
			}
			target, backup, _, err := settingsPaths(repo, user)
			if err != nil {
				t.Fatal(err)
			}
			original := []byte(`{"theme":"light","hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo keep"},{"type":"command","command":"/old/gaugestone hook claude extra"}]}],"PreToolUse":[]}}`)
			if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(target, original, 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"install", "claude"}
			if user {
				args = append(args, "--user")
			}
			run := func(a ...string) {
				t.Helper()
				result := testutil.Run(New("test", Options{Dir: repo}), a...)
				if result.Exit != 0 {
					t.Fatalf("%+v", result)
				}
			}
			run(append(args, "--dry-run")...)
			before, _ := os.ReadFile(target)
			if !bytes.Equal(before, original) {
				t.Fatal("preview changed settings")
			}
			matches, _ := filepath.Glob(filepath.Join(backup, "settings-*.bak"))
			if len(matches) != 0 {
				t.Fatal("preview backup")
			}
			run(args...)
			installed, _ := os.ReadFile(target)
			matches, _ = filepath.Glob(filepath.Join(backup, "settings-*.bak"))
			if len(matches) != 1 {
				t.Fatal(matches)
			}
			b, _ := os.ReadFile(matches[0])
			if !bytes.Equal(b, original) {
				t.Fatal("backup differs")
			}
			run(args...)
			again, _ := os.ReadFile(target)
			if !bytes.Equal(installed, again) {
				t.Fatal("repeat changed settings")
			}
			matches, _ = filepath.Glob(filepath.Join(backup, "settings-*.bak"))
			if len(matches) != 1 {
				t.Fatal("repeat backup")
			}
			if !user {
				b, e := exec.Command("git", "-C", repo, "check-ignore", ".claude/settings.local.json").CombinedOutput()
				if e != nil {
					t.Fatalf("not ignored %v %s", e, b)
				}
			}
			// Simulate a binary moved since installation.
			var settings map[string]json.RawMessage
			_ = json.Unmarshal(installed, &settings)
			b, _, e := mergeHook(settings, "/old location/gaugestone", true)
			if e != nil {
				t.Fatal(e)
			}
			_ = json.Unmarshal(b, &settings)
			b, _, e = mergeHook(settings, "/old location/gaugestone", false)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(target, b, 0600); e != nil {
				t.Fatal(e)
			}
			args[0] = "uninstall"
			run(append(args, "--dry-run")...)
			run(args...)
			removed, _ := os.ReadFile(target)
			if strings.Contains(string(removed), "/old location") || !strings.Contains(string(removed), "echo keep") || !strings.Contains(string(removed), "hook claude extra") || !strings.Contains(string(removed), `"theme": "light"`) {
				t.Fatal(string(removed))
			}
			run(args...)
			if e = os.WriteFile(target, []byte("{broken"), 0600); e != nil {
				t.Fatal(e)
			}
			result := testutil.Run(New("test", Options{Dir: repo}), args...)
			if result.Exit == 0 {
				t.Fatal("accepted invalid JSON")
			}
			b, _ = os.ReadFile(target)
			if string(b) != "{broken" {
				t.Fatal("overwrote invalid JSON")
			}
		})
	}
}

func TestExcludePreservesEntriesAndPreviewCreatesNothing(t *testing.T) {
	base := isolated(t)
	b, e := exec.Command("git", "init", "-q", base).CombinedOutput()
	if e != nil {
		t.Fatalf("%v %s", e, b)
	}
	exclude := filepath.Join(base, ".git", "info", "exclude")
	original := []byte("# keep\nlocal-file")
	if e = os.WriteFile(exclude, original, 0600); e != nil {
		t.Fatal(e)
	}
	r := testutil.Run(New("test", Options{Dir: base}), "install", "claude", "--dry-run")
	if r.Exit != 0 {
		t.Fatal(r)
	}
	if _, e = os.Stat(filepath.Join(base, ".claude")); !os.IsNotExist(e) {
		t.Fatal("preview created settings directory")
	}
	after, _ := os.ReadFile(exclude)
	if !bytes.Equal(after, original) {
		t.Fatal("preview modified excludes")
	}
	for range 2 {
		if e = excludeSettings(base); e != nil {
			t.Fatal(e)
		}
	}
	after, _ = os.ReadFile(exclude)
	if string(after) != string(original)+"\n/.claude/settings.local.json\n" {
		t.Fatal(string(after))
	}
}

func TestTrackedLocalSettingsAreProtected(t *testing.T) {
	base := isolated(t)
	if out, err := exec.Command("git", "init", "-q", base).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	path := filepath.Join(base, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"hooks":{}}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", base, "add", ".claude/settings.local.json").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	result := testutil.Run(New("test", Options{Dir: base}), "install", "claude")
	if result.Exit == 0 || !strings.Contains(result.Stderr, "is tracked") {
		t.Fatal(result)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, original) {
		t.Fatal("tracked settings changed")
	}
	if _, err := os.Stat(filepath.Join(base, ".git", "gaugestone")); !os.IsNotExist(err) {
		t.Fatal("created backup")
	}
}

func TestSnippetAndCodexConfiguration(t *testing.T) {
	isolated(t)
	result := testutil.Run(New("test", Options{}), "snippet")
	if result.Exit != 0 || result.Stdout != selfCheck {
		t.Fatal(result)
	}
	result = testutil.Run(New("test", Options{}), "install", "codex", "--json")
	var value map[string]string
	if err := json.Unmarshal([]byte(result.Stdout), &value); err != nil || result.Exit != 0 {
		t.Fatalf("%+v %v", result, err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The array is valid JSON as well as TOML, so it can be checked without
	// introducing a TOML dependency into the application.
	var argv []string
	if err = json.Unmarshal([]byte(strings.TrimPrefix(value["config"], "notify = ")), &argv); err != nil {
		t.Fatal(err)
	}
	if len(argv) != 3 || argv[0] != executable || argv[1] != "hook" || argv[2] != "codex" || !strings.Contains(value["instructions"], "wrapper") {
		t.Fatal(value)
	}
}
