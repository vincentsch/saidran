package cli

import (
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
)

func openReport(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	if runtime.GOOS == "windows" {
		u.Path = "/" + u.Path
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u.String())
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u.String())
	default:
		cmd = exec.Command("xdg-open", u.String())
	}
	return cmd.Run()
}
