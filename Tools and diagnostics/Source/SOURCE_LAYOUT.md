# Source layout

The project intentionally uses one Go `main` package for both Windows transports.
USB and Bluetooth share telemetry decoding, gameplay logic, trigger logic, the
Common Feel Engine, haptic synthesis and regression tests. Only the final
transport adapters are build-tag specific.

Key files:

- `telemetry.go`: BeamNG UDP protocol and decoding.
- `common_feel_engine.go`: transport-neutral state orchestration.
- `gameplay_effects.go`: surfaces, ABS/TCS, throttle logic and gameplay cues.
- `haptic_mixer.go`: stereo synthesis, event voices and spatial mixing.
- `haptic_pipeline.go`: canonical 48 kHz PCM and Bluetooth 48 kHz -> 3 kHz adapter.
- `dualsense_reports.go`: shared DualSense trigger/state report construction.
- `bluetooth_audio_packets.go`: Bluetooth haptic packet framing.
- `bluetooth_main_windows.go`: Bluetooth runtime.
- `usb_audio_shared_windows.go`: Windows WASAPI endpoint/output layer.
- `usb_hid_windows.go`: USB HID device/report transport and keep-alive.
- `usb_main_windows.go`: USB runtime.

Keeping shared behavior in one package prevents the two transports from silently
drifting apart while still keeping platform-specific I/O isolated in clearly
named files.

## Public launcher

The two root launchers are produced from one implementation in `launcher/main.go`.
`START_BRIDGE.exe` is built once, then copied byte-for-byte to
`START_BRIDGE_AND_BEAMNG.exe`. At runtime the launcher reads its own filename to
select whether it should start BeamNG.drive before detecting USB/Bluetooth and
starting the corresponding Bridge executable. This keeps the launcher logic in
one place.

## Public updater

`updater/main.go` builds the root `UPDATE_BRIDGE.exe`. The updater checks GitHub's latest stable Release and downloads the release asset named exactly `Enhanced_PS5_DualSense_Haptics_Bridge.zip`; it never synchronizes normal users against the development `main` branch. The downloaded package is verified against its `SHA256SUMS.txt` manifest before installation.

The updater copies itself to a temporary folder before applying an update, so the installed updater executable can be replaced safely without a second helper binary.

The GitHub repository keeps the old `UPDATE_BRIDGE.bat` and `Tools and diagnostics/Updater/Update-Bridge.ps1` only for V1.1 migration compatibility. The legacy V1.1 updater still uses `main` because that behavior is already shipped and cannot be changed retroactively; after migration, `UPDATE_BRIDGE.exe` uses stable Releases only. The EXE updater treats those scripts as compatibility-only files and removes them from modern installations.

## Updater manifest

`UPDATE_BRIDGE.exe` uses the repository `SHA256SUMS.txt` as the list of files managed by the public updater. After changing, adding or removing distributed files, run:

```text
GENERATE_SHA256SUMS.bat
```

from this `Source` folder before pushing. CI verifies that the manifest matches the repository contents.
