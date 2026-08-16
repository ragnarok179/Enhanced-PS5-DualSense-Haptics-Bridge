# Changelog

## V1.3

- Unified and optimized Common Feel runtime for USB and Bluetooth.
- Force48 trigger model with hardware-specific quantization only at the HID boundary.
- Compatibility-aware updater: local protocol check first, GitHub access only after explicit user approval/manual update.
- Compatibility index is read from stable GitHub Release assets, never from the development `main` branch.
- Preserved V1.1 migration support and V1.2 EXE-updater upgrade support.
- Bluetooth haptic calibration set to 0.80 relative to USB 1.00 after physical A/B testing.
- Restored BeamNG `Device.setRGB()` as the sole runtime LED writer for stable USB/Bluetooth switching.
- Fixed suspension-bump event finalization after the runtime optimization pass.
- Simplified diagnostics and removed redundant/development-only tools.
- Simplified BeamNG settings UI; independent L2/R2 start/end resistance controls and integer percentage display.
- Added a staged V1.0/V1.2 -> V1.1/V1.3 migration path: Bridge V1.3 still consumes real protocol-v40 Mod V1.0 telemetry, while Mod V1.1 can publish a marked v40 mirror for Bridge V1.2 without causing duplicate effects in V1.3.

## V1.2

- Replaced public BAT launchers with EXE launchers.
- Added `UPDATE_BRIDGE.exe` using stable GitHub Release packages with verification and rollback.
- Preserved migration support for V1.1 users.

## V1.1

- Added the optional legacy `UPDATE_BRIDGE.bat` / PowerShell updater.

## V1

- Initial public release.
