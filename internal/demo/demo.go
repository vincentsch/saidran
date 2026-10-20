// Package demo unpacks the recorded example for a first run without agent logs.
package demo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:generate go run ./generate
//go:embed data/repo.tar.gz data/session.jsonl
var data embed.FS

func Extract() (repo, transcript string, err error) {
	parent, err := os.MkdirTemp("", "gaugestone-demo-*")
	if err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(parent)
		}
	}()
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return "", "", err
	}
	archive, err := data.ReadFile("data/repo.tar.gz")
	if err != nil {
		return "", "", err
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return "", "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var directories []*tar.Header
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", "", e
		}
		name := filepath.FromSlash(h.Name)
		if !filepath.IsLocal(name) {
			return "", "", fmt.Errorf("Invalid demo archive path")
		}
		target := filepath.Join(parent, name)
		switch h.Typeflag {
		case tar.TypeDir:
			directories = append(directories, h)
			err = os.MkdirAll(target, 0700)
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return "", "", err
			}
			var f *os.File
			f, err = os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)&0777)
			if err == nil {
				_, err = io.Copy(f, tr)
				closeErr := f.Close()
				if err == nil {
					err = closeErr
				}
			}
			if err == nil {
				err = os.Chtimes(target, h.ModTime, h.ModTime)
			}
		default:
			return "", "", fmt.Errorf("Unsupported demo archive entry")
		}
		if err != nil {
			return "", "", err
		}
	}
	for i := len(directories) - 1; i >= 0; i-- {
		h := directories[i]
		if err = os.Chtimes(filepath.Join(parent, filepath.FromSlash(h.Name)), h.ModTime, h.ModTime); err != nil {
			return "", "", err
		}
	}
	repo = filepath.Join(parent, "repo")
	b, err := data.ReadFile("data/session.jsonl")
	if err != nil {
		return "", "", err
	}
	// Escape the replacement as JSON, including paths inside command text.
	replacement, err := json.Marshal(repo)
	if err != nil {
		return "", "", err
	}
	b = bytes.ReplaceAll(b, []byte("/fixtures/claude-stale/repo"), replacement[1:len(replacement)-1])
	transcript = filepath.Join(parent, "session.jsonl")
	err = os.WriteFile(transcript, b, 0600)
	return repo, transcript, err
}
