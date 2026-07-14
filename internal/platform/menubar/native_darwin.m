//go:build darwin

#import <Cocoa/Cocoa.h>
#import <math.h>

#include "native_darwin.h"

extern void SSHManMenuBarQuitRequested(void);

static const CGFloat SSHManPopupWidth = 420.0;
static const CGFloat SSHManPopupHeight = 720.0;
static const CGFloat SSHManPopupMargin = 8.0;
static const CGFloat SSHManPopupOffset = 6.0;

static NSStatusItem *sshManStatusItem = nil;
static NSMenu *sshManContextMenu = nil;
static NSWindow *sshManPopupWindow = nil;
static id sshManGlobalEventMonitor = nil;

@protocol SSHManWailsApplicationDelegate
- (NSWindow *)mainWindow;
@end

static NSWindow *SSHManFindPopupWindow(void) {
    id delegate = [NSApp delegate];
    if ([delegate respondsToSelector:@selector(mainWindow)]) {
        NSWindow *window = [(id<SSHManWailsApplicationDelegate>)delegate mainWindow];
        if (window != nil) {
            return window;
        }
    }

    Class wailsWindowClass = NSClassFromString(@"WailsWindow");
    for (NSWindow *window in [NSApp windows]) {
        if (window == sshManStatusItem.button.window) {
            continue;
        }
        if (wailsWindowClass != Nil && [window isKindOfClass:wailsWindowClass]) {
            return window;
        }
        if ([[window title] isEqualToString:@"SSH Man"]) {
            return window;
        }
    }
    return nil;
}

static NSImage *SSHManMenuBarIcon(void) {
    NSImage *image = nil;
    if (@available(macOS 11.0, *)) {
        image = [NSImage imageWithSystemSymbolName:@"terminal"
                         accessibilityDescription:@"SSH Man"];
    }
    if (image == nil) {
        image = [[[NSImage alloc] initWithSize:NSMakeSize(18.0, 18.0)] autorelease];
        [image lockFocus];
        [[NSColor blackColor] setStroke];

        NSBezierPath *chevron = [NSBezierPath bezierPath];
        [chevron setLineWidth:1.8];
        [chevron setLineCapStyle:NSLineCapStyleRound];
        [chevron setLineJoinStyle:NSLineJoinStyleRound];
        [chevron moveToPoint:NSMakePoint(4.0, 5.0)];
        [chevron lineToPoint:NSMakePoint(8.0, 9.0)];
        [chevron lineToPoint:NSMakePoint(4.0, 13.0)];
        [chevron stroke];

        NSBezierPath *underscore = [NSBezierPath bezierPath];
        [underscore setLineWidth:1.8];
        [underscore setLineCapStyle:NSLineCapStyleRound];
        [underscore moveToPoint:NSMakePoint(9.5, 5.0)];
        [underscore lineToPoint:NSMakePoint(14.0, 5.0)];
        [underscore stroke];
        [image unlockFocus];
    }

    [image setSize:NSMakeSize(18.0, 18.0)];
    [image setTemplate:YES];
    return image;
}

static void SSHManPositionPopup(void) {
    if (sshManPopupWindow == nil || sshManStatusItem.button.window == nil) {
        return;
    }

    NSStatusBarButton *button = sshManStatusItem.button;
    NSRect buttonInWindow = [button convertRect:[button bounds] toView:nil];
    NSRect anchor = [button.window convertRectToScreen:buttonInWindow];
    NSScreen *screen = button.window.screen ?: [NSScreen mainScreen];
    NSRect visible = [screen visibleFrame];

    CGFloat width = MIN(SSHManPopupWidth, MAX(1.0, visible.size.width - (SSHManPopupMargin * 2.0)));
    CGFloat height = MIN(SSHManPopupHeight, MAX(1.0, visible.size.height - (SSHManPopupMargin * 2.0)));
    CGFloat minX = NSMinX(visible) + SSHManPopupMargin;
    CGFloat maxX = NSMaxX(visible) - SSHManPopupMargin - width;
    CGFloat x = NSMidX(anchor) - (width / 2.0);
    x = MAX(minX, MIN(x, maxX));

    CGFloat minY = NSMinY(visible) + SSHManPopupMargin;
    CGFloat y = NSMinY(anchor) - height - SSHManPopupOffset;
    y = MAX(minY, MIN(y, NSMaxY(visible) - SSHManPopupMargin - height));

    NSRect frame = NSMakeRect(round(x), round(y), round(width), round(height));
    [sshManPopupWindow setFrame:frame display:YES animate:NO];
}

static BOOL SSHManShowPopup(void) {
    if (sshManPopupWindow == nil) {
        sshManPopupWindow = [SSHManFindPopupWindow() retain];
    }
    if (sshManPopupWindow == nil) {
        return NO;
    }

    [NSApp unhide:nil];
    SSHManPositionPopup();
    [sshManPopupWindow makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
    return YES;
}

static void SSHManHidePopup(void) {
    [sshManPopupWindow orderOut:nil];
    if ([NSApp isActive]) {
        [NSApp hide:nil];
    }
}

@interface SSHManMenuBarDelegate : NSObject
- (void)statusItemClicked:(id)sender;
- (void)openPopup:(id)sender;
- (void)quitApplication:(id)sender;
- (void)applicationDidResignActive:(NSNotification *)notification;
@end

@implementation SSHManMenuBarDelegate
- (void)statusItemClicked:(id)sender {
    NSEvent *event = [NSApp currentEvent];
    BOOL rightClick = event.type == NSEventTypeRightMouseUp;
    BOOL controlClick = (event.modifierFlags & NSEventModifierFlagControl) != 0;
    if (rightClick || controlClick) {
        [NSMenu popUpContextMenu:sshManContextMenu
                       withEvent:event
                         forView:sshManStatusItem.button];
        return;
    }

    if ([sshManPopupWindow isVisible] && [NSApp isActive]) {
        SSHManHidePopup();
        return;
    }
    SSHManShowPopup();
}

- (void)openPopup:(id)sender {
    SSHManShowPopup();
}

- (void)quitApplication:(id)sender {
    // Leave the menu action before Go tears down the retained target and menu.
    dispatch_async(dispatch_get_main_queue(), ^{
        SSHManMenuBarQuitRequested();
    });
}

- (void)applicationDidResignActive:(NSNotification *)notification {
    [sshManPopupWindow orderOut:nil];
}
@end

static SSHManMenuBarDelegate *sshManMenuBarDelegate = nil;

static int SSHManMenuBarStartOnMainThread(void) {
    if (sshManStatusItem != nil) {
        return 1;
    }

    // The popup and status item outlive this method. Own them explicitly
    // because this file is compiled with manual reference counting.
    sshManPopupWindow = [SSHManFindPopupWindow() retain];
    if (sshManPopupWindow == nil) {
        return 0;
    }

    sshManMenuBarDelegate = [[SSHManMenuBarDelegate alloc] init];
    sshManStatusItem = [[[NSStatusBar systemStatusBar]
        statusItemWithLength:NSSquareStatusItemLength] retain];
    if (sshManStatusItem == nil || sshManStatusItem.button == nil) {
        if (sshManStatusItem != nil) {
            [[NSStatusBar systemStatusBar] removeStatusItem:sshManStatusItem];
            [sshManStatusItem release];
        }
        [sshManPopupWindow release];
        [sshManMenuBarDelegate release];
        sshManPopupWindow = nil;
        sshManMenuBarDelegate = nil;
        sshManStatusItem = nil;
        return 0;
    }

    sshManStatusItem.button.image = SSHManMenuBarIcon();
    sshManStatusItem.button.toolTip = @"SSH Man";
    sshManStatusItem.button.accessibilityLabel = @"SSH Man";
    sshManStatusItem.button.accessibilityHelp = @"Show or hide SSH Man";
    sshManStatusItem.button.target = sshManMenuBarDelegate;
    sshManStatusItem.button.action = @selector(statusItemClicked:);
    [sshManStatusItem.button sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];

    sshManContextMenu = [[NSMenu alloc] initWithTitle:@"SSH Man"];
    NSMenuItem *openItem = [[[NSMenuItem alloc] initWithTitle:@"Open SSH Man"
                                                       action:@selector(openPopup:)
                                                keyEquivalent:@""] autorelease];
    openItem.target = sshManMenuBarDelegate;
    [sshManContextMenu addItem:openItem];
    [sshManContextMenu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *quitItem = [[[NSMenuItem alloc] initWithTitle:@"Quit SSH Man"
                                                       action:@selector(quitApplication:)
                                                keyEquivalent:@"q"] autorelease];
    quitItem.target = sshManMenuBarDelegate;
    [sshManContextMenu addItem:quitItem];

    [[NSNotificationCenter defaultCenter] addObserver:sshManMenuBarDelegate
                                             selector:@selector(applicationDidResignActive:)
                                                 name:NSApplicationDidResignActiveNotification
                                               object:NSApp];
    sshManGlobalEventMonitor = [[NSEvent
        addGlobalMonitorForEventsMatchingMask:(NSEventMaskLeftMouseDown |
                                                NSEventMaskRightMouseDown)
                                   handler:^(NSEvent *event) {
        dispatch_async(dispatch_get_main_queue(), ^{
            [sshManPopupWindow orderOut:nil];
        });
    }] retain];

    [sshManPopupWindow setLevel:NSFloatingWindowLevel];
    [sshManPopupWindow setHasShadow:YES];
    [sshManPopupWindow setCollectionBehavior:(NSWindowCollectionBehaviorMoveToActiveSpace |
                                               NSWindowCollectionBehaviorTransient |
                                               NSWindowCollectionBehaviorFullScreenAuxiliary)];
    [sshManPopupWindow orderOut:nil];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    [NSApp hide:nil];
    return 1;
}

int SSHManMenuBarStart(void) {
    if ([NSThread isMainThread]) {
        return SSHManMenuBarStartOnMainThread();
    }

    __block int result = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        result = SSHManMenuBarStartOnMainThread();
    });
    return result;
}

int SSHManMenuBarShow(void) {
    if ([NSThread isMainThread]) {
        return SSHManShowPopup() ? 1 : 0;
    }

    __block int result = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        result = SSHManShowPopup() ? 1 : 0;
    });
    return result;
}

static void SSHManMenuBarStopOnMainThread(void) {
    if (sshManMenuBarDelegate != nil) {
        [[NSNotificationCenter defaultCenter] removeObserver:sshManMenuBarDelegate];
    }
    if (sshManGlobalEventMonitor != nil) {
        [NSEvent removeMonitor:sshManGlobalEventMonitor];
        [sshManGlobalEventMonitor release];
    }
    if (sshManStatusItem != nil) {
        [[NSStatusBar systemStatusBar] removeStatusItem:sshManStatusItem];
        [sshManStatusItem release];
    }
    [sshManContextMenu release];
    [sshManMenuBarDelegate release];
    [sshManPopupWindow release];
    sshManContextMenu = nil;
    sshManMenuBarDelegate = nil;
    sshManStatusItem = nil;
    sshManPopupWindow = nil;
    sshManGlobalEventMonitor = nil;
}

void SSHManMenuBarStop(void) {
    if ([NSThread isMainThread]) {
        SSHManMenuBarStopOnMainThread();
        return;
    }
    dispatch_sync(dispatch_get_main_queue(), ^{
        SSHManMenuBarStopOnMainThread();
    });
}
