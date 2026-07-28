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
		// Rounding must not go through the layer on an effect view.
		"WailsSidebarRoundedMask",
		// The column defaults to 100pt; unset, every label truncates.
		"setWidth:WailsSidebarDefaultWidth",
		"sizeLastColumnToFit",
		// The panel must BE the effect view: nesting one inside another
		// layer-backed view breaks within-window backdrop sampling, and the
		// sidebar shows the desktop instead of the page.
		"@interface WailsSidebar : NSVisualEffectView",
		// Traffic lights placed by hand, since there is no toolbar to inset them.
		"layoutTrafficLights",
		"standardWindowButton:NSWindowCloseButton",
		// Emphasis must be pinned by overriding the getter: AppKit re-applies
		// it on focus changes, so setting the property once does not hold.
		"@interface WailsSidebarRowView : NSTableRowView",
		"- (BOOL) isEmphasized {",
		// A subclass AppKit never instantiates does nothing: the delegate has
		// to hand it back, or the accent highlight returns.
		"rowViewForRow:(NSInteger)row {",
		"[[[WailsSidebarRowView alloc] initWithFrame:NSZeroRect] autorelease]",
		// Layer masking silently kills backdrop sampling, leaving a flat panel.
		"setMaskImage:",
		"NSVisualEffectStateActive",
		// Rows are built during reloadData, before the table has a width, so
		// any label sized from that width comes out truncated.
		"setTranslatesAutoresizingMaskIntoConstraints:NO",
		"NSLayoutConstraint activateConstraints:",
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
