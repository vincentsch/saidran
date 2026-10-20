package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/assaylight/internal/agents"
	"github.com/vincentsch/assaylight/internal/gitstate"
	"github.com/vincentsch/assaylight/internal/redact"
	"github.com/vincentsch/assaylight/internal/render"
	"github.com/vincentsch/assaylight/internal/report"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/rungrad/output"
	"golang.org/x/term"
)

// Run resolves output aliases before rungrad sees arguments, including invalid
// invocations, so JSON errors and prompt suppression have identical behavior.
func Run(version string, opts Options, args []string, stdout, stderr io.Writer) int {
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			normalized = append(normalized, args[i:]...)
			break
		}
		if a == "--format=json" {
			normalized = append(normalized, "--json")
			continue
		}
		if a == "--format" && i+1 < len(args) && args[i+1] == "json" {
			normalized = append(normalized, "--json")
			i++
			continue
		}
		normalized = append(normalized, a)
	}
	return New(version, opts).Run(normalized, stdout, stderr)
}
func addCheck(app *rungrad.App, version string, opts Options, r redact.Redactor) {
	configure := func(cmd *cobra.Command) {
		cmd.Flags().String("session", "", "Select a session ID or agent:ID")
		cmd.Flags().String("transcript", "", "Read an explicit session recording")
		cmd.Flags().String("base", "", "Compare against this Git revision")
		cmd.Flags().String("format", "", "Output format: text or json")
		cmd.Flags().String("out", "", "Write report.json directly into this directory")
		cmd.Flags().Bool("no-write", false, "Print evidence without writing a report")
	}
	run := func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		get := func(name string) string { v, _ := cmd.Flags().GetString(name); return v }
		format := get("format")
		if format != "" && format != "text" {
			return rungrad.NewError(rungrad.ExitUsage, "Choose --format text or --format json.")
		}
		if format == "text" && f.Flags.JSON {
			return rungrad.NewError(rungrad.ExitUsage, "Conflicting output formats: choose text or JSON.")
		}
		if get("session") != "" && get("transcript") != "" {
			return rungrad.NewError(rungrad.ExitUsage, "Choose either --session or --transcript.")
		}
		root, err := gitstate.Root(opts.Dir)
		if err != nil {
			return err
		}
		var d agents.Discovery
		if get("transcript") == "" {
			d, err = agents.Discover(root)
			if err != nil {
				return err
			}
		}
		transcript := get("transcript")
		if transcript != "" && !filepath.IsAbs(transcript) {
			transcript = filepath.Join(opts.Dir, transcript)
		}
		selected, err := agents.Select(d.Sessions, get("session"), transcript, opts.Now(), f)
		if err != nil {
			return err
		}
		s := selected.Session
		if !agents.Within(root, s.Cwd) {
			return rungrad.NewError(rungrad.ExitNotFound, "The selected session belongs to another repository.")
		}
		state, err := gitstate.Inspect(root, get("base"), s.Ended)
		if err != nil {
			return err
		}
		notes := []string{}
		if selected.Note != "" {
			notes = append(notes, selected.Note)
		}
		model := report.Build(version, opts.Now(), state, s, notes)
		safe, err := model.SafeValue(r)
		if err != nil {
			return err
		}
		noWrite, _ := cmd.Flags().GetBool("no-write")
		dir := get("out")
		if dir == "" {
			// Session IDs come from local recordings; keep them within one path component.
			if s.ID == "" || s.ID == "." || s.ID == ".." || strings.ContainsAny(s.ID, "/\\") {
				return rungrad.NewError(rungrad.ExitAPI, "The session has an invalid ID; choose another recording.")
			}
			dir = filepath.Join(state.GitDir, "assaylight", s.Agent+"-"+s.ID)
		} else if !filepath.IsAbs(dir) {
			dir = filepath.Join(opts.Dir, dir)
		}
		if !noWrite {
			if f.DryRun() {
				return f.WritePreview(output.DryRunPreview{Method: "write", Path: r.Text(filepath.Join(dir, "report.json"))})
			}
			if err = report.Write(dir, safe); err != nil {
				return fmt.Errorf("Could not write report.json: %w", err)
			}
		}
		// Human and machine outputs consume the same redacted model.
		b, err := json.Marshal(safe)
		if err != nil {
			return err
		}
		var display report.Report
		if err = json.Unmarshal(b, &display); err != nil {
			return err
		}
		return f.WriteResult(safe, func(w io.Writer) {
			width := 100
			if file, ok := f.Stdout.(*os.File); ok {
				if columns, _, err := term.GetSize(int(file.Fd())); err == nil {
					width = columns
				}
			}
			mode := f.TerminalMode()
			if _, set := os.LookupEnv("NO_COLOR"); set {
				mode.Color = false
			}
			render.Text(w, display, r.Text(agents.DisplayID(s, d.Sessions)), width, mode)
			if !noWrite {
				fmt.Fprintf(w, "Report: %s\n", r.Text(filepath.Join(dir, "report.json")))
			}
		})
	}
	app.AddCommand(&rungrad.Command{Use: "check", Short: "Show session evidence and repository changes", Long: "Compare repository changes with recorded commands and edits.\nWrites a local JSON evidence report; --no-write only prints it.\nReports summarize local evidence and do not certify correctness.", Examples: []string{"assaylight check", "assaylight check --transcript session.jsonl --json", "assaylight check --base origin/main --no-write"}, Related: []string{"assaylight sessions"}, Args: cobra.NoArgs, Mutates: true, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}, Configure: configure, Run: run})
	root := app.Root()
	configure(root)
	root.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return rungrad.NewError(rungrad.ExitUsage, "Unknown command: "+args[0]+". Use assaylight --help.")
		}
		return nil
	}
	root.RunE = func(cmd *cobra.Command, args []string) error { return run(app.Factory(), cmd, args) }
	if root.Annotations == nil {
		root.Annotations = map[string]string{}
	}
	root.Annotations[rungrad.AnnotationMutates] = "true"
	root.Annotations[rungrad.AnnotationOutputs] = "human,json"
}
