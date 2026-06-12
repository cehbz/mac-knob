package main

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>
#include <stdio.h>

// Mission Control ignores synthetic shortcuts posted to the session tap from
// a session-state source. Building the event from the HID system state and
// posting to the HID tap is the documented bypass (MacGesture fix). Physical
// arrow keys carry the secondary-fn flag, so the synthetic ones do too.
static const CGEventFlags kSpaceSwitchFlags = kCGEventFlagMaskControl | kCGEventFlagMaskSecondaryFn;

static int post_ctrl_arrow(int right) {
	CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
	if (!src) return -1;
	CGKeyCode key = right ? 124 : 123;
	CGEventRef down = CGEventCreateKeyboardEvent(src, key, true);
	CGEventRef up = CGEventCreateKeyboardEvent(src, key, false);
	int rc = 0;
	if (down && up) {
		CGEventSetFlags(down, kSpaceSwitchFlags);
		CGEventSetFlags(up, kSpaceSwitchFlags);
		CGEventPost(kCGHIDEventTap, down);
		CGEventPost(kCGHIDEventTap, up);
	} else {
		rc = -2;
	}
	if (down) CFRelease(down);
	if (up) CFRelease(up);
	CFRelease(src);
	return rc;
}

static int ax_trusted(int prompt) {
	if (!prompt) return AXIsProcessTrusted();
	CFStringRef keys[] = { kAXTrustedCheckOptionPrompt };
	CFBooleanRef vals[] = { kCFBooleanTrue };
	CFDictionaryRef opts = CFDictionaryCreate(NULL, (const void **)keys, (const void **)vals, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	Boolean ok = AXIsProcessTrustedWithOptions(opts);
	CFRelease(opts);
	return ok;
}

// Daemon: an active session event tap on scroll-wheel events. A tilt click is
// a discrete horizontal-only delta (M720: exactly one event per click,
// h = ±1). Continuous deltas (trackpads, Magic Mouse) and shift+wheel
// (synthesized into horizontal scroll downstream of HID) pass through.
// The callback stays in C and posts directly: tap callbacks that run slow get
// auto-disabled by the system, so no cgo round-trip per event.
static int g_invert = 0;
static CFMachPortRef g_tap = NULL;

static CGEventRef tilt_callback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *info) {
	if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
		if (g_tap) CGEventTapEnable(g_tap, true);
		fprintf(stderr, "spaceswitch: tap re-enabled (disable type=%d)\n", (int)type);
		return event;
	}
	if (type != kCGEventScrollWheel) return event;

	int64_t continuous = CGEventGetIntegerValueField(event, kCGScrollWheelEventIsContinuous);
	int64_t v = CGEventGetIntegerValueField(event, kCGScrollWheelEventDeltaAxis1);
	int64_t h = CGEventGetIntegerValueField(event, kCGScrollWheelEventDeltaAxis2);
	if (continuous || h == 0 || v != 0) return event;
	if (CGEventGetFlags(event) & kCGEventFlagMaskShift) return event;

	// CGEvent scroll deltas are positive for up/left, so h > 0 is tilt-left.
	int right = (h < 0) ? !g_invert : g_invert;
	post_ctrl_arrow(right);
	return NULL; // consume the tilt so it doesn't also scroll horizontally
}

static int run_daemon(int invert) {
	g_invert = invert;
	g_tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault,
		CGEventMaskBit(kCGEventScrollWheel), tilt_callback, NULL);
	if (!g_tap) return -1;
	CFRunLoopSourceRef src = CFMachPortCreateRunLoopSource(NULL, g_tap, 0);
	CFRunLoopAddSource(CFRunLoopGetCurrent(), src, kCFRunLoopCommonModes);
	CGEventTapEnable(g_tap, true);
	CFRunLoopRun();
	return 0;
}
*/
import "C"

import "fmt"

func postCtrlArrow(right bool) error {
	r := C.int(0)
	if right {
		r = 1
	}
	if rc := C.post_ctrl_arrow(r); rc != 0 {
		return fmt.Errorf("posting ctrl+arrow failed (rc=%d)", rc)
	}
	return nil
}

func axTrusted(prompt bool) bool {
	p := C.int(0)
	if prompt {
		p = 1
	}
	return C.ax_trusted(p) != 0
}

// runDaemon blocks in CFRunLoopRun and only returns on setup failure.
func runDaemon(invert bool) error {
	i := C.int(0)
	if invert {
		i = 1
	}
	if rc := C.run_daemon(i); rc != 0 {
		return fmt.Errorf("event tap creation failed — is Accessibility granted to this binary?")
	}
	return nil
}
