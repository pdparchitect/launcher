package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const dataRootEnvironment = "PDPARCHITECT_LAUNCHER_HOME"

func DataRoot() (string, error) {
	if override := os.Getenv(dataRootEnvironment); override != "" {
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return platformDataRoot(
		runtime.GOOS,
		home,
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("XDG_DATA_HOME"),
	)
}

func platformDataRoot(
	goos string,
	home string,
	localApp string,
	xdgData string,
) (string, error) {
	switch goos {
	case "darwin":
		if home == "" {
			return "", errors.New("home directory is unavailable")
		}
		return filepath.Join(
			home,
			"Library",
			"Application Support",
			"Launcher",
		), nil
	case "windows":
		if localApp == "" {
			if home == "" {
				return "", errors.New("local application data is unavailable")
			}
			localApp = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localApp, "Launcher"), nil
	default:
		if xdgData != "" {
			return filepath.Join(xdgData, "launcher"), nil
		}
		if home == "" {
			return "", errors.New("home directory is unavailable")
		}
		return filepath.Join(home, ".local", "share", "launcher"), nil
	}
}
