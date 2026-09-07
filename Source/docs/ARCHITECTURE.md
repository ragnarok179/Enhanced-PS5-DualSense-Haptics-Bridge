# Architecture

## Data flow

```text
Supported legacy BeamNG Mods V1.3 / V1.4, then V1.41+
    |
    | localhost gameplay telemetry (protocol 41 / 42 / 43)
    v
Supported telemetry decoder
    v
Common Feel Engine
    |-- adaptive triggers
    |-- road surface model
    |-- suspension / collision / landing haptics
    |-- stereo mixer
    v
Canonical 48 kHz stereo PCM
    |
    +-----------------------------+
    |                             |
    v                             v
USB adapter                  Bluetooth adapter
WASAPI 48 kHz               anti-alias + 3 kHz haptics
USB HID state               single 0x36 HID writer
                              + triggers + speaker

Exact BeamNG collision one-shot
    v
local BeamNG sample decode (48 kHz PCM)
    v
shared overlapping speaker voices
    |                         |
    v                         v
USB WASAPI speaker       Bluetooth 512-sample mix tick
                         -> 480-sample Opus frame
                         -> 0x36 speaker sub-packet
```

BeamNG owns DualSense RGB/RPM lighting on both transports.

## Update routing

- Supported legacy migration starts at **V1.3**.
- Legacy **V1.3** and **V1.4-era** updaters can select **Bridge V1.42 directly** through `BRIDGE_COMPATIBILITY.json`.
- Once Bridge V1.42 is installed, `UPDATE_DUALSENSE.exe` downloads the matching BeamNG mod ZIP and guides the manual BeamNG installation.
- `V1.41 -> V1.42` also uses `UPDATE_DUALSENSE.exe`.

## Source responsibilities

- `protocol.go`: supported gameplay compatibility generations (41-43).
- `telemetry.go`: protocol 41/42/43 decoding and normalization.
- `protocol_compatibility.go`: runtime update guard.
- `common_feel_engine.go`: transport-neutral orchestration.
- `gameplay_effects.go`: surfaces, ABS/TCS, throttle logic and gameplay cues.
- `haptic_mixer.go`: stereo synthesis and event mixing.
- `haptic_pipeline.go`: canonical PCM transport boundary.
- `dualsense_reports.go`: trigger/state encoding and hardware output fields.
- `controller_speaker_windows.go`: shared USB/Bluetooth collision voice mixer.
- `beamng_sound_events_windows.go`: exact collision/reset event parsing.
- `native_beamng_samples.go`: local BeamNG collision sample extraction/decoding.
- `bluetooth_opus_ffmpeg_windows.go`: persistent runtime Opus stream encoder.
- `bluetooth_audio_packets.go`: production Bluetooth `0x36` framing.
- `bluetooth_main_windows.go`: Bluetooth runtime and single HID writer.
- `usb_main_windows.go`: USB runtime.

## Invariants

1. Gameplay interpretation does not branch on USB vs Bluetooth.
2. USB and Bluetooth start from the same Common Feel state.
3. USB is the canonical 48 kHz haptic reference path.
4. Bluetooth has exactly one runtime HID writer: report `0x36`.
5. BeamNG owns RGB/RPM lighting; the Bridge does not implement a competing RGB writer.
6. Speaker audio is collision-only and starts only from exact BeamNG sound calls.
7. USB and Bluetooth speaker collisions are independent voices that can overlap.
8. Reset/reload packets flush active voices; there is no deferred collision FIFO.
9. Bluetooth output calibration is transport-level and does not alter BeamNG event strength.
10. BeamNG audio assets are read locally and never redistributed.
11. User-owned settings are never overwritten by the updater.
12. V1.3 is the oldest supported migration release. Protocols 41 and 42 remain only where needed for V1.3/V1.4 migration.
