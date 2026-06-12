# Controlling macOS Spaces programmatically — research report

Date: 2026-06-12. Target: macOS Sequoia 15 / Tahoe 26, Apple Silicon.
Method: deep-research workflow (105 agents, 23 sources fetched, 109 claims extracted, 25 adversarially verified → 21 confirmed, 4 refuted) plus two follow-up agents for tilt-wheel mapping and save/restore tooling.

## 1. API landscape

There is no public Apple API for enumerating, switching, or assigning windows to Spaces. The nearest public surfaces are `NSWindow.collectionBehavior` (canJoinAllSpaces etc., own-process only), CGEvent synthesis (keyboard shortcuts and gestures), the Accessibility (AX) API, and the Dock's per-app "Assign To" setting. Everything else is private SkyLight/CGS.

### Read side (works with SIP fully enabled, any process)

| Call | Returns | Notes |
|---|---|---|
| `SLSCopyManagedDisplaySpaces(cid)` | per-display dict: `Display Identifier` (display UUID), `Current Space`, `Spaces[]` each with `ManagedSpaceID`/`id64`, `uuid`, `type` (0=desktop, 4=fullscreen/tiled) | Used by Hammerspoon `hs.spaces.allSpaces()`, yabai query, DockDoor/WindowKit, FlashSpace. Confirmed working on Sequoia 15.0.1 ([HS #3698](https://github.com/Hammerspoon/hammerspoon/issues/3698)) |
| `SLSCopySpacesForWindows(cid, 0x7, windowIDs)` | window → space IDs | Mask values in [CGSInternal/CGSSpace.h](https://github.com/NUIKit/CGSInternal/blob/master/CGSSpace.h). Flaky for hidden/minimized windows (anecdotal) |
| `SLSCopyWindowsWithOptionsAndTags(...)` | space → window IDs, including non-visible spaces | `hs.spaces.windowsForSpace()` |
| `CGWindowListCopyWindowInfo` (public) | window metadata (number, PID, bounds) | Window *titles* need Screen Recording permission since 10.15 |
| `SLSManagedDisplayGetCurrentSpace`, `SLSSpaceGetType`, `SLSCopyManagedDisplayForWindow`, `SLSSpaceCopyName` | misc | Full list: [yabai src/misc/extern.h](https://github.com/asmvik/yabai/blob/master/src/misc/extern.h) |

Reference code: [ejbills/WindowKit SkyLightSpace.swift](https://github.com/ejbills/WindowKit/blob/main/Sources/WindowKit/SystemBridge/SkyLightSpace.swift), [Hammerspoon libspaces.m](https://github.com/Hammerspoon/hammerspoon/blob/master/extensions/spaces/libspaces.m).

### Write side — moving windows between spaces (version history)

| Era | Mechanism | SIP |
|---|---|---|
| ≤ 14.4 | `SLSMoveWindowsToManagedSpace` from any process | enabled OK |
| 14.5 – 14.x | WindowServer rejects non-Dock callers; workaround: `SLSSpaceSetCompatID` + `SLSSetWindowListWorkspace` (yabai commit 98bbdbd, Hammerspoon ≥1.0.0) | enabled OK |
| 15.0 (Sequoia) | Compat-ID workaround dead — `hs.spaces.moveWindowToSpace` returns true but does nothing ([HS #3698](https://github.com/Hammerspoon/hammerspoon/issues/3698), [#3666](https://github.com/Hammerspoon/hammerspoon/issues/3666)). yabai fell back to its SIP-disabled Dock scripting addition | **disable required** |
| 26.4 (Tahoe, **May 2026**) | `SLSBridgedMoveWindowsToManagedSpaceOperation` — ObjC class in SkyLight (`initWithWindows:spaceID:`, subclass of `SLSAsynchronousBridgedWindowManagementOperation`). Validated SIP-enabled on 26.4/26.4.1 by @ejbills/@magicmark; shipped in yabai 7.1.25 (2026-05-08) ([asmvik/yabai #2788](https://github.com/asmvik/yabai/issues/2788), [headers](https://github.com/thatmarcel/macOS-26.4-headers/blob/main/headers/SkyLight/SLSBridgedMoveWindowsToManagedSpaceOperation.h)) | **enabled OK** |

Two invocation styles for the bridged operation:
- yabai: alloc/init, then call non-exported `SLSPerformAsynchronousBridgedWindowManagementOperation` resolved by parsing the SkyLight Mach-O ([space_manager.c](https://github.com/asmvik/yabai/blob/master/src/space_manager.c) ~665–705).
- DockDoor/WindowKit: call the operation's own `performWithWMBridgeDelegate` selector via the ObjC runtime — no Mach-O parsing ([PrivateApis.swift](https://github.com/ejbills/DockDoor/blob/main/DockDoor/Utilities/PrivateApis.swift)).

Caveats: asynchronous (focus races the move — issue #2788); user-space → user-space only (no fullscreen/tiled); the class symbol exists in SDKs back to 14.0 but SIP-enabled *function* is only validated on Tahoe 26.4+ — Sequoia 15 behavior unverified; one month old, Apple could close it. Graceful-degradation example: [y3owk1n/mimi space.m](https://github.com/y3owk1n/mimi/blob/main/internal/native/space.m). Hammerspoon has not adopted it as of June 2026.

### Switching the current space

| Technique | Animation | SIP | Status |
|---|---|---|---|
| Synthetic ctrl+arrow / ctrl+number, session-level source | full slide animation | enabled | **Ignored by Mission Control** — verified dead on 26.5.1 (Hammerspoon `keyStroke` etc.) |
| **Synthetic ctrl+arrow from `hidSystemState` source posted to `kCGHIDEventTap`** | native slide | enabled, Accessibility only | **Verified working on macOS 26.5.1 (2026-06-12)** — the MacGesture bypass; shipped in spaceswitch |
| Focus a window on the target space (yabai SIP-on mode) | animated; cannot reach empty spaces | enabled | yabai v7.1.19+ has `space --focus` SIP-enabled on Tahoe |
| Dock AX Mission Control automation (`hs.spaces.gotoSpace`) | opens Mission Control UI — visible flashing | enabled | Mitigate with Reduce Motion; slow |
| **Synthetic high-velocity dock-swipe gesture** (InstantSpaceSwitcher) | **none — animation skipped** | enabled, Accessibility only | Works on Tahoe 26; see §2 |
| `CGSManagedDisplaySetCurrentSpace`-class via Dock scripting-addition injection (yabai SA) | instant; can also disable animations system-wide | **partial disable required** | Works but fragile; see §4 |

Refuted claims (do not rely on): "macOS 14.5 added the SIP requirement to the move call itself" (0–3 — the mechanism framing was wrong); "AeroSpace's emulated switching is glitch-free" (0–3 — its hide/show has its own artifacts); "Tahoe SA support fully restored via PR #2644, tracking issue closed 2025-10-03" (0–3).

## 2. Goal 1 — animation-free switching from mouse tilt buttons

### Switching, validated outcome: HID-tap Ctrl+Arrow

What actually shipped (spaceswitch, this repo): post Ctrl+Arrow built from `CGEventSourceCreate(kCGEventSourceStateHIDSystemState)` to `CGEventPost(kCGHIDEventTap, …)` with flags `maskControl|maskSecondaryFn`. Verified switching both directions on macOS 26.5.1 (2026-06-12). Native slide animation, no flash, no SIP changes, multi-display and Mission Control shortcut semantics handled natively by macOS. The same keystroke posted from a session-level source is silently ignored by Mission Control (verified), which is why Hammerspoon/Karabiner-level approaches fail. Prior art for the bypass: MacGesture's fix for this exact failure.

### Switching, zero-animation alternative: synthetic dock-swipe gesture

If no animation at all is wanted (this report's pre-test recommendation, superseded for this project by the preference for the native slide): [InstantSpaceSwitcher](https://github.com/jurplel/InstantSpaceSwitcher) (verified 3-0 across three claim groups) synthesizes a trackpad Dock-swipe gesture CGEvent with artificially high velocity (`kCGEventGestureSwipeVelocityX/Y = 2000.0 × steps`) and near-zero progress, posted via `CGEventPost`. The system performs a real native space switch but skips the slide animation.

- No SIP changes; sole permission is Accessibility (TCC). Only private symbols are read-only introspection (`CGSMainConnectionID`, `CGSGetActiveSpace`, `CGSCopyManagedDisplaySpaces`).
- Current: v2.0 released 2026-04-18 built on macOS 26; nightlies through 2026-06-10; two independent reimplementations ([writeup](https://arhan.sh/blog/native-instant-space-switching-on-macos)).
- Known issues: occasional invisible windows ([#58](https://github.com/jurplel/InstantSpaceSwitcher/issues/58)), limited multi-display support ([#43](https://github.com/jurplel/InstantSpaceSwitcher/issues/43)), and **[#72](https://github.com/jurplel/InstantSpaceSwitcher/issues/72) (2026-06-09): the gesture approach fails on the macOS 27.0 beta** — the main forward risk.
- Relative left/right moves; targeting a specific space index is an open question.

Alternative if a partial SIP disable is acceptable: yabai's scripting addition gives true instant switching plus system-wide animation disabling (`csrutil enable --without fs --without debug --without nvram` + `-arm64e_preview_abi` boot-arg). See §4 fragility.

### Capturing the tilt wheel

Tilt clicks are **not button events**. The mouse reports HID Consumer page "AC Pan" (0x0C/0x0238); macOS delivers `kCGEventScrollWheel` with discrete axis-2 delta (`DeltaAxis2 = ±1`, `FixedPtDeltaAxis2 = ±0.1` per notch, `IsContinuous = 0`). Held tilt auto-repeats identical events; there is no down/up pair.

| Tool | Tilt → arbitrary action | Notes |
|---|---|---|
| Karabiner-Elements | **No** | Cannot use any scroll event as a from-event; open since 2018 ([#1362](https://github.com/pqrs-org/Karabiner-Elements/issues/1362), [#2834](https://github.com/pqrs-org/Karabiner-Elements/issues/2834) closed not-planned). Dead end |
| BetterTouchTool | **Yes — best off-the-shelf** | (a) Logitech HID++ direct support records tilt as button presses → any action incl. shell script ([docs](https://docs.folivora.ai/docs/normal-mouse/logitech/)); requires Logi Options+/G-Hub uninstalled. (b) Generic tilt trigger with configurable repeat delay 0.05–2 s. Bug: shift+vertical-scroll misdetected as tilt ([forum](https://community.folivora.ai/t/shift-scroll-triggers-tilt-wheel-button-actions-instead-of-scrolling-horizontally/5289)) |
| SteerMouse | Yes | Dedicated Tilt Wheel tab → keyboard shortcut / open app; no direct shell action (tilt tab partially verified — official site 403'd) |
| USB Overdrive | Yes (partially verified) | Tilt as "Pan Up/Down" → keystroke; Tahoe compatibility unverified |
| BetterMouse | Yes (vendor-claimed) | Closed source; changelog mentions wheel-tilt handling |
| Mac Mouse Fix | No | Open requests [#272](https://github.com/noah-nuebling/mac-mouse-fix/issues/272), #928, #1302 |
| Mos | Partial | v4 beta added event binding; tilt mapping unreliable ([discussion #849](https://github.com/Caldis/Mos/discussions/849)) |
| Logi Options+ | Interferes | Can remap tilt to its own actions only; blocks BTT's HID++ path; by default passes tilt through as horizontal scroll |

DIY (most robust): active `CGEventTap` on `kCGEventScrollWheel`; classify `IsContinuous == 0 && DeltaAxis2 != 0 && DeltaAxis1 == 0`; return NULL to consume (prevents the app under the cursor from horizontally scrolling); debounce ~150–300 ms (fire on first event, suppress same-direction until quiet gap); ignore shift-flagged events to dodge the shift+scroll false positive (or match raw AC Pan via IOHIDManager to be immune). Permissions: active/filtering tap → Accessibility; listen-only → Input Monitoring. SensibleSideButtons is the architectural template (consume in tap, post synthetic gesture — [repo](https://github.com/archagon/sensible-side-buttons)). Tag self-posted events via an event user-data field so your own tap ignores them. Gotcha: re-signing the binary during development can silently disable the tap.

Accepted tradeoff: consuming tilt sacrifices tilt-driven horizontal scrolling in apps (shift+wheel still works if your detector excludes it).

## 3. Goal 2 — save and restore window-to-space mappings

### Why native restore fails

- The Dock's "Assign To" is stored in `~/Library/Preferences/com.apple.spaces.plist` under `app-bindings` (bundle ID → `"AllSpaces"` or a space UUID). It is **app-level, not window-level**; macOS has no native per-window space persistence.
- Display sleep/disconnect merges spaces onto the remaining display and dumps windows; on Apple Silicon the monitor de-registers momentarily during sleep ([Apple threads](https://discussions.apple.com/thread/253803495), [253718328](https://discussions.apple.com/thread/253718328)).
- "Automatically rearrange Spaces based on most recent use" reshuffles indices — disable it.
- `killall Dock` reloads the plist; the spaces themselves are owned by WindowServer and survive.

### Recommended architecture (all SIP-enabled on Tahoe 26.4+)

1. **Save**: `SLSCopyManagedDisplaySpaces` (display UUID → spaces with id64 + uuid + index) + `SLSCopySpacesForWindows` + `CGWindowListCopyWindowInfo`. Persist per window: app bundle ID, title (+ tab titles for browsers/terminals), frame, display UUID, **space uuid + Mission Control index** — not raw `ManagedSpaceID` (reboot stability of id64 unverified; UUIDs are the durable identity the Dock itself uses in app-bindings; first desktop historically has an empty UUID).
2. **Restore**: re-resolve uuid → current space ID via `SLSCopyManagedDisplaySpaces`; recreate missing spaces via Dock AX (`hs.spaces.addSpaceToScreen` technique); match windows heuristically (CGWindowIDs are session-scoped — bundle ID + title + frame proximity scoring; restore-spaces' AppleScript tab-title enumeration with similarity threshold is the most developed prior art); move via `SLSBridgedMoveWindowsToManagedSpaceOperation`. Handle async completion before focusing.
3. Fallbacks: Dock AX Mission Control drag emulation ([Drag.spoon](https://github.com/mogenson/Drag.spoon), active 2025-12), or per-app `app-bindings` plist edits + Dock restart (unpredictable with multiple displays).

### Existing tools surveyed

| Tool | Restores space assignment? | Notes |
|---|---|---|
| [tplobo/restore-spaces](https://github.com/tplobo/restore-spaces) (Hammerspoon) | **Yes — only tool that tries** | Cycles spaces, records to JSON, title matching. **Broken on Sequoia** (hs.spaces.moveWindowToSpace dead); last commit 2025-03 |
| Stay (cordlessdog) | Partial | Frames within current space per display config; "cannot move windows between Spaces" (FAQ) |
| Moom | No | Frames only; one layout per space |
| Rectangle Pro | No | Dev: can't apply layouts to non-visible spaces, no public API ([discussion](https://github.com/rxhanson/RectanglePro-Community/discussions/573)) |
| Display Maid | No | Space-agnostic frame restore |
| Workspaces (Apptorium), Lasso | No | Project launcher / grid positioning |
| MacLayout | Claims spaces awareness — unverified marketing |
| [FlashSpace](https://github.com/wojciech-kulik/FlashSpace) | Sidesteps spaces | App→workspace bindings, hide/show per display; identity = bundle ID so inherently reboot-stable; per-app only |
| yabai 7.1.25+ | Building block | `yabai -m window --space N` SIP-enabled on Tahoe; scriptable with `yabai -m query --windows/--spaces` |
| [AeroSpace](https://github.com/nikitabobko/AeroSpace) | No — replaces spaces | Emulates workspaces by parking windows off-screen (~1px strip visible); public AX API + only `_AXUIElementGetWindow`; never requires SIP disable. Does not drive real Spaces; its switching is NOT glitch-free (claim refuted 0–3) |

## 4. The yabai/SIP path and its fragility

yabai injects a scripting addition into Dock.app, whose WindowServer connection is flagged as "universal owner" of all windows (SkyLight is the per-process mach-IPC interface to WindowServer; the connection gates which private operations are allowed). Requires partial SIP disable on Apple Silicon: `csrutil enable --without fs --without debug --without nvram` + `sudo nvram boot-args=-arm64e_preview_abi`. SIP-requiring features: move/swap/create/destroy space, animations/shadows/transparency, sticky windows, PiP. Space *focus* is not on the SIP list (but SIP-on focus is window-focus-based, animated, can't reach empty spaces — until v7.1.19's SIP-enabled `space --focus` on Tahoe).

Fragility record (verified 3-0): every major release and several point releases broke the SA until per-version byte-pattern updates shipped. Sequoia beta 1 broke injection (2024-06-10, fixed 06-24); beta 2 re-broke it ~1.5 h after the fix. Tahoe betas (June 2025) broke programmatic space switching specifically; community re-found the dock.spaces byte pattern at +0x200000. The technique fundamentally depends on per-OS-version byte-pattern scanning of the Dock binary, with contributors tracking per-beta Dock hashes in Ghidra ([#2634](https://github.com/koekeishiya/yabai/issues/2634), [#2324](https://github.com/koekeishiya/yabai/issues/2324)). Despite this, yabai is actively maintained through Tahoe 26.4 (v7.1.16–7.1.25; `space --destroy` reportedly failing silently on 26 per #2730).

## 5. Gotchas checklist

- Disable "automatically rearrange Spaces based on most recent use".
- "Displays have separate Spaces" affects everything: per-display current space, gesture targeting (ISS multi-display support is limited), and merge behavior on disconnect.
- Fullscreen/tiled apps are spaces of `type 4` — cannot be move targets; restore-spaces can't restore Split View.
- `com.apple.spaces.plist` updates lazily (on space create/delete, not on switch); edit + `killall Dock` to apply; spaces survive Dock restarts.
- CGWindowIDs do not survive reboot; window titles need Screen Recording permission to read.
- Shift+scroll is synthesized into horizontal scroll downstream of HID — indistinguishable at the CGEvent level from tilt (BTT's long-standing bug); filter on shift flags or match raw AC Pan HID usage.
- TCC: Accessibility for active event taps and AX; Input Monitoring for listen-only taps/IOHIDManager; Screen Recording for window titles. Re-signing during development can silently kill tap permissions.
- The bridged move operation is async — sequence move → then focus, or focus races the move.
- App Sandbox is incompatible with all of this (event taps, private APIs); distribute outside the Mac App Store, notarized with hardened runtime.

## 6. Future outlook

- **macOS 27 beta already breaks the synthetic-gesture switch** (ISS [#72](https://github.com/jurplel/InstantSpaceSwitcher/issues/72), 2026-06-09, open). Watch this before committing to the gesture path long-term.
- The SIP-enabled bridged move path is one month old (May 2026); Apple has repeatedly tightened these surfaces (14.5 move restriction, 15.0 compat-ID closure) and could close it in any point release. Resolve the class/selector dynamically and degrade gracefully (mimi's pattern).
- yabai's SA breaks on essentially every beta and point release; treat it as a maintenance subscription, not a dependency.
- No verified Apple signals about Spaces/Mission Control public APIs at WWDC 2025/2026; nothing found suggesting a public Spaces API is coming.
- Design implication: isolate the switch mechanism and the move mechanism behind interfaces with runtime capability detection and fallbacks (gesture → ctrl+arrow; bridged op → Dock AX drag → app-bindings plist).

## Refuted during verification (do not cite)

1. "Tahoe SA support was fully restored via PR #2644, tracking issue closed 2025-10-03" — 0-3.
2. "macOS 14.5 made SLS window moves require SIP disable, check added to the target window" — 0-3 as framed.
3. "AeroSpace's emulated switching is fast and glitch-free" — 0-3.
4. "No public API exists, therefore private APIs or input synthesis are the only options" — 1-2 (too absolute).

## Primary sources

- yabai: [repo](https://github.com/koekeishiya/yabai), [wiki](https://github.com/koekeishiya/yabai/wiki), issues [#2324](https://github.com/koekeishiya/yabai/issues/2324), [#2634](https://github.com/koekeishiya/yabai/issues/2634), [asmvik#2788](https://github.com/asmvik/yabai/issues/2788), [discussion #2274](https://github.com/koekeishiya/yabai/discussions/2274)
- [InstantSpaceSwitcher](https://github.com/jurplel/InstantSpaceSwitcher) + [arhan.sh writeup](https://arhan.sh/blog/native-instant-space-switching-on-macos)
- [AeroSpace](https://github.com/nikitabobko/AeroSpace) + [guide](https://nikitabobko.github.io/AeroSpace/guide)
- [Hammerspoon libspaces.m](https://github.com/Hammerspoon/hammerspoon/blob/master/extensions/spaces/libspaces.m), issues [#3698](https://github.com/Hammerspoon/hammerspoon/issues/3698)/[#3666](https://github.com/Hammerspoon/hammerspoon/issues/3666)
- [WindowKit](https://github.com/ejbills/WindowKit), [DockDoor PrivateApis.swift](https://github.com/ejbills/DockDoor/blob/main/DockDoor/Utilities/PrivateApis.swift), [mimi space.m](https://github.com/y3owk1n/mimi/blob/main/internal/native/space.m)
- [CGSInternal headers](https://github.com/NUIKit/CGSInternal), [macOS 26.4 SkyLight headers](https://github.com/thatmarcel/macOS-26.4-headers)
- [restore-spaces](https://github.com/tplobo/restore-spaces), [FlashSpace](https://github.com/wojciech-kulik/FlashSpace), [Stay FAQ](https://cordlessdog.com/stay/documentation/faq/), [Drag.spoon](https://github.com/mogenson/Drag.spoon)
- [com.apple.spaces.plist anatomy gist](https://gist.github.com/0xdevalias/8bc497546d5f036cbaeae5d0e389aa35), [ianyh: Identifying Spaces](https://ianyh.com/blog/identifying-spaces-in-mac-os-x/)
- [BTT Logitech docs](https://docs.folivora.ai/docs/normal-mouse/logitech/), [Karabiner #1362](https://github.com/pqrs-org/Karabiner-Elements/issues/1362), [SensibleSideButtons](https://github.com/archagon/sensible-side-buttons), [scroll event field semantics](https://gist.github.com/svoisen/5215826)
