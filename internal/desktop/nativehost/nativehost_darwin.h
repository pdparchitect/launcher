#ifndef nativehost_darwin_h
#define nativehost_darwin_h

#include <stdbool.h>

// Finds the Wails window and WKWebView, then gives both to the statically
// linked SwiftUI shell. Safe to call repeatedly and from any goroutine.
bool LauncherNativeHostInstall(void);

// Badges this process's Dock icon for an agent viewer window.
void LauncherNativeHostBadgeDockIcon(void);

// Hides the agent viewer's title bar until the pointer nears the top edge.
// Safe to call repeatedly and from any goroutine.
bool LauncherNativeHostInstallViewerChrome(void);

// Brings another process's windows to the front. Reports whether a running
// application with that identifier was found and activated.
bool LauncherNativeHostActivateProcess(int pid);

#endif /* nativehost_darwin_h */
