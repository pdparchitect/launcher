// Package nativehost installs the macOS SwiftUI shell around Wails' webview.
//
// Wails still owns the process, window lifecycle, custom asset scheme and
// JavaScript runtime. On macOS, the shell reparents that exact WKWebView into a
// SwiftUI NavigationSplitView. Other platforms keep Wails' normal layout.
package nativehost
