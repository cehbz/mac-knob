# mac-knob

Tools for driving macOS Spaces from a mouse, plus notes toward a DIY jog wheel for video editing.

## spaceswitch

Switches Mission Control spaces with the native slide animation. No SIP changes, no private write APIs. The mouse tilt wheel (horizontal scroll click) becomes previous/next space.

macOS ignores synthetic Ctrl+Arrow shortcuts posted from ordinary session-level event sources, which is why Hammerspoon's `keyStroke` and similar approaches silently fail. spaceswitch builds the keystroke from the HID system state event source and posts it to the HID event tap (`kCGEventSourceStateHIDSystemState`, `kCGHIDEventTap`). Mission Control accepts that and performs a real switch.

Daemon mode runs an event tap on scroll events. A tilt click arrives as a single discrete horizontal-only scroll delta. The daemon consumes it and posts Ctrl+Left/Right. Vertical scrolling, continuous (trackpad and Magic Mouse) scrolling, and shift+wheel synthesized horizontal scroll all pass through untouched.

Tested on macOS 26.5 (Tahoe), Apple Silicon, with a Logitech M720. The only private API use is read-only SkyLight introspection for `status` and `-verify`; the switch itself is plain CGEvent posting.

### Build

```
go build -o spaceswitch .
```

### Usage

```
spaceswitch left|right [-verify]   one-shot switch; -verify confirms the active space changed
spaceswitch status                 print the active space ID
spaceswitch daemon [-invert]       tilt wheel -> space switch
```

### Run at login

Save as `~/Library/LaunchAgents/spaceswitch.plist` (adjust the binary path), then `launchctl bootstrap gui/$UID ~/Library/LaunchAgents/spaceswitch.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>spaceswitch</string>
	<key>ProgramArguments</key>
	<array>
		<string>/Users/you/bin/spaceswitch</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardErrorPath</key>
	<string>/tmp/spaceswitch.log</string>
</dict>
</plist>
```

### Permissions

The daemon needs Accessibility (System Settings, Privacy & Security, Accessibility). Under launchd the binary is its own TCC subject, so grant it to the binary itself, not your terminal. Rebuilding changes the ad-hoc code signature and silently invalidates the grant; toggle it off and on again after a rebuild if tilts stop working.

### Caveats

- Ctrl+Arrow switches the space on the display that has keyboard focus, not the display under the cursor. That matches what the physical shortcut does.
- The Mission Control shortcuts "Move left/right a space" must be enabled (they are by default).
- Mapping the tilt to space switching costs you tilt-driven horizontal scrolling. Shift+wheel still scrolls horizontally.

## Research

`macos-spaces-research.md` is a survey of the macOS Spaces control landscape as of mid-2026: public and private APIs, version-by-version breakage, switching techniques, window-to-space persistence, and existing tools (yabai, AeroSpace, InstantSpaceSwitcher, Hammerspoon, and others), with sources.
