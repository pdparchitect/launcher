package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMacOSNativeHostEmbedsTheWailsWebViewInSwiftUI(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate native host test")
	}
	launcher := filepath.Join(filepath.Dir(filename), "..", "..")

	swift := readNativeHostSource(
		t,
		filepath.Join(
			launcher,
			"macos",
			"Sources",
			"LauncherNative",
			"NativeShell.swift",
		),
	)
	for _, expected := range []string{
		"NavigationSplitView(",
		"struct WailsWebView: NSViewRepresentable",
		"let webView: WKWebView",
		"NSHostingController(",
		"window.contentViewController = hostingController",
		".backgroundExtensionEffect()",
		".navigationSplitViewStyle(.prominentDetail)",
		"ToolbarDefaultItemKind.sidebarToggle",
		"columnVisibility == .detailOnly ? .all : .detailOnly",
		`systemImage: "sidebar.leading"`,
		".frame(minWidth: 820, minHeight: 560)",
		`name: "launcherNative"`,
		"let windowAddress = UInt(bitPattern: windowPointer)",
		"let webViewAddress = UInt(bitPattern: webViewPointer)",
	} {
		if !strings.Contains(swift, expected) {
			t.Fatalf("SwiftUI native host missing %q", expected)
		}
	}
	if strings.Contains(swift, "@main") {
		t.Fatal("SwiftUI native host must share Wails' process, not add an entry point")
	}
	if strings.Contains(swift, "NSHostingView(") {
		t.Fatal("SwiftUI must own the Wails window through a hosting controller")
	}

	wailsHost := readNativeHostSource(
		t,
		filepath.Join(launcher, "internal", "desktop", "run_wails.go"),
	)
	for _, expected := range []string{
		"macOSWindowWidth     = 1180",
		"macOSWindowHeight    = 760",
		"macOSWindowMinWidth  = 820",
		"macOSWindowMinHeight = 560",
		"geometry := mainWindowGeometry()",
		"TitleBar: mac.TitleBarHidden()",
	} {
		if !strings.Contains(wailsHost, expected) {
			t.Fatalf("Wails window does not match the SwiftUI reference: missing %q", expected)
		}
	}

	bridge := readNativeHostSource(
		t,
		filepath.Join(
			launcher,
			"internal",
			"desktop",
			"nativehost",
			"nativehost_darwin.m",
		),
	)
	for _, expected := range []string{
		"FindWebView",
		"LauncherNativeInstall",
		"wails:openInspector",
		"NSApp.applicationIconImage = badged",
	} {
		if !strings.Contains(bridge, expected) {
			t.Fatalf("native AppKit bridge missing %q", expected)
		}
	}

	sidebar := readNativeHostSource(
		t,
		filepath.Join(
			launcher,
			"internal",
			"httpapi",
			"web",
			"native-sidebar.js",
		),
	)
	for _, expected := range []string{
		"messageHandlers?.launcherNative",
		"handler.postMessage({",
		"wails:sidebar-ready",
		"wails:sidebar",
	} {
		if !strings.Contains(sidebar, expected) {
			t.Fatalf("web-to-Swift sidebar bridge missing %q", expected)
		}
	}

	app := readNativeHostSource(
		t,
		filepath.Join(
			launcher,
			"internal",
			"httpapi",
			"web",
			"components",
			"launcher-app.js",
		),
	)
	for _, expected := range []string{
		"const sidebarWidth = preview ? SIDEBAR_COLUMN_WIDTH : 0",
		"classList.toggle('is-swiftui-host', !preview)",
	} {
		if !strings.Contains(app, expected) {
			t.Fatalf("SwiftUI detail layout missing %q", expected)
		}
	}
}

func TestMacOSNativeHostIsStaticallyLinkedIntoWails(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate native host test")
	}
	launcher := filepath.Join(filepath.Dir(filename), "..", "..")

	makefile := readNativeHostSource(t, filepath.Join(launcher, "Makefile"))
	for _, expected := range []string{
		"--product LauncherNative",
		"-extld $(SWIFT_LINKER)",
		"build-macos: native-macos",
	} {
		if !strings.Contains(makefile, expected) {
			t.Fatalf("single-binary macOS build missing %q", expected)
		}
	}
	for _, unwanted := range []string{
		"Contents/Resources/launcher",
		"build-swift:",
	} {
		if strings.Contains(makefile, unwanted) {
			t.Fatalf("macOS build still contains helper design %q", unwanted)
		}
	}

	manifest := readNativeHostSource(
		t,
		filepath.Join(launcher, "macos", "Package.swift"),
	)
	if !strings.Contains(manifest, "type: .static") {
		t.Fatal("LauncherNative must be a static Swift library")
	}
	if strings.Contains(manifest, ".executableTarget(") {
		t.Fatal("LauncherNative must not build a second executable")
	}

	linkage := readNativeHostSource(
		t,
		filepath.Join(
			launcher,
			"internal",
			"desktop",
			"nativehost",
			"nativehost_darwin.go",
		),
	)
	if !strings.Contains(linkage, "-lLauncherNative") {
		t.Fatal("cgo bridge does not link the static Swift library")
	}

	linker := readNativeHostSource(
		t,
		filepath.Join(launcher, "scripts", "swift-linker.sh"),
	)
	for _, expected := range []string{
		"libLauncherNative.a",
		`swiftc_path="$(xcrun --find swiftc)"`,
		`-target "${target_arch}-apple-macosx${deployment_target}"`,
		`-sdk "$sdk_path"`,
		`swift_link_args+=("-Xlinker" "$linker_option")`,
		`"$swiftc_path"`,
	} {
		if !strings.Contains(linker, expected) {
			t.Fatalf("Swift external linker missing %q", expected)
		}
	}
	for _, unwanted := range []string{
		"swift-autolink-extract",
		"xcrun clang",
	} {
		if strings.Contains(linker, unwanted) {
			t.Fatalf("Swift external linker still uses unavailable tool %q", unwanted)
		}
	}
}

func readNativeHostSource(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
