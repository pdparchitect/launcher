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
 QuickTime-style chrome for the agent viewer.

 Collapsed, the window is nothing but the agent's interface: the content view
 fills the frame, the title bar is transparent and its controls are hidden.
 Approaching the top edge grows the window upward by exactly one title bar and
 hands that strip back to AppKit, so the bar appears above the content rather
 than over it or in place of it.

 The frame's origin is never touched, and AppKit measures it from the bottom, so
 the content keeps both its size and its position on screen throughout. Only the
 window's top edge moves. Two earlier attempts got this wrong in opposite ways:
 a full-size content view left the controls floating over the agent's own
 interface with nothing to drag the window by, and a permanent title bar pushed
 the content down and resized it.
 */
static const NSTimeInterval kTitlebarFadeDuration = 0.18;

/*
 Hysteresis, not one threshold. Revealing lifts the top edge by the height of
 the title bar, so a stationary pointer is suddenly that much further from it -
 collapsing at anything less than the reveal distance plus a bar's height would
 leave the window flipping between the two states without the mouse moving.
 */
static const CGFloat kTitlebarRevealDistance = 48.0;
static const CGFloat kTitlebarCollapseDistance = 120.0;

static id gTitlebarMonitor = nil;
static NSWindow *gViewerWindow = nil;
static CGFloat gTitlebarHeight = 0.0;
static BOOL gTitlebarRevealed = YES;

static const NSWindowButton kTitlebarButtons[] = {
    NSWindowCloseButton,
    NSWindowMiniaturizeButton,
    NSWindowZoomButton,
};

static void SetTitlebarButtonsRevealed(BOOL revealed) {
    for (size_t index = 0;
         index < sizeof(kTitlebarButtons) / sizeof(kTitlebarButtons[0]);
         index++) {
        NSButton *button =
            [gViewerWindow standardWindowButton:kTitlebarButtons[index]];
        if (button == nil) {
            continue;
        }

        // Hidden as well as faded: a zero-alpha control still takes clicks, and
        // an invisible close button is a window that shuts by accident.
        if (revealed) {
            button.hidden = NO;
        }
        [NSAnimationContext
            runAnimationGroup:^(NSAnimationContext *context) {
                context.duration = kTitlebarFadeDuration;
                button.animator.alphaValue = revealed ? 1.0 : 0.0;
            }
            completionHandler:^{
                // A reveal can land mid fade-out, so only the state as it
                // stands now may hide the controls.
                if (!gTitlebarRevealed) {
                    button.hidden = YES;
                }
            }];
    }
}

static void SetTitlebarRevealed(BOOL revealed) {
    if (gViewerWindow == nil
        || gTitlebarHeight <= 0.0
        || gTitlebarRevealed == revealed) {
        return;
    }
    gTitlebarRevealed = revealed;

    NSRect frame = gViewerWindow.frame;
    frame.size.height += revealed ? gTitlebarHeight : -gTitlebarHeight;

    /*
     Style first, then frame, both within one event so AppKit lays out once:
     each change alone would move the content, and together they cancel.
     Collapsing gives the strip to the content and then takes it off the window;
     revealing takes it from the content and then gives it back to the window.
     */
    if (revealed) {
        gViewerWindow.styleMask &= ~NSWindowStyleMaskFullSizeContentView;
        gViewerWindow.titlebarAppearsTransparent = NO;
    } else {
        gViewerWindow.styleMask |= NSWindowStyleMaskFullSizeContentView;
        gViewerWindow.titlebarAppearsTransparent = YES;
    }
    [gViewerWindow setFrame:frame display:NO];

    SetTitlebarButtonsRevealed(revealed);
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

    /*
     Measured while the window still has its title bar, which is why the viewer
     is configured without FullSizeContent: once the strip belongs to the
     content view, the frame and the content rect are the same and there is
     nothing left to measure.
     */
    gTitlebarHeight = NSHeight(window.frame)
        - NSHeight([window contentRectForFrameRect:window.frame]);
    if (gTitlebarHeight <= 0.0) {
        return false;
    }

    gViewerWindow = window;
    window.titleVisibility = NSWindowTitleHidden;
    // Without this the window system never delivers moved events to the app, so
    // the monitor below would only ever see drags.
    window.acceptsMouseMovedEvents = YES;
    SetTitlebarRevealed(NO);

    /*
     A local monitor rather than a tracking area: collapsed, the WKWebView
     covers the whole window, and it consumes moved events before any view of
     ours could see them. Screen coordinates because the window's own top edge
     is what moves.
     */
    gTitlebarMonitor = [NSEvent
        addLocalMonitorForEventsMatchingMask:NSEventMaskMouseMoved
                                             | NSEventMaskLeftMouseDragged
        handler:^NSEvent *(NSEvent *event) {
            if (event.window != gViewerWindow) {
                return event;
            }
            // Fullscreen reveals the title bar on its own, and the window has
            // no room to grow into.
            if ((gViewerWindow.styleMask & NSWindowStyleMaskFullScreen) != 0) {
                return event;
            }

            CGFloat distance =
                NSMaxY(gViewerWindow.frame) - NSEvent.mouseLocation.y;
            if (distance <= kTitlebarRevealDistance) {
                SetTitlebarRevealed(YES);
            } else if (distance > kTitlebarCollapseDistance) {
                SetTitlebarRevealed(NO);
            }

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
                    SetTitlebarRevealed(NO);
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
