import AppKit
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

    init(webView: WKWebView) {
        self.webView = webView
    }

    func select(_ identifier: String?) {
        guard let identifier, selection != identifier else {
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
            window.dispatchEvent(
                new CustomEvent(
                    'wails:sidebar',
                    { detail: { id: \(encoded) } }
                )
            );
            """
        )
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

private struct WailsWebView: NSViewRepresentable {
    let webView: WKWebView

    func makeNSView(context: Context) -> WKWebView {
        webView
    }

    func updateNSView(_ webView: WKWebView, context: Context) {}
}

private struct RootView: View {
    @Bindable var model: NativeShellModel

    @State private var columnVisibility:
        NavigationSplitViewVisibility = .all

    private var selection: Binding<String?> {
        Binding(
            get: { model.selection },
            set: { model.select($0) }
        )
    }

    @ViewBuilder
    var body: some View {
        if #available(macOS 26.0, *) {
            splitView
                .toolbarBackgroundVisibility(
                    Visibility.hidden,
                    for: ToolbarPlacement.windowToolbar
                )
        } else {
            splitView
        }
    }

    private var splitView: some View {
        NavigationSplitView(
            columnVisibility: $columnVisibility
        ) {
            List(model.items, selection: selection) { item in
                Label(
                    item.title,
                    systemImage: item.symbol
                )
                .tag(item.id)
            }
            .listStyle(.sidebar)
            .navigationSplitViewColumnWidth(
                min: 210,
                ideal: 240,
                max: 280
            )
        } detail: {
            webDetail
        }
        .navigationSplitViewStyle(.prominentDetail)
    }

    @ViewBuilder
    private var webDetail: some View {
        if #available(macOS 26.0, *) {
            WailsWebView(webView: model.webView)
                .backgroundExtensionEffect()
                .ignoresSafeArea(
                    .container,
                    edges: .all
                )
        } else {
            WailsWebView(webView: model.webView)
                .ignoresSafeArea(
                    .container,
                    edges: .all
                )
        }
    }
}

@MainActor
private final class NativeShell {
    static let shared = NativeShell()

    private var hostingView: NSHostingView<RootView>?
    private var model: NativeShellModel?

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

            let hostingView = NSHostingView(
                rootView: RootView(model: model)
            )
            hostingView.frame = window.contentView?.bounds ?? .zero
            hostingView.autoresizingMask = [.width, .height]
            if #available(macOS 26.0, *) {
                hostingView.sceneBridgingOptions = [
                    .toolbars,
                    .title,
                ]
            }
            self.hostingView = hostingView
            window.contentView = hostingView

            if #available(macOS 11.0, *) {
                window.toolbarStyle = .unified
            }
            window.titleVisibility = .hidden
            window.titlebarAppearsTransparent = true
        }

        /*
         OnStartup normally installs the handler before the document loads.
         OnDomReady calls this function again as a race-safe fallback. Asking
         the page to retry setup makes that second path equivalent.
         */
        webView.evaluateJavaScript(
            """
            document.querySelector('launcher-app')?.setUpNativeSidebar();
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
