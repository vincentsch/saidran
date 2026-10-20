package cli

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentsch/gaugestone/internal/findings"
	"github.com/vincentsch/gaugestone/internal/gitstate"
	"github.com/vincentsch/gaugestone/internal/redact"
	"github.com/vincentsch/gaugestone/internal/report"
	"github.com/vincentsch/rungrad"
)

func dismissalLocation(dir string) (string, string, error) {
	root, err := gitstate.Root(dir)
	if err != nil {
		return "", "", err
	}
	b, err := exec.Command("git", "-C", root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return "", "", fmt.Errorf("Could not locate this worktree's Git directory")
	}
	return root, strings.TrimSpace(string(b)), nil
}
func addDismissals(app *rungrad.App, opts Options, r redact.Redactor) {
	app.AddCommand(&rungrad.Command{Use: "dismiss <finding-id>", Short: "Mark a finding as intentional", Long: "Resolve a finding from the current diff, without session logs.\nUse F- plus a full ID or a unique prefix of at least six hex characters.\nReasons are redacted before storage in this worktree's Git directory.", Examples: []string{`gaugestone dismiss F-0123456789ab --reason "Intentional skip"`, `gaugestone dismiss F-012345 --reason test --dry-run`}, Related: []string{"gaugestone dismissals", "gaugestone check"}, Args: cobra.ExactArgs(1), Mutates: true, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}, Configure: func(cmd *cobra.Command) {
		cmd.Flags().String("reason", "", "Why this finding is intentional (required)")
		cmd.Flags().String("base", "", "Compare against this Git revision")
	}, Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		if strings.TrimSpace(reason) == "" {
			return rungrad.NewError(rungrad.ExitUsage, "Give a reason with --reason.")
		}
		root, gitDir, err := dismissalLocation(opts.Dir)
		if err != nil {
			return err
		}
		base, _ := cmd.Flags().GetString("base")
		state, err := gitstate.Inspect(root, base, time.Time{})
		if err != nil {
			return err
		}
		current := findings.Detect(state.Files)
		ids := make([]string, len(current))
		for i, v := range current {
			ids[i] = v.ID
		}
		i, err := report.ResolveFinding(args[0], ids)
		if err != nil {
			return err
		}
		finding := current[i]
		store, err := report.LoadDismissals(gitDir)
		if err != nil {
			return err
		}
		entry := report.Dismissal{ID: finding.ID, Reason: r.Text(reason), Time: opts.Now(), Code: finding.Code, Path: r.Text(finding.Path), Context: r.Text(finding.Context)}
		if f.DryRun() {
			preview := fmt.Sprintf("Dry run: would dismiss %s (%s in %s:%d). Nothing was written.", finding.ID, strings.ReplaceAll(finding.Code, "_", " "), r.Text(finding.Path), finding.Line)
			return f.WriteResult(map[string]any{"dry_run": true, "preview": preview}, func(w io.Writer) { fmt.Fprintln(w, preview) })
		}
		replaced := false
		for i := range store.Entries {
			if store.Entries[i].ID == entry.ID {
				store.Entries[i] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			store.Entries = append(store.Entries, entry)
		}
		if err = store.Save(gitDir); err != nil {
			return err
		}
		return f.WriteResult(entry, func(w io.Writer) { fmt.Fprintf(w, "Dismissed %s: %s\n", entry.ID, entry.Reason) })
	}})
	app.AddCommand(&rungrad.Command{Use: "dismissals", Short: "List or remove intentional findings", Long: "List reasons stored in this worktree's Git directory.\n--remove accepts a full finding ID or a unique F- prefix of at least six hex characters.", Examples: []string{"gaugestone dismissals --json", "gaugestone dismissals --remove F-012345"}, Related: []string{"gaugestone dismiss", "gaugestone check"}, Args: cobra.NoArgs, Mutates: true, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}, Configure: func(cmd *cobra.Command) { cmd.Flags().String("remove", "", "Remove a stored dismissal") }, Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		_, gitDir, err := dismissalLocation(opts.Dir)
		if err != nil {
			return err
		}
		store, err := report.LoadDismissals(gitDir)
		if err != nil {
			return err
		}
		ref, _ := cmd.Flags().GetString("remove")
		if cmd.Flags().Changed("remove") {
			ids := make([]string, len(store.Entries))
			for i, v := range store.Entries {
				ids[i] = v.ID
			}
			i, err := report.ResolveFinding(ref, ids)
			if err != nil {
				return err
			}
			id := store.Entries[i].ID
			if f.DryRun() {
				preview := "Dry run: would remove dismissal " + id + ". Nothing was written."
				return f.WriteResult(map[string]any{"dry_run": true, "preview": preview}, func(w io.Writer) { fmt.Fprintln(w, preview) })
			}
			store.Entries = append(store.Entries[:i], store.Entries[i+1:]...)
			if err = store.Save(gitDir); err != nil {
				return err
			}
		}
		// Redact again in case an older file was written with another home directory.
		for i := range store.Entries {
			store.Entries[i].Reason = r.Text(store.Entries[i].Reason)
			store.Entries[i].Path = r.Text(store.Entries[i].Path)
			store.Entries[i].Context = r.Text(store.Entries[i].Context)
		}
		return f.WriteResult(store, func(w io.Writer) {
			if len(store.Entries) == 0 {
				fmt.Fprintln(w, "No dismissals.")
			}
			for _, d := range store.Entries {
				fmt.Fprintf(w, "%s · %s in %s · %s\n", d.ID, d.Code, d.Path, d.Reason)
			}
		})
	}})
}
