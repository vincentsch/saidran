package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vincentsch/rungrad"
)

type DismissalStore struct {
	Version int         `json:"version"`
	Entries []Dismissal `json:"dismissals"`
}

func LoadDismissals(gitDir string) (DismissalStore, error) {
	s := DismissalStore{Version: 1, Entries: []Dismissal{}}
	b, err := os.ReadFile(filepath.Join(gitDir, "saidran", "dismissals.json"))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("Could not read dismissals: %w", err)
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("Could not read dismissals: %w", err)
	}
	if s.Version != 1 {
		return s, fmt.Errorf("Unsupported dismissals version %d; update Saidran", s.Version)
	}
	return s, nil
}
func (s DismissalStore) Save(gitDir string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return WriteDocument(filepath.Join(gitDir, "saidran"), "dismissals.json", append(b, '\n'))
}
func (r *Report) ApplyDismissals(s DismissalStore) {
	for _, f := range r.Findings {
		for _, d := range s.Entries {
			if d.ID == f.ID {
				// The stored diff hunk stays in the local store; reports show
				// only the finding's own excerpt.
				d.Context = ""
				r.Dismissals = append(r.Dismissals, d)
				break
			}
		}
	}
}

var findingRef = regexp.MustCompile(`^F-[0-9a-f]{6,12}$`)

func ResolveFinding(ref string, ids []string) (int, error) {
	if !findingRef.MatchString(ref) {
		return -1, rungrad.NewError(rungrad.ExitUsage, "Use a finding ID with F- and at least six hex characters. Claim verdicts cannot be dismissed.")
	}
	match := -1
	for i, id := range ids {
		if strings.HasPrefix(id, ref) {
			if match >= 0 {
				return -1, rungrad.NewError(rungrad.ExitUsage, "Ambiguous finding ID. Use a longer prefix.")
			}
			match = i
		}
	}
	if match < 0 {
		return -1, rungrad.NewError(rungrad.ExitNotFound, "No matching finding. Check the current diff and baseline, or run saidran dismissals.")
	}
	return match, nil
}
