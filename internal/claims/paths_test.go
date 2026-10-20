package claims

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vincentsch/gaugestone/internal/commands"
	"github.com/vincentsch/gaugestone/internal/session"
)

func TestSymlinkedEvidenceKeepsLaterEdits(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(root, "calc"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "calc", "value.go"), []byte("package calc\n"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	zero := 0
	for _, sessionCwd := range []string{root, alias} {
		for _, commandCwd := range []string{root, alias, "."} {
			for _, mode := range []string{"native", "relative native", "shell", "deleted native", "deleted shell", "none"} {
				t.Run(filepath.Base(sessionCwd)+"/"+filepath.Base(commandCwd)+"/"+mode, func(t *testing.T) {
					// The missing leaf exercises edits recorded before a file was deleted.
					leaf := "calc/value.go"
					if mode == "deleted native" || mode == "deleted shell" {
						leaf = "calc/deleted/sub/value.go"
					}
					line, final := "go test ./...", "All tests pass."
					if mode == "none" {
						line, final = "go test ./calc", "`go test ./calc` passed."
					}
					original := &session.Command{Line: line, Cwd: commandCwd, Shell: true, ExitReliable: true, ExitCode: &zero, Start: at, End: at.Add(time.Second)}
					source := session.Source{File: "recording.jsonl", Line: 12}
					s := &session.Session{Cwd: sessionCwd, Final: final, FinalTime: at.Add(5 * time.Second), Events: []session.Event{{Seq: 1, Source: source, Time: original.End, Command: original}}}
					switch mode {
					case "native", "deleted native":
						s.Events = append(s.Events, session.Event{Seq: 2, Source: source, Time: at.Add(2 * time.Second), Edit: &session.Edit{Path: filepath.Join(alias, leaf), Op: "edit"}})
					case "relative native":
						s.Events = append(s.Events, session.Event{Seq: 2, Source: source, Time: at.Add(2 * time.Second), Edit: &session.Edit{Path: leaf, Op: "edit"}})
					case "shell", "deleted shell":
						edit := &session.Command{Line: "rm '" + filepath.Join(alias, leaf) + "'", Cwd: root, Shell: true, ExitReliable: true, ExitCode: &zero, Start: at.Add(2 * time.Second), End: at.Add(3 * time.Second)}
						s.Events = append(s.Events, session.Event{Seq: 2, Source: source, Time: edit.End, Command: edit})
					}
					ledger := commands.Build(s)
					got, _ := Evaluate(root, s, ledger)
					want := "stale"
					if mode == "none" {
						want = "verified"
					}
					if len(got) != 1 || got[0].Verdict != want {
						t.Fatalf("claims=%+v", got)
					}
					if mode != "none" && (len(got[0].Edits) != 1 || got[0].Edits[0].Path != leaf || got[0].Edits[0].Seq != 2) {
						t.Fatalf("later edits=%+v", got[0].Edits)
					}
					if ledger.Commands[0].Original != original || original.Cwd != commandCwd || original.Line != line || ledger.Entries[0].Source != source {
						t.Fatal("recorded evidence changed")
					}
				})
			}
		}
	}
}
