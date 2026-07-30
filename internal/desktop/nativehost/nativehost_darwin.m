#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
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
 Approaching the top edge reserves one title-bar strip above the content, then
 fades translucent native chrome into that strip. Collapsing performs those
 operations in reverse: the chrome becomes fully invisible before the strip is
 handed back to the content view. Visible controls therefore never overlap the
 agent's interface.

 The frame's origin is never touched, and AppKit measures it from the bottom, so
 the content keeps both its size and its position on screen throughout. Only the
 window's top edge moves. Two earlier attempts got this wrong in opposite ways:
 an unconstrained full-size web view left the controls floating over the
 agent's own interface, and a permanent inset title bar resized the content.
 */
static const NSTimeInterval kTitlebarFadeDuration = 0.30;

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
static WKWebView *gViewerWebView = nil;
static NSVisualEffectView *gTitlebarBackdrop = nil;
static NSView *gViewerBorder = nil;
static CGFloat gTitlebarHeight = 0.0;
static BOOL gTitlebarRevealed = NO;
static BOOL gTitlebarLayoutRevealed = NO;
static BOOL gViewerFullscreen = NO;
static NSUInteger gTitlebarTransition = 0;

static const NSWindowButton kTitlebarButtons[] = {
    NSWindowCloseButton,
    NSWindowMiniaturizeButton,
    NSWindowZoomButton,
};

@interface LauncherTitlebarBackdrop : NSVisualEffectView
@end

@implementation LauncherTitlebarBackdrop

// The native title bar underneath retains window dragging and button clicks.
- (NSView *)hitTest:(NSPoint)point {
    return nil;
}

@end

@interface LauncherViewerBorder : NSView
@end

@implementation LauncherViewerBorder

- (NSView *)hitTest:(NSPoint)point {
    return nil;
}

@end

static void InstallViewerBorder(NSWindow *window) {
    NSView *contentView = window.contentView;
    if (contentView == nil) {
        return;
    }

    CGFloat scale = MAX(1.0, window.backingScaleFactor);
    CGFloat borderWidth = 1.0 / scale;
    CGFloat inset = borderWidth / 2.0;
    CGFloat cornerRadius = contentView.superview.layer.cornerRadius;
    if (cornerRadius <= 0.0) {
        cornerRadius = 12.0;
    }

    LauncherViewerBorder *border = [[LauncherViewerBorder alloc]
        initWithFrame:NSInsetRect(contentView.bounds, inset, inset)];
    border.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    border.wantsLayer = YES;
    border.layer.backgroundColor = NSColor.clearColor.CGColor;
    border.layer.borderColor =
        [NSColor colorWithWhite:1.0 alpha:0.24].CGColor;
    border.layer.borderWidth = 1.0 / scale;
    border.layer.cornerRadius = MAX(0.0, cornerRadius - inset);

    [contentView addSubview:border positioned:NSWindowAbove relativeTo:nil];
    gViewerBorder = border;
}

static void InstallTitlebarBackdrop(NSWindow *window) {
    NSView *titlebar =
        [window standardWindowButton:NSWindowCloseButton].superview;
    if (titlebar == nil) {
        return;
    }

    LauncherTitlebarBackdrop *backdrop =
        [[LauncherTitlebarBackdrop alloc] initWithFrame:titlebar.bounds];
    backdrop.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    backdrop.wantsLayer = YES;
    if (@available(macOS 10.14, *)) {
        backdrop.material = NSVisualEffectMaterialHeaderView;
    } else {
        backdrop.material = NSVisualEffectMaterialTitlebar;
    }
    backdrop.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    backdrop.state = NSVisualEffectStateActive;

    NSTextField *title = [NSTextField labelWithString:window.title];
    title.translatesAutoresizingMaskIntoConstraints = NO;
    title.font = [NSFont systemFontOfSize:12.0 weight:NSFontWeightMedium];
    title.textColor = NSColor.secondaryLabelColor;
    [backdrop addSubview:title];
    [NSLayoutConstraint activateConstraints:@[
        [title.centerXAnchor constraintEqualToAnchor:backdrop.centerXAnchor],
        [title.centerYAnchor constraintEqualToAnchor:backdrop.centerYAnchor],
    ]];

    [titlebar addSubview:backdrop positioned:NSWindowBelow relativeTo:nil];
    gTitlebarBackdrop = backdrop;
}

static void SetTitlebarChromeImmediately(BOOL revealed) {
    [gTitlebarBackdrop.layer removeAllAnimations];
    gTitlebarBackdrop.alphaValue = revealed ? 1.0 : 0.0;
    gTitlebarBackdrop.hidden = !revealed;

    for (size_t index = 0;
         index < sizeof(kTitlebarButtons) / sizeof(kTitlebarButtons[0]);
         index++) {
        NSButton *button =
            [gViewerWindow standardWindowButton:kTitlebarButtons[index]];
        if (button == nil) {
            continue;
        }

        button.wantsLayer = YES;
        [button.layer removeAllAnimations];
        button.alphaValue = revealed ? 1.0 : 0.0;
        // A zero-alpha control still takes clicks, so invisible controls must
        // leave hit testing as well as the screen.
        button.hidden = !revealed;
    }
}

static void MakeFullscreenControlsAvailable(void) {
    for (size_t index = 0;
         index < sizeof(kTitlebarButtons) / sizeof(kTitlebarButtons[0]);
         index++) {
        NSButton *button =
            [gViewerWindow standardWindowButton:kTitlebarButtons[index]];
        if (button == nil) {
            continue;
        }

        button.alphaValue = 1.0;
        button.hidden = NO;
    }
}

static BOOL ViewerIsFullscreen(void) {
    return gViewerFullscreen
        || (gViewerWindow != nil
            && (gViewerWindow.styleMask & NSWindowStyleMaskFullScreen) != 0);
}

static void AnimateTitlebarChrome(
    BOOL revealed,
    void (^completion)(void)
) {
    if (revealed) {
        gTitlebarBackdrop.hidden = NO;
        for (size_t index = 0;
             index < sizeof(kTitlebarButtons) / sizeof(kTitlebarButtons[0]);
             index++) {
            NSButton *button =
                [gViewerWindow standardWindowButton:kTitlebarButtons[index]];
            button.hidden = NO;
        }
    }

    [NSAnimationContext
        runAnimationGroup:^(NSAnimationContext *context) {
            context.duration = kTitlebarFadeDuration;
            context.timingFunction = [CAMediaTimingFunction
                functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
            gTitlebarBackdrop.animator.alphaValue = revealed ? 1.0 : 0.0;
            for (size_t index = 0;
                 index
                     < sizeof(kTitlebarButtons) / sizeof(kTitlebarButtons[0]);
                 index++) {
                NSButton *button = [gViewerWindow
                    standardWindowButton:kTitlebarButtons[index]];
                button.animator.alphaValue = revealed ? 1.0 : 0.0;
            }
        }
        completionHandler:^{
            if (!gTitlebarRevealed) {
                gTitlebarBackdrop.hidden = YES;
                for (size_t index = 0;
                     index
                         < sizeof(kTitlebarButtons)
                             / sizeof(kTitlebarButtons[0]);
                     index++) {
                    NSButton *button = [gViewerWindow
                        standardWindowButton:kTitlebarButtons[index]];
                    button.hidden = YES;
                }
            }
            if (completion != nil) {
                completion();
            }
        }];
}

static void SetTitlebarLayoutRevealed(BOOL revealed, BOOL animated) {
    if (gViewerWindow == nil
        || gViewerWebView == nil
        || gTitlebarHeight <= 0.0
        || gTitlebarLayoutRevealed == revealed) {
        return;
    }
    gTitlebarLayoutRevealed = revealed;

    NSRect frame = gViewerWindow.frame;
    frame.size.height += revealed ? gTitlebarHeight : -gTitlebarHeight;

    /*
     During the top-edge change, only the web view's top margin is flexible.
     Its origin and size therefore stay fixed while the full-size content view
     grows or shrinks around it. Restoring height autoresizing afterwards keeps
     ordinary user-driven window resizing working with a constant title strip.
     */
    gViewerWebView.autoresizingMask =
        NSViewWidthSizable | NSViewMaxYMargin;
    [gViewerWindow setFrame:frame display:NO animate:animated];

    NSView *contentView = gViewerWebView.superview;
    NSRect contentBounds = contentView.bounds;
    CGFloat chromeHeight = revealed ? gTitlebarHeight : 0.0;
    gViewerWebView.frame = NSMakeRect(
        NSMinX(contentBounds),
        NSMinY(contentBounds),
        NSWidth(contentBounds),
        MAX(0.0, NSHeight(contentBounds) - chromeHeight)
    );
    gViewerWebView.autoresizingMask =
        NSViewWidthSizable | NSViewHeightSizable;
}

static void SetTitlebarRevealed(BOOL revealed) {
    if (gViewerWindow == nil
        || ViewerIsFullscreen()
        || gTitlebarRevealed == revealed) {
        return;
    }
    gTitlebarRevealed = revealed;
    NSUInteger transition = ++gTitlebarTransition;

    if (revealed) {
        // The new strip is transparent until the chrome fades in, so adding it
        // does not produce a flash while the content remains anchored below.
        SetTitlebarLayoutRevealed(YES, YES);
        AnimateTitlebarChrome(YES, nil);
        return;
    }

    /*
     The controls must finish fading while they still own a separate strip;
     only then can the web view occupy that space without ever drawing
     underneath visible window controls.
     */
    AnimateTitlebarChrome(NO, ^{
        if (gTitlebarRevealed || transition != gTitlebarTransition) {
            return;
        }
        SetTitlebarLayoutRevealed(NO, YES);
    });
}

static void SetViewerFullscreen(BOOL fullscreen) {
    if (gViewerWindow == nil) {
        return;
    }

    /*
     Fullscreen is content-only at rest. Collapse the custom hover strip and
     keep its backdrop hidden, but leave the standard controls eligible for
     AppKit's own transient top-edge title bar. AppKit hides that native title
     bar with the menu bar and reveals both together when the pointer reaches
     the screen edge.
     */
    if (fullscreen) {
        SetTitlebarLayoutRevealed(NO, NO);
    }
    gViewerFullscreen = fullscreen;
    gTitlebarRevealed = NO;
    ++gTitlebarTransition;
    SetTitlebarChromeImmediately(NO);
    if (fullscreen) {
        MakeFullscreenControlsAvailable();
    }
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

    // Full-size content makes the frame and content rect equal, so use the
    // standard titlebar view first and retain the content-rect calculation as a
    // fallback for any future Wails configuration that starts inset.
    NSView *titlebar =
        [window standardWindowButton:NSWindowCloseButton].superview;
    gTitlebarHeight = NSHeight(titlebar.bounds);
    if (gTitlebarHeight <= 0.0) {
        gTitlebarHeight = NSHeight(window.frame)
            - NSHeight([window contentRectForFrameRect:window.frame]);
    }
    if (gTitlebarHeight <= 0.0) {
        return false;
    }

    gViewerWindow = window;
    gViewerWebView = webView;
    window.titleVisibility = NSWindowTitleHidden;
    gViewerWindow.titlebarAppearsTransparent = YES;
    if (@available(macOS 11.0, *)) {
        window.titlebarSeparatorStyle = NSTitlebarSeparatorStyleNone;
    }
    InstallViewerBorder(window);
    if (gViewerBorder == nil) {
        return false;
    }
    // Without this the window system never delivers moved events to the app, so
    // the monitor below would only ever see drags.
    window.acceptsMouseMovedEvents = YES;
    InstallTitlebarBackdrop(window);
    if (gTitlebarBackdrop == nil) {
        return false;
    }
    SetTitlebarChromeImmediately(NO);

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
            // Fullscreen has no room for the hover chrome to grow into.
            if (ViewerIsFullscreen()) {
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
     Fullscreen uses AppKit's transient title bar, while ordinary windows use
     the custom hover strip.
     */
    [[NSNotificationCenter defaultCenter]
        addObserverForName:NSWindowWillEnterFullScreenNotification
                    object:window
                     queue:[NSOperationQueue mainQueue]
                usingBlock:^(NSNotification *notification) {
                    SetViewerFullscreen(YES);
                }];
    [[NSNotificationCenter defaultCenter]
        addObserverForName:NSWindowDidExitFullScreenNotification
                    object:window
                     queue:[NSOperationQueue mainQueue]
                usingBlock:^(NSNotification *notification) {
                    SetViewerFullscreen(NO);
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
                    if (ViewerIsFullscreen()) {
                        return;
                    }
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

/*
 Each agent viewer is its own process, so bringing one back is a matter of
 activating that application rather than raising a window of ours.
 */
static bool ActivateProcessOnMainThread(int pid) {
    NSRunningApplication *application =
        [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    if (application == nil) {
        return false;
    }

    // Hidden and merely-behind are different states, and the launcher can be
    // looking at either one.
    [application unhide];

    return [application
        activateFromApplication:NSRunningApplication.currentApplication
                        options:NSApplicationActivateAllWindows];
}

bool LauncherNativeHostActivateProcess(int pid) {
    if ([NSThread isMainThread]) {
        return ActivateProcessOnMainThread(pid);
    }
    __block bool activated = false;
    dispatch_sync(dispatch_get_main_queue(), ^{
        activated = ActivateProcessOnMainThread(pid);
    });
    return activated;
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
