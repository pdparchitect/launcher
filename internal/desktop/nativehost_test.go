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
		"var nativeSidebarTrailingEdge: CGFloat = 280",
		"container.nativeSidebarTrailingEdge = sidebarTrailingEdge",
		"override func hitTest(_ point: NSPoint) -> NSView?",
		"guard pointInWindow.x >= nativeSidebarTrailingEdge",
		"func publish(insets: PageInsets)",
		"window.wailsSidebarInset = \\(sidebar);",
		"window.wailsTitlebarInset = \\(titlebar);",
		"wails:sidebar-inset",
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
		"columnVisibility == .detailOnly ? 0 : sidebarEdge",
		".onGeometryChange(for: CGFloat.self)",
		"max(0, proxy.frame(in: .global).maxX)",
		"proxy.safeAreaInsets.top",
		".onChange(of: pageInsets, initial: true)",
		"Button {\n                    model.select(item.id)",
		".buttonStyle(.plain)",
		"} detail: {\n            WailsWebView(",
		"sidebarTrailingEdge: sidebarInset",
		".backgroundExtensionEffect()",
		// The page underlaps the sidebar so the glass samples the real content
		// instead of the effect's mirrored copy of it. What keeps the sidebar
		// usable is hitTest, not a gap in the web view.
		"edges: .all",
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
		"edges: [.top, .trailing, .bottom]",
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
		"Frameless:                false",
		"TitleBar: mac.TitleBarHidden()",
		`StartHidden:              runtime.GOOS == "darwin"`,
		"func viewerWindowChrome() *mac.Options",
		"Mac:                      viewerWindowChrome(),",
		"nativehost.InstallViewerChrome()",
		// The native host fixes the web view's frame below the title-bar strip,
		// so full-size content can provide genuinely transparent chrome without
		// putting the agent's interface under the controls.
		"FullSizeContent:            true,",
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
		// The viewer's title bar. Approaching the top edge grows the window
		// upward by one title bar rather than resizing the agent's interface,
		// so the frame height changes while its origin never does. Two
		// thresholds because revealing moves the edge the pointer is measured
		// against. Controls are hidden as well as faded: a zero-alpha control
		// still takes clicks.
		"LauncherNativeHostInstallViewerChrome",
		"frame.size.height += revealed ? gTitlebarHeight : -gTitlebarHeight;",
		"[gViewerWindow setFrame:frame display:NO animate:YES];",
		"contentRectForFrameRect:window.frame]",
		"kTitlebarRevealDistance",
		"kTitlebarCollapseDistance",
		"NSEventMaskMouseMoved",
		"button.hidden = YES",
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
		"{ sidebar: SIDEBAR_COLUMN_WIDTH, titlebar: 0 }",
		": nativeSidebar.insets",
		"nativeSidebar.onInsets((insets) => this.applyNativeInsets(insets))",
		"style.setProperty('--sidebar-width', `${sidebar}px`)",
		"style.setProperty('--content-top', `${titlebar}px`)",
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
		// The drag region and dialogs clear the native sidebar because the page
		// runs underneath it rather than starting after it.
		"left: var(--sidebar-width)",
		".is-macos-desktop .native-window-drag-region {",
		".is-macos-desktop .launcher-dialog {",
		"transform: translateX(calc(var(--sidebar-width) / 2))",
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("native sidebar spacing or drag layout missing %q", expected)
		}
	}
}

func TestMacOSViewerChromeNeverFadesOverTheWebContent(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate native host test")
	}
	launcher := filepath.Join(filepath.Dir(filename), "..", "..")
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
		"@interface LauncherTitlebarBackdrop : NSVisualEffectView",
		"@interface LauncherViewerBorder : NSView",
		"InstallViewerBorder(window);",
		"border.layer.borderWidth = 1.0 / scale;",
		"border.layer.cornerRadius = MAX(0.0, cornerRadius - inset);",
		"if (@available(macOS 10.14, *))",
		"NSVisualEffectMaterialHeaderView",
		"if (@available(macOS 11.0, *))",
		"NSTitlebarSeparatorStyleNone",
		"SetTitlebarLayoutRevealed(YES);",
		"AnimateTitlebarChrome(YES",
		"AnimateTitlebarChrome(NO",
		"SetTitlebarLayoutRevealed(NO);",
		"gViewerWindow.titlebarAppearsTransparent = YES;",
		"NSViewWidthSizable | NSViewMaxYMargin",
		"MAX(0.0, NSHeight(contentBounds) - chromeHeight)",
		"kCAMediaTimingFunctionEaseInEaseOut",
	} {
		if !strings.Contains(bridge, expected) {
			t.Fatalf("viewer chrome missing %q", expected)
		}
	}
	if strings.Contains(
		bridge,
		"gViewerWindow.titlebarAppearsTransparent = NO;",
	) {
		t.Fatal("revealed viewer title bar must remain translucent")
	}
	if strings.Contains(bridge, "NSWindowTitlebarSeparatorStyleNone") {
		t.Fatal("viewer chrome uses a nonexistent AppKit separator constant")
	}

	revealLayout := strings.Index(
		bridge,
		"SetTitlebarLayoutRevealed(YES);",
	)
	revealChrome := strings.Index(bridge, "AnimateTitlebarChrome(YES")
	collapseChrome := strings.Index(bridge, "AnimateTitlebarChrome(NO")
	collapseLayout := strings.LastIndex(
		bridge,
		"SetTitlebarLayoutRevealed(NO);",
	)
	if revealLayout < 0 || revealChrome < revealLayout {
		t.Fatal("viewer must reserve the title-bar strip before fading chrome in")
	}
	if collapseChrome < 0 || collapseLayout < collapseChrome {
		t.Fatal("viewer must fade chrome out before returning its strip to content")
	}
}

func TestMacOSViewerChromeRestoresTrafficLightsInFullscreen(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate native host test")
	}
	launcher := filepath.Join(filepath.Dir(filename), "..", "..")
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
		"NSWindowWillEnterFullScreenNotification",
		"NSWindowDidExitFullScreenNotification",
		"static BOOL gViewerFullscreen = NO;",
		"gViewerFullscreen = fullscreen;",
		"SetTitlebarChromeImmediately(fullscreen);",
		"[gTitlebarBackdrop.layer removeAllAnimations];",
		"[button.layer removeAllAnimations];",
		"if (ViewerIsFullscreen())",
	} {
		if !strings.Contains(bridge, expected) {
			t.Fatalf("fullscreen viewer chrome missing %q", expected)
		}
	}

	enter := strings.Index(
		bridge,
		"NSWindowWillEnterFullScreenNotification",
	)
	exit := strings.Index(bridge, "NSWindowDidExitFullScreenNotification")
	if enter < 0 {
		t.Fatal("viewer missing its fullscreen-entry observer")
	}
	if !strings.Contains(
		bridge[enter:],
		"SetViewerFullscreen(YES);",
	) {
		t.Fatal("viewer must reveal its hidden traffic lights before fullscreen")
	}
	if exit < 0 {
		t.Fatal("viewer missing its fullscreen-exit observer")
	}
	if !strings.Contains(
		bridge[exit:],
		"SetViewerFullscreen(NO);",
	) {
		t.Fatal("viewer must restore collapsed chrome after leaving fullscreen")
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
	if !strings.Contains(linkage, "-framework QuartzCore") {
		t.Fatal("cgo bridge does not link the title-bar animation framework")
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
