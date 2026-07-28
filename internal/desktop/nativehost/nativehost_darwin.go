//go:build darwin && cgo

package nativehost

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework SwiftUI
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
