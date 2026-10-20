package agents_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vincentsch/assaylight/internal/agents"
	"github.com/vincentsch/assaylight/internal/commands"
)

// These expectations are hand-labelled from the original tool invocations and
// their results. U means the line exit cannot be attributed; R means it can.
func TestRecordedCommandLedgers(t *testing.T) {
	cases := []struct {
		name        string
		rows, edits []string
	}{
		{"claude-contradicted", []string{"other/U,other/U", "test/R/failed/packages=1"}, []string{"calc/stats.go"}},
		{"claude-honest-failure", []string{"other/U,other/U", "test/R/failed/packages=1"}, []string{"calc/stats.go"}},
		{"claude-pytest", []string{"test/U/failed/passed=2/failed=1,other/U,git/R", "test/U/passed/passed=3,other/U"}, []string{"pycalc/__init__.py"}},
		{"claude-skip", []string{"git/R,other/U,other/U", "test/U/passed/skipped=1/packages=1,other/U"}, []string{"calc/stats.go", "calc/stats_test.go"}},
		{"claude-stale", []string{"other/U,other/U", "test/R/passed/packages=1"}, []string{"calc/stats.go", "calc/stats.go"}},
		{"claude-verified", []string{"other/R,test/U/failed/packages=1,other/U", "other/R,other/R,other/R", "other/R", "other/U,format/U,test/R/passed/packages=1"}, []string{"calc/stats.go"}},
		{"codex-stale", []string{"other/U,other/R", "other/R", "test/R/passed/packages=1"}, []string{"calc/stats.go", "calc/stats.go"}},
		{"codex-verified", []string{"other/R,other/R", "other/R", "git/U,other/R", "test/R/failed/packages=1", "format/U,test/U/passed/packages=1,git/U,git/R"}, []string{"calc/stats.go", "calc/stats.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, repo, file := fixture(t, tc.name)
			s, err := agents.Read(file)
			if err != nil {
				t.Fatal(err)
			}
			l := commands.Build(s)
			var rows, paths []string
			for _, c := range l.Commands {
				row := ""
				for i, st := range c.Statements {
					if i > 0 {
						row += ","
					}
					reliability := "U"
					if st.ExitReliable {
						reliability = "R"
					}
					row += st.Class + "/" + reliability
					if st.Class == "test" {
						if st.Scope != "whole" {
							t.Errorf("unexpected test scope %s", st.Scope)
						}
						row += "/" + st.Result()
						if st.Tests != nil {
							for _, count := range []struct {
								name  string
								value *int
							}{{"passed", st.Tests.Passed}, {"failed", st.Tests.Failed}, {"skipped", st.Tests.Skipped}, {"packages", st.Tests.Packages}} {
								if count.value != nil {
									row += fmt.Sprintf("/%s=%d", count.name, *count.value)
								}
							}
						}
					}
					if !st.ExitReliable && st.ExitNote == "" {
						t.Error("unattributed exit without explanation")
					}
					if st.MayEditFiles {
						t.Errorf("unexpected uncertain editor: %s", st.Line)
					}
				}
				rows = append(rows, row)
			}
			for _, e := range l.Entries {
				if e.Edit != nil {
					rel, err := filepath.Rel(repo, e.Edit.Path)
					if err != nil {
						t.Fatal(err)
					}
					paths = append(paths, rel)
				}
			}
			if !reflect.DeepEqual(rows, tc.rows) {
				t.Errorf("ledger rows:\ngot  %q\nwant %q", rows, tc.rows)
			}
			if !reflect.DeepEqual(paths, tc.edits) {
				t.Errorf("edits: %v want %v", paths, tc.edits)
			}
			if tc.name == "claude-verified" || tc.name == "codex-verified" {
				var edited bool
				for _, e := range l.Entries {
					if e.Edit != nil {
						edited = true
					}
					if e.Command != nil && e.Command.Class == "test" && e.Command.Result() == "passed" && !edited {
						t.Error("test precedes its shell edit")
					}
				}
			}
		})
	}
}
