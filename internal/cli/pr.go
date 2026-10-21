package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/saidran/internal/agents"
	"github.com/vincentsch/saidran/internal/gitstate"
	"github.com/vincentsch/saidran/internal/redact"
	"github.com/vincentsch/saidran/internal/render"
	"github.com/vincentsch/saidran/internal/report"
)

const sectionStart = "<!-- saidran:start -->"
const sectionEnd = "<!-- saidran:end -->"
const prBodyLimit = 65536

type pullRequest struct {
	Number      int    `json:"number"`
	Body        string `json:"body"`
	URL         string `json:"url"`
	BaseRefName string `json:"baseRefName"`
	BaseRefOID  string `json:"baseRefOid"`
}
type prResult struct {
	Section string `json:"section"`
	URL     string `json:"url"`
	Status  string `json:"status"`
}

// Treat broken marker comments as errors too, rather than appending a second section.
var sectionMarkers = regexp.MustCompile(`(?is)<!--\s*saidran\b.*?(?:-->|$)`)

func preservedPRBody(body string) (before, after string, err error) {
	markers := sectionMarkers.FindAllStringIndex(body, -1)
	if len(markers) == 0 {
		before = body
		if body != "" {
			before += "\n\n"
		}
		return before, "", nil
	}
	if len(markers) != 2 || body[markers[0][0]:markers[0][1]] != sectionStart || body[markers[1][0]:markers[1][1]] != sectionEnd {
		return "", "", rungrad.NewError(rungrad.ExitAPI, "The PR has malformed, duplicate or unbalanced Saidran markers. Keep one ordered start/end pair or remove both markers.")
	}
	return body[:markers[0][0]], body[markers[1][1]:], nil
}

func ghOutput(root, gh string, args ...string) ([]byte, error) {
	cmd := exec.Command(gh, args...)
	cmd.Dir = root
	// Authentication is supplied by gh's own configuration. Never include its output in errors.
	return cmd.Output()
}

func readPullRequest(root, gh string) (pullRequest, error) {
	var pr pullRequest
	b, err := ghOutput(root, gh, "pr", "view", "--json", "number,body,url,baseRefName,baseRefOid")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 4 {
			return pr, rungrad.NewError(rungrad.ExitAuth, "GitHub CLI is not authenticated. Run gh auth login.")
		}
		if _, authErr := ghOutput(root, gh, "auth", "status"); authErr != nil {
			return pr, rungrad.NewError(rungrad.ExitAuth, "GitHub CLI is not authenticated. Run gh auth login.")
		}
		if errors.As(err, &exit) && strings.Contains(strings.ToLower(string(exit.Stderr)), "no pull requests found") {
			return pr, rungrad.NewError(rungrad.ExitNotFound, "No pull request found for this branch. Create one with gh pr create.")
		}
		return pr, rungrad.NewError(rungrad.ExitAPI, "Could not read the pull request. Check gh pr view and your network connection.")
	}
	if err := json.Unmarshal(b, &pr); err != nil || pr.Number <= 0 || pr.URL == "" {
		return pr, rungrad.NewError(rungrad.ExitAPI, "GitHub CLI returned an invalid pull request. Check gh pr view.")
	}
	return pr, nil
}

func pullRequestBase(root string, pr pullRequest) (string, error) {
	// Only an object ID identifies the target across forks. Never fetch or use origin's branch.
	if !regexp.MustCompile(`^[0-9a-fA-F]{40,64}$`).MatchString(pr.BaseRefOID) {
		return "", rungrad.NewError(rungrad.ExitAPI, "GitHub CLI returned an invalid PR base commit. Check gh pr view.")
	}
	if _, err := exec.Command("git", "-C", root, "cat-file", "-e", pr.BaseRefOID+"^{commit}").Output(); err != nil {
		return "", rungrad.NewError(rungrad.ExitAPI, "The PR base commit is unavailable locally. Fetch the PR target repository's base branch ("+pr.BaseRefName+") and try again.")
	}
	b, err := exec.Command("git", "-C", root, "merge-base", "HEAD", pr.BaseRefOID).Output()
	if err != nil {
		return "", rungrad.NewError(rungrad.ExitAPI, "Could not find a merge base with the PR target commit. Fetch the PR target repository's base branch and try again.")
	}
	return strings.TrimSpace(string(b)), nil
}

func addPR(app *rungrad.App, version string, opts Options, r redact.Redactor) {
	app.AddCommand(&rungrad.Command{
		Use: "pr", Short: "Preview and update a report section in this branch's pull request",
		Long:     "Build a report in memory and show the section before publication.\nUses the merge base with the PR target commit unless --base is given.\nRequires GitHub CLI (gh); never writes local report files or runs transcript commands.",
		Examples: []string{"saidran pr --dry-run", "saidran pr --confirm", "saidran pr --transcript session.jsonl --dry-run --json"},
		Related:  []string{"saidran check", "saidran sessions"}, Args: cobra.NoArgs,
		Mutates: true, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON},
		Configure: func(cmd *cobra.Command) {
			cmd.Flags().String("session", "", "Select an ID or claude:ID, codex:ID, opencode:ID, pi:ID")
			cmd.Flags().String("transcript", "", "Read a JSONL recording or OpenCode storage/session/<project>/<id>.json")
			cmd.Flags().String("base", "", "Compare directly against this Git revision instead of the PR merge base")
			cmd.Flags().Bool("confirm", false, "Publish the preview without asking")
		},
		Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
			get := func(name string) string { v, _ := cmd.Flags().GetString(name); return v }
			if get("session") != "" && get("transcript") != "" {
				return rungrad.NewError(rungrad.ExitUsage, "Choose either --session or --transcript.")
			}
			root, err := gitstate.Root(opts.Dir)
			if err != nil {
				return err
			}
			gh, err := exec.LookPath("gh")
			if err != nil {
				return rungrad.NewError(rungrad.ExitAuth, "GitHub CLI (gh) is missing. Install gh, then run gh auth login.")
			}
			pr, err := readPullRequest(root, gh)
			if err != nil {
				return err
			}
			before, after, err := preservedPRBody(pr.Body)
			if err != nil {
				return err
			}
			base := get("base")
			if base == "" {
				base, err = pullRequestBase(root, pr)
				if err != nil {
					return err
				}
			}
			var discovery agents.Discovery
			transcript := get("transcript")
			if transcript == "" {
				discovery, err = agents.Discover(root)
				if err != nil {
					return err
				}
			} else if !filepath.IsAbs(transcript) {
				transcript = filepath.Join(opts.Dir, transcript)
			}
			selected, err := agents.Select(discovery.Sessions, get("session"), transcript, opts.Now(), f)
			if err != nil {
				return err
			}
			s := selected.Session
			if !agents.Within(root, s.Cwd) {
				return rungrad.NewError(rungrad.ExitNotFound, "The selected session belongs to another repository.")
			}
			state, err := gitstate.Inspect(root, base, s.Ended)
			if err != nil {
				return err
			}
			if get("base") == "" {
				state.Baseline = gitstate.Baseline{Ref: pr.BaseRefName, Commit: base, Reason: "merge base with PR target commit " + pr.BaseRefOID}
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
			b, err := json.Marshal(safe)
			if err != nil {
				return err
			}
			var display report.Report
			if err = json.Unmarshal(b, &display); err != nil {
				return err
			}
			budget := min(60000, prBodyLimit-len(before)-len(after)-len(sectionStart)-len(sectionEnd)-2)
			var markdown bytes.Buffer
			if err = render.MarkdownWithOptions(&markdown, display, render.MarkdownOptions{ByteBudget: budget, LocalEvidence: true}); err != nil {
				return err
			}
			result := prResult{Section: sectionStart + "\n" + markdown.String() + "\n" + sectionEnd, URL: pr.URL, Status: "previewed"}
			if !f.Flags.JSON {
				if _, err = fmt.Fprintln(f.Stdout, result.Section); err != nil {
					return err
				}
			}
			if !f.DryRun() {
				confirmed, _ := cmd.Flags().GetBool("confirm")
				if err = f.ConfirmDestructive(rungrad.ConfirmOptions{Action: "update the report section in", Target: pr.URL, Confirmed: confirmed}); err != nil {
					return err
				}
				// gh reads the entire body from a private disposable file, never shell source.
				file, err := os.CreateTemp("", "saidran-pr-*")
				if err != nil {
					return err
				}
				defer os.Remove(file.Name())
				if _, err = file.WriteString(before + result.Section + after); err != nil {
					file.Close()
					return err
				}
				if err = file.Close(); err != nil {
					return err
				}
				if _, err = ghOutput(root, gh, "pr", "edit", fmt.Sprint(pr.Number), "--body-file", file.Name()); err != nil {
					var exit *exec.ExitError
					if errors.As(err, &exit) && exit.ExitCode() == 4 {
						return rungrad.NewError(rungrad.ExitAuth, "GitHub CLI is not authenticated. Run gh auth login.")
					}
					return rungrad.NewError(rungrad.ExitAPI, "Could not update the pull request. Check gh authentication, permissions and your network connection.")
				}
				result.Status = "updated"
			}
			if f.Flags.JSON {
				return f.WriteResult(result, nil)
			}
			_, err = fmt.Fprintf(f.Stdout, "\nPull request %s: %s\n", result.Status, r.Text(result.URL))
			return err
		},
	})
}
