package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/gaugestone/internal/agents"
	"github.com/vincentsch/gaugestone/internal/gitstate"
	"github.com/vincentsch/gaugestone/internal/redact"
	"github.com/vincentsch/gaugestone/internal/render"
	"github.com/vincentsch/gaugestone/internal/report"
	"github.com/vincentsch/gaugestone/internal/session"
	"github.com/vincentsch/rungrad"
)

const selfCheck = "Before your final response, run `gaugestone --json --no-write` for your\ncurrent session (use --session or --transcript when you know it) and\nconfirm it picked the right session. For every test, build, lint or type\ncheck you are about to call passing, look at its result and at the edits\nafter it, even when the report lists no claims yet. Run stale or failed\nchecks again, or say plainly what you could not check. Treat results\nmarked \"can't tell\" as unknown. Never change tests just to make this\nreport look better.\n"

func addHooks(app *rungrad.App, version string, opts Options, r redact.Redactor) {
	app.AddCommand(&rungrad.Command{Use: "snippet", Short: "Print agent self-check instructions", Examples: []string{"gaugestone snippet"}, Related: []string{"gaugestone check"}, Args: cobra.NoArgs, OutputModes: []string{"human", "json"}, Run: func(f *rungrad.Factory, _ *cobra.Command, _ []string) error {
		return f.WriteResult(map[string]string{"snippet": selfCheck}, func(w io.Writer) { fmt.Fprint(w, selfCheck) })
	}})
	app.AddCommand(&rungrad.Command{Use: "hook <claude|codex> [json]", Short: "Write a report for an agent completion event", Long: "Claude reads Stop-hook JSON from stdin. Codex reads notify JSON from its argument.\nHooks never block completion; Codex emits no output.", Examples: []string{"gaugestone hook claude", `gaugestone hook codex '{"thread-id":"session-id","cwd":"/repo"}'`}, Related: []string{"gaugestone install"}, Mutates: true, Run: func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		// Provider hooks must also stay nonblocking on malformed payloads.
		if len(args) == 0 {
			return nil
		}
		if args[0] == "codex" {
			if len(args) == 2 && !f.DryRun() {
				_, _ = hookReport(version, opts, "codex", strings.NewReader(args[1]), r)
			}
			return nil
		}
		if args[0] != "claude" {
			return nil
		}
		message := "Gaugestone: expected Stop-hook JSON on stdin."
		if len(args) == 1 && !f.DryRun() {
			var err error
			message, err = hookReport(version, opts, "claude-code", f.Stdin, r)
			if err != nil {
				message = "Gaugestone: could not create report: " + r.Text(err.Error())
			}
		}
		if message != "" {
			return json.NewEncoder(f.Stdout).Encode(map[string]string{"systemMessage": strings.Join(strings.Fields(message), " ")})
		}
		return nil
	}})
}

func hookReport(version string, opts Options, agent string, input io.Reader, r redact.Redactor) (string, error) {
	started := opts.Now()
	var p struct {
		Transcript  string  `json:"transcript_path"`
		Cwd         string  `json:"cwd"`
		Thread      string  `json:"thread-id"`
		ClaudeFinal *string `json:"last_assistant_message"`
		CodexFinal  *string `json:"last-assistant-message"`
	}
	d := json.NewDecoder(input)
	if err := d.Decode(&p); err != nil {
		return "", fmt.Errorf("invalid completion JSON; check the hook payload")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return "", fmt.Errorf("expected one completion JSON object")
	}
	if p.Cwd == "" {
		return "", fmt.Errorf("completion payload has no cwd")
	}
	root, err := gitstate.Root(p.Cwd)
	if err != nil {
		if rungrad.ExitCodeFor(err) == rungrad.ExitNotFound {
			return "", nil
		}
		return "", err
	}
	var s *session.Session
	final := p.ClaudeFinal
	if agent == "claude-code" {
		if p.Transcript == "" {
			return "", fmt.Errorf("completion payload has no transcript_path")
		}
		transcript := p.Transcript
		if !filepath.IsAbs(transcript) {
			transcript = filepath.Join(p.Cwd, transcript)
		}
		s, err = agents.Read(transcript)
	} else {
		final = p.CodexFinal
		if p.Thread == "" {
			return "", fmt.Errorf("completion payload has no thread-id")
		}
		var discovery agents.Discovery
		discovery, err = agents.Discover(root)
		if err == nil {
			for _, candidate := range discovery.Sessions {
				if candidate.Agent == "codex" && candidate.ID == p.Thread {
					if s != nil {
						return "", fmt.Errorf("multiple transcripts match thread-id")
					}
					s = candidate
				}
			}
		}
		if err == nil && s == nil {
			return "", fmt.Errorf("no transcript matches thread-id")
		}
	}
	if err != nil {
		return "", err
	}
	if !agents.Within(root, s.Cwd) {
		return "", nil
	}
	if s.Agent != agent {
		return "", fmt.Errorf("transcript does not match the hook agent")
	}
	if final != nil {
		s.Final = *final
		s.FinalTime = started
		s.Ended = started
	}
	if s.ID == "" || s.ID == "." || s.ID == ".." || strings.ContainsAny(s.ID, "/\\") {
		return "", fmt.Errorf("invalid session ID")
	}
	state, err := gitstate.Inspect(root, "", s.Ended)
	if err != nil {
		return "", err
	}
	model := report.Build(version, started, state, s, nil)
	store, err := report.LoadDismissals(state.GitDir)
	if err != nil {
		return "", err
	}
	model.ApplyDismissals(store)
	safe, err := model.SafeValue(r)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(safe)
	if err != nil {
		return "", err
	}
	var display report.Report
	if err = json.Unmarshal(b, &display); err != nil {
		return "", err
	}
	dir := filepath.Join(state.GitDir, "gaugestone", s.Agent+"-"+s.ID)
	if err = render.Write(dir, display); err != nil {
		return "", err
	}
	summary := fmt.Sprintf("%d claims, %d commands, %d findings", len(display.Claims), len(display.Commands), len(display.Findings))
	if len(display.Claims) == 0 {
		summary = "no claims in the final message; review recorded checks and later edits"
	} else {
		var verdicts []string
		for _, c := range display.Claims {
			verdicts = append(verdicts, strings.ReplaceAll(c.Verdict, "_", " "))
		}
		summary += " (" + strings.Join(verdicts, ", ") + ")"
	}
	return "Gaugestone: " + summary + ". Report: " + r.Text(filepath.Join(dir, "report.html")), nil
}
