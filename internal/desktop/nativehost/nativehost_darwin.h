#ifndef nativehost_darwin_h
#define nativehost_darwin_h

#include <stdbool.h>

// Finds the Wails window and WKWebView, then gives both to the statically
// linked SwiftUI shell. Safe to call repeatedly and from any goroutine.
bool LauncherNativeHostInstall(void);

// Badges this process's Dock icon for an agent viewer window.
void LauncherNativeHostBadgeDockIcon(void);

#endif /* nativehost_darwin_h */
