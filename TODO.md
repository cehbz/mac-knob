# TODO

## spacekeeper

- Run the opt-in login agent for a while (`make restore-agent-install`); promote to default / tune the settle delay once it proves reliable.
- Multi-display space creation assumes SLS display order matches the Mission Control AX `mc.display` order. Held on this 2-display setup; revisit if it ever creates desktops on the wrong display.

## jog wheel hardware

- On parts arrival: check cheap USB knob in Karabiner-EventViewer (VID/PID, emitted events, QMK/VIA reflashable?); assess LPD3806-class encoder bearing drag with the milled aluminum knob mounted.
- If coast disappoints: dedicated spindle (two 608 bearings, printed housing, shoulder-bolt shaft) + MT6701 in ABZ mode.
- RP2040 firmware: PIO quadrature decode, velocity-thresholded J-K-L with ramp, sub-threshold deadband.
- Optional adjustable eddy brake: aluminum disc on shaft collar, magnet on printed thumbscrew bracket.
