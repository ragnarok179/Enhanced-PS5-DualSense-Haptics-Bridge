# Enhanced PS5 DualSense Bridge V1.4

Windows companion for the **Enhanced PS5 DualSense Haptics** BeamNG.drive mod.
It provides controller-side stereo haptics, adaptive triggers, extended inputs,
motion sensors and collision-only controller-speaker audio.

## Compatibility

Bridge V1.4 is backward-compatible with the released BeamNG mod generations and
introduces protocol 42 for Mod V1.2:

| BeamNG mod | Gameplay protocol | Bridge V1.4 |
| --- | ---: | --- |
| V1.0 | 40 | Supported |
| V1.1 | 41 | Supported |
| V1.2 | 42 | Supported |

Mod V1.2 intentionally does **not** publish an older gameplay mirror. It requires
Bridge V1.4. Therefore an installed Bridge V1.3 (40/41 only) sees protocol 42 as
unsupported and starts its compatibility-update flow instead of silently running
with an incomplete feature set.

This also makes release ordering safe: Bridge V1.4 may be published first because
it still accepts Mod V1.0/protocol 40 and Mod V1.1/protocol 41. When Mod V1.2 is
published later, the same Bridge already accepts protocol 42.

## Install

1. Install the BeamNG mod ZIP separately.
2. Extract the complete Bridge ZIP to a writable folder.
3. Run `START_BRIDGE.exe`.
4. Optional: `START_BRIDGE_AND_BEAMNG.exe` starts BeamNG through Steam first.

USB is selected automatically when a wired DualSense is present; otherwise the
launcher checks Bluetooth.

## Normal use

The normal terminal deliberately shows only useful state: Bridge version,
controller/transport, waiting for BeamNG, successful BeamNG connection,
compatibility/update errors and important device failures.

All user settings are managed from the unified **Enhanced PS5 DualSense Settings**
menu inside BeamNG.drive. Detailed runtime telemetry is available only through
explicit tools in `Diagnostics/`.

## Touchpad and motion inputs

The Bridge exposes the extra DualSense controls to BeamNG through one virtual device named **DualSense Extended Inputs**. The physical DualSense remains the normal game controller; the virtual device is only for the extra touch/motion channels.

### Touchpad

The touchpad can work in two ways:

- **Mouse mode**: the touch surface controls the Windows cursor. BeamNG binding capture takes priority automatically, so using the touchpad to assign a control does not also click the desktop.
- **BeamNG input mode**: one-finger and two-finger horizontal/vertical channels can each be set to **Off**, **Swipe**, **Relative** or **Absolute**, then bound like other BeamNG inputs.

Mouse mode is toggled from the mod UI; the old two-finger mouse shortcut is not used.

### Motion sensors

Three configurable motion slots can use the DualSense gyroscope/orientation as **Rotation angle**, **Rotation speed**, **Tilt** or **Orientation**. Each slot can output either an axis or a button. Advanced options include axis selection, inversion, calibration/centering, deadzone, stability, range, button threshold/direction and hold/pulse behavior.

The accelerometer X/Y/Z channels can also be exposed as axes or buttons, with configurable direction, threshold/range and pulse/hold behavior. These options can be used for custom steering, camera, head-look or other BeamNG bindings.

### Bluetooth sleep

The Inputs page also contains the Bluetooth idle timeout. Controller movement can optionally reset the sleep timer.

## Controller speaker

The public profile mirrors **collision sounds only**. Speaker audio is created
only after BeamNG itself emits the corresponding collision/break one-shot.

USB and Bluetooth now use the same PCM voice model: independent BeamNG one-shots
can overlap naturally and a BeamNG reset/reload immediately flushes the affected
vehicle's active voices. Bluetooth no longer serializes complete collision cues in
a FIFO.

USB reads compatible collision samples from the user's local BeamNG sound banks.
Bluetooth mixes the same local PCM voices, encodes the mixed stream to Opus at
runtime and sends it through the single `0x36` writer.

Speaker output routing is owned by the BeamNG mod: **Controller only** suppresses
only the matching BeamNG collision one-shot on the system output, while
**Controller + PC** leaves BeamNG's original system sound untouched. The Bridge
never creates an additional PC copy, so there is no duplicate system collision audio.

Bluetooth speaker calibration is applied at the **controller speaker output
level**, not to BeamNG collision strength or event gain. USB is the reference
output; Bluetooth uses speaker level `0x50` (80 on the DualSense practical
0–100 speaker scale) in both setup and continuous `0x36` state.

No BeamNG audio asset or FFmpeg binary is redistributed.

## Bluetooth ownership

Bluetooth has one production HID writer: report `0x36`, carrying haptics,
adaptive-trigger state and speaker frames. BeamNG owns DualSense RGB/RPM lighting;
the Bridge does not run a competing RGB ownership path.

## Diagnostics

`Diagnostics/` contains opt-in support tools for hardware/audio listing,
USB/Bluetooth logs, stereo tests, extended inputs, touchpad/bindings and motion.
Normal launchers do not enable verbose diagnostic output.

## Updates

`UPDATE_BRIDGE.exe` reads `BRIDGE_COMPATIBILITY.json` from published GitHub
Releases, then selects the newest stable Bridge that explicitly supports the
protocol requested by the running mod. It never updates from the development
branch and never downgrades the installed Bridge.

If a mod protocol is newer than the installed Bridge supports, the runtime writes
a pending compatibility request and can launch `UPDATE_BRIDGE.exe`. Managed files
are verified against `SHA256SUMS.txt`; user logs and unmanaged local files are
preserved.

## Repository vs runtime ZIP

The GitHub repository contains `Source/`, tests, documentation and release
scripts. The user-facing Bridge ZIP intentionally excludes all source/build files
and contains only runtime files, configuration, support diagnostics and legal
notices.

## Requirements

- Windows 10/11;
- BeamNG.drive;
- Sony DualSense over USB or Bluetooth.

## License

Project code is distributed under the MIT License. See `THIRD_PARTY_NOTICES.md`
for protocol-reference and runtime-interoperability notices.
