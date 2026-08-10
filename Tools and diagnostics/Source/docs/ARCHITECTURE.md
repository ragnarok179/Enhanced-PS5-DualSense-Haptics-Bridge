# Architecture

## Data flow

```text
BeamNG.drive
    |
    | UDP telemetry (localhost)
    v
Telemetry decoder
    |
    v
Common Feel Engine
    |-- adaptive-trigger state
    |-- RPM / LED state
    |-- road surface model
    |-- suspension / collision / landing events
    |-- stereo haptic mixer
    v
Canonical 48 kHz stereo PCM
    |
    +-----------------------------+
    |                             |
    v                             v
USB adapter                  Bluetooth adapter
48 kHz PCM                   anti-alias + 16:1 decimation
WASAPI haptic endpoint       3 kHz signed stereo int8
HID state + 25 ms keepalive  report 0x36 + HID state
```

## Source responsibilities

- `telemetry.go`: transport-neutral BeamNG telemetry schema and decoding.
- `common_feel_engine.go`: one orchestration layer shared by USB and Bluetooth.
- `gameplay_effects.go`: gameplay interpretation, triggers, surfaces and events.
- `haptic_mixer.go`: stereo waveform generation and event mixing.
- `haptic_pipeline.go`: final transport boundary; byte-stable USB reference output,
  Bluetooth-only anti-alias filtering, transport gains and 3 kHz derivation.
- `dualsense_reports.go`: controller-state and adaptive-trigger report encoding.
- `bluetooth_audio_packets.go`: Bluetooth packet framing only.
- `bluetooth_main_windows.go`: Bluetooth runtime / CLI.
- `usb_main_windows.go`: USB runtime / CLI.
- `usb_audio_shared_windows.go`: GameInput/WASAPI endpoint discovery and PCM write.
- `usb_hid_windows.go`: USB HID output and keep-alive.
- `hid_windows.go`: Bluetooth HID discovery/read/write.
- `feel_profile.go`: V1 configuration loading and validated defaults.

## Invariants

1. Gameplay interpretation must never branch on USB vs Bluetooth.
2. LEDs and adaptive triggers never pass through the PCM pipeline.
3. Bluetooth material/effect-specific gain compensation is forbidden.
4. USB must remain the unfiltered canonical reference at unity gain; Bluetooth-only anti-alias filtering stays inside the wireless adapter.
5. Transport calibration is applied only at the final PCM boundary.
6. USB HID keep-alive must remain independent from LED activity.
7. Bluetooth runtime RGB ownership remains with the BeamNG `Device.setRGB()` writer.
