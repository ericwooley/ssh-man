//go:build darwin

#import <Cocoa/Cocoa.h>

#include "activation_darwin.h"

static int SSHManActivateBrowserProcessOnMainThread(int pid) {
    NSRunningApplication *application = [NSRunningApplication
        runningApplicationWithProcessIdentifier:(pid_t)pid];
    if (application == nil || application.terminated) {
        return 0;
    }
    #pragma clang diagnostic push
    #pragma clang diagnostic ignored "-Wdeprecated-declarations"
    NSApplicationActivationOptions options = NSApplicationActivateAllWindows |
                                              NSApplicationActivateIgnoringOtherApps;
    #pragma clang diagnostic pop
    return [application activateWithOptions:options] ? 1 : 0;
}

int SSHManActivateBrowserProcess(int pid) {
    if ([NSThread isMainThread]) {
        return SSHManActivateBrowserProcessOnMainThread(pid);
    }

    __block int result = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        result = SSHManActivateBrowserProcessOnMainThread(pid);
    });
    return result;
}
