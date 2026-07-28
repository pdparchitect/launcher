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
