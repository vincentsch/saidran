// Package pi reads Pi's version 3 session journal.
package pi

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	j "github.com/vincentsch/gaugestone/internal/agents/logjson"
	"github.com/vincentsch/gaugestone/internal/session"
)

var exitTrailer = regexp.MustCompile(`(?:^|\n)Command exited with code ([1-9][0-9]*)\s*$`)
var interruptionTrailer = regexp.MustCompile(`(?:^|\n)Command (?:aborted|timed out after [^\n]+)\s*$`)

type call struct {
	event      session.Event
	name, path string
	resolved   bool
}

func Read(file string) (*session.Session, error) {
	s := &session.Session{Agent: "pi", File: file}
	calls := map[string]*call{}
	var order []string
	seen := map[string]bool{}
	header := false
	err := session.Lines(file, s, func(b []byte, src session.Source) error {
		var o j.Object
		json.Unmarshal(b, &o)
		t := session.Time(o.String("timestamp"))
		s.Observe(t)
		if id := o.String("id"); id != "" {
			if seen[id] {
				return nil
			}
			seen[id] = true
		}
		switch o.String("type") {
		case "session":
			v := o.Int("version")
			s.ID = o.String("id")
			s.Cwd = o.String("cwd")
			if v == nil || *v != 3 || s.ID == "" || s.Cwd == "" {
				return fmt.Errorf("%s: unsupported Pi log format; record a new session with Pi 0.85.1 (journal version 3)", file)
			}
			header = true
		case "model_change":
			s.Model(o.String("modelId"))
		case "message":
			if !header {
				return fmt.Errorf("%s: unsupported Pi log format; missing session header", file)
			}
			m := o.Object("message")
			switch m.String("role") {
			case "user":
				s.Prompts = append(s.Prompts, session.Prompt{Time: t, Text: m.Text("content"), Source: src})
			case "assistant":
				s.Model(m.String("model"))
				text := m.Text("content")
				if strings.TrimSpace(text) != "" {
					s.Events = append(s.Events, session.Event{Time: t, Kind: "message", Source: src, Text: text})
				}
				// Only a successfully completed assistant turn supplies the final response.
				if m.String("stopReason") == "stop" {
					s.Final = text
					s.FinalTime = t
				}
				for _, block := range m.List("content") {
					switch block.String("type") {
					case "toolCall":
						id := block.String("id")
						if calls[id] != nil {
							continue
						}
						a := block.Object("arguments")
						name := block.String("name")
						c := &call{name: name, path: a.String("path"), event: session.Event{Time: t, Source: src, Status: "pending"}}
						if c.path != "" && !filepath.IsAbs(c.path) {
							c.path = filepath.Join(s.Cwd, c.path)
						}
						if name == "bash" {
							c.event.Kind = "command"
							c.event.Command = &session.Command{Line: a.String("command"), Cwd: s.Cwd, Shell: true, Start: t, Status: "pending", ExitNote: "tool call has no completed result"}
						}
						calls[id] = c
						order = append(order, id)
					case "text", "thinking", "image":
					default:
						s.Unsupported++
					}
				}
			case "toolResult":
				c := calls[m.String("toolCallId")]
				if c == nil || c.resolved {
					return nil
				}
				c.resolved = true
				text := m.Text("content")
				status := "pending"
				var resultError *bool
				json.Unmarshal(m["isError"], &resultError)
				if resultError != nil && !*resultError {
					status = "completed"
				}
				if m.Bool("isError") {
					status = "failed"
				}
				if m.Bool("isError") && interruptionTrailer.MatchString(text) {
					status = "interrupted"
				}
				c.event.Status = status
				if cmd := c.event.Command; cmd != nil {
					cmd.RawOutput = text
					cmd.OutputTruncatedByAgent = m.Object("details").Object("truncation").Bool("truncated")
					cmd.End = t
					cmd.Status = status
					cmd.ExitNote = "no completed exit status"
					// details describes output truncation, never an exit status.
					if status == "completed" {
						n := 0
						cmd.ExitCode = &n
						cmd.ExitReliable = true
						cmd.ExitNote = ""
					} else if status == "failed" {
						if match := exitTrailer.FindStringSubmatch(text); match != nil {
							n, e := strconv.Atoi(match[1])
							if e == nil {
								cmd.ExitCode = &n
								cmd.ExitReliable = true
								cmd.ExitNote = ""
							}
						}
					}
				} else if (c.name == "edit" || c.name == "write") && c.path != "" {
					c.event.Time = t
					if status == "completed" {
						c.event.Kind = "edit"
						c.event.Edit = &session.Edit{Path: c.path, Op: c.name}
					} else {
						c.event.Kind = "message"
						c.event.Text = c.name + " " + c.path + ": " + status
					}
				}
			default:
				s.Unsupported++
			}
		case "thinking_level_change", "session_info", "compaction", "branch_summary", "custom", "custom_message", "label":
		default:
			s.Unsupported++
		}
		return nil
	})
	if err != nil {
		return s, err
	}
	if !header {
		return s, fmt.Errorf("%s: unsupported Pi log format; record a new session with Pi 0.85.1", file)
	}
	for _, id := range order {
		c := calls[id]
		if c.event.Kind != "" {
			s.Events = append(s.Events, c.event)
		} else if !c.resolved && (c.name == "edit" || c.name == "write") {
			s.Notes = append(s.Notes, "Pending "+c.name+" tool call: "+c.path)
		}
	}
	s.Order()
	return s, nil
}
