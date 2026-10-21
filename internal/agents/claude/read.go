package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	j "github.com/vincentsch/saidran/internal/agents/logjson"
	"github.com/vincentsch/saidran/internal/session"
)

var exitText = regexp.MustCompile(`^Exit code ([0-9]+)\b`)

type call struct {
	event      session.Event
	name, path string
	background bool
}

func Read(file string) (*session.Session, error) {
	s := &session.Session{Agent: "claude-code", File: file}
	seen := map[string]bool{}
	mentioned := map[string]bool{}
	files := []string{file}
	children, err := filepath.Glob(filepath.Join(strings.TrimSuffix(file, filepath.Ext(file)), "subagents", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	files = append(files, children...)
	for _, path := range files {
		parent := path == file
		calls := map[string]*call{}
		var callOrder []string
		messageEvents := map[string]int{}
		messageTimes := map[string]time.Time{}
		finalID := ""
		err := session.Lines(path, s, func(b []byte, src session.Source) error {
			var o j.Object
			json.Unmarshal(b, &o)
			id := o.String("uuid")
			if id != "" {
				if seen[id] {
					return nil
				}
				seen[id] = true
			}
			t := session.Time(o.String("timestamp"))
			s.Observe(t)
			if parent {
				if s.ID == "" {
					s.ID = o.String("sessionId")
				}
				if s.Cwd == "" {
					s.Cwd = o.String("cwd")
				}
				if v := o.String("version"); v != "" {
					s.AgentVersion = v
				}
				if v := o.String("gitBranch"); v != "" {
					s.Branch = v
				}
			}
			m := o.Object("message")
			s.Model(m.String("model"))
			side := !parent || o.Bool("isSidechain")
			switch o.String("type") {
			case "assistant", "user":
				if o.String("type") == "user" {
					if text := m.String("content"); text != "" && !side {
						s.Prompts = append(s.Prompts, session.Prompt{Time: t, Text: text, Source: src})
					}
				}
				for _, block := range m.List("content") {
					switch block.String("type") {
					case "text":
						text := block.String("text")
						if text == "" {
							continue
						}
						if o.String("type") == "user" {
							if !side {
								s.Prompts = append(s.Prompts, session.Prompt{Time: t, Text: text, Source: src})
							}
							continue
						}
						mid := m.String("id")
						if mid == "" {
							mid = fmt.Sprintf("%s:%d", path, src.Line)
						}
						messageTimes[mid] = t
						if idx, ok := messageEvents[mid]; ok {
							s.Events[idx].Text += "\n" + text
						} else {
							messageEvents[mid] = len(s.Events)
							s.Events = append(s.Events, session.Event{Time: t, Kind: "message", Source: src, Text: text})
						}
						if !side {
							finalID = mid
						}
					case "tool_use":
						input := block.Object("input")
						name := block.String("name")
						c := &call{name: name, path: input.String("file_path"), background: input.Bool("run_in_background"), event: session.Event{Time: t, Source: src, Status: "pending"}}
						if name == "NotebookEdit" && c.path == "" {
							c.path = input.String("notebook_path")
						}
						if name == "Bash" {
							cwd := o.String("cwd")
							if v := input.String("cwd"); v != "" {
								cwd = v
							}
							c.event.Kind = "command"
							c.event.Command = &session.Command{Line: input.String("command"), Shell: true, Cwd: cwd, Start: t, Status: "pending", ExitNote: "tool call has no completed result"}
						}
						calls[block.String("id")] = c
						callOrder = append(callOrder, block.String("id"))
					case "tool_result":
						text := block.Text("content")
						c := calls[block.String("tool_use_id")]
						if c == nil {
							continue
						}
						result := o.Object("toolUseResult")
						if id := result.String("agentId"); id != "" {
							mentioned[id] = true
						}
						status := "completed"
						if block.Bool("is_error") {
							status = "failed"
						}
						if result.Bool("interrupted") {
							status = "interrupted"
						}
						if c.background {
							status = "background"
						}
						c.event.Status = status
						if cmd := c.event.Command; cmd != nil {
							cmd.RawOutput = text
							if _, ok := result["stdout"]; ok {
								cmd.RawOutput = result.String("stdout")
								if stderr := result.String("stderr"); stderr != "" {
									if cmd.RawOutput != "" && !strings.HasSuffix(cmd.RawOutput, "\n") {
										cmd.RawOutput += "\n"
									}
									cmd.RawOutput += stderr
								}
							}
							cmd.End = t
							if c.background {
								cmd.End = time.Time{}
							}
							cmd.Status = status
							cmd.ExitNote = ""
							if match := exitText.FindStringSubmatch(text); match != nil && block.Bool("is_error") && status != "background" && status != "interrupted" {
								n, _ := strconv.Atoi(match[1])
								cmd.ExitCode = &n
								if n != 0 {
									cmd.Status = "failed"
									c.event.Status = "failed"
								}
								cmd.ExitReliable = true
							} else if status == "completed" {
								n := 0
								cmd.ExitCode = &n
								cmd.ExitReliable = true
							} else {
								cmd.ExitNote = "no completed exit status"
							}
						} else if c.path != "" && (c.name == "Edit" || c.name == "MultiEdit" || c.name == "Write" || c.name == "NotebookEdit") {
							c.event.Kind = "edit"
							c.event.Time = t
							if status == "completed" {
								op := "edit"
								if c.name == "Write" {
									op = "write"
								}
								c.event.Edit = &session.Edit{Path: c.path, Op: op}
							}
							if status != "completed" {
								c.event.Kind = "message"
								c.event.Text = c.name + " " + c.path + ": " + status
							}
						}
					case "thinking", "image", "document":
					default:
						s.Unsupported++
					}
				}
			case "attachment", "queue-operation", "atis-latch", "last-prompt", "cost-state", "system", "progress", "file-history-snapshot":
			default:
				s.Unsupported++
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if parent && finalID != "" {
			s.Final = s.Events[messageEvents[finalID]].Text
			s.FinalTime = messageTimes[finalID]
		}
		for _, id := range callOrder {
			c := calls[id]
			if c.event.Kind != "" {
				s.Events = append(s.Events, c.event)
			} else if c.path != "" && (c.name == "Edit" || c.name == "MultiEdit" || c.name == "Write" || c.name == "NotebookEdit") {
				s.Events = append(s.Events, session.Event{Time: c.event.Time, Kind: "message", Status: "pending", Source: c.event.Source, Text: c.name + " " + c.path + ": pending"})
			}
		}
	}
	for id := range mentioned {
		if _, err := os.Stat(filepath.Join(strings.TrimSuffix(file, filepath.Ext(file)), "subagents", "agent-"+id+".jsonl")); os.IsNotExist(err) {
			s.Notes = append(s.Notes, "Missing sub-agent transcript: "+id)
		}
	}
	sort.Strings(s.Notes)
	s.Order()
	return s, nil
}
