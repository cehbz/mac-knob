#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include <dlfcn.h>
#include <string.h>

// Private SkyLight class, interface from the macOS 26.4 runtime headers.
// Declared as a category on NSObject so the compiler accepts the dynamic
// selectors; dispatch is resolved at runtime against the real class.
@interface NSObject (SKBridgedMoveDecls)
- (id)initWithWindows:(NSArray *)windows spaceID:(unsigned long long)spaceID;
- (void)performWithWMBridgeDelegate;
@end

int sk_move_windows_to_space(const uint32_t *wids, int count, unsigned long long sid) {
	// SkyLight is already loaded via CoreGraphics, but be explicit.
	dlopen("/System/Library/PrivateFrameworks/SkyLight.framework/SkyLight", RTLD_LAZY);
	Class cls = NSClassFromString(@"SLSBridgedMoveWindowsToManagedSpaceOperation");
	if (!cls) return 1;
	if (![cls instancesRespondToSelector:@selector(performWithWMBridgeDelegate)]) return 2;

	NSMutableArray *windows = [NSMutableArray arrayWithCapacity:(NSUInteger)count];
	for (int i = 0; i < count; i++) [windows addObject:@(wids[i])];

	id op = [[cls alloc] initWithWindows:windows spaceID:sid];
	if (!op) return 3;
	[op performWithWMBridgeDelegate];
	[op release];
	return 0;
}

char *sk_bundle_id_for_pid(int pid) {
	NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pid];
	NSString *bid = app.bundleIdentifier;
	if (!bid) return NULL;
	return strdup([bid UTF8String]);
}
