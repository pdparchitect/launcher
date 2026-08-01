//go:build darwin && cgo

package nativehost

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework WebKit -framework SwiftUI
#cgo LDFLAGS: -L${SRCDIR}/../../../macos/.build/arm64-apple-macosx/release
#cgo LDFLAGS: -lLauncherNative
#include "nativehost_darwin.h"
*/
import "C"

// Install replaces Wails' content view with the SwiftUI native shell while
// preserving the WKWebView instance Wails configured.
func Install() bool {
	return bool(C.LauncherNativeHostInstall())
}

// BadgeDockIcon marks a viewer process so its Dock tile is distinguishable.
func BadgeDockIcon() {
	C.LauncherNativeHostBadgeDockIcon()
}

// InstallViewerChrome hides the agent viewer's title bar until the pointer
// approaches the top edge of the window.
func InstallViewerChrome() bool {
	return bool(C.LauncherNativeHostInstallViewerChrome())
}

// InstallArrowKeyFix corrects the numeric-pad location WebKit reports for
// arrow keys on macOS, before the agent's interface reads it.
func InstallArrowKeyFix() bool {
	return bool(C.LauncherNativeHostInstallArrowKeyFix())
}

// ActivateProcess brings the windows of another process to the front, and
// reports whether that process was still running to be activated.
func ActivateProcess(pid int) bool {
	return bool(C.LauncherNativeHostActivateProcess(C.int(pid)))
}
