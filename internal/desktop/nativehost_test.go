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
		"final class WailsWebViewContainer: NSView",
		"struct WailsWebView: NSViewRepresentable",
		"let webView: WKWebView",
		"var nativeSidebarInteractionWidth: CGFloat = 280",
		"container.nativeSidebarInteractionWidth = sidebarInteractionWidth",
		"override func hitTest(_ point: NSPoint) -> NSView?",
		"guard pointInWindow.x >= nativeSidebarInteractionWidth",
		"container.layer?.masksToBounds = true",
		"container.addSubview(webView)",
		"NSHostingSceneRepresentation",
		"WindowGroup(id: Self.identifier)",
		"NSApplication.shared.addSceneRepresentation(representation)",
		"representation.environment.openWindow(",
		"wailsWindow?.orderOut(nil)",
		"func applicationShouldHandleReopen(",
		"override func forwardingTarget(for selector: Selector!) -> Any?",
		"NSApplication.shared.delegate = reopenDelegate",
		".backgroundExtensionEffect()",
		".navigationSplitViewStyle(",
		".prominentDetail",
		".frame(minWidth: 820, minHeight: 560)",
		".defaultSize(width: 1180, height: 760)",
		".windowToolbarStyle(.unified(showsTitle: false))",
		`name: "launcherNative"`,
		`config["action"] as? String == "dragWindow"`,
		"mouseDownEvent ?? window.currentEvent",
		"window.performDrag(with: event)",
		"NSEvent.addLocalMonitorForEvents(",
		"matching: [.leftMouseDown, .leftMouseUp]",
		"customElements.whenDefined('launcher-app').then(",
		"const link = launcher?.querySelector(",
		"link.click();",
		"?.setUpNativeSidebar?.();",
		"launcher.setScreen(",
		"let windowAddress = UInt(bitPattern: windowPointer)",
		"let webViewAddress = UInt(bitPattern: webViewPointer)",
	} {
		if !strings.Contains(swift, expected) {
			t.Fatalf("SwiftUI native host missing %q", expected)
		}
	}

	rootStart := strings.Index(swift, "private struct RootView: View")
	rootEnd := strings.Index(swift, "private struct NativeWindowScene: Scene")
	if rootStart < 0 || rootEnd <= rootStart {
		t.Fatal("locate native RootView")
	}
	root := swift[rootStart:rootEnd]
	for _, expected := range []string{
		"var body: some View {\n        NavigationSplitView(",
		"columnVisibility: $columnVisibility",
		"NavigationSplitViewVisibility = .all",
		"columnVisibility == .detailOnly ? 0 : 280",
		"Button {\n                    model.select(item.id)",
		".buttonStyle(.plain)",
		"} detail: {\n            WailsWebView(",
		"sidebarInteractionWidth: sidebarInteractionWidth",
		".backgroundExtensionEffect()",
		"edges: [.top, .trailing, .bottom]",
		".navigationSplitViewStyle(\n            .prominentDetail\n        )",
		".toolbarBackgroundVisibility(",
	} {
		if !strings.Contains(root, expected) {
			t.Fatalf("macOS 26 RootView diverges from the reference: missing %q", expected)
		}
	}
	for _, unwanted := range []string{
		"private var splitView",
		"private var webDetail",
		"if #available",
		"edges: .all",
		// The sidebar toggle is a default item of the sidebar column's toolbar,
		// and that toolbar is what holds the window's titlebar band open.
		// Removing it costs the sidebar its inset and displaces the traffic
		// lights, so the button stays and collapses the sidebar for real.
		"ToolbarDefaultItemKind.sidebarToggle",
		"Binding.constant(",
	} {
		if strings.Contains(root, unwanted) {
			t.Fatalf("RootView must directly match the reference, found %q", unwanted)
		}
	}

	if strings.Contains(swift, "@main") {
		t.Fatal("SwiftUI native host must share Wails' process, not add an entry point")
	}
	for _, unwanted := range []string{
		"Button(action: toggleSidebar)",
		`systemImage: "sidebar.leading"`,
		"LegacyRootView",
		"NSHostingController",
		"#available(macOS",
	} {
		if strings.Contains(swift, unwanted) {
			t.Fatalf("native scene contains a competing or compatibility implementation %q", unwanted)
		}
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
		`StartHidden:              runtime.GOOS == "darwin"`,
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
		"installWindowDragBridge()",
		"event.stopImmediatePropagation()",
		"handler.postMessage({ action: 'dragWindow' })",
		"{ capture: true }",
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

	styles := readNativeHostSource(
		t,
		filepath.Join(launcher, "internal", "httpapi", "web", "styles.css"),
	)
	for _, expected := range []string{
		".launcher-shell.has-native-sidebar .sidebar {",
		"visibility: hidden",
		"pointer-events: none",
		".is-swiftui-host .topbar {",
		"left: 0",
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("native sidebar spacing or drag layout missing %q", expected)
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
		"MACOS_DEPLOYMENT_TARGET ?= 26.0",
		"SWIFT_ARCHIVE_HASH = $(shell shasum -a 256",
		"-X main.swiftArchiveHash=$(SWIFT_ARCHIVE_HASH)",
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

	mainSource := readNativeHostSource(t, filepath.Join(launcher, "main.go"))
	if !strings.Contains(mainSource, `swiftArchiveHash = "none"`) {
		t.Fatal("macOS build lacks the Swift archive cache-key variable")
	}

	manifest := readNativeHostSource(
		t,
		filepath.Join(launcher, "macos", "Package.swift"),
	)
	if !strings.Contains(manifest, "type: .static") {
		t.Fatal("LauncherNative must be a static Swift library")
	}
	if !strings.Contains(manifest, "platforms: [.macOS(.v26)]") {
		t.Fatal("LauncherNative must target macOS 26 for Liquid Glass")
	}
	if strings.Contains(manifest, ".executableTarget(") {
		t.Fatal("LauncherNative must not build a second executable")
	}

	infoPlist := readNativeHostSource(
		t,
		filepath.Join(launcher, "build", "darwin", "Info.plist"),
	)
	if !strings.Contains(
		infoPlist,
		"<key>LSMinimumSystemVersion</key>\n    <string>26.0</string>",
	) {
		t.Fatal("application bundle must require macOS 26")
	}
	if strings.Contains(infoPlist, "UIDesignRequiresCompatibility") {
		t.Fatal("application bundle must not opt out of the current design")
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
		`deployment_target="${MACOSX_DEPLOYMENT_TARGET:-26.0}"`,
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
