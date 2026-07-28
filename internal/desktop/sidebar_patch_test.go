package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWailsSidebarPatchLayersOverWebview(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate sidebar patch test")
	}
	patchPath := filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"patches",
		"wails",
		"002-macos-native-sidebar.patch",
	)
	content, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("read Wails sidebar patch: %v", err)
	}
	for _, expected := range []string{
		// The sidebar must sit above the webview or it cannot blur the page.
		"addSubview:sidebar positioned:NSWindowAbove relativeTo:nil",
		"NSVisualEffectBlendingModeWithinWindow",
		"NSVisualEffectMaterialSidebar",
		// Driven from JS so no Go-side Wails options need patching.
		`[m hasPrefix:@"sidebar:"]`,
		"configureWithJSON",
		"wails:sidebar",
		// Dragging the window by the traffic-light strip.
		"mouseDownCanMoveWindow",
		// Floating inset panel rather than a full-height column.
		"setCornerRadius",
		"setBorderWidth",
		// Squircle corners.
		`forKey:@"cornerCurve"`,
		// The panel must BE the effect view: nesting one inside another
		// layer-backed view breaks within-window backdrop sampling, and the
		// sidebar shows the desktop instead of the page.
		"@interface WailsSidebar : NSVisualEffectView",
		// Traffic lights placed by hand, since there is no toolbar to inset them.
		"layoutTrafficLights",
		"standardWindowButton:NSWindowCloseButton",
	} {
		if !strings.Contains(string(content), expected) {
			t.Fatalf("Wails sidebar patch missing %q", expected)
		}
	}
}

func TestWailsDockBadgePatchDistinguishesViewerProcess(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate dock badge patch test")
	}
	patchPath := filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"patches",
		"wails",
		"003-macos-viewer-dock-icon.patch",
	)
	content, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("read Wails dock badge patch: %v", err)
	}
	for _, expected := range []string{
		`[m isEqualToString:@"dockbadge"]`,
		"WailsApplyDockBadge",
		"setApplicationIconImage",
	} {
		if !strings.Contains(string(content), expected) {
			t.Fatalf("Wails dock badge patch missing %q", expected)
		}
	}
}
