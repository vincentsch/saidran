package scripts_test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerVerifiesBeforeReplacingBinary(t *testing.T) {
	asset := "gaugestone-linux-amd64"
	payload := []byte("#!/bin/sh\nprintf 'installed test binary\\n'\n")
	sum := fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), asset)
	for _, tc := range []struct {
		name, version, checksum, osName, arch string
		failed                                bool
	}{
		{name: "latest", checksum: sum, osName: "Linux", arch: "x86_64"},
		{name: "default directory", checksum: sum, osName: "Linux", arch: "x86_64"},
		{name: "macOS", checksum: sum, osName: "Darwin", arch: "arm64"},
		{name: "pinned", version: "v0.1.0", checksum: sum, osName: "Linux", arch: "x86_64"},
		{name: "checksum mismatch", checksum: strings.Repeat("0", 64) + "  " + asset + "\n", osName: "Linux", arch: "x86_64", failed: true},
		{name: "missing checksum", checksum: "", osName: "Linux", arch: "x86_64", failed: true},
		{name: "failed download", checksum: sum, osName: "Linux", arch: "x86_64", failed: true},
		{name: "unsupported architecture", osName: "Linux", arch: "riscv64", failed: true},
		{name: "windows guidance", osName: "MINGW64_NT", arch: "x86_64", failed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			tree := filepath.Join(base, "release")
			layout := "latest/download"
			if tc.version != "" {
				layout = "download/" + tc.version
			}
			asset := asset
			if tc.osName == "Darwin" {
				asset = "gaugestone-darwin-arm64"
			}
			checksum := strings.ReplaceAll(tc.checksum, "gaugestone-linux-amd64", asset)
			dir := filepath.Join(tree, filepath.FromSlash(layout))
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(checksum), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.name != "failed download" {
				if err := os.WriteFile(filepath.Join(dir, asset), payload, 0600); err != nil {
					t.Fatal(err)
				}
			}
			server := httptest.NewServer(http.FileServer(http.Dir(tree)))
			defer server.Close()
			installDir := filepath.Join(base, "install with spaces")
			if tc.name == "default directory" {
				installDir = filepath.Join(base, "home", ".local", "bin")
			}
			if err := os.MkdirAll(installDir, 0700); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(installDir, "gaugestone")
			previous := []byte("existing installation")
			if err := os.WriteFile(binary, previous, 0755); err != nil {
				t.Fatal(err)
			}
			shims := filepath.Join(base, "shims")
			if err := os.Mkdir(shims, 0700); err != nil {
				t.Fatal(err)
			}
			uname := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in -s) echo '%s';; -m) echo '%s';; esac\n", tc.osName, tc.arch)
			if err := os.WriteFile(filepath.Join(shims, "uname"), []byte(uname), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", shims+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GAUGESTONE_DOWNLOAD_BASE", server.URL)
			t.Setenv("GAUGESTONE_VERSION", tc.version)
			t.Setenv("HOME", filepath.Join(base, "home"))
			t.Setenv("INSTALL_DIR", installDir)
			if tc.name == "default directory" {
				t.Setenv("INSTALL_DIR", "")
			}
			command := exec.Command("bash", "../install.sh")
			output, err := command.CombinedOutput()
			if (err != nil) != tc.failed {
				t.Fatalf("installer: %v\n%s", err, output)
			}
			installed, readErr := os.ReadFile(binary)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if tc.failed {
				if string(installed) != string(previous) {
					t.Fatal("failed install replaced working binary")
				}
				if !strings.Contains(string(output), "Error:") {
					t.Fatalf("missing failure explanation: %s", output)
				}
				if tc.name == "windows guidance" && !strings.Contains(string(output), ".exe") {
					t.Fatal("missing Windows download guidance")
				}
				return
			}
			if string(installed) != string(payload) {
				t.Fatal("installed wrong binary")
			}
			if !strings.Contains(string(output), "Installed: "+binary+"\n") || !strings.Contains(string(output), "export PATH=") {
				t.Fatalf("install instructions: %s", output)
			}
			printed := strings.Split(strings.TrimPrefix(string(output), "Installed: "), "\n")[0]
			run, err := exec.Command(printed).CombinedOutput()
			if err != nil || string(run) != "installed test binary\n" {
				t.Fatalf("printed path: %v %s", err, run)
			}
		})
	}
}
