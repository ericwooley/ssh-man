#import <AppKit/AppKit.h>
#import <CoreServices/CoreServices.h>
#import <dispatch/dispatch.h>

#import "default_darwin.h"

static CFStringRef sshManBundleID(void) {
    return CFSTR("tech.moonpixels.ssh-man");
}

static int handlerMatches(NSString *scheme) {
    NSURL *sampleURL = [NSURL URLWithString:[NSString stringWithFormat:@"%@://ssh-man.invalid/", scheme]];
    NSURL *applicationURL = [[NSWorkspace sharedWorkspace] URLForApplicationToOpenURL:sampleURL];
    if (applicationURL == nil) {
        return 0;
    }
    NSBundle *bundle = [NSBundle bundleWithURL:applicationURL];
    return [[bundle bundleIdentifier] isEqualToString:(NSString *)sshManBundleID()] ? 1 : 0;
}

static int setHandler(NSString *scheme) {
    NSWorkspace *workspace = [NSWorkspace sharedWorkspace];
    NSURL *applicationURL = [[NSBundle mainBundle] bundleURL];
    if (@available(macOS 12.0, *)) {
        dispatch_semaphore_t completed = dispatch_semaphore_create(0);
        __block NSInteger errorCode = 0;
        [workspace setDefaultApplicationAtURL:applicationURL
                         toOpenURLsWithScheme:scheme
                           completionHandler:^(NSError *error) {
            if (error != nil) {
                errorCode = [error code];
            }
            dispatch_semaphore_signal(completed);
        }];
        long waitResult = dispatch_semaphore_wait(
            completed,
            dispatch_time(DISPATCH_TIME_NOW, (int64_t)(120 * NSEC_PER_SEC))
        );
        dispatch_release(completed);
        if (waitResult != 0) {
            return -9999;
        }
        return (int)errorCode;
    }
    return (int)LSSetDefaultHandlerForURLScheme((CFStringRef)scheme, sshManBundleID());
}

int sshManIsDefaultBrowser(void) {
    @autoreleasepool {
        return handlerMatches(@"http") && handlerMatches(@"https");
    }
}

int sshManSetDefaultBrowser(void) {
    @autoreleasepool {
        int httpStatus = setHandler(@"http");
        if (httpStatus != 0) {
            return httpStatus;
        }
        return setHandler(@"https");
    }
}
