package scripts_test

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Check handwritten entry points too: generated reference checks do not catch
// broken links from the README or installation guides.
func TestDocumentationLocalLinks(t *testing.T) {
	pages, err := filepath.Glob("../docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	pages = append(pages, "../README.md", "../AGENTS.md", "../CONTRIBUTING.md", "../RELEASE.md", "../SECURITY.md")
	links := regexp.MustCompile(`\]\(([^)]+)\)|(?:src|srcset|href)="([^"]+)"`)
	for _, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range links.FindAllStringSubmatch(string(data), -1) {
			target := match[1] + match[2]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "#") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if _, err := os.Stat(filepath.Join(filepath.Dir(page), target)); err != nil {
				t.Errorf("%s: broken local link %q: %v", page, target, err)
			}
		}
	}
}

func TestLogoAssetsAreStandaloneSVG(t *testing.T) {
	assets, err := filepath.Glob("../docs/media/*.svg")
	if err != nil || len(assets) == 0 {
		t.Fatalf("SVG assets: %v, %v", assets, err)
	}
	for _, asset := range assets {
		t.Run(filepath.Base(asset), func(t *testing.T) {
			data, err := os.ReadFile(asset)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "prefers-color-scheme") {
				t.Error("embedded logo must keep the colors selected by its page")
			}
			decoder := xml.NewDecoder(strings.NewReader(string(data)))
			root := false
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if el, ok := token.(xml.StartElement); ok {
					if !root {
						root = true
						if el.Name.Local != "svg" || el.Name.Space != "http://www.w3.org/2000/svg" {
							t.Error("expected an SVG root")
						}
					}
					if el.Name.Local == "script" || el.Name.Local == "image" || el.Name.Local == "foreignObject" {
						t.Errorf("unexpected active or linked content: %s", el.Name.Local)
					}
					for _, attr := range el.Attr {
						if attr.Name.Local == "href" && !strings.HasPrefix(attr.Value, "#") {
							t.Errorf("external resource %q", attr.Value)
						}
					}
				}
			}
			if !root {
				t.Error("empty SVG")
			}
		})
	}
}
