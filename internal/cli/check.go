package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/gaugestone/internal/agents"
	"github.com/vincentsch/gaugestone/internal/demo"
	"github.com/vincentsch/gaugestone/internal/gitstate"
	"github.com/vincentsch/gaugestone/internal/redact"
	"github.com/vincentsch/gaugestone/internal/render"
	"github.com/vincentsch/gaugestone/internal/report"
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

type outputFormat struct {
	value string
}

func (v *outputFormat) String() string { return v.value }
func (v *outputFormat) Type() string   { return "string" }
func (v *outputFormat) Set(value string) error {
	// The empty default also resets the value when rungrad reuses a command.
	if v.value != "" && value != "" && value != v.value {
		return fmt.Errorf("Conflicting output formats: choose one format.")
	}
	v.value = value
	return nil
}

func addCheck(app *rungrad.App, version string, opts Options, r redact.Redactor) {
	configure := func(cmd *cobra.Command) {
		cmd.Flags().String("fail-on", "", "Exit 10 after reporting matching verdicts or active finding codes (comma-separated)")
		cmd.Flags().String("session", "", "Select a session ID or agent:ID")
		cmd.Flags().String("transcript", "", "Read an explicit session recording")
		cmd.Flags().String("base", "", "Compare against this Git revision")
		cmd.Flags().Var(&outputFormat{}, "format", "Output format: text, html, markdown or json")
		cmd.Flags().String("out", "", "Write report.html, report.md and report.json into this directory")
		cmd.Flags().Bool("no-write", false, "Print evidence without writing a report")
		cmd.Flags().Bool("open", false, "Open the written HTML report in your browser")
	}
	execute := func(opts Options, f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		get := func(name string) string { v, _ := cmd.Flags().GetString(name); return v }
		var policy map[string]bool
		if cmd.Flags().Changed("fail-on") {
			var err error
			policy, err = parsePolicy(get("fail-on"))
			if err != nil {
				return err
			}
		}
		format := get("format")
		if format != "" && format != "text" && format != "html" && format != "markdown" {
			return rungrad.NewError(rungrad.ExitUsage, "Choose --format text, html, markdown or json.")
		}
		if format != "" && f.Flags.JSON {
			return rungrad.NewError(rungrad.ExitUsage, "Conflicting output formats: choose one format.")
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
		store, err := report.LoadDismissals(state.GitDir)
		if err != nil {
			return err
		}
		model.ApplyDismissals(store)
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
			dir = filepath.Join(state.GitDir, "gaugestone", s.Agent+"-"+s.ID)
		} else if !filepath.IsAbs(dir) {
			dir = filepath.Join(opts.Dir, dir)
		}
		// Every renderer consumes the same redacted model.
		b, err := json.Marshal(safe)
		if err != nil {
			return err
		}
		var display report.Report
		if err = json.Unmarshal(b, &display); err != nil {
			return err
		}
		if !noWrite {
			if f.DryRun() {
				return f.WritePreview(output.DryRunPreview{Method: "write", Path: r.Text(filepath.Join(dir, "report.html"))})
			}
			if err = render.Write(dir, display); err != nil {
				return fmt.Errorf("Could not write reports: %w", err)
			}
			open, _ := cmd.Flags().GetBool("open")
			if open {
				opener := opts.Open
				if opener == nil {
					opener = openReport
				}
				if err = opener(filepath.Join(dir, "report.html")); err != nil {
					return fmt.Errorf("Reports saved, but the browser could not open report.html: %w", err)
				}
			}
		}
		if format == "html" {
			if err := render.HTML(f.Stdout, display); err != nil {
				return err
			}
			return policyResult(model, policy)
		}
		if format == "markdown" {
			if err := render.Markdown(f.Stdout, display); err != nil {
				return err
			}
			return policyResult(model, policy)
		}

		err = f.WriteResult(safe, func(w io.Writer) {
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
				fmt.Fprintf(w, "Report: %s\n", r.Text(filepath.Join(dir, "report.html")))
			}
		})
		if err != nil {
			return err
		}
		return policyResult(model, policy)
	}
	run := func(f *rungrad.Factory, cmd *cobra.Command, args []string) error { return execute(opts, f, cmd, args) }
	app.AddCommand(&rungrad.Command{Use: "demo", Short: "Try a recorded session without agent history", Long: "Extract a recorded Go project to a temporary directory and write all three reports.\nThe example stays there until you delete it.", Examples: []string{"gaugestone demo", "gaugestone demo --json"}, Related: []string{"gaugestone check", "gaugestone doctor"}, Args: cobra.NoArgs, Mutates: true, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}, Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		if f.DryRun() {
			return f.WritePreview(output.DryRunPreview{Method: "extract", Path: "temporary demo directory"})
		}
		repo, transcript, err := demo.Extract()
		if err != nil {
			return err
		}
		if !f.Flags.JSON {
			fmt.Fprintln(f.Stdout, "Demo: a recorded Claude Code session on a small Go project. Nothing here touches your repositories.")
		}
		check := &cobra.Command{}
		configure(check)
		_ = check.Flags().Set("transcript", transcript)
		demoOpts := opts
		demoOpts.Dir = repo
		return execute(demoOpts, f, check, nil)
	}})
	app.AddCommand(&rungrad.Command{Use: "check", Short: "Show session evidence and repository changes", Long: "Compare repository changes with recorded commands and edits.\nWrites local HTML, Markdown and JSON reports; --no-write only prints evidence.\nReports summarize local evidence and do not certify correctness.", Examples: []string{"gaugestone check", "gaugestone check --transcript session.jsonl --json", "gaugestone check --base origin/main --no-write"}, Related: []string{"gaugestone sessions"}, Args: cobra.NoArgs, Mutates: true, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}, Configure: configure, Run: run})
	root := app.Root()
	configure(root)
	root.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return rungrad.NewError(rungrad.ExitUsage, "Unknown command: "+args[0]+". Use gaugestone --help.")
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
