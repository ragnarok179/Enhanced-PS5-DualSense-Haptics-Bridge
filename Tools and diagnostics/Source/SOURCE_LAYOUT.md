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
