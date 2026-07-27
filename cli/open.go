package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

type Opener interface{ Open(string) error }
type SystemOpener struct{}

func (SystemOpener) Open(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	path, err := exec.LookPath(name)
	if errors.Is(err, exec.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find browser opener: %w", err)
	}
	if err := exec.Command(path, args...).Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
