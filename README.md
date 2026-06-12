# mac-knob

Tools for driving macOS Spaces from a mouse, plus notes toward a DIY jog wheel for video editing.

## spaceswitch

Switches Mission Control spaces with the native slide animation. No SIP changes, no private write APIs. The mouse tilt wheel (horizontal scroll click) becomes previous/next space.

macOS ignores synthetic Ctrl+Arrow shortcuts posted from ordinary session-level event sources, which is why Hammerspoon's `keyStroke` and similar approaches silently fail. spaceswitch builds the keystroke from the HID system state event source and posts it to the HID event tap (`kCGEventSourceStateHIDSystemState`, `kCGHIDEventTap`). Mission Control accepts that and performs a real switch.

Daemon mode runs an event tap on scroll events. A tilt click arrives as a single discrete horizontal-only scroll delta. The daemon consumes it and posts Ctrl+Left/Right. Vertical scrolling, continuous (trackpad and Magic Mouse) scrolling, and shift+wheel synthesized horizontal scroll all pass through untouched.

Tested on macOS 26.5 (Tahoe), Apple Silicon, with a Logitech M720. The only private API use is read-only SkyLight introspection for `status` and `-verify`; the switch itself is plain CGEvent posting.

### Build

```
go build -o spaceswitch ./cmd/spaceswitch
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

The daemon needs Accessibility (System Settings, Privacy & Security, Accessibility). Under launchd the binary is its own TCC subject, so grant it to the binary itself, not your terminal.

### Stable signing

macOS binds an Accessibility grant to a binary's code signature. An ad-hoc signature (the default) changes its cdhash on every rebuild, so each rebuild orphans the grant, makes macOS re-prompt, and leaves a stale duplicate entry in the Accessibility list. Signing with a stable self-signed identity fixes this for good: the grant then keys on the identifier plus certificate (`identifier "bz.ceh.spaceswitch" and certificate leaf = H"…"`) and survives rebuilds.

The signing identity lives in 1Password, not in any keychain. The build pulls it into a throwaway keychain that is deleted when signing finishes — nothing is written to the login keychain.

One-time setup:

1. `./scripts/gen-signing-cert.sh` — generates a self-signed Code Signing certificate (via `openssl`, no Keychain Access) as a password-protected `.p12` and prints the password plus a ready-to-run `op item create`.
2. Run the printed `op item create` to store the `.p12` and its password in 1Password, then `rm` the local `.p12`.
3. `make install` — pulls the identity from 1Password into a temporary keychain, signs both binaries, installs to `~/bin`. Falls back to ad-hoc with a warning if the secret or `op` is unavailable.
4. Remove any existing `spaceswitch`/`spacekeeper` entries from the Accessibility list, then grant once. Future rebuilds keep the grant.

Overrides: `CODESIGN_IDENTITY` (certificate CN, default `mac-knob`), `OP_P12_REF` / `OP_P12PW_REF` (the `op://` secret references).

### Caveats

- Ctrl+Arrow switches the space on the display that has keyboard focus, not the display under the cursor. That matches what the physical shortcut does.
- The Mission Control shortcuts "Move left/right a space" must be enabled (they are by default).
- Mapping the tilt to space switching costs you tilt-driven horizontal scrolling. Shift+wheel still scrolls horizontally.

## spacekeeper

Saves window-to-space assignments and moves windows back later, after a reboot or a display reconfiguration has scattered them. SIP stays enabled.

```
go build -o spacekeeper ./cmd/spacekeeper

spacekeeper save              snapshot to ~/.config/spacekeeper/layout.json
spacekeeper restore [-n]      move windows back; -n prints the plan without moving
spacekeeper show              print the saved layout
```

The read side is SkyLight introspection (`SLSCopyManagedDisplaySpaces`, `SLSCopySpacesForWindows`), callable from any process. The write side is `SLSBridgedMoveWindowsToManagedSpaceOperation`, the bridged operation that works with SIP enabled since macOS 26.4. It is private and may vanish in any update; spacekeeper resolves it at runtime and reports clearly when it is unavailable. Verified working on macOS 26.5.

Spaces are identified by UUID, with a (display, position) fallback when a UUID is gone. Window IDs do not survive reboots, so windows are re-matched by app, then title, then frame proximity. Grant Screen Recording to make titles visible; without it matching uses app and frame only.

Limitations, by design for now: spaces are not recreated (restore into your existing set), and fullscreen windows and all-spaces (sticky) windows are skipped.

## Research

`macos-spaces-research.md` is a survey of the macOS Spaces control landscape as of mid-2026: public and private APIs, version-by-version breakage, switching techniques, window-to-space persistence, and existing tools (yabai, AeroSpace, InstantSpaceSwitcher, Hammerspoon, and others), with sources.
