package config

import (
	"path/filepath"
	"testing"
)

func TestDataRootUsesOverride(t *testing.T) {
	t.Setenv("PDPARCHITECT_LAUNCHER_HOME", "/tmp/custom-launcher-home")
	root, err := DataRoot()
	if err != nil {
		t.Fatalf("DataRoot() error = %v", err)
	}
	if root != "/tmp/custom-launcher-home" {
		t.Fatalf("DataRoot() = %q", root)
	}
}

func TestPlatformDataRoot(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		home     string
		localApp string
		xdgData  string
		want     string
	}{
		{"macOS", "darwin", "/Users/alice", "", "",
			"/Users/alice/Library/Application Support/Launcher"},
		{"Windows", "windows", `C:\Users\alice`,
			`C:\Users\alice\AppData\Local`, "",
			filepath.Join(`C:\Users\alice\AppData\Local`, "Launcher")},
		{"Linux XDG", "linux", "/home/alice", "", "/data/alice",
			"/data/alice/launcher"},
		{"Linux fallback", "linux", "/home/alice", "", "",
			"/home/alice/.local/share/launcher"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := platformDataRoot(
				test.goos,
				test.home,
				test.localApp,
				test.xdgData,
			)
			if err != nil {
				t.Fatalf("platformDataRoot() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("platformDataRoot() = %q, want %q", got, test.want)
			}
		})
	}
}
