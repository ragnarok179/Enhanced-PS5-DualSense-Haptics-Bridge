# Changelog

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
