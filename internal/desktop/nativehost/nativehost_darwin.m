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
 QuickTime-style chrome for the agent viewer. The window keeps a real title bar
 - the agent's interface stops below it rather than running underneath - but the
 bar is transparent and its controls are hidden, so the window reads as nothing
 but the agent's interface, rounded by the window itself. Moving the pointer
 into the bar turns it opaque and fades the controls back in.

 The alternative, a full-size content view with the bar hidden over the top of
 it, is what this replaced: the controls then float over the agent's own
 interface and the window has nothing left to drag it by.
 */
static const NSTimeInterval kTitlebarFadeDuration = 0.18;

static NSWindow *gViewerWindow = nil;
static BOOL gTitlebarRevealed = YES;

static const NSWindowButton kTitlebarButtons[] = {
    NSWindowCloseButton,
    NSWindowMiniaturizeButton,
    NSWindowZoomButton,
};

static void SetTitlebarRevealed(BOOL revealed) {
    if (gViewerWindow == nil || gTitlebarRevealed == revealed) {
        return;
    }
    gTitlebarRevealed = revealed;

    // A transparent title bar shows the window's own background, which against
    // the agent's dark interface reads as no title bar at all. Turning it off
    // brings back the standard material along with the controls.
    gViewerWindow.titlebarAppearsTransparent = !revealed;

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

@interface LauncherTitlebarHover : NSView
@end

@implementation LauncherTitlebarHover

/*
 Never take clicks. The window controls and the title bar's own drag handling
 are underneath this view and have to keep receiving them; tracking areas are
 resolved geometrically, so hover still works from a view that hit-tests away.
 */
- (NSView *)hitTest:(NSPoint)point {
    return nil;
}

- (void)updateTrackingAreas {
    [super updateTrackingAreas];
    for (NSTrackingArea *area in [self.trackingAreas copy]) {
        [self removeTrackingArea:area];
    }
    [self addTrackingArea:
        [[NSTrackingArea alloc]
            initWithRect:NSZeroRect
                 options:NSTrackingMouseEnteredAndExited
                         | NSTrackingActiveAlways
                         | NSTrackingInVisibleRect
                   owner:self
                userInfo:nil]];
}

- (void)mouseEntered:(NSEvent *)event {
    SetTitlebarRevealed(YES);
}

- (void)mouseExited:(NSEvent *)event {
    SetTitlebarRevealed(NO);
}

@end

static bool InstallViewerChromeOnMainThread(void) {
    if (gViewerWindow != nil) {
        return true;
    }
    WKWebView *webView = nil;
    NSWindow *window = FindLauncherWindow(&webView);
    if (window == nil) {
        return false;
    }
    NSView *titlebar = [window standardWindowButton:NSWindowCloseButton]
        .superview;
    if (titlebar == nil) {
        return false;
    }

    gViewerWindow = window;
    window.titleVisibility = NSWindowTitleHidden;
    SetTitlebarRevealed(NO);

    /*
     The title bar is the one part of this window the WKWebView does not cover,
     so hover can be tracked on the view itself instead of by watching moved
     events that the web view would otherwise consume.
     */
    LauncherTitlebarHover *hover =
        [[LauncherTitlebarHover alloc] initWithFrame:titlebar.bounds];
    hover.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [titlebar addSubview:hover positioned:NSWindowAbove relativeTo:nil];

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
