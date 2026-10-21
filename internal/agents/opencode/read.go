// Package opencode reads OpenCode's file-based session storage.
package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	j "github.com/vincentsch/saidran/internal/agents/logjson"
	"github.com/vincentsch/saidran/internal/session"
)

type record struct {
	object j.Object
	source session.Source
}

func object(file string) (j.Object, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var o j.Object
	if err = json.Unmarshal(b, &o); err != nil {
		return nil, fmt.Errorf("%s: malformed JSON record", file)
	}
	return o, nil
}
func records(dir string) ([]record, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var result []record
	for _, file := range files {
		o, e := object(file)
		if e != nil {
			return nil, e
		}
		result = append(result, record{o, session.Source{File: file, Line: 1}})
	}
	// IDs are sortable within one millisecond; timestamps remain the primary key.
	sort.SliceStable(result, func(i, k int) bool {
		a, b := moment(result[i].object.Object("time"), "created"), moment(result[k].object.Object("time"), "created")
		if a.Equal(b) {
			return result[i].object.String("id") < result[k].object.String("id")
		}
		return a.Before(b)
	})
	return result, nil
}
func moment(o j.Object, key string) time.Time {
	var n int64
	json.Unmarshal(o[key], &n)
	if n == 0 {
		return time.Time{}
	}
	return time.UnixMilli(n).UTC()
}

func Read(file string) (*session.Session, error) {
	s := &session.Session{Agent: "opencode", File: file}
	o, err := object(file)
	if err != nil {
		return s, err
	}
	s.ID = o.String("id")
	s.Cwd = o.String("directory")
	s.AgentVersion = o.String("version")
	s.Observe(moment(o.Object("time"), "created"))
	s.Observe(moment(o.Object("time"), "updated"))
	if s.AgentVersion != "1.1.4" || s.ID == "" || s.Cwd == "" {
		return s, fmt.Errorf("%s: unsupported OpenCode log format; use an OpenCode 1.1.4 session JSON with its message and part siblings", file)
	}
	projectDir := filepath.Dir(file)
	sessionDir := filepath.Dir(projectDir)
	root := filepath.Dir(sessionDir)
	if filepath.Base(sessionDir) != "session" {
		return s, fmt.Errorf("%s: OpenCode session must be inside storage/session/<project>/; preserve its message/ and part/ siblings", file)
	}
	messages, err := records(filepath.Join(root, "message", s.ID))
	if err != nil {
		return s, err
	}
	if len(messages) == 0 {
		s.Notes = append(s.Notes, "Missing OpenCode messages; preserve the storage/message and storage/part directories")
	}
	seen := map[string]bool{}
	for _, m := range messages {
		info := m.object
		if info.String("sessionID") != s.ID {
			return s, fmt.Errorf("%s: OpenCode message belongs to another session", m.source.File)
		}
		start, end := moment(info.Object("time"), "created"), moment(info.Object("time"), "completed")
		s.Observe(start)
		s.Observe(end)
		s.Model(info.String("modelID"))
		s.Model(info.Object("model").String("modelID"))
		parts, e := records(filepath.Join(root, "part", info.String("id")))
		if e != nil {
			return s, e
		}
		if len(parts) == 0 {
			s.Notes = append(s.Notes, "Missing OpenCode message parts: "+info.String("id"))
		}
		var texts []string
		for _, p := range parts {
			part := p.object
			if part.String("sessionID") != s.ID || part.String("messageID") != info.String("id") {
				return s, fmt.Errorf("%s: OpenCode part belongs to another message or session", p.source.File)
			}
			id := part.String("id")
			if seen[id] {
				continue
			}
			seen[id] = true
			switch part.String("type") {
			case "text":
				if text := part.String("text"); text != "" && !part.Bool("synthetic") && !part.Bool("ignored") {
					texts = append(texts, text)
				}
			case "tool":
				state := part.Object("state")
				input := state.Object("input")
				tool := part.String("tool")
				t, u := moment(state.Object("time"), "start"), moment(state.Object("time"), "end")
				if t.IsZero() {
					t = start
				}
				s.Observe(t)
				s.Observe(u)
				status := state.String("status")
				if status == "running" || status == "" {
					status = "pending"
				}
				if status == "error" {
					status = "failed"
				}
				callID := part.String("callID")
				if callID != "" {
					key := "call:" + callID
					if seen[key] {
						continue
					}
					seen[key] = true
				}
				switch tool {
				case "bash":
					cwd := info.Object("path").String("cwd")
					if cwd == "" {
						cwd = s.Cwd
					}
					if w := input.String("workdir"); w != "" {
						cwd = w
					}
					if !filepath.IsAbs(cwd) {
						cwd = filepath.Join(s.Cwd, cwd)
					}
					cmd := &session.Command{Line: input.String("command"), Cwd: cwd, Shell: true, Start: t, End: u, Status: status, RawOutput: state.String("output"), ExitNote: "no completed exit status"}
					if status == "completed" {
						cmd.ExitCode = state.Object("metadata").Int("exit")
						if cmd.ExitCode != nil {
							cmd.ExitReliable = true
							cmd.ExitNote = ""
							if *cmd.ExitCode != 0 {
								cmd.Status = "failed"
								status = "failed"
							}
						}
					}
					if cmd.RawOutput == "" {
						cmd.RawOutput = state.String("error")
					}
					s.Events = append(s.Events, session.Event{Time: t, Kind: "command", Status: status, Source: p.source, Command: cmd})
				case "edit", "write":
					path := input.String("filePath")
					if path == "" {
						s.Unsupported++
						continue
					}
					if !filepath.IsAbs(path) {
						path = filepath.Join(s.Cwd, path)
					}
					if status == "completed" {
						s.Events = append(s.Events, session.Event{Time: u, Kind: "edit", Status: status, Source: p.source, Edit: &session.Edit{Path: path, Op: tool}})
					} else {
						s.Notes = append(s.Notes, tool+" "+path+": "+status)
					}
				case "read", "list", "glob", "grep", "todowrite", "todoread":
				default:
					s.Unsupported++
				}
			case "step-start", "step-finish", "reasoning", "snapshot", "patch", "file", "agent", "retry", "compaction":
			default:
				s.Unsupported++
			}
		}
		text := strings.Join(texts, "\n")
		switch info.String("role") {
		case "user":
			if text != "" {
				s.Prompts = append(s.Prompts, session.Prompt{Time: start, Text: text, Source: m.source})
			}
		case "assistant":
			if text != "" {
				s.Events = append(s.Events, session.Event{Time: start, Kind: "message", Source: m.source, Text: text})
			}
			if info.String("finish") == "stop" && !end.IsZero() && info.Object("error") == nil {
				s.Final = text
				s.FinalTime = end
			}
		default:
			s.Unsupported++
		}
	}
	s.Order()
	return s, nil
}
