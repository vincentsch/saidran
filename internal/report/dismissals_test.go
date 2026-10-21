package report

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/saidran/internal/findings"
)

func TestFindingReferences(t *testing.T) {
	ids := []string{"F-123456abcdef", "F-123456789abc"}
	for _, tc := range []struct {
		ref         string
		index, code int
	}{
		{ids[0], 0, 0}, {"F-123456a", 0, 0}, {"F-123456", -1, 1}, {"F-ffffff", -1, 5}, {"F-12345", -1, 1}, {"verified", -1, 1},
	} {
		index, err := ResolveFinding(tc.ref, ids)
		if index != tc.index || rungrad.ExitCodeFor(err) != tc.code {
			t.Fatalf("%s: index=%d error=%v", tc.ref, index, err)
		}
	}
}
func TestDismissalStoreErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "saidran"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{`{"version":2,"dismissals":[]}`, `{broken`} {
		if err := os.WriteFile(filepath.Join(dir, "saidran", "dismissals.json"), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadDismissals(dir); err == nil {
			t.Fatal("invalid store accepted")
		}
	}
}

func TestReportDismissalsOmitStoredHunk(t *testing.T) {
	r := Report{Findings: []findings.Finding{{ID: "F-123456abcdef"}}}
	r.ApplyDismissals(DismissalStore{Version: 1, Entries: []Dismissal{{ID: "F-123456abcdef", Reason: "Integration test", Context: "+func TestIntegration(t *testing.T) {"}}})
	if len(r.Dismissals) != 1 || r.Dismissals[0].Reason != "Integration test" || r.Dismissals[0].Context != "" {
		t.Fatalf("%+v", r.Dismissals)
	}
}
