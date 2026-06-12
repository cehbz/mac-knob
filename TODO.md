# TODO

## spacekeeper

- Run manually for a while; wire a launchd login invocation once it proves reliable.
- Recreate missing spaces on restore (Dock AX automation, the flashy part — deferred from v1).
- Consider restoring window frames as well as spaces.

## spaceswitch hardening — durable signing (mechanism built, needs your secret)

- `op signin`, then `./scripts/gen-signing-cert.sh`, run the printed `op item create` to store the cert in 1Password, `rm` the local .p12.
- `make install` (signs from 1Password via a throwaway keychain), then remove ALL stale `spaceswitch`/`spacekeeper` rows from the Accessibility list and grant once.
- Reload the daemon: `launchctl kickstart -k gui/$UID/bz.ceh.spaceswitch`.

## jog wheel hardware

- On parts arrival: check cheap USB knob in Karabiner-EventViewer (VID/PID, emitted events, QMK/VIA reflashable?); assess LPD3806-class encoder bearing drag with the milled aluminum knob mounted.
- If coast disappoints: dedicated spindle (two 608 bearings, printed housing, shoulder-bolt shaft) + MT6701 in ABZ mode.
- RP2040 firmware: PIO quadrature decode, velocity-thresholded J-K-L with ramp, sub-threshold deadband.
- Optional adjustable eddy brake: aluminum disc on shaft collar, magnet on printed thumbscrew bracket.
