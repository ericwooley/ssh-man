//go:build darwin

#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#import <math.h>
#import <stdint.h>

#include "native_darwin.h"

extern void SSHManMenuBarQuitRequested(void);
extern void SSHManBrowserShortcutRequested(unsigned int direction, unsigned long long sessionID);
extern void SSHManBrowserShortcutCommitted(unsigned long long sessionID);
extern void SSHManBrowserShortcutCanceled(unsigned long long sessionID);

static const CGFloat SSHManPopupWidth = 420.0;
static const CGFloat SSHManPopupHeight = 720.0;
static const CGFloat SSHManPopupMargin = 8.0;
static const CGFloat SSHManPopupOffset = 6.0;
static const CGFloat SSHManSwitcherWidth = 720.0;
static const CGFloat SSHManSwitcherHeight = 340.0;
static const OSType SSHManBrowserHotKeySignature = 0x5353484D; // SSHM

static NSStatusItem *sshManStatusItem = nil;
static NSMenu *sshManContextMenu = nil;
static NSWindow *sshManPopupWindow = nil;
static id sshManGlobalEventMonitor = nil;
static id sshManLocalModifierEventMonitor = nil;
static EventHotKeyRef sshManBrowserForwardHotKey = NULL;
static EventHotKeyRef sshManBrowserBackwardHotKey = NULL;
static EventHandlerRef sshManHotKeyHandler = NULL;
static UInt32 sshManBrowserForwardModifiers = 0;
static UInt32 sshManBrowserBackwardModifiers = 0;
static NSEventModifierFlags sshManBrowserSessionModifiers = 0;
static BOOL sshManBrowserSessionActive = NO;
static uint64_t sshManBrowserSessionID = 0;
static uint64_t sshManBrowserNextSessionID = 1;

static void SSHManMenuBarStopOnMainThread(void);

int SSHManMenuBarShouldDismissPopupForOutsideClick(int applicationActive,
                                                    int popupVisible) {
    return applicationActive == 0 && popupVisible != 0 ? 1 : 0;
}

static uint64_t SSHManEndBrowserSwitchSession(void) {
    if (!sshManBrowserSessionActive) {
        return 0;
    }
    uint64_t sessionID = sshManBrowserSessionID;
    sshManBrowserSessionActive = NO;
    sshManBrowserSessionModifiers = 0;
    sshManBrowserSessionID = 0;
    return sessionID;
}

static void SSHManQueueBrowserSwitchCancel(uint64_t sessionID) {
    if (sessionID == 0) {
        return;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        SSHManBrowserShortcutCanceled((unsigned long long)sessionID);
    });
}

static void SSHManCancelBrowserSwitchSessionAndNotify(void) {
    SSHManQueueBrowserSwitchCancel(SSHManEndBrowserSwitchSession());
}

static void SSHManCancelBrowserSwitchSessionSilently(void) {
    SSHManEndBrowserSwitchSession();
}

static NSEventModifierFlags SSHManSessionModifiersForCarbon(UInt32 modifiers) {
    NSEventModifierFlags result = 0;
    if ((modifiers & cmdKey) != 0) {
        result |= NSEventModifierFlagCommand;
    }
    if ((modifiers & optionKey) != 0) {
        result |= NSEventModifierFlagOption;
    }
    if ((modifiers & controlKey) != 0) {
        result |= NSEventModifierFlagControl;
    }
    // Shortcut validation currently requires Command, Option, or Control.
    // Retain a Shift-only fallback so a future validator change cannot create
    // a switcher session that has no modifier capable of committing it.
    if (result == 0 && (modifiers & shiftKey) != 0) {
        result = NSEventModifierFlagShift;
    }
    return result;
}

static void SSHManCommitBrowserSwitchIfReleased(NSEventModifierFlags currentModifiers,
                                                 uint64_t expectedSessionID) {
    if (!sshManBrowserSessionActive || sshManBrowserSessionModifiers == 0 ||
        sshManBrowserSessionID != expectedSessionID ||
        (currentModifiers & sshManBrowserSessionModifiers) == sshManBrowserSessionModifiers) {
        return;
    }

    BOOL shouldCommit = [NSApp isActive] && [sshManPopupWindow isVisible];
    uint64_t sessionID = SSHManEndBrowserSwitchSession();
    if (shouldCommit) {
        // Leave AppKit's input dispatch before entering Go. The Go callback
        // then emits into Wails from its serial worker.
        dispatch_async(dispatch_get_main_queue(), ^{
            SSHManBrowserShortcutCommitted((unsigned long long)sessionID);
        });
    } else {
        SSHManQueueBrowserSwitchCancel(sessionID);
    }
}

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

static NSScreen *SSHManActiveScreen(void) {
    NSPoint mouseLocation = [NSEvent mouseLocation];
    for (NSScreen *screen in [NSScreen screens]) {
        if (NSPointInRect(mouseLocation, screen.frame)) {
            return screen;
        }
    }
    return [NSScreen mainScreen];
}

static BOOL SSHManShowBrowserSwitcherOnMainThread(void) {
    if (sshManPopupWindow == nil) {
        sshManPopupWindow = [SSHManFindPopupWindow() retain];
    }
    if (sshManPopupWindow == nil) {
        return NO;
    }

    NSScreen *screen = SSHManActiveScreen();
    NSRect visible = screen.visibleFrame;
    CGFloat width = MIN(SSHManSwitcherWidth, MAX(1.0, visible.size.width - (SSHManPopupMargin * 2.0)));
    CGFloat height = MIN(SSHManSwitcherHeight, MAX(1.0, visible.size.height - (SSHManPopupMargin * 2.0)));
    NSRect frame = NSMakeRect(
        round(NSMidX(visible) - (width / 2.0)),
        round(NSMidY(visible) - (height / 2.0)),
        round(width),
        round(height)
    );

    [NSApp unhide:nil];
    [sshManPopupWindow setFrame:frame display:YES animate:NO];
    [NSApp activateIgnoringOtherApps:YES];
    [sshManPopupWindow makeKeyAndOrderFront:nil];
    // A very fast shortcut tap can release its modifier before the newly
    // active window receives flagsChanged. Check current synthesized state
    // after a short grace period so the initiating modifier-down event has
    // settled before deciding that it was already released.
    uint64_t expectedSessionID = sshManBrowserSessionID;
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.05 * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
        SSHManCommitBrowserSwitchIfReleased([NSEvent modifierFlags], expectedSessionID);
    });
    return YES;
}

static void SSHManHidePopup(void) {
    SSHManCancelBrowserSwitchSessionAndNotify();
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
- (void)windowDidResignKey:(NSNotification *)notification;
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
    SSHManCancelBrowserSwitchSessionAndNotify();
    [sshManPopupWindow orderOut:nil];
}

- (void)windowDidResignKey:(NSNotification *)notification {
    SSHManCancelBrowserSwitchSessionAndNotify();
}
@end

static SSHManMenuBarDelegate *sshManMenuBarDelegate = nil;

static OSStatus SSHManHandleHotKey(EventHandlerCallRef nextHandler, EventRef event, void *userData) {
    EventHotKeyID hotKeyID;
    OSStatus status = GetEventParameter(event,
                                        kEventParamDirectObject,
                                        typeEventHotKeyID,
                                        NULL,
                                        sizeof(hotKeyID),
                                        NULL,
                                        &hotKeyID);
    if (status != noErr || hotKeyID.signature != SSHManBrowserHotKeySignature) {
        return CallNextEventHandler(nextHandler, event);
    }

    UInt32 direction = hotKeyID.id;
    if (direction != SSHManBrowserSwitchForward &&
        direction != SSHManBrowserSwitchBackward) {
        return CallNextEventHandler(nextHandler, event);
    }

    if (!sshManBrowserSessionActive) {
        UInt32 registeredModifiers = direction == SSHManBrowserSwitchForward
            ? sshManBrowserForwardModifiers
            : sshManBrowserBackwardModifiers;
        sshManBrowserSessionModifiers = SSHManSessionModifiersForCarbon(registeredModifiers);
        sshManBrowserSessionActive = sshManBrowserSessionModifiers != 0;
        if (!sshManBrowserSessionActive) {
            return CallNextEventHandler(nextHandler, event);
        }
        sshManBrowserSessionID = sshManBrowserNextSessionID++;
        if (sshManBrowserNextSessionID == 0) {
            sshManBrowserNextSessionID = 1;
        }
    }
    uint64_t sessionID = sshManBrowserSessionID;

    // Do not cross into Go while Carbon is still dispatching the hot-key
    // event. The Go callback shows the AppKit window and emits a Wails event;
    // doing either re-entrantly from this handler can corrupt the native event
    // dispatch stack. Queue the handoff so this handler returns first.
    dispatch_async(dispatch_get_main_queue(), ^{
        SSHManBrowserShortcutRequested(direction, (unsigned long long)sessionID);
    });
    return noErr;
}

static int SSHManInstallHotKeyHandler(void) {
    if (sshManHotKeyHandler != NULL) {
        return 1;
    }
    EventTypeSpec eventType = {kEventClassKeyboard, kEventHotKeyPressed};
    OSStatus status = InstallApplicationEventHandler(&SSHManHandleHotKey,
                                                      1,
                                                      &eventType,
                                                      NULL,
                                                      &sshManHotKeyHandler);
    return status == noErr ? 1 : 0;
}

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
    [[NSNotificationCenter defaultCenter] addObserver:sshManMenuBarDelegate
                                             selector:@selector(windowDidResignKey:)
                                                 name:NSWindowDidResignKeyNotification
                                               object:sshManPopupWindow];
    sshManGlobalEventMonitor = [[NSEvent
        addGlobalMonitorForEventsMatchingMask:(NSEventMaskLeftMouseDown |
                                                NSEventMaskRightMouseDown)
                                   handler:^(NSEvent *event) {
        // AppKit invokes global monitor handlers on the main thread, but it
        // delivers the copied outside-app event asynchronously. Ignore a
        // stale copy if SSH Man has already become active again; otherwise it
        // can order the popup out during the user's next in-app click.
        if (!SSHManMenuBarShouldDismissPopupForOutsideClick(
                [NSApp isActive] ? 1 : 0,
                [sshManPopupWindow isVisible] ? 1 : 0)) {
            return;
        }
        SSHManCancelBrowserSwitchSessionAndNotify();
        [sshManPopupWindow orderOut:nil];
    }] retain];
    sshManLocalModifierEventMonitor = [[NSEvent
        addLocalMonitorForEventsMatchingMask:NSEventMaskFlagsChanged
                                   handler:^NSEvent *(NSEvent *event) {
        SSHManCommitBrowserSwitchIfReleased(event.modifierFlags, sshManBrowserSessionID);
        return event;
    }] retain];
    if (sshManLocalModifierEventMonitor == nil) {
        SSHManMenuBarStopOnMainThread();
        return 0;
    }

    [sshManPopupWindow setLevel:NSFloatingWindowLevel];
    [sshManPopupWindow setHasShadow:YES];
    [sshManPopupWindow setCollectionBehavior:(NSWindowCollectionBehaviorMoveToActiveSpace |
                                               NSWindowCollectionBehaviorTransient |
                                               NSWindowCollectionBehaviorFullScreenAuxiliary)];
    [sshManPopupWindow orderOut:nil];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    [NSApp hide:nil];
    if (!SSHManInstallHotKeyHandler()) {
        SSHManMenuBarStopOnMainThread();
        return 0;
    }
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

int SSHManMenuBarShowBrowserSwitcher(void) {
    if ([NSThread isMainThread]) {
        return SSHManShowBrowserSwitcherOnMainThread() ? 1 : 0;
    }

    __block int result = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        result = SSHManShowBrowserSwitcherOnMainThread() ? 1 : 0;
    });
    return result;
}

void SSHManMenuBarCancelBrowserSwitchSession(void) {
    if ([NSThread isMainThread]) {
        SSHManCancelBrowserSwitchSessionAndNotify();
        return;
    }
    dispatch_sync(dispatch_get_main_queue(), ^{
        SSHManCancelBrowserSwitchSessionAndNotify();
    });
}

static void SSHManUnregisterBrowserShortcutsOnMainThread(BOOL notifyCancel) {
    if (notifyCancel) {
        SSHManCancelBrowserSwitchSessionAndNotify();
    } else {
        SSHManCancelBrowserSwitchSessionSilently();
    }
    if (sshManBrowserForwardHotKey != NULL) {
        UnregisterEventHotKey(sshManBrowserForwardHotKey);
        sshManBrowserForwardHotKey = NULL;
    }
    if (sshManBrowserBackwardHotKey != NULL) {
        UnregisterEventHotKey(sshManBrowserBackwardHotKey);
        sshManBrowserBackwardHotKey = NULL;
    }
    sshManBrowserForwardModifiers = 0;
    sshManBrowserBackwardModifiers = 0;
}

static int SSHManSetBrowserShortcutsOnMainThread(unsigned int forwardKeyCode,
                                                  unsigned int forwardModifiers,
                                                  unsigned int backwardKeyCode,
                                                  unsigned int backwardModifiers) {
    if (!SSHManInstallHotKeyHandler()) {
        return 0;
    }
    SSHManUnregisterBrowserShortcutsOnMainThread(YES);

    EventHotKeyID forwardHotKeyID = {
        SSHManBrowserHotKeySignature,
        SSHManBrowserSwitchForward,
    };
    OSStatus status = RegisterEventHotKey((UInt32)forwardKeyCode,
                                          (UInt32)forwardModifiers,
                                          forwardHotKeyID,
                                          GetApplicationEventTarget(),
                                          0,
                                          &sshManBrowserForwardHotKey);
    if (status != noErr) {
        SSHManUnregisterBrowserShortcutsOnMainThread(NO);
        return 0;
    }
    sshManBrowserForwardModifiers = (UInt32)forwardModifiers;

    EventHotKeyID backwardHotKeyID = {
        SSHManBrowserHotKeySignature,
        SSHManBrowserSwitchBackward,
    };
    status = RegisterEventHotKey((UInt32)backwardKeyCode,
                                 (UInt32)backwardModifiers,
                                 backwardHotKeyID,
                                 GetApplicationEventTarget(),
                                 0,
                                 &sshManBrowserBackwardHotKey);
    if (status != noErr) {
        SSHManUnregisterBrowserShortcutsOnMainThread(NO);
        return 0;
    }
    sshManBrowserBackwardModifiers = (UInt32)backwardModifiers;
    return 1;
}

int SSHManMenuBarSetBrowserShortcuts(unsigned int forwardKeyCode,
                                     unsigned int forwardModifiers,
                                     unsigned int backwardKeyCode,
                                     unsigned int backwardModifiers) {
    if ([NSThread isMainThread]) {
        return SSHManSetBrowserShortcutsOnMainThread(forwardKeyCode,
                                                      forwardModifiers,
                                                      backwardKeyCode,
                                                      backwardModifiers);
    }

    __block int result = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        result = SSHManSetBrowserShortcutsOnMainThread(forwardKeyCode,
                                                        forwardModifiers,
                                                        backwardKeyCode,
                                                        backwardModifiers);
    });
    return result;
}

static void SSHManMenuBarStopOnMainThread(void) {
    SSHManUnregisterBrowserShortcutsOnMainThread(NO);
    if (sshManHotKeyHandler != NULL) {
        RemoveEventHandler(sshManHotKeyHandler);
        sshManHotKeyHandler = NULL;
    }
    if (sshManMenuBarDelegate != nil) {
        [[NSNotificationCenter defaultCenter] removeObserver:sshManMenuBarDelegate];
    }
    if (sshManGlobalEventMonitor != nil) {
        [NSEvent removeMonitor:sshManGlobalEventMonitor];
        [sshManGlobalEventMonitor release];
    }
    if (sshManLocalModifierEventMonitor != nil) {
        [NSEvent removeMonitor:sshManLocalModifierEventMonitor];
        [sshManLocalModifierEventMonitor release];
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
    sshManLocalModifierEventMonitor = nil;
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
