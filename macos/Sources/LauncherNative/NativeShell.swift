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
    private var sidebarInset: CGFloat = 0

    init(webView: WKWebView) {
        self.webView = webView
    }

    func select(_ identifier: String?) {
        guard let identifier else {
            return
        }
        selection = identifier

        /*
         A bare string is not a valid top-level JSON object, so encoding it
         needs fragmentsAllowed. Without it the serialization fails, try?
         swallows the error, and the selection never reaches the page.
         */
        guard
            let data = try? JSONSerialization.data(
                withJSONObject: identifier,
                options: [.fragmentsAllowed]
            ),
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

    /*
     How far the sidebar reaches across the window, in points. The web view now
     spans the whole window and draws underneath the sidebar, so the page needs
     the same number to keep its own content and dialogs clear of it. Published
     rather than assumed because the column is resizable and collapsible.
     */
    func publish(sidebarInset: CGFloat) {
        self.sidebarInset = sidebarInset
        publishSidebarInset()
    }

    private func publishSidebarInset() {
        let inset = Int(sidebarInset.rounded())

        webView.evaluateJavaScript(
            """
            window.wailsSidebarInset = \(inset);
            window.dispatchEvent(
                new CustomEvent(
                    'wails:sidebar-inset',
                    { detail: { inset: \(inset) } }
                )
            );
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

        /*
         The first measurement is published as the window is built, which can be
         before this document exists. A page that has just configured itself is
         a page that missed it, so it is sent again here.
         */
        publishSidebarInset()
    }
}

private final class WailsWebViewContainer: NSView {
    /*
     Where the sidebar stops, in window points. The web view underlaps it, and
     AppKit-backed views win hit testing even where SwiftUI is compositing the
     sidebar above them, so everything to the left of this belongs to the List.
     Measured from the live layout rather than assumed: the column is resizable,
     and once collapsed this is zero and the whole window is the page's.
     */
    var nativeSidebarTrailingEdge: CGFloat = 280

    override func hitTest(_ point: NSPoint) -> NSView? {
        guard let superview, nativeSidebarTrailingEdge > 0 else {
            return super.hitTest(point)
        }

        let pointInWindow = superview.convert(point, to: nil)
        guard pointInWindow.x >= nativeSidebarTrailingEdge else {
            return nil
        }

        return super.hitTest(point)
    }
}

private struct WailsWebView: NSViewRepresentable {
    let webView: WKWebView
    let sidebarTrailingEdge: CGFloat

    func makeNSView(context: Context) -> WailsWebViewContainer {
        /*
         Keep Wails' WKWebView inside a detail-column container. Returning the
         already full-window WKWebView directly allows its backing layers to
         retain the old window geometry and cover the sidebar's hit-test area.
         SwiftUI owns the container's frame; the web view can only fill it.
         */
        let container = WailsWebViewContainer()
        container.wantsLayer = true
        container.layer?.masksToBounds = true
        container.nativeSidebarTrailingEdge = sidebarTrailingEdge

        webView.removeFromSuperview()
        webView.frame = container.bounds
        webView.autoresizingMask = [.width, .height]
        container.addSubview(webView)

        return container
    }

    func updateNSView(_ container: WailsWebViewContainer, context: Context) {
        container.nativeSidebarTrailingEdge = sidebarTrailingEdge

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

    /*
     Mutable rather than a constant binding, so the sidebar toggle collapses the
     sidebar instead of being inert.

     The toggle is not decoration: it is a default item of the sidebar column's
     toolbar, and that toolbar is what holds the window's titlebar band open.
     Removing it collapses the band, which costs the glass panel its top inset
     and leaves the traffic lights beside the sidebar rather than above it.
     */
    @State private var columnVisibility:
        NavigationSplitViewVisibility = .all

    /*
     Where the sidebar's trailing edge sits in the window, measured from the
     List itself. Starts at the column maximum so the first frame errs towards
     the sidebar owning the strip rather than the page.
     */
    @State private var sidebarEdge: CGFloat = 280

    /*
     prominentDetail keeps the detail column window-sized and floats the sidebar
     over it, so the page underlaps the sidebar and both the hit-test guard and
     the page's own layout have to be trimmed by exactly this much - and by
     nothing at all once the sidebar is collapsed, which would otherwise leave a
     dead strip down the leading edge of the page.
     */
    private var sidebarInset: CGFloat {
        columnVisibility == .detailOnly ? 0 : sidebarEdge
    }

    var body: some View {
        NavigationSplitView(
            columnVisibility: $columnVisibility
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
            .onGeometryChange(for: CGFloat.self) { proxy in
                max(0, proxy.frame(in: .global).maxX)
            } action: { edge in
                sidebarEdge = edge
            }
        } detail: {
            WailsWebView(
                webView: model.webView,
                sidebarTrailingEdge: sidebarInset
            )
            /*
             The effect only mirrors and blurs the leading edge of the page into
             the sidebar's safe area. Ignoring that safe area as well puts the
             real page under the sidebar, which is what the glass then samples -
             hitTest keeps the AppKit surface from stealing the List's clicks,
             and the page insets its own content by the same measurement.
             */
            .backgroundExtensionEffect()
            .ignoresSafeArea(
                .container,
                edges: .all
            )
        }
        .onChange(of: sidebarInset, initial: true) { _, inset in
            model.publish(sidebarInset: inset)
        }
        .navigationSplitViewStyle(
            .prominentDetail
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
        openWindow()
    }

    /*
     Closing the window destroys the scene's view hierarchy but not the scene
     itself, so reopening is the same request as the first one: WindowGroup
     builds a fresh RootView, and WailsWebView moves the retained WKWebView
     into it. The page it is displaying is never reloaded.
     */
    func openWindow() {
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

/*
 Wails owns the NSApplicationDelegate and answers a Dock click by restoring the
 window it created - the bootstrap window this shell orders out, which no longer
 holds the web view. Wrapping the delegate leaves every other message with
 Wails and answers only this one, by reopening the window the user actually
 closed.

 Not @MainActor: responds(to:) and forwardingTarget(for:) override nonisolated
 NSObject methods, so the isolation lives on the delegate callback instead.
 */
private final class ReopenDelegate: NSObject, NSApplicationDelegate {
    private let wrapped: NSApplicationDelegate?
    private let reopen: @MainActor () -> Void

    init(
        wrapped: NSApplicationDelegate?,
        reopen: @escaping @MainActor () -> Void
    ) {
        self.wrapped = wrapped
        self.reopen = reopen
    }

    /*
     AppKit asks before sending, and forwardingTarget is only consulted for
     selectors this object claims. Both have to account for Wails' delegate or
     its half of the protocol goes silently unanswered.
     */
    override func responds(to selector: Selector!) -> Bool {
        if super.responds(to: selector) {
            return true
        }

        return wrapped?.responds(to: selector) ?? false
    }

    override func forwardingTarget(for selector: Selector!) -> Any? {
        wrapped
    }

    @MainActor
    func applicationShouldHandleReopen(
        _ sender: NSApplication,
        hasVisibleWindows flag: Bool
    ) -> Bool {
        // Deliberately not forwarded: Wails' own answer is what shows the
        // empty bootstrap window.
        if !flag {
            reopen()
        }

        return true
    }
}

@MainActor
private final class NativeShell {
    static let shared = NativeShell()

    private var sceneHost: NativeSceneHost?
    private var model: NativeShellModel?
    private var mouseEventMonitor: Any?
    // NSApplication references its delegate weakly, so the wrapper below has to
    // be owned for the lifetime of the process.
    private var reopenDelegate: ReopenDelegate?

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

            let reopenDelegate = ReopenDelegate(
                wrapped: NSApplication.shared.delegate
            ) { [weak sceneHost] in
                sceneHost?.openWindow()
            }
            self.reopenDelegate = reopenDelegate
            NSApplication.shared.delegate = reopenDelegate
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
