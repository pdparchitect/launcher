import AppKit
import Foundation
import Observation
import SwiftUI
import WebKit

private struct SidebarItem: Identifiable {
    let id: String
    let title: String
    let symbol: String
}

@MainActor
@Observable
private final class NativeShellModel: NSObject, WKScriptMessageHandler {
    var items: [SidebarItem] = [
        SidebarItem(id: "home", title: "Home", symbol: "house"),
        SidebarItem(
            id: "agents",
            title: "Agents",
            symbol: "square.stack.3d.up"
        ),
        SidebarItem(id: "marketplace", title: "Marketplace", symbol: "bag"),
        SidebarItem(
            id: "activity",
            title: "Activity",
            symbol: "waveform.path.ecg"
        ),
    ]
    var selection: String? = "home"

    let webView: WKWebView
    private var mouseDownEvent: NSEvent?

    init(webView: WKWebView) {
        self.webView = webView
    }

    func select(_ identifier: String?) {
        guard let identifier else {
            return
        }
        selection = identifier

        guard
            let data = try? JSONSerialization.data(withJSONObject: identifier),
            let encoded = String(data: data, encoding: .utf8)
        else {
            return
        }
        webView.evaluateJavaScript(
            """
            customElements.whenDefined('launcher-app').then(() => {
                const launcher = document.querySelector('launcher-app');
                const link = launcher?.querySelector(
                    `[data-screen-link=\(encoded)]`
                );

                if (link instanceof HTMLElement) {
                    link.click();
                    return;
                }

                if (typeof launcher?.setScreen === 'function') {
                    launcher.setScreen(\(encoded));
                    return;
                }

                window.dispatchEvent(
                    new CustomEvent(
                        'wails:sidebar',
                        { detail: { id: \(encoded) } }
                    )
                );
            });
            """
        )
    }

    func trackMouseEvent(_ event: NSEvent) {
        guard event.window === webView.window else {
            return
        }

        switch event.type {
        case .leftMouseDown:
            mouseDownEvent = event

        case .leftMouseUp:
            mouseDownEvent = nil

        default:
            break
        }
    }

    func userContentController(
        _ userContentController: WKUserContentController,
        didReceive message: WKScriptMessage
    ) {
        guard
            message.name == "launcherNative",
            let config = message.body as? [String: Any]
        else {
            return
        }

        if config["action"] as? String == "dragWindow" {
            guard
                let window = webView.window,
                let event = mouseDownEvent ?? window.currentEvent
            else {
                return
            }
            window.performDrag(with: event)

            return
        }

        if let entries = config["items"] as? [[String: Any]] {
            let configured = entries.compactMap { entry -> SidebarItem? in
                guard
                    entry["group"] == nil,
                    let id = entry["id"] as? String,
                    let title = entry["title"] as? String
                else {
                    return nil
                }
                return SidebarItem(
                    id: id,
                    title: title,
                    symbol: entry["symbol"] as? String ?? "circle"
                )
            }
            if !configured.isEmpty {
                items = configured
            }
        }

        if let selected = config["selected"] as? String {
            selection = selected
        }

        webView.evaluateJavaScript(
            """
            window.wailsNativeSidebar = true;
            window.dispatchEvent(
                new CustomEvent('wails:sidebar-ready')
            );
            """
        )
    }
}

private final class WailsWebViewContainer: NSView {
    /*
     NavigationSplitView's sidebar is permanently visible and is at most 280
     points wide. AppKit-backed views can otherwise win hit testing even where
     SwiftUI is visually compositing the sidebar above them.
     */
    private let nativeSidebarInteractionWidth: CGFloat = 280

    override func hitTest(_ point: NSPoint) -> NSView? {
        guard let superview else {
            return super.hitTest(point)
        }

        let pointInWindow = superview.convert(point, to: nil)
        guard pointInWindow.x >= nativeSidebarInteractionWidth else {
            return nil
        }

        return super.hitTest(point)
    }
}

private struct WailsWebView: NSViewRepresentable {
    let webView: WKWebView

    func makeNSView(context: Context) -> NSView {
        /*
         Keep Wails' WKWebView inside a detail-column container. Returning the
         already full-window WKWebView directly allows its backing layers to
         retain the old window geometry and cover the sidebar's hit-test area.
         SwiftUI owns the container's frame; the web view can only fill it.
         */
        let container = WailsWebViewContainer()
        container.wantsLayer = true
        container.layer?.masksToBounds = true

        webView.removeFromSuperview()
        webView.frame = container.bounds
        webView.autoresizingMask = [.width, .height]
        container.addSubview(webView)

        return container
    }

    func updateNSView(_ container: NSView, context: Context) {
        guard webView.superview !== container else {
            return
        }

        webView.removeFromSuperview()
        webView.frame = container.bounds
        webView.autoresizingMask = [.width, .height]
        container.addSubview(webView)
    }
}

private struct RootView: View {
    @Bindable var model: NativeShellModel

    private var selection: Binding<String?> {
        Binding(
            get: { model.selection },
            set: { model.select($0) }
        )
    }

    private var lockedColumnVisibility:
        Binding<NavigationSplitViewVisibility>
    {
        Binding.constant(
            NavigationSplitViewVisibility.all
        )
    }

    var body: some View {
        NavigationSplitView(
            columnVisibility: lockedColumnVisibility
        ) {
            List(model.items, selection: selection) { item in
                Button {
                    model.select(item.id)
                } label: {
                    Label(
                        item.title,
                        systemImage: item.symbol
                    )
                    .frame(
                        maxWidth: .infinity,
                        alignment: .leading
                    )
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .tag(item.id)
            }
            .listStyle(.sidebar)
            .navigationSplitViewColumnWidth(
                min: 210,
                ideal: 240,
                max: 280
            )
        } detail: {
            WailsWebView(webView: model.webView)
                .backgroundExtensionEffect()
                /*
                 The effect supplies the visual copy beneath the sidebar.
                 Extending the AppKit-backed WKWebView itself through the
                 leading safe area would put its hit-test surface above the
                 native List and sidebar toolbar button.
                 */
                .ignoresSafeArea(
                    .container,
                    edges: [.top, .trailing, .bottom]
                )
        }
        .navigationSplitViewStyle(
            .prominentDetail
        )
        .toolbar(
            removing:
                ToolbarDefaultItemKind.sidebarToggle
        )
        .toolbarBackgroundVisibility(
            Visibility.hidden,
            for: ToolbarPlacement.windowToolbar
        )
    }
}

private struct NativeWindowScene: Scene {
    static let identifier = "launcher-main-window"

    let model: NativeShellModel

    var body: some Scene {
        WindowGroup(id: Self.identifier) {
            RootView(model: model)
                .frame(minWidth: 820, minHeight: 560)
        }
        .defaultSize(width: 1180, height: 760)
        .windowToolbarStyle(.unified(showsTitle: false))
    }
}

@MainActor
private final class NativeSceneHost {
    private let representation:
        NSHostingSceneRepresentation<NativeWindowScene>
    private weak var wailsWindow: NSWindow?

    init(model: NativeShellModel, wailsWindow: NSWindow) {
        representation = NSHostingSceneRepresentation {
            NativeWindowScene(model: model)
        }
        self.wailsWindow = wailsWindow
    }

    func present() {
        NSApplication.shared.addSceneRepresentation(representation)
        representation.environment.openWindow(
            id: NativeWindowScene.identifier
        )

        /*
         Wails created the configured WKWebView and still owns the backend
         lifecycle, but its temporary AppKit window is not part of the visible
         interface. The SwiftUI WindowGroup above owns the real app window.
         */
        wailsWindow?.orderOut(nil)
        NSApplication.shared.activate(ignoringOtherApps: true)
    }
}

@MainActor
private final class NativeShell {
    static let shared = NativeShell()

    private var sceneHost: NativeSceneHost?
    private var model: NativeShellModel?
    private var mouseEventMonitor: Any?

    func install(window: NSWindow, webView: WKWebView) {
        if model == nil {
            let model = NativeShellModel(webView: webView)
            self.model = model

            webView.removeFromSuperview()
            webView.autoresizingMask = []
            webView.configuration.userContentController.add(
                model,
                name: "launcherNative"
            )
            mouseEventMonitor = NSEvent.addLocalMonitorForEvents(
                matching: [.leftMouseDown, .leftMouseUp]
            ) { [weak model] event in
                model?.trackMouseEvent(event)

                return event
            }

            let sceneHost = NativeSceneHost(
                model: model,
                wailsWindow: window
            )
            self.sceneHost = sceneHost
            sceneHost.present()
        }

        /*
         OnStartup normally installs the handler before the document loads.
         OnDomReady calls this function again as a race-safe fallback. Asking
         the page to retry setup makes that second path equivalent.
         */
        webView.evaluateJavaScript(
            """
            customElements.whenDefined('launcher-app').then(() => {
                document
                    .querySelector('launcher-app')
                    ?.setUpNativeSidebar?.();
            });
            """
        )
    }
}

@_cdecl("LauncherNativeInstall")
public func LauncherNativeInstall(
    _ windowPointer: UnsafeMutableRawPointer?,
    _ webViewPointer: UnsafeMutableRawPointer?
) {
    guard let windowPointer, let webViewPointer else {
        return
    }

    /*
     Raw pointers are not Sendable in Swift 6. Preserve only their numeric
     addresses across the isolation boundary, then reconstruct them after
     assumeIsolated has established the main-actor context. The Objective-C
     caller already guarantees that this synchronous entry point runs on the
     AppKit main thread.
     */
    let windowAddress = UInt(bitPattern: windowPointer)
    let webViewAddress = UInt(bitPattern: webViewPointer)

    MainActor.assumeIsolated {
        guard
            let windowPointer = UnsafeMutableRawPointer(
                bitPattern: windowAddress
            ),
            let webViewPointer = UnsafeMutableRawPointer(
                bitPattern: webViewAddress
            )
        else {
            return
        }
        let window = Unmanaged<NSWindow>
            .fromOpaque(windowPointer)
            .takeUnretainedValue()
        let webView = Unmanaged<WKWebView>
            .fromOpaque(webViewPointer)
            .takeUnretainedValue()
        NativeShell.shared.install(
            window: window,
            webView: webView
        )
    }
}
