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
	patchPath := filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"patches",
		"wails",
		"001-macos-context-menu.patch",
	)
	content, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("read Wails context-menu patch: %v", err)
	}
	for _, expected := range []string{
		"if (window.wails && window.wails.flags)",
		"- (NSMenu *)menuForEvent:(NSEvent *)event",
		"if ( !defaultContextMenuEnabled )",
		"return nil;",
	} {
		if !strings.Contains(string(content), expected) {
			t.Fatalf("Wails context-menu patch missing %q", expected)
		}
	}
}
