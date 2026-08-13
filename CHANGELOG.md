# Changelog

## Unreleased

- `UPDATE_BRIDGE.exe` now updates only from the latest stable GitHub Release instead of the development `main` branch.
- Stable releases used by the updater must provide the `Enhanced_PS5_DualSense_Haptics_Bridge.zip` release asset.
- Replaced the public manual updater entry point with `UPDATE_BRIDGE.exe`.
- The updater now uses a temporary copy of itself so it can safely update its own executable without a separate helper binary.
- Preserved V1.1 updater compatibility through repository-only legacy marker files.
- Replaced the two public BAT launchers with `START_BRIDGE.exe` and `START_BRIDGE_AND_BEAMNG.exe`.
- Both public launchers use one shared Go implementation; the launcher is built once and copied byte-for-byte under the second filename.
- Preserved the existing USB/Bluetooth detection and Bridge launch behavior.

## V1

- Added optional `UPDATE_BRIDGE.bat` manual updater with SHA-256 manifest verification and safe managed-file synchronization.
- Added a maintainer checksum generator for `SHA256SUMS.txt`.

- USB canonical reference is now transport-unfiltered and byte-stable at unity gain; Bluetooth-only anti-alias filtering no longer changes wired feel.

- Unified USB/Bluetooth Common Feel Engine.
- Canonical 48 kHz stereo haptic source with Bluetooth 3 kHz derivation.
- Removed per-material Bluetooth transport compensation.
- Added final-boundary USB/Bluetooth output gains.
- PeakForce-based spatial suspension impacts.
- USB HID 25 ms keep-alive independent from LEDs.
- Bluetooth RGB ownership delegated to BeamNG `Device.setRGB()`.
- R2 grounded baseline fixed at constant 1/8 resistance.
- R2 airborne state fixed at exact 1/255 fine feedback.
- Clean public source layout and English-only documentation/messages.
