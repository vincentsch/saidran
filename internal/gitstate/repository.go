package gitstate

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vincentsch/rungrad"
)

func Root(dir string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		install := "sudo apt install git"
		switch runtime.GOOS {
		case "darwin":
			install = "xcode-select --install"
		case "windows":
			install = "winget install Git.Git"
		}
		return "", rungrad.NewError(rungrad.ExitAPI, "Git is missing. Install it with `"+install+"`.")
	}
	cmd := exec.Command(git, "-C", dir, "rev-parse", "--show-toplevel")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	b, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && strings.Contains(string(exit.Stderr), "not a git repository") {
			return "", rungrad.NewError(rungrad.ExitNotFound, "This folder is outside a Git repository. Run saidran sessions inside your repository.")
		}
		return "", rungrad.NewError(rungrad.ExitAPI, "Could not read the Git repository. Check its permissions and Git configuration.")
	}
	return strings.TrimSpace(string(b)), nil
}
