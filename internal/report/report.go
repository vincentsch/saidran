// Package report assembles the shared evidence model used by all output formats.
package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/vincentsch/saidran/internal/claims"
	"github.com/vincentsch/saidran/internal/commands"
	"github.com/vincentsch/saidran/internal/findings"
	"github.com/vincentsch/saidran/internal/gitstate"
	"github.com/vincentsch/saidran/internal/redact"
	"github.com/vincentsch/saidran/internal/session"
)

type Session struct {
	Agent       string    `json:"agent"`
	Version     string    `json:"version"`
	ID          string    `json:"id"`
	Started     time.Time `json:"started"`
	Ended       time.Time `json:"ended"`
	Models      []string  `json:"models"`
	PromptCount int       `json:"prompt_count"`
	Prompts     []string  `json:"prompts"`
}
type Completeness struct {
	Unsupported  int      `json:"unsupported_entries"`
	MissingExits int      `json:"missing_exit_codes"`
	Notes        []string `json:"notes"`
}
type Dismissal struct {
	ID      string    `json:"id"`
	Reason  string    `json:"reason"`
	Time    time.Time `json:"time,omitempty"`
	Code    string    `json:"code,omitempty"`
	Path    string    `json:"path,omitempty"`
	Context string    `json:"context,omitempty"`
}

type Report struct {
	Type         string                     `json:"type"`
	Version      string                     `json:"tool_version"`
	Generated    time.Time                  `json:"generated"`
	Repo         gitstate.State             `json:"repo"`
	Session      *Session                   `json:"session"`
	Completeness Completeness               `json:"completeness"`
	Commands     []commands.Command         `json:"commands"`
	Ledger       []commands.Entry           `json:"ledger"`
	Edits        []commands.Entry           `json:"edits"`
	Groups       map[string][]gitstate.File `json:"groups"`
	Findings     []findings.Finding         `json:"findings"`
	Claims       []claims.Claim             `json:"claims"`
	AdmittedGaps []claims.Gap               `json:"admitted_gaps"`
	Dismissals   []Dismissal                `json:"dismissals"`
}

func Build(version string, now time.Time, state gitstate.State, s *session.Session, notes []string) Report {
	r := Report{Type: "saidran.report/v1", Version: version, Generated: now, Repo: state, Groups: map[string][]gitstate.File{}, Findings: findings.Detect(state.Files), Commands: []commands.Command{}, Ledger: []commands.Entry{}, Edits: []commands.Entry{}, Claims: []claims.Claim{}, AdmittedGaps: []claims.Gap{}, Dismissals: []Dismissal{}}
	r.Completeness.Notes = append([]string{}, notes...)
	for _, group := range []string{"tests", "ci", "lockfiles", "config", "docs", "source", "other"} {
		r.Groups[group] = []gitstate.File{}
	}
	for _, file := range state.Files {
		for _, h := range file.Hunks {
			for _, line := range h.Lines {
				if line.Kind == '+' {
					file.Additions++
				}
				if line.Kind == '-' {
					file.Deletions++
				}
			}
		}
		r.Groups[file.Group] = append(r.Groups[file.Group], file)
	}
	if s == nil {
		return r
	}
	home, _ := os.UserHomeDir()
	redactor := redact.Redactor{Home: home}
	r.Session = &Session{Agent: s.Agent, Version: s.AgentVersion, ID: s.ID, Started: s.Started, Ended: s.Ended, Models: append([]string{}, s.Models...), PromptCount: len(s.Prompts), Prompts: []string{}}
	for _, p := range s.Prompts {
		r.Session.Prompts = append(r.Session.Prompts, redactor.Prompt(p.Text))
	}
	r.Completeness.Unsupported = s.Unsupported
	r.Completeness.Notes = append(r.Completeness.Notes, s.Notes...)
	ledger := commands.Build(s)
	r.Claims, r.AdmittedGaps = claims.Evaluate(state.Root, s, ledger)
	r.Commands = append(r.Commands, ledger.Commands...)
	r.Ledger = append(r.Ledger, ledger.Entries...)
	for i := range r.Commands {
		c := *r.Commands[i].Original
		c.Output = r.Commands[i].Output
		r.Commands[i].Original = &c
		if c.ExitCode == nil {
			r.Completeness.MissingExits++
		}
	}
	for _, entry := range ledger.Entries {
		if entry.Edit != nil {
			r.Edits = append(r.Edits, entry)
		}
	}
	return r
}

// SafeValue redacts individual string values, never serialized JSON syntax. That
// preserves commit hashes and IDs and applies equally to stdout and saved files.
func (r Report) SafeValue(redactor redact.Redactor) (any, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var value any
	if err = json.Unmarshal(b, &value); err != nil {
		return nil, err
	}
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case string:
			return redactor.Text(x)
		case []any:
			for i := range x {
				x[i] = walk(x[i])
			}
		case map[string]any:
			for k, v := range x {
				x[k] = walk(v)
			}
		}
		return v
	}
	return walk(value), nil
}
func Write(dir string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return WriteDocument(dir, "report.json", b)
}

// WriteDocument replaces one complete report without exposing a partial file.
func WriteDocument(dir, document string, b []byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".report-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, document))
}
