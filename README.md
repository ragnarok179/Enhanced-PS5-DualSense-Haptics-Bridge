# Enhanced PS5 DualSense Haptics Bridge

Physics-driven DualSense haptics and adaptive triggers for **BeamNG.drive**, with direct **USB and Bluetooth** support on Windows.

This bridge works with the separate **Enhanced PS5 DualSense Haptics** BeamNG mod. It receives local BeamNG telemetry and converts it into stereo haptics, adaptive-trigger states and controller feedback without requiring DSX or other app.

## Features

- Physics-driven stereo haptics for road surfaces.
- Left/right suspension impacts based on per-wheel BeamNG impact data.
- Collision and landing feedback with strength-dependent signatures.
- Wheelspin, TCS, rev-limiter and shift feedback.
- Adaptive L2 brake feedback with ABS pulses.
- Adaptive R2 throttle feedback.
- RPM-reactive DualSense lightbar behavior.
- Direct USB and Bluetooth transport.
- One shared haptic gameplay engine for both transports.
- Hardware probes, stereo tests and diagnostic log tools.
- Full Go source code and reproducible Windows build scripts.

## Requirements

- Windows.
- BeamNG.drive.
- Sony DualSense controller.
- The separate `Enhanced_PS5_DualSense_Haptics.zip` BeamNG mod.

## Quick start

1. Install the BeamNG mod ZIP in your BeamNG mods folder or through the BeamNG Repository.
2. Download this repository or the latest GitHub Release.
3. Connect the DualSense over USB or Bluetooth.
4. Run `START_BRIDGE.bat`.
5. Start BeamNG.drive.

You can also use:

```text
START_BRIDGE_AND_BEAMNG.bat
```

to launch BeamNG through Steam and start the bridge.

## Important controller settings

Disable BeamNG's native gamepad vibration while using this bridge. Running two haptic writers at the same time can produce duplicated or conflicting feedback.

If you encounter controller conflicts, close other software that actively writes DualSense haptics or LED state, such as DSX/DS4Windows features or another DualSense bridge.

## Haptic architecture

The gameplay interpretation is shared between USB and Bluetooth:

```text
BeamNG telemetry
      |
      v
Common Feel Engine
      |
      v
Canonical 48 kHz stereo haptics
      |                         |
      v                         v
USB / WASAPI              Bluetooth adapter
48 kHz grips              anti-alias + 48k -> 3k
                                |
                                v
                          DualSense 0x36 packets
```

USB is the canonical reference path. Bluetooth is derived from the same gameplay signal and uses only a final transport-level gain.


## License

MIT. See `LICENSE` and `THIRD_PARTY_NOTICES.md`.
