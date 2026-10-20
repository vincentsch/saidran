package codex

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	j "github.com/vincentsch/assaylight/internal/agents/logjson"
	"github.com/vincentsch/assaylight/internal/session"
)

func cwd(v string) string {
	if strings.HasPrefix(v, "file://") {
		u, e := url.Parse(v)
		if e == nil {
			return filepath.FromSlash(u.Path)
		}
	}
	return v
}
func Read(file string) (*session.Session, error) {
	s := &session.Session{Agent: "codex", File: file}
	completed := false
	seen := map[string]bool{}
	err := session.Lines(file, s, func(b []byte, src session.Source) error {
		var o j.Object
		json.Unmarshal(b, &o)
		p := o.Object("payload")
		t := session.Time(o.String("timestamp"))
		s.Observe(t)
		switch o.String("type") {
		case "session_meta":
			s.ID = p.String("id")
			if s.ID == "" {
				s.ID = p.String("session_id")
			}
			s.Cwd = cwd(p.String("cwd"))
			s.AgentVersion = p.String("cli_version")
			s.Branch = p.Object("git").String("branch")
		case "turn_context":
			if s.Cwd == "" {
				s.Cwd = cwd(p.String("cwd"))
			}
			s.Model(p.String("model"))
		case "response_item", "token_usage_record", "world_state":
		case "event_msg":
			switch p.String("type") {
			case "task_complete":
				if text := p.String("last_agent_message"); text != "" {
					if s.Final != text {
						s.Events = append(s.Events, session.Event{Time: t, Kind: "message", Source: src, Text: text})
					}
					if s.Final != text || s.FinalTime.IsZero() {
						s.FinalTime = t
					}
					s.Final = text
				}
			case "task_started", "thread_settings_applied", "token_count", "token_usage_record", "world_state":
			case "item_completed":
				completed = true
				i := p.Object("item")
				id := i.String("id")
				if id != "" {
					if seen[id] {
						return nil
					}
					seen[id] = true
				}
				status := i.String("status")
				ev := session.Event{Time: t, Source: src, Status: status}
				switch i.String("type") {
				case "CommandExecution":
					var argv []string
					json.Unmarshal(i["command"], &argv)
					d := i.Object("duration")
					var duration time.Duration
					if n := d.Int("secs"); n != nil {
						duration += time.Duration(*n) * time.Second
					}
					if n := d.Int("nanos"); n != nil {
						duration += time.Duration(*n)
					}
					c := &session.Command{Argv: argv, Cwd: cwd(i.String("cwd")), Start: t.Add(-duration), End: t, RawOutput: i.String("aggregated_output"), Status: status, ExitCode: i.Int("exit_code")}
					if len(argv) == 3 && (filepath.Base(argv[0]) == "bash" || filepath.Base(argv[0]) == "sh" || filepath.Base(argv[0]) == "zsh") && (argv[1] == "-c" || argv[1] == "-lc") {
						c.Shell = true
						c.Line = argv[2]
					} else {
						q := make([]string, len(argv))
						for k, a := range argv {
							q[k] = strconv.Quote(a)
						}
						c.Line = strings.Join(q, " ")
					}
					c.ExitReliable = c.ExitCode != nil && (status == "completed" || status == "failed")
					if !c.ExitReliable {
						c.ExitCode = nil
						c.ExitNote = "no completed exit status"
					}
					ev.Kind = "command"
					ev.Time = c.Start
					ev.Command = c
					s.Events = append(s.Events, ev)
					s.Observe(c.Start)
				case "FileChange":
					changes := i.Object("changes")
					paths := make([]string, 0, len(changes))
					for path := range changes {
						paths = append(paths, path)
					}
					sort.Strings(paths)
					for _, path := range paths {
						change := changes.Object(path)
						e := ev
						if status == "completed" {
							op := map[string]string{"add": "write", "update": "patch", "delete": "delete"}[change.String("type")]
							if op == "" {
								s.Unsupported++
								continue
							}
							e.Kind = "edit"
							e.Edit = &session.Edit{Path: path, Op: op}
							s.Events = append(s.Events, e)
							if move := change.String("move_path"); move != "" {
								e.Edit = &session.Edit{Path: move, Op: "write"}
								s.Events = append(s.Events, e)
							}
						} else {
							e.Kind = "message"
							e.Text = "File change " + path + ": " + status
							s.Events = append(s.Events, e)
						}
					}
				case "UserMessage":
					s.Prompts = append(s.Prompts, session.Prompt{Time: t, Text: i.Text("content"), Source: src})
				case "AgentMessage":
					text := i.Text("content")
					ev.Kind = "message"
					ev.Text = text
					s.Events = append(s.Events, ev)
					s.Final = text
					s.FinalTime = t
				case "Reasoning":
				default:
					s.Unsupported++
				}
			default:
				s.Unsupported++
			}
		default:
			s.Unsupported++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !completed {
		return nil, fmt.Errorf("%s: unsupported Codex log format; update Codex", file)
	}
	s.Order()
	return s, nil
}
