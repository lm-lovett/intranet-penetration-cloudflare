package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func OpenBrowser(rawURL string) error {
	name, args := browserCommand(rawURL)
	if name == "" {
		return fmt.Errorf("no browser command for %s", runtime.GOOS)
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

func browserCommand(rawURL string) (string, []string) {
	switch runtime.GOOS {
	case "linux":
		return "xdg-open", []string{rawURL}
	case "darwin":
		return "open", []string{rawURL}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}
	default:
		return "", nil
	}
}

func stdinIsTerminal() bool {
	st, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
