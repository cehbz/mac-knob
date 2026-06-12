#import <Foundation/Foundation.h>
#import <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>
#import <CoreGraphics/CoreGraphics.h>

// Creating a Dock-managed space without disabling SIP is only possible by
// driving Mission Control's accessibility tree. The tree (macOS 26):
//   Dock > AXGroup id=mc > AXGroup id=mc.display (one per monitor)
//        > ... > AXButton id=mc.spaces.add  ("add desktop")
// This is deliberately flashy (Mission Control animates in) and is only used
// for one-time space recreation, never for routine switching.

static NSString *axStr(AXUIElementRef e, CFStringRef a) {
	CFTypeRef v = NULL;
	if (AXUIElementCopyAttributeValue(e, a, &v) != kAXErrorSuccess || !v) return @"";
	NSString *s = [NSString stringWithFormat:@"%@", v];
	CFRelease(v);
	return s;
}

static NSArray *axKids(AXUIElementRef e) {
	CFTypeRef c = NULL;
	if (AXUIElementCopyAttributeValue(e, kAXChildrenAttribute, &c) == kAXErrorSuccess && c)
		return CFBridgingRelease(c);
	return @[];
}

static AXUIElementRef axFindById(AXUIElementRef e, NSString *ident) {
	if ([axStr(e, kAXIdentifierAttribute) isEqualToString:ident]) return e;
	for (id k in axKids(e)) {
		AXUIElementRef r = axFindById((__bridge AXUIElementRef)k, ident);
		if (r) return r;
	}
	return NULL;
}

static pid_t dockPid(void) {
	for (NSRunningApplication *a in NSWorkspace.sharedWorkspace.runningApplications)
		if ([a.bundleIdentifier isEqualToString:@"com.apple.dock"]) return a.processIdentifier;
	return 0;
}

// Ordered mc.display groups under the mc group.
static NSArray *mcDisplays(pid_t pid) {
	AXUIElementRef dock = AXUIElementCreateApplication(pid);
	AXUIElementRef mc = axFindById(dock, @"mc");
	NSMutableArray *out = [NSMutableArray array];
	if (mc) {
		for (id k in axKids(mc))
			if ([axStr((__bridge AXUIElementRef)k, kAXIdentifierAttribute) isEqualToString:@"mc.display"])
				[out addObject:k];
	}
	CFRelease(dock);
	return out;
}

static void postEscape(void) {
	CGEventRef d = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)53, true);
	CGEventRef u = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)53, false);
	CGEventPost(kCGHIDEventTap, d);
	CGEventPost(kCGHIDEventTap, u);
	if (d) CFRelease(d);
	if (u) CFRelease(u);
}

// Press each display's add-desktop button counts[i] times. Returns 0 on
// success, 1 if Mission Control could not be reached, 2 if a display that
// needs spaces has no add button (and reports how many it managed via *added).
int sk_add_spaces(const int *counts, int n, int *added) {
	if (added) *added = 0;
	if (!AXIsProcessTrusted()) return 1;
	pid_t pid = dockPid();
	if (!pid) return 1;

	@autoreleasepool {
		system("open -a 'Mission Control'");
		[NSThread sleepForTimeInterval:1.2];

		if (mcDisplays(pid).count == 0) { postEscape(); return 1; }

		int rc = 0;
		for (int i = 0; i < n; i++) {
			for (int j = 0; j < counts[i]; j++) {
				// Re-traverse each press: the tree reflows as desktops appear.
				NSArray *displays = mcDisplays(pid);
				if ((NSUInteger)i >= displays.count) { rc = 2; break; }
				AXUIElementRef add = axFindById((__bridge AXUIElementRef)displays[i], @"mc.spaces.add");
				if (!add) { rc = 2; break; }
				if (AXUIElementPerformAction(add, kAXPressAction) == kAXErrorSuccess && added) (*added)++;
				[NSThread sleepForTimeInterval:0.4];
			}
		}
		postEscape();
		[NSThread sleepForTimeInterval:0.3];
		return rc;
	}
}
