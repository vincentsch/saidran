package scripts_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionStep struct {
	Name string            `yaml:"name"`
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Run  string            `yaml:"run"`
	Uses string            `yaml:"uses"`
	Env  map[string]string `yaml:"env"`
	With map[string]string `yaml:"with"`
}

func TestCompositeActionLocally(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(filepath.Join(root, "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var action struct {
		Runs struct {
			Using string       `yaml:"using"`
			Steps []actionStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(metadata, &action); err != nil {
		t.Fatal(err)
	}
	const releaseVersion = "v0.1.0"
	steps := action.Runs.Steps
	if action.Runs.Using != "composite" || len(steps) != 6 {
		t.Fatal("unexpected composite structure")
	}
	if steps[2].Name != "Write job summary" || steps[3].With["path"] != "${{ steps.report.outputs.report_dir }}/report.html" || !strings.HasPrefix(steps[3].Uses, "actions/upload-artifact@") || len(strings.Split(steps[3].Uses, "@")[1]) != 40 {
		t.Fatal("outputs must precede publication with pinned upload")
	}
	if steps[4].Name != "Publish PR comment" || !strings.Contains(steps[4].If, "github.event.pull_request") || steps[5].If != "${{ always() && steps.report.outputs.status != '' }}" {
		t.Fatal("publication/policy ordering")
	}
	for _, i := range []int{2, 3, 4} {
		if !strings.Contains(steps[i].If, "success()") || !strings.Contains(steps[i].If, "'10'") {
			t.Fatalf("policy failure would suppress outputs: %s", steps[i].If)
		}
	}
	if steps[0].Env["SAIDRAN_VERSION"] != releaseVersion {
		t.Fatal("Action must pin its release binary")
	}
	binary := filepath.Join(t.TempDir(), "saidran")
	cmd := exec.Command("go", "build", "-ldflags", "-X github.com/vincentsch/saidran/internal/version.Version="+releaseVersion, "-o", binary, "./cmd/saidran")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %v %s", err, b)
	}
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	asset := "saidran-" + runtime.GOOS + "-" + runtime.GOARCH
	var downloadMu sync.Mutex
	var downloads []string
	release := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadMu.Lock()
		downloads = append(downloads, r.URL.Path)
		downloadMu.Unlock()
		switch r.URL.Path {
		case "/download/" + releaseVersion + "/" + asset:
			w.Write(payload)
		case "/download/" + releaseVersion + "/checksums.txt":
			fmt.Fprintf(w, "%x  %s\n", sha256.Sum256(payload), asset)
		default:
			http.NotFound(w, r)
		}
	}))
	defer release.Close()
	for _, tc := range []struct {
		name, base, policy, publication              string
		transcript, nonPR, reportError, installError bool
		status                                       int
	}{
		{name: "missing pinned release", installError: true, status: 1},
		{name: "create", status: 0}, {name: "replace", base: "HEAD", status: 0},
		{name: "transcript", transcript: true, status: 0},
		{name: "policy", policy: "test_skipped", status: 10},
		{name: "forbidden", publication: "403", status: 0},
		{name: "unexpected publication failure", publication: "500", status: 1},
		{name: "policy and unexpected publication failure", publication: "500", policy: "test_skipped", status: 10},
		{name: "report error", reportError: true, status: 2},
		{name: "non PR missing base", nonPR: true, status: 1},
		{name: "non PR explicit base", nonPR: true, base: "HEAD", status: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			repo := filepath.Join(dir, "repo")
			os.Mkdir(repo, 0700)
			env := append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
			git := func(args ...string) string {
				c := exec.Command("git", args...)
				c.Dir = repo
				c.Env = env
				b, e := c.CombinedOutput()
				if e != nil {
					t.Fatalf("git %v %s", e, b)
				}
				return strings.TrimSpace(string(b))
			}
			git("init", "-q")
			os.WriteFile(filepath.Join(repo, "sample.txt"), []byte("initial\n"), 0600)
			git("add", ".")
			git("commit", "-qm", "Initial")
			baseSHA := git("rev-parse", "HEAD")
			os.WriteFile(filepath.Join(repo, "sample_test.go"), []byte("package sample\nfunc TestSample(t *testing.T) { t.Skip(\"reason\") }\n"), 0600)
			if tc.name == "create" {
				if err := os.WriteFile(filepath.Join(repo, "<!-- saidran -->.txt"), []byte("untrusted filename\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// An old output tree must never become this invocation's artifact.
			previous := filepath.Join(dir, "saidran-reports.previous")
			os.Mkdir(previous, 0700)
			os.WriteFile(filepath.Join(previous, "report.html"), []byte("stale report"), 0600)
			event := map[string]any{}
			if !tc.nonPR {
				event["pull_request"] = map[string]any{"number": 42, "base": map[string]string{"sha": baseSHA}}
			}
			data, _ := json.Marshal(event)
			eventPath := filepath.Join(dir, "event.json")
			os.WriteFile(eventPath, data, 0600)
			fakeDir := filepath.Join(dir, "fake")
			os.Mkdir(fakeDir, 0700)
			fakeGH := `#!/usr/bin/env python3
import json,os,pathlib,sys
args=sys.argv[1:]
root=pathlib.Path(os.environ['FAKE_ROOT'])
with open(root/'calls','a') as f: f.write(json.dumps(args)+'\n')
if '--paginate' in args:
 print(json.dumps([[{'id':1,'user':{'login':'human','type':'User'},'body':'<!-- saidran -->\nhuman'}, {'id':2,'user':{'login':'other[bot]','type':'Bot'},'body':'<!-- saidran -->\nother'}, {'id':3,'user':{'login':'github-actions[bot]','type':'Bot'},'body':'unrelated'}], [{'id':4,'user':{'login':'github-actions[bot]','type':'Bot'},'body':'<!-- saidran -->\nprevious'}] if os.environ['CASE_NAME']=='replace' else []]))
else:
 (root/'posted').write_text(pathlib.Path(args[args.index('--input')+1]).read_text())
 if os.environ['PUBLICATION_ERROR']:
  print('gh: failed (HTTP '+os.environ['PUBLICATION_ERROR']+')',file=sys.stderr);sys.exit(1)
 print('{}')
`
			os.WriteFile(filepath.Join(fakeDir, "gh"), []byte(fakeGH), 0700)
			transcript := ""
			if tc.transcript {
				transcript = filepath.Join(dir, "session.jsonl")
				log := map[string]any{"type": "user", "sessionId": "ci-local", "cwd": repo, "timestamp": "2026-10-20T12:00:00Z", "message": map[string]string{"content": "Review the checkout"}}
				data, _ := json.Marshal(log)
				os.WriteFile(transcript, append(data, '\n'), 0600)
			}
			base := tc.base
			if tc.reportError {
				base = "missing-base"
			}
			downloadBase := release.URL
			if tc.installError {
				downloadBase += "/missing"
			}
			env = append(env, "RUNNER_OS="+map[string]string{"linux": "Linux", "darwin": "macOS"}[runtime.GOOS], "RUNNER_TEMP="+dir, "GITHUB_ACTION_PATH="+root, "GITHUB_EVENT_PATH="+eventPath, "GITHUB_REPOSITORY=example/repo", "GITHUB_PATH="+filepath.Join(dir, "path"), "GITHUB_OUTPUT="+filepath.Join(dir, "output"), "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"), "INPUT_BASE="+base, "INPUT_TRANSCRIPT="+transcript, "INPUT_FAIL_ON="+tc.policy, "SAIDRAN_DOWNLOAD_BASE="+downloadBase, "SAIDRAN_VERSION="+steps[0].Env["SAIDRAN_VERSION"], "GH_TOKEN=fake-local-only", "FAKE_ROOT="+dir, "CASE_NAME="+tc.name, "PUBLICATION_ERROR="+tc.publication)
			run := func(source string) (int, string) {
				c := exec.Command("bash", "-e", "-o", "pipefail", "-c", source)
				c.Dir = repo
				c.Env = env
				b, e := c.CombinedOutput()
				if e == nil {
					return 0, string(b)
				}
				if x, ok := e.(*exec.ExitError); ok {
					return x.ExitCode(), string(b)
				}
				t.Fatal(e)
				return -1, ""
			}
			status, out := run(steps[0].Run)
			if tc.installError {
				if status != tc.status || !strings.Contains(out, "Download failed") {
					t.Fatalf("missing pinned release: %d %s", status, out)
				}
				if _, err := os.Stat(filepath.Join(dir, "path")); !os.IsNotExist(err) {
					t.Fatal("failed install exported a binary path")
				}
				return
			}
			if status != 0 {
				t.Fatalf("install %d %s", status, out)
			}
			pathBytes, _ := os.ReadFile(filepath.Join(dir, "path"))
			env = append(env, "PATH="+strings.TrimSpace(string(pathBytes))+string(os.PathListSeparator)+fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			status, out = run("saidran --version")
			if status != 0 || !strings.Contains(out, "version "+releaseVersion) {
				t.Fatalf("installed version: %d %s", status, out)
			}
			status, out = run(steps[1].Run)
			outputs, _ := os.ReadFile(filepath.Join(dir, "output"))
			values := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(outputs)), "\n") {
				k, v, ok := strings.Cut(line, "=")
				if ok {
					values[k] = v
				}
			}
			if status != 0 {
				if status != tc.status {
					t.Fatalf("report %d %s", status, out)
				}
				if _, e := os.Stat(filepath.Join(dir, "calls")); !os.IsNotExist(e) {
					t.Fatal("report failure published")
				}
				return
			}
			reportDir := values["report_dir"]
			env = append(env, "REPORT_DIR="+reportDir, "REPORT_STATUS="+values["status"])
			reportBytes, e := os.ReadFile(filepath.Join(reportDir, "report.json"))
			if e != nil {
				t.Fatal(e)
			}
			var model struct {
				Session any `json:"session"`
				Repo    struct{ Baseline struct{ Commit, Ref string } }
			}
			if e := json.Unmarshal(reportBytes, &model); e != nil {
				t.Fatal(e)
			}
			if (model.Session != nil) != tc.transcript || model.Repo.Baseline.Commit != baseSHA {
				t.Fatal("wrong report inputs")
			}
			if tc.base != "" && model.Repo.Baseline.Ref != tc.base {
				t.Fatal("explicit base lost")
			}
			status, out = run(steps[2].Run)
			if status != 0 {
				t.Fatalf("summary %d %s", status, out)
			}
			// Stub the upload action at its actual input, before any publication call.
			artifact := strings.ReplaceAll(steps[3].With["path"], "${{ steps.report.outputs.report_dir }}", reportDir)
			artifactBytes, e := os.ReadFile(artifact)
			if e != nil || !strings.Contains(string(artifactBytes), "Saidran") {
				t.Fatal("missing new artifact")
			}
			summary, _ := os.ReadFile(filepath.Join(dir, "summary"))
			if len(summary) == 0 {
				t.Fatal("empty summary")
			}
			publicationStatus := 0
			if !tc.nonPR {
				publicationStatus, out = run(steps[4].Run)
				if tc.publication == "403" && !strings.Contains(out, "::warning::") {
					t.Fatal(out)
				}
			}
			status, _ = run(steps[5].Run)
			if status == 0 {
				status = publicationStatus
			}
			if status != tc.status {
				t.Fatalf("final %d want %d: %s", status, tc.status, out)
			}
			calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
			if tc.nonPR {
				if len(calls) != 0 {
					t.Fatal("non PR comment")
				}
				return
			}
			if tc.name == "replace" {
				if !strings.Contains(string(calls), "issues/comments/4") || !strings.Contains(string(calls), "PATCH") {
					t.Fatal(string(calls))
				}
			} else if !strings.Contains(string(calls), "POST") {
				t.Fatal(string(calls))
			}
			posted, _ := os.ReadFile(filepath.Join(dir, "posted"))
			var body struct{ Body string }
			json.Unmarshal(posted, &body)
			if !strings.HasPrefix(body.Body, "<!-- saidran -->\n") || strings.Count(body.Body, "<!-- saidran -->") != 1 {
				t.Fatal("wrong marker")
			}
		})
	}
	downloadMu.Lock()
	defer downloadMu.Unlock()
	if len(downloads) != 21 {
		t.Fatalf("expected two versioned downloads per successful install and one missing asset, got %v", downloads)
	}
	for _, path := range downloads {
		if path != "/download/"+releaseVersion+"/"+asset && path != "/download/"+releaseVersion+"/checksums.txt" && path != "/missing/download/"+releaseVersion+"/"+asset {
			t.Fatalf("unexpected download path (must never use latest): %s", path)
		}
	}
}
