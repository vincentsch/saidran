package render

import (
	"bytes"
	"encoding/json"

	"github.com/vincentsch/gaugestone/internal/report"
)

// Write renders all documents before saving any of them. Each file replacement
// is atomic; callers announce success only when all three writes succeeded.
func Write(dir string, r report.Report) error {
	var html, md bytes.Buffer
	if err := HTML(&html, r); err != nil {
		return err
	}
	if err := Markdown(&md, r); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	for _, doc := range []struct {
		name string
		data []byte
	}{{"report.html", html.Bytes()}, {"report.md", md.Bytes()}, {"report.json", append(jsonData, '\n')}} {
		if err := report.WriteDocument(dir, doc.name, doc.data); err != nil {
			return err
		}
	}
	return nil
}
