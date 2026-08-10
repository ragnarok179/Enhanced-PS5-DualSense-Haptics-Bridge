# Enhanced PS5 DualSense Haptics Bridge

Physics-driven DualSense haptics and adaptive triggers for **BeamNG.drive**, with direct **USB and Bluetooth** support on Windows.

This bridge works with the separate **Enhanced PS5 DualSense Haptics** BeamNG mod. It receives local BeamNG telemetry and converts it into stereo haptics, adaptive-trigger states and controller feedback without requiring DSX.

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

`START_BRIDGE.bat` probes USB first and then Bluetooth, and starts the matching bridge automatically.

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

USB is the canonical reference path. Bluetooth is derived from the same gameplay signal and uses only a final transport-level gain; there are no separate per-surface Bluetooth tuning tables.

## Adaptive triggers

### R2 — throttle

- Normal grounded driving: constant `1/8` resistance across the full trigger travel.
- Airborne vehicle: exact `1/255` fine-feedback state.
- No progressive `1/8 -> 2/8` increase at full throttle.
- Wheelspin, TCS, rev limiter and shift events can temporarily use dedicated states.

### L2 — brake

L2 includes brake resistance and ABS pulse behavior driven by BeamNG telemetry.

## Repository layout

```text
START_BRIDGE.bat
START_BRIDGE_AND_BEAMNG.bat
README.md
LICENSE
CHANGELOG.md
THIRD_PARTY_NOTICES.md
SHA256SUMS.txt
Tools and diagnostics/
  Bridge/
    EnhancedPS5DualSenseHapticsBluetooth.exe
    EnhancedPS5DualSenseHapticsUSB.exe
  Config/
    feel_profile_v1.json
  Diagnostics/
    LIST_HARDWARE.bat
    LIST_AUDIO_OUTPUTS.bat
    SHOW_FEEL_PROFILE.bat
    START_USB_DIAGNOSTIC_LOG.bat
    START_BLUETOOTH_DIAGNOSTIC_LOG.bat
    TEST_USB_STEREO.bat
    TEST_BLUETOOTH_STEREO.bat
    TEST_BLUETOOTH_BUMP_CARRIER.bat
    Logs/
  Source/
    *.go
    go.mod
    SOURCE_LAYOUT.md
    docs/
    scripts/
```

## Diagnostics

If the bridge cannot find the controller, start with:

```text
Tools and diagnostics\Diagnostics\LIST_HARDWARE.bat
```

Useful transport-only tests:

```text
TEST_USB_STEREO.bat
TEST_BLUETOOTH_STEREO.bat
TEST_BLUETOOTH_BUMP_CARRIER.bat
```

Diagnostic bridge logs are written to:

```text
Tools and diagnostics\Diagnostics\Logs\
```

## Building from source

Go 1.23+ is recommended.

From:

```text
Tools and diagnostics\Source\
```

run:

```text
BUILD_WINDOWS.bat
```

or execute the PowerShell build script directly.

The public executables are built with trimmed source paths. GitHub Actions also runs the test/build pipeline on pushes and pull requests.

## Configuration

Transport and feel settings are stored in:

```text
Tools and diagnostics\Config\feel_profile_v1.json
```

USB/Bluetooth strength compensation is intentionally global and applied only at the final transport boundary. Avoid changing individual surface or collision gains to compensate for a transport-level strength difference.

## BeamNG protocol

The BeamNG mod sends local UDP telemetry to the bridge. The bridge does not modify BeamNG game files and the controller transport is handled outside the game process.

Protocol and architecture documentation is available under:

```text
Tools and diagnostics\Source\docs\
```

## License

MIT. See `LICENSE` and `THIRD_PARTY_NOTICES.md`.

## Disclaimer

This is a community project and is not affiliated with or endorsed by BeamNG GmbH or Sony Interactive Entertainment. DualSense is a trademark of Sony Interactive Entertainment.
