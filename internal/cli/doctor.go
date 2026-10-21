package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/saidran/internal/agents"
	"github.com/vincentsch/saidran/internal/gitstate"
	"github.com/vincentsch/saidran/internal/redact"
	"github.com/vincentsch/saidran/internal/session"
)

type logHealth struct {
	Agent              string `json:"agent"`
	Root               string `json:"root"`
	Found              bool   `json:"found"`
	Sessions           int    `json:"sessions"`
	NewestVersion      string `json:"newest_observed_version"`
	UnsupportedFormats int    `json:"unsupported_formats"`
	UnsupportedEntries int    `json:"unsupported_entries"`
	Fix                string `json:"fix"`
}
type health struct {
	Git        string      `json:"git"`
	Repository string      `json:"repository"`
	Logs       []logHealth `json:"logs"`
	Notes      []string    `json:"notes"`
}

func addDoctor(app *rungrad.App, opts Options, r redact.Redactor) {
	app.AddCommand(&rungrad.Command{Use: "doctor", Short: "Check Git and agent log availability", Long: "Show log roots, session counts and newest observed agent format versions.\nDiagnostics use metadata only and never print private session messages.", Examples: []string{"saidran doctor", "saidran doctor --json"}, Related: []string{"saidran sessions", "saidran check"}, Args: cobra.NoArgs, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}, Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		root, err := gitstate.Root(opts.Dir)
		if err != nil {
			return err
		}
		version, err := exec.Command("git", "--version").Output()
		if err != nil {
			return fmt.Errorf("Could not read Git version; check your Git installation")
		}
		d, err := agents.Discover(root)
		if err != nil {
			return err
		}
		result := health{Git: strings.TrimSpace(string(version)), Repository: r.Text(root), Logs: []logHealth{}, Notes: summarizeNotes(d.Notes)}
		if len(result.Notes) > 0 {
			result.Notes = append(result.Notes[:len(result.Notes)-1], "Fix: Check log folder permissions and record a new session. Update Codex for older formats.")
		}
		for i, path := range d.Roots {
			agent := []string{"claude-code", "codex", "opencode", "pi"}[i]
			env := []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "XDG_DATA_HOME", "PI_CODING_AGENT_DIR"}[i]
			info, statErr := os.Stat(path)
			h := logHealth{Agent: agent, Root: r.Text(path), Found: statErr == nil && info.IsDir(), NewestVersion: "unknown"}
			for _, s := range d.Sessions {
				if s.Agent == agent {
					h.Sessions++
				}
			}
			var newest time.Time
			for _, observation := range d.Observed {
				if observation.Agent != agent {
					continue
				}
				if !observation.Supported {
					h.UnsupportedFormats++
				}
				h.UnsupportedEntries += observation.Unsupported
				if !observation.Ended.Before(newest) {
					newest = observation.Ended
					h.NewestVersion = r.Text(observation.Version)
					if h.NewestVersion == "" {
						h.NewestVersion = "not recorded"
					}
				}
			}
			switch {
			case h.UnsupportedFormats > 0:
				h.Fix = "Record a supported session with " + session.AgentLabel(agent) + "; check its version and preserve the complete log layout."
				if agent == "codex" {
					h.Fix = "Update Codex and record a new session; older logs lack completed-item events."
				}
			case h.UnsupportedEntries > 0:
				h.Fix = "Update Saidran to read newer log entries; check report completeness notes."
			case !h.Found:
				location := "the agent's configuration folder"
				if agent == "opencode" {
					location = "the data directory containing opencode/storage"
				}
				if agent == "pi" {
					location = "the agent directory containing sessions"
				}
				h.Fix = "Record a session or set " + env + " to " + location + "."
			case h.Sessions == 0:
				h.Fix = "Record a session inside this repository, or check " + env + "."
			default:
				h.Fix = "No change needed. Use saidran sessions to select a recording."
			}
			result.Logs = append(result.Logs, h)
		}
		return f.WriteResult(result, func(w io.Writer) {
			fmt.Fprintf(w, "Git: %s\n  Fix: No change needed.\nRepository: %s\n  Fix: No change needed.\n", result.Git, result.Repository)
			for _, h := range result.Logs {
				fmt.Fprintf(w, "%s logs: %s (found: %t)\n  Sessions for this repository: %d\n  Newest observed format version: %s\n  Unsupported formats: %d; unsupported entries: %d\n  Fix: %s\n", session.AgentLabel(h.Agent), h.Root, h.Found, h.Sessions, h.NewestVersion, h.UnsupportedFormats, h.UnsupportedEntries, h.Fix)
			}
			for _, note := range result.Notes {
				fmt.Fprintln(w, note)
			}
		})
	}})
}
