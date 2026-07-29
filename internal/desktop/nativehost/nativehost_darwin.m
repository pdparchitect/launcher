#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

#import "nativehost_darwin.h"

// Implemented by the statically linked LauncherNative Swift library.
extern void LauncherNativeInstall(void *window, void *webView);

static id gInspectorMonitor = nil;

static WKWebView *FindWebView(NSView *view) {
    if ([view isKindOfClass:[WKWebView class]]) {
        return (WKWebView *)view;
    }
    for (NSView *child in view.subviews) {
        WKWebView *result = FindWebView(child);
        if (result != nil) {
            return result;
        }
    }
    return nil;
}

static NSWindow *FindLauncherWindow(WKWebView **webView) {
    for (NSWindow *window in NSApp.windows) {
        WKWebView *candidate = FindWebView(window.contentView);
        if (candidate != nil) {
            *webView = candidate;
            return window;
        }
    }
    return nil;
}

static void InstallInspectorShortcut(WKWebView *webView) {
    if (gInspectorMonitor != nil) {
        return;
    }
    gInspectorMonitor = [NSEvent
        addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown
        handler:^NSEvent *(NSEvent *event) {
            if (event.window != webView.window) {
                return event;
            }
            NSEventModifierFlags required =
                NSEventModifierFlagCommand | NSEventModifierFlagOption;
            NSEventModifierFlags flags =
                event.modifierFlags
                & NSEventModifierFlagDeviceIndependentFlagsMask;
            if ((flags & required) != required) {
                return event;
            }
            NSString *key = event.charactersIgnoringModifiers.lowercaseString;
            if (![key isEqualToString:@"i"]) {
                return event;
            }
            [webView evaluateJavaScript:
                @"window.webkit.messageHandlers.external"
                 ".postMessage('wails:openInspector');"
                completionHandler:nil];
            return nil;
        }];
}

static bool InstallOnMainThread(void) {
    WKWebView *webView = nil;
    NSWindow *window = FindLauncherWindow(&webView);
    if (window == nil || webView == nil) {
        return false;
    }
    LauncherNativeInstall((__bridge void *)window, (__bridge void *)webView);
    InstallInspectorShortcut(webView);
    return true;
}

bool LauncherNativeHostInstall(void) {
    if ([NSThread isMainThread]) {
        return InstallOnMainThread();
    }
    __block bool installed = false;
    dispatch_sync(dispatch_get_main_queue(), ^{
        installed = InstallOnMainThread();
    });
    return installed;
}

/*
 QuickTime-style chrome for the agent viewer. The window opens clean and the
 title bar fades in only while the pointer is near the top edge. The viewer
 navigates to the agent's own interface cross-origin, so HTML window controls
 are not available to it - this has to be the real title bar.
 */
static const CGFloat kTitlebarRevealHeight = 80.0;
static const NSTimeInterval kTitlebarFadeDuration = 0.18;

static id gTitlebarMonitor = nil;
static NSWindow *gViewerWindow = nil;
static BOOL gTitlebarVisible = YES;

static NSView *TitlebarContainer(NSWindow *window) {
    // Close button -> NSTitlebarView -> NSTitlebarContainerView.
    return [window standardWindowButton:NSWindowCloseButton]
        .superview.superview;
}

static void SetTitlebarVisible(BOOL visible) {
    NSView *container = TitlebarContainer(gViewerWindow);
    if (container == nil || gTitlebarVisible == visible) {
        return;
    }
    gTitlebarVisible = visible;

    // A hidden view is what stops the invisible traffic lights from still
    // taking clicks: alpha alone leaves them hit-testable.
    if (visible) {
        container.hidden = NO;
    }
    [NSAnimationContext
        runAnimationGroup:^(NSAnimationContext *context) {
            context.duration = kTitlebarFadeDuration;
            container.animator.alphaValue = visible ? 1.0 : 0.0;
        }
        completionHandler:^{
            // A reveal can land mid fade-out, so only the state as it stands
            // now may hide the bar.
            if (!gTitlebarVisible) {
                container.hidden = YES;
            }
        }];
}

static bool InstallViewerChromeOnMainThread(void) {
    if (gTitlebarMonitor != nil) {
        return true;
    }
    WKWebView *webView = nil;
    NSWindow *window = FindLauncherWindow(&webView);
    if (window == nil) {
        return false;
    }

    gViewerWindow = window;
    window.titlebarAppearsTransparent = YES;
    window.titleVisibility = NSWindowTitleHidden;
    // Without this the window system never delivers moved events to the app,
    // so the monitor below would only ever see drags.
    window.acceptsMouseMovedEvents = YES;
    SetTitlebarVisible(NO);

    /*
     A local monitor rather than a tracking area: the WKWebView covers the
     content view and consumes moved events before they reach it.
     */
    gTitlebarMonitor = [NSEvent
        addLocalMonitorForEventsMatchingMask:NSEventMaskMouseMoved
                                             | NSEventMaskLeftMouseDragged
        handler:^NSEvent *(NSEvent *event) {
            if (event.window != gViewerWindow) {
                return event;
            }
            // Fullscreen reveals the title bar on its own; running both means
            // two animations fighting over the same view.
            if ((gViewerWindow.styleMask & NSWindowStyleMaskFullScreen) != 0) {
                return event;
            }
            CGFloat height = NSHeight(gViewerWindow.contentView.bounds);
            CGFloat distance = height - event.locationInWindow.y;
            SetTitlebarVisible(distance <= kTitlebarRevealHeight);

            return event;
        }];

    /*
     Moved events stop arriving once the pointer leaves the window, which would
     otherwise strand a revealed title bar on screen.
     */
    [[NSNotificationCenter defaultCenter]
        addObserverForName:NSWindowDidResignKeyNotification
                    object:window
                     queue:[NSOperationQueue mainQueue]
                usingBlock:^(NSNotification *notification) {
                    SetTitlebarVisible(NO);
                }];

    return true;
}

bool LauncherNativeHostInstallViewerChrome(void) {
    if ([NSThread isMainThread]) {
        return InstallViewerChromeOnMainThread();
    }
    __block bool installed = false;
    dispatch_sync(dispatch_get_main_queue(), ^{
        installed = InstallViewerChromeOnMainThread();
    });
    return installed;
}

void LauncherNativeHostBadgeDockIcon(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSImage *base = NSApp.applicationIconImage;
        if (base == nil) {
            return;
        }

        NSSize size = NSMakeSize(512, 512);
        NSImage *badged = [[NSImage alloc] initWithSize:size];
        [badged lockFocus];
        [base drawInRect:NSMakeRect(0, 0, size.width, size.height)
                fromRect:NSZeroRect
               operation:NSCompositingOperationSourceOver
                fraction:1.0];

        NSRect badge = NSMakeRect(280, 16, 216, 216);
        NSBezierPath *disc = [NSBezierPath bezierPathWithOvalInRect:badge];
        [[NSColor colorWithSRGBRed:0.04 green:0.05 blue:0.03 alpha:1.0]
            setFill];
        [disc fill];
        [[NSColor colorWithSRGBRed:0.92 green:1.0 blue:0.0 alpha:1.0]
            setStroke];
        disc.lineWidth = 14.0;
        [disc stroke];

        NSRect glyph =
            NSMakeRect(badge.origin.x + 56, badge.origin.y + 64, 104, 88);
        NSBezierPath *frame =
            [NSBezierPath bezierPathWithRoundedRect:glyph
                                            xRadius:10
                                            yRadius:10];
        frame.lineWidth = 14.0;
        [frame stroke];
        NSBezierPath *titlebar = [NSBezierPath bezierPath];
        [titlebar moveToPoint:
            NSMakePoint(glyph.origin.x, NSMaxY(glyph) - 24)];
        [titlebar lineToPoint:
            NSMakePoint(NSMaxX(glyph), NSMaxY(glyph) - 24)];
        titlebar.lineWidth = 14.0;
        [titlebar stroke];
        [badged unlockFocus];

        NSApp.applicationIconImage = badged;
    });
}
