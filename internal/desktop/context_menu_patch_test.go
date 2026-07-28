package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWailsContextMenuPatchUsesCapturePhase(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate context-menu patch test")
	}
	scriptPath := filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"scripts",
		"with-wails-context-menu-fix.sh",
	)
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read Wails context-menu patch: %v", err)
	}
	expected := "window.addEventListener('contextmenu', function(event) { event.preventDefault(); }, true);"
	if !strings.Contains(string(content), expected) {
		t.Fatalf("Wails context-menu patch must capture iframe events")
	}
}
