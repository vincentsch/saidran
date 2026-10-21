package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/rungrad/output"
	"github.com/vincentsch/saidran/internal/gitstate"
	"github.com/vincentsch/saidran/internal/redact"
	"github.com/vincentsch/saidran/internal/report"
	"mvdan.cc/sh/v3/syntax"
)

func addInstall(app *rungrad.App, opts Options, r redact.Redactor) {
	for _, remove := range []bool{false, true} {
		name, short := "install", "Install a Claude hook or print Codex notify configuration"
		if remove {
			name, short = "uninstall", "Remove the Saidran Claude hook"
		}
		app.AddCommand(&rungrad.Command{Use: name + " <claude|codex>", Short: short, Long: "Project scope uses .claude/settings.local.json and Git-local backups.\nUser scope uses CLAUDE_CONFIG_DIR/settings.json. Codex configuration is printed only.", Examples: []string{"saidran " + name + " claude --dry-run", "saidran " + name + " claude --user"}, Related: []string{"saidran hook", "saidran snippet"}, Args: cobra.ExactArgs(1), Mutates: true, OutputModes: []string{"human", "json"}, Configure: func(cmd *cobra.Command) {
			cmd.Flags().Bool("project", false, "Use repository-local Claude settings (default)")
			cmd.Flags().Bool("user", false, "Use user Claude settings")
			cmd.MarkFlagsMutuallyExclusive("project", "user")
		}, Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
			if args[0] == "codex" && !remove {
				executable, err := os.Executable()
				if err != nil {
					return err
				}
				// JSON escaping also supplies valid TOML basic-string escapes.
				quoted, _ := json.Marshal(executable)
				config := "notify = [" + string(quoted) + ", \"hook\", \"codex\"]"
				instructions := "Add this to config.toml. If notify already exists, use a wrapper that calls both notifiers and forwards the JSON argument. Remove this notify entry (or wrapper call) to uninstall. Saidran does not edit config.toml."
				return f.WriteResult(map[string]string{"config": config, "instructions": instructions}, func(w io.Writer) { fmt.Fprintln(w, config); fmt.Fprintln(w, instructions) })
			}
			if args[0] != "claude" {
				return rungrad.NewError(rungrad.ExitUsage, "Choose claude; install codex prints manual notify instructions.")
			}
			user, _ := cmd.Flags().GetBool("user")
			target, backup, root, err := settingsPaths(opts.Dir, user)
			if err != nil {
				return err
			}
			original, err := os.ReadFile(target)
			exists := err == nil
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			settings := map[string]json.RawMessage{}
			if exists {
				if err = json.Unmarshal(original, &settings); err != nil || settings == nil {
					return fmt.Errorf("Invalid Claude settings JSON in %s; repair it before installing or uninstalling.", r.Text(target))
				}
			}
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			updated, changed, err := mergeHook(settings, executable, remove)
			if err != nil {
				return err
			}
			if f.DryRun() {
				return f.WritePreview(output.DryRunPreview{Method: name, Path: r.Text(target)})
			}
			if !remove && !user {
				if err = excludeSettings(root); err != nil {
					return err
				}
			}
			if changed {
				if exists {
					if err = os.MkdirAll(backup, 0700); err != nil {
						return err
					}
					file, e := os.CreateTemp(backup, "settings-*.bak")
					if e != nil {
						return e
					}
					_, e = file.Write(original)
					closeErr := file.Close()
					if e != nil {
						return e
					}
					if closeErr != nil {
						return closeErr
					}
				}
				if err = report.WriteDocument(filepath.Dir(target), filepath.Base(target), updated); err != nil {
					return err
				}
			}
			status := "installed"
			if remove {
				status = "removed"
			}
			if !changed {
				status = "unchanged"
			}
			return f.WriteResult(map[string]string{"path": r.Text(target), "status": status}, func(w io.Writer) { fmt.Fprintf(w, "Claude hook %s: %s\n", status, r.Text(target)) })
		}})
	}
}

func gitPath(root string, args ...string) (string, error) {
	b, err := exec.Command("git", append([]string{"-C", root, "rev-parse"}, args...)...).Output()
	return strings.TrimSpace(string(b)), err
}
func settingsPaths(dir string, user bool) (target, backup, root string, err error) {
	if user {
		config := os.Getenv("CLAUDE_CONFIG_DIR")
		if config == "" {
			var home string
			home, err = os.UserHomeDir()
			if err != nil {
				return
			}
			config = filepath.Join(home, ".claude")
		}
		target = filepath.Join(config, "settings.json")
		backup = filepath.Dir(target)
		return
	}
	root, err = gitstate.Root(dir)
	if err != nil {
		return
	}
	var gitDir string
	gitDir, err = gitPath(root, "--absolute-git-dir")
	if err != nil {
		return
	}
	target = filepath.Join(root, ".claude", "settings.local.json")
	backup = filepath.Join(gitDir, "saidran")
	// Existing shared tracked settings must not gain machine-specific paths.
	b, e := exec.Command("git", "-C", root, "ls-files", "--", ".claude/settings.local.json").Output()
	if e != nil {
		err = e
		return
	}
	if len(b) > 0 {
		err = fmt.Errorf(".claude/settings.local.json is tracked; untrack it before installing a machine-local hook.")
	}
	return
}
func excludeSettings(root string) error {
	cmd := exec.Command("git", "-C", root, "check-ignore", "-q", "--", ".claude/settings.local.json")
	if err := cmd.Run(); err == nil {
		return nil
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		return err
	}
	path, err := gitPath(root, "--path-format=absolute", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	b = append(b, []byte("/.claude/settings.local.json\n")...)
	return report.WriteDocument(filepath.Dir(path), filepath.Base(path), b)
}
func hookCommand(executable string) string {
	return "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "' hook claude"
}

// Only literal words are accepted: substitutions and extra shell operations
// must never cause an unrelated command to be removed.
func ownedHook(command string) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil || len(file.Stmts) != 1 {
		return false
	}
	stmt := file.Stmts[0]
	if stmt.Background || stmt.Negated || len(stmt.Redirs) > 0 {
		return false
	}
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Assigns) > 0 || len(call.Args) != 3 {
		return false
	}
	literal := func(parts []syntax.WordPart) (string, bool) {
		var b strings.Builder
		for _, part := range parts {
			switch x := part.(type) {
			case *syntax.Lit:
				// Remove shell escapes from literal, unquoted words.
				value := x.Value
				if strings.Contains(value, "\\") {
					var out strings.Builder
					for i := 0; i < len(value); i++ {
						if value[i] == '\\' && i+1 < len(value) {
							i++
						}
						out.WriteByte(value[i])
					}
					value = out.String()
				}
				b.WriteString(value)
			case *syntax.SglQuoted:
				if x.Dollar {
					return "", false
				}
				b.WriteString(x.Value)
			case *syntax.DblQuoted:
				for _, p := range x.Parts {
					l, ok := p.(*syntax.Lit)
					if !ok {
						return "", false
					}
					v := l.Value
					for _, c := range []string{`\$`, `\` + "`", `\"`, `\\`} {
						v = strings.ReplaceAll(v, c, c[1:])
					}
					b.WriteString(v)
				}
			default:
				return "", false
			}
		}
		return b.String(), true
	}
	words := []string{}
	for _, arg := range call.Args {
		v, ok := literal(arg.Parts)
		if !ok {
			return false
		}
		words = append(words, v)
	}
	executable := words[0]
	base := filepath.Base(strings.ReplaceAll(executable, "\\", "/"))
	absolute := filepath.IsAbs(executable) || len(executable) > 2 && executable[1] == ':' && (executable[2] == '\\' || executable[2] == '/')
	return absolute && (base == "saidran" || base == "saidran.exe") && words[1] == "hook" && words[2] == "claude"
}
func mergeHook(settings map[string]json.RawMessage, executable string, remove bool) ([]byte, bool, error) {
	hooks := map[string]json.RawMessage{}
	if raw, ok := settings["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil || hooks == nil {
			return nil, false, fmt.Errorf("Claude hooks must be a JSON object; settings were not changed")
		}
	}
	var groups []map[string]json.RawMessage
	if raw, ok := hooks["Stop"]; ok {
		if err := json.Unmarshal(raw, &groups); err != nil {
			return nil, false, fmt.Errorf("Claude Stop hooks must be an array; settings were not changed")
		}
	}
	found, changed := false, false
	var kept []map[string]json.RawMessage
	for _, group := range groups {
		var entries []map[string]json.RawMessage
		if raw, ok := group["hooks"]; ok {
			if err := json.Unmarshal(raw, &entries); err != nil {
				return nil, false, fmt.Errorf("Claude hook entries must be an array; settings were not changed")
			}
		}
		var remaining []map[string]json.RawMessage
		for _, entry := range entries {
			var kind, command string
			_ = json.Unmarshal(entry["type"], &kind)
			_ = json.Unmarshal(entry["command"], &command)
			if kind == "command" && (ownedHook(command) || command == hookCommand(executable)) {
				found = true
				if remove {
					changed = true
					continue
				}
			}
			remaining = append(remaining, entry)
		}
		if len(remaining) != len(entries) {
			if len(remaining) == 0 {
				continue
			}
			group["hooks"], _ = json.Marshal(remaining)
		}
		kept = append(kept, group)
	}
	if !remove && !found {
		entry := map[string]any{"hooks": []map[string]string{{"type": "command", "command": hookCommand(executable)}}}
		b, _ := json.Marshal(entry)
		var group map[string]json.RawMessage
		_ = json.Unmarshal(b, &group)
		kept = append(kept, group)
		changed = true
	}
	if !changed {
		return nil, false, nil
	}
	if len(kept) == 0 {
		delete(hooks, "Stop")
	} else {
		hooks["Stop"], _ = json.Marshal(kept)
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"], _ = json.Marshal(hooks)
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	return append(bytes.TrimSpace(b), '\n'), true, err
}
