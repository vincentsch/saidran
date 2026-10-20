package agents

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vincentsch/gaugestone/internal/agents/claude"
	"github.com/vincentsch/gaugestone/internal/agents/codex"
	"github.com/vincentsch/gaugestone/internal/agents/opencode"
	"github.com/vincentsch/gaugestone/internal/agents/pi"
	"github.com/vincentsch/gaugestone/internal/pathutil"
	"github.com/vincentsch/gaugestone/internal/redact"
	"github.com/vincentsch/gaugestone/internal/session"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/rungrad/choose"
)

func Read(file string) (*session.Session, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	for {
		var o struct {
			Type      string `json:"type"`
			SessionID string `json:"sessionId"`
			ID        string `json:"id"`
			Directory string `json:"directory"`
		}
		if err := decoder.Decode(&o); err != nil {
			if err != io.EOF {
				return nil, fmt.Errorf("%s: malformed session JSON: %w", file, err)
			}
			break
		}
		switch {
		case o.Type == "session_meta":
			return codex.Read(file)
		case o.Type == "session":
			return pi.Read(file)
		case o.Directory != "" && o.ID != "":
			return opencode.Read(file)
		case o.SessionID != "" || o.Type == "assistant" || o.Type == "user":
			return claude.Read(file)
		}
	}
	return nil, fmt.Errorf("%s: unsupported session format; use a Claude Code, current Codex, OpenCode 1.1.4 session JSON or Pi 0.85.1 journal", file)
}

type Observation struct {
	Agent, Version string
	Ended          time.Time
	Unsupported    int
	Supported      bool
}

type Discovery struct {
	Observed []Observation
	Sessions []*session.Session
	Roots    []string
	Notes    []string
}

func Roots() []string {
	home, _ := os.UserHomeDir()
	claudeRoot := os.Getenv("CLAUDE_CONFIG_DIR")
	if claudeRoot == "" {
		claudeRoot = filepath.Join(home, ".claude")
	}
	codexRoot := os.Getenv("CODEX_HOME")
	if codexRoot == "" {
		codexRoot = filepath.Join(home, ".codex")
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".local", "share")
	}
	piRoot := os.Getenv("PI_CODING_AGENT_DIR")
	if piRoot == "" {
		piRoot = filepath.Join(home, ".pi", "agent")
	}
	return []string{filepath.Join(claudeRoot, "projects"), filepath.Join(codexRoot, "sessions"), filepath.Join(xdg, "opencode", "storage"), filepath.Join(piRoot, "sessions")}
}
func Within(repo, path string) bool {
	return pathutil.Within(repo, path)
}

func Discover(repo string) (Discovery, error) {
	d := Discovery{Roots: Roots()}
	identities := map[string]bool{}
	for n, configuredRoot := range d.Roots {
		root, err := filepath.EvalSymlinks(configuredRoot)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return d, err
		}
		walkRoot := root
		if n == 2 {
			walkRoot = filepath.Join(root, "session")
			if _, err := os.Stat(walkRoot); os.IsNotExist(err) {
				continue
			}
		}
		err = filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "subagents" {
					return filepath.SkipDir
				}
				if n == 0 && path != root {
					rel, _ := filepath.Rel(root, path)
					if strings.Contains(rel, string(filepath.Separator)) {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if (n == 2 && filepath.Ext(path) != ".json") || (n != 2 && filepath.Ext(path) != ".jsonl") {
				return nil
			}
			if n == 0 || n == 1 {
				belongs, e := jsonlCandidate(path, repo)
				if e != nil {
					d.Notes = append(d.Notes, e.Error())
					return nil
				}
				if !belongs {
					return nil
				}
			}
			if n == 2 || n == 3 {
				belongs, e := metadataCandidate(path, repo)
				if e != nil {
					d.Notes = append(d.Notes, e.Error())
					return nil
				}
				if !belongs {
					return nil
				}
			}
			// Confirm the cwd from the recording rather than trusting encoded names.
			s, e := Read(path)
			if s != nil && Within(repo, s.Cwd) {
				d.Observed = append(d.Observed, Observation{Agent: s.Agent, Version: s.AgentVersion, Ended: s.Ended, Unsupported: s.Unsupported, Supported: e == nil})
			}
			if e != nil {
				d.Notes = append(d.Notes, e.Error())
				return nil
			}
			if Within(repo, s.Cwd) {
				key := s.Agent + ":" + s.ID + ":" + s.File
				if !identities[key] {
					d.Sessions = append(d.Sessions, s)
					identities[key] = true
				}
			}
			return nil
		})
		if err != nil {
			return d, err
		}
	}
	sort.SliceStable(d.Sessions, func(i, j int) bool {
		if d.Sessions[i].Ended.Equal(d.Sessions[j].Ended) {
			return d.Sessions[i].Agent+":"+d.Sessions[i].ID < d.Sessions[j].Agent+":"+d.Sessions[j].ID
		}
		return d.Sessions[i].Ended.After(d.Sessions[j].Ended)
	})
	return d, nil
}
func DisplayID(s *session.Session, all []*session.Session) string {
	n := 8
	if len(s.ID) < n {
		n = len(s.ID)
	}
	for ; n < len(s.ID); n++ {
		unique := true
		for _, other := range all {
			if other != s && strings.HasPrefix(other.ID, s.ID[:n]) && other.ID != s.ID {
				unique = false
				break
			}
		}
		if unique {
			break
		}
	}
	return s.ID[:n]
}
func Automatic(all []*session.Session, now time.Time) []*session.Session {
	var eligible []*session.Session
	for _, s := range all {
		active := false
		for _, e := range s.Events {
			if e.Command != nil || e.Edit != nil {
				active = true
				break
			}
		}
		if active && !s.Ended.Before(now.Add(-7*24*time.Hour)) {
			eligible = append(eligible, s)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool { return eligible[i].Ended.After(eligible[j].Ended) })
	if len(eligible) > 1 && eligible[0].Agent != eligible[1].Agent && eligible[0].Ended.Sub(eligible[1].Ended) < 10*time.Minute {
		return eligible[:2]
	}
	if len(eligible) > 0 {
		return eligible[:1]
	}
	return nil
}

type Selection struct {
	Session *session.Session
	Note    string
}

func Select(all []*session.Session, ref, transcript string, now time.Time, f *rungrad.Factory) (Selection, error) {
	if ref != "" && transcript != "" {
		return Selection{}, rungrad.NewError(rungrad.ExitUsage, "Choose either --session or --transcript.")
	}
	if transcript != "" {
		s, e := Read(transcript)
		if os.IsNotExist(e) {
			return Selection{}, rungrad.NewError(rungrad.ExitNotFound, "Transcript not found. Check the --transcript path.")
		}
		return Selection{Session: s}, e
	}
	if ref != "" {
		agent, id, qualified := strings.Cut(ref, ":")
		if !qualified {
			id = ref
			agent = ""
		}
		if agent == "claude" {
			agent = "claude-code"
		}
		var matches []*session.Session
		for _, s := range all {
			if (agent == "" || s.Agent == agent) && strings.HasPrefix(s.ID, id) {
				matches = append(matches, s)
			}
		}
		if len(matches) > 1 {
			return Selection{}, rungrad.NewError(rungrad.ExitUsage, "Ambiguous session reference. Use an agent label and a longer ID.")
		}
		if len(matches) == 1 {
			return Selection{Session: matches[0]}, nil
		}
		return Selection{}, rungrad.NewError(rungrad.ExitNotFound, "No matching session. Run gaugestone doctor to check Claude Code, Codex, OpenCode or Pi log roots, or use --transcript.")
	}
	picks := Automatic(all, now)
	if len(picks) == 0 {
		return Selection{}, rungrad.NewError(rungrad.ExitNotFound, "No recent session with commands or edits. Run gaugestone doctor to check Claude Code, Codex, OpenCode or Pi log roots, or use --transcript.")
	}
	if len(picks) == 2 {
		if f != nil && f.CanPrompt() {
			return chooseSession(picks, all, f)
		}
		home, _ := os.UserHomeDir()
		r := redact.Redactor{Home: home}
		return Selection{picks[0], r.Text(fmt.Sprintf("%s:%s and %s:%s ended less than ten minutes apart; using the newest session.", picks[0].Agent, DisplayID(picks[0], all), picks[1].Agent, DisplayID(picks[1], all)))}, nil
	}
	return Selection{Session: picks[0]}, nil
}

func chooseSession(picks, all []*session.Session, f *rungrad.Factory) (Selection, error) {
	home, _ := os.UserHomeDir()
	r := redact.Redactor{Home: home}
	opts := make([]choose.Option, len(picks))
	for i, s := range picks {
		opts[i] = choose.Option{Label: r.Text(s.Agent + ":" + DisplayID(s, all)), Hint: s.Ended.Format(time.RFC3339)}
	}
	idx, err := f.Chooser().Choose("Two agents finished less than ten minutes apart. Which session?", opts, 0)
	if err != nil {
		return Selection{}, err
	}
	return Selection{Session: picks[idx]}, nil
}

func jsonlCandidate(path, repo string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		var o struct {
			Type    string `json:"type"`
			Cwd     string `json:"cwd"`
			Payload struct {
				Cwd string `json:"cwd"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &o) == nil {
			v := o.Cwd
			if o.Type == "session_meta" || o.Type == "turn_context" {
				v = o.Payload.Cwd
			}
			if v == "" {
				if errors.Is(err, io.EOF) {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				continue
			}
			if strings.HasPrefix(v, "file://") {
				u, e := url.Parse(v)
				if e != nil {
					return false, e
				}
				v = filepath.FromSlash(u.Path)
			}
			return Within(repo, v), nil
		}
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
}

// OpenCode and Pi put repository membership in the first JSON object. Avoid
// loading unrelated repositories' message trees or potentially large journals.
func metadataCandidate(path, repo string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var header struct {
		Cwd       string `json:"cwd"`
		Directory string `json:"directory"`
	}
	if err = json.NewDecoder(f).Decode(&header); err != nil {
		return false, fmt.Errorf("%s: malformed JSON record", path)
	}
	cwd := header.Cwd
	if header.Directory != "" {
		cwd = header.Directory
	}
	return Within(repo, cwd), nil
}
