# TODO

## spacekeeper — save/restore window→space mappings (goal 2)

- `save`: enumerate `SLSCopyManagedDisplaySpaces` + `SLSCopySpacesForWindows` + `CGWindowListCopyWindowInfo` → JSON of {space uuid, display UUID, Mission Control index, bundle ID, window title, frame}. Persist space UUIDs, not ManagedSpaceIDs.
- `restore`: re-resolve uuid → current space ID; match windows by bundle ID + title + frame scoring (matcher as pure function, unit-tested); move via `SLSBridgedMoveWindowsToManagedSpaceOperation` (`performWithWMBridgeDelegate` via ObjC runtime, resolved dynamically, clear error if Apple closes it). Move first, focus after (op is async).
- v1 scope cuts: no recreating missing spaces; skip fullscreen/tiled (type 4) windows.
- Window titles need Screen Recording permission.
- Reference code: y3owk1n/mimi `internal/native/space.m` (Go + cgo ObjC), ejbills/WindowKit `SkyLightSpace.swift`.
- launchd login invocation only after manual save/restore proves reliable.

## spaceswitch hardening

- Stable self-signed code-signing identity so rebuilds stop invalidating the Accessibility grant.
- Uninstall Hammerspoon after the daemon has run clean for a few days.

## jog wheel hardware

- On parts arrival: check cheap USB knob in Karabiner-EventViewer (VID/PID, emitted events, QMK/VIA reflashable?); assess LPD3806-class encoder bearing drag with the milled aluminum knob mounted.
- If coast disappoints: dedicated spindle (two 608 bearings, printed housing, shoulder-bolt shaft) + MT6701 in ABZ mode.
- RP2040 firmware: PIO quadrature decode, velocity-thresholded J-K-L with ramp, sub-threshold deadband.
- Optional adjustable eddy brake: aluminum disc on shaft collar, magnet on printed thumbscrew bracket.
