package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentsch/gaugestone/internal/agents"
	"github.com/vincentsch/gaugestone/internal/gitstate"
	"github.com/vincentsch/gaugestone/internal/redact"
	"github.com/vincentsch/gaugestone/internal/session"
	"github.com/vincentsch/rungrad"
)

type Options struct {
	Now  func() time.Time
	Dir  string
	Open func(string) error
}
type Row struct {
	Agent         string    `json:"agent"`
	ID            string    `json:"id"`
	Started       time.Time `json:"started"`
	Ended         time.Time `json:"ended"`
	Duration      string    `json:"duration"`
	Prompt        string    `json:"prompt"`
	Commands      int       `json:"commands"`
	Edits         int       `json:"edits"`
	AutomaticPick bool      `json:"automatic_pick"`
}
type Listing struct {
	Sessions []Row    `json:"sessions"`
	Searched []string `json:"searched"`
	Notes    []string `json:"notes,omitempty"`
}

func New(version string, opts Options) *rungrad.App {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Dir == "" {
		opts.Dir = "."
	}
	home, _ := os.UserHomeDir()
	r := redact.Redactor{Home: home}
	app := rungrad.New(rungrad.AppConfig{Name: "gaugestone", Short: "See what your coding agent checked", Long: "Review Claude Code, Codex, OpenCode and Pi evidence and repository changes.\nRun check (the default) for a report, or sessions to list recordings.", Version: version, ErrorPolicy: &rungrad.ErrorPolicy{Render: func(c rungrad.ErrorContext) error {
		msg := r.Text(c.Err.Error())
		if c.ExitCode == 10 {
			_, e := fmt.Fprintln(c.Stderr, msg)
			return e
		}
		if c.MachineOutput {
			return json.NewEncoder(c.Stderr).Encode(map[string]any{"error": msg, "exit_code": c.ExitCode})
		}
		_, e := fmt.Fprintln(c.Stderr, "Error: "+msg)
		return e
	}}})
	app.AddCommand(&rungrad.Command{Use: "sessions", Short: "List agent sessions for this repository", Long: "Show prompts, times, command counts and confirmed edit counts.\nAutomatic selection uses active sessions from the last seven days.\nSet CLAUDE_CONFIG_DIR, CODEX_HOME, XDG_DATA_HOME or PI_CODING_AGENT_DIR to read another log root.", Examples: []string{"gaugestone sessions", "gaugestone sessions --json"}, Related: []string{"gaugestone --help"}, Args: cobra.NoArgs, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}, Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		root, err := gitstate.Root(opts.Dir)
		if err != nil {
			return err
		}
		d, err := agents.Discover(root)
		if err != nil {
			return err
		}
		result := Listing{Sessions: []Row{}}
		for _, path := range d.Roots {
			result.Searched = append(result.Searched, r.Text(path))
		}
		for _, note := range d.Notes {
			result.Notes = append(result.Notes, r.Text(note))
		}
		picks := agents.Automatic(d.Sessions, opts.Now())
		for _, s := range d.Sessions {
			for _, note := range s.Notes {
				result.Notes = append(result.Notes, r.Text(note))
			}
			if s.Unsupported > 0 {
				result.Notes = append(result.Notes, fmt.Sprintf("%s:%s has %d unsupported entries.", s.Agent, r.Text(agents.DisplayID(s, d.Sessions)), s.Unsupported))
			}
			row := row(s, d.Sessions, r)
			row.AutomaticPick = len(picks) > 0 && picks[0] == s
			result.Sessions = append(result.Sessions, row)
		}
		if len(picks) == 2 {
			result.Notes = append(result.Notes, "Two agents finished less than ten minutes apart. Interactive selection asks which to use; otherwise the newest session is picked.")
		}
		return f.WriteResult(result, func(w io.Writer) {
			if len(result.Sessions) == 0 {
				fmt.Fprintf(w, "No sessions found. Searched %s. Run gaugestone doctor to check agent logs.\n", strings.Join(result.Searched, ", "))
			} else {
				tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "AGENT\tID\tSTART\tEND\tDURATION\tPROMPT\tCOMMANDS\tEDITS\tPICK")
				for _, row := range result.Sessions {
					pick := ""
					if row.AutomaticPick {
						pick = "*"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\n", session.AgentLabel(row.Agent), row.ID, row.Started.Format(time.RFC3339), row.Ended.Format(time.RFC3339), row.Duration, strings.Join(strings.Fields(row.Prompt), " "), row.Commands, row.Edits, pick)
				}
				tw.Flush()
			}
			for _, n := range summarizeNotes(result.Notes) {
				fmt.Fprintln(w, n)
			}
		})
	}})
	addCheck(app, version, opts, r)
	addDismissals(app, opts, r)
	addDoctor(app, opts, r)
	return app
}
func row(s *session.Session, all []*session.Session, r redact.Redactor) Row {
	v := Row{Agent: s.Agent, ID: r.Text(agents.DisplayID(s, all)), Started: s.Started, Ended: s.Ended, Duration: s.Ended.Sub(s.Started).Round(time.Second).String()}
	if len(s.Prompts) > 0 {
		v.Prompt = redact.Limit(r.Prompt(s.Prompts[0].Text), 60)
	}
	for _, e := range s.Events {
		if e.Command != nil {
			v.Commands++
		}
		if e.Edit != nil {
			v.Edits++
		}
	}
	return v
}

func summarizeNotes(notes []string) []string {
	counts := make([]int, 6)
	var selection []string
	for _, note := range notes {
		switch {
		case strings.Contains(note, "unsupported Codex log format"):
			counts[0]++
		case strings.Contains(note, "malformed JSON record"):
			counts[1]++
		case strings.Contains(note, "incomplete final line skipped"):
			counts[2]++
		case strings.Contains(note, "Missing sub-agent transcript:"):
			counts[3]++
		case strings.HasSuffix(note, "unsupported entries."):
			counts[4]++
		case strings.HasPrefix(note, "Two agents finished less than ten minutes apart."):
			selection = append(selection, note)
		default:
			counts[5]++
		}
	}
	formats := []string{
		"Older Codex recordings skipped: %d. These logs lack completed-item events.",
		"Malformed recordings skipped: %d.",
		"Incomplete final lines skipped: %d.",
		"Missing sub-agent transcripts: %d.",
		"Sessions with unsupported entries: %d.",
		"Other discovery notes: %d.",
	}
	var summary []string
	for i, count := range counts {
		if count > 0 {
			summary = append(summary, fmt.Sprintf(formats[i], count))
		}
	}
	if len(summary) > 0 {
		summary = append(summary, "Use --json for individual note details.")
	}
	return append(summary, selection...)
}
