package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/vincentsch/assaylight/internal/redact"
)

type Source struct {
	File string `json:"file"`
	Line int    `json:"line"`
}
type Prompt struct {
	Time   time.Time
	Text   string
	Source Source
}
type Command struct {
	Line         string    `json:"line"`
	Cwd          string    `json:"cwd"`
	Argv         []string  `json:"argv"`
	Shell        bool      `json:"shell"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	ExitCode     *int      `json:"exit_code"`
	ExitReliable bool      `json:"exit_reliable"`
	ExitNote     string    `json:"exit_note"`
	Status       string    `json:"status"`
	Output       string    `json:"output"`
	RawOutput    string    `json:"-"`
}
type Edit struct {
	Path string `json:"path"`
	Op   string `json:"op"`
}
type Event struct {
	Seq          int
	Time         time.Time
	Kind, Status string
	Source       Source
	Command      *Command
	Edit         *Edit
	Text         string
}
type Session struct {
	Agent, AgentVersion, ID, File, Cwd, Branch string
	Started, Ended                             time.Time
	Models                                     []string
	Prompts                                    []Prompt
	Events                                     []Event
	FinalTime                                  time.Time
	Final                                      string
	Unsupported                                int
	Notes                                      []string
}

func (s *Session) Observe(t time.Time) {
	if t.IsZero() {
		return
	}
	if s.Started.IsZero() || t.Before(s.Started) {
		s.Started = t
	}
	if t.After(s.Ended) {
		s.Ended = t
	}
}
func (s *Session) Order() {
	sort.SliceStable(s.Events, func(i, j int) bool {
		if s.Events[i].Time.Equal(s.Events[j].Time) {
			if s.Events[i].Source.File != s.Events[j].Source.File {
				return s.Events[i].Source.File < s.Events[j].Source.File
			}
			return s.Events[i].Source.Line < s.Events[j].Source.Line
		}
		return s.Events[i].Time.Before(s.Events[j].Time)
	})
	home, _ := os.UserHomeDir()
	r := redact.Redactor{Home: home}
	for i := range s.Events {
		s.Events[i].Seq = i + 1
		if c := s.Events[i].Command; c != nil {
			c.Output = r.Tail(c.RawOutput)
		}
	}
}
func (s *Session) Model(m string) {
	if m != "" {
		for _, v := range s.Models {
			if v == m {
				return
			}
		}
		s.Models = append(s.Models, m)
	}
}
func Time(v string) time.Time { t, _ := time.Parse(time.RFC3339Nano, v); return t }

// Lines has no record-size limit. A partial final write is not evidence.
func Lines(file string, s *Session, visit func([]byte, Source) error) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for n := 1; ; n++ {
		b, e := r.ReadBytes('\n')
		if len(strings.TrimSpace(string(b))) > 0 {
			if !json.Valid(b) {
				if e == io.EOF {
					s.Notes = append(s.Notes, fmt.Sprintf("%s:%d: incomplete final line skipped", file, n))
					return nil
				}
				return fmt.Errorf("%s:%d: malformed JSON record", file, n)
			}
			if err := visit(b, Source{file, n}); err != nil {
				return err
			}
		}
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
	}
}
