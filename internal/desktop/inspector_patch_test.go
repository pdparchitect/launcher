package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The Web Inspector is reachable from a packaged build, but only one built with
// the devtools tag. The hotkey itself is compiled in unconditionally: the
// message it sends is a no-op without that tag, so a release binary carries no
// private-API inspector code.
func TestWailsInspectorHotkeyUsesTheStandardCombo(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate inspector patch test")
	}
	content, err := os.ReadFile(filepath.Join(
		filepath.Dir(filename), "..", "..",
		"patches", "wails", "004-macos-inspector-hotkey.patch",
	))
	if err != nil {
		t.Fatalf("read Wails inspector patch: %v", err)
	}
	for _, expected := range []string{
		"NSEventModifierFlagCommand | NSEventModifierFlagOption",
		// A key code would break on non-QWERTY layouts.
		"charactersIgnoringModifiers",
		`processMessage("wails:openInspector")`,
	} {
		if !strings.Contains(string(content), expected) {
			t.Fatalf("inspector hotkey patch missing %q", expected)
		}
	}
}
