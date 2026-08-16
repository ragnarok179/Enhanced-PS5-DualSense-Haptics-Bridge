# Architecture

## Runtime data flow

```text
BeamNG vehicle physics
        |
        | localhost UDP / protocol 41
        v
Telemetry decoder
        |
        | normalized vehicle state + semantic trigger intents
        v
Common Feel Engine
   |        |        |
   |        |        +--> trigger_effects.go --> triggerEffect
   |        |                               force = canonical 0..48
   |        +-----------> surface/body effects
   +--------------------> 48 kHz stereo haptic mixer
        |
        +-----------------------------+
        |                             |
        v                             v
USB output                       Bluetooth output
48 kHz WASAPI PCM                adapted haptic stream
DualSense HID                    DualSense 0x36/HID
        |                             |
        +--> final HID adapter <-------+
```

## Canonical representations

- Trigger intent on the current BeamNG wire: semantic objects with normalized `0.0..1.0` position/force/amplitude values.
- Trigger force inside the Bridge: one canonical `0..48` lattice (`triggerForce`). Fine Feedback is the highest-resolution trigger-force mode used by the project, so force is quantized once to that common lattice.
- Official Feedback / Official Vibration: the final HID adapter derives the controller's eight physical strength levels from `0..48` only while packing the report.
- Fine Feedback: the final HID adapter writes force `0..48` directly.
- Fine Feedback position: normalized logically, converted to the controller's real `0..255` position byte only at the HID boundary.
- Trigger frequency: Hz logically; converted to the required output byte at the HID boundary.
- Haptic synthesis: floating-point audio signal, converted to PCM only at the audio boundary.
- LED logic: logical color/brightness until RGB bytes are emitted by the LED writer.

`0..255` is therefore not a trigger-force unit in current runtime code. It remains only where the hardware quantity is genuinely byte-sized (for example Fine Feedback position, frequency/RGB output), in non-trigger haptic/settings storage, or inside isolated legacy migrations/tests.

## Responsibilities

- `telemetry.go`: current protocol schema/JSON decoding plus old settings fields required for migration.
- `protocol_compatibility.go`: local compatibility gate. It reads the packet envelope, accepts compiled protocol generations, and only launches the updater after explicit user approval.
- `internal/compatibility`: shared stable-release/protocol compatibility model used by runtime and updater.
- `trigger_model.go`: semantic trigger effects, normalized-wire decoding and the common `triggerForce` 0..48 type.
- `legacy_trigger_compat.go`: protocol-v40 trigger decoder only. Historical trigger integer formats do not enter current v41 gameplay.
- `trigger_effects.go`: gameplay trigger behavior (ABS, TCS, limiter, wheelspin, shift, airborne).
- `surface_effects.go`: road-material model.
- `body_effects.go`: discrete body/tyre events.
- `common_feel_engine.go`: single transport-neutral orchestration path.
- `haptic_waveforms.go`: pure waveform/profile definitions.
- `haptic_mixer.go`: voice scheduling and stereo renderer. Runtime PCM buffers are reused; expensive RMS/peak/surface diagnostic analysis is collected only when diagnostics are enabled.
- `transport_calibration.go`: narrow physical output compensation by transport. It does not alter vehicle/event detection or trigger/LED logic.
- `haptic_pipeline.go`: PCM transport boundary. USB and Bluetooth have separate runtime processing entry points so only the active transport is adapted.
- `dualsense_reports.go`: final HID hardware adapter and the only place where force48 is reduced to the Official eight-level format.
- `led_controller.go`: RPM color-state logic.
- `user_settings_schema.go`: defaults and migrations, including conversion of old trigger-strength storage to force48.
- `user_settings.go`: persistence/runtime settings access; current adaptive-trigger settings are stored as `0..48`.

## Invariants

1. Vehicle/gameplay interpretation never branches on USB vs Bluetooth; only the physical output-calibration layer may differ by transport.
2. USB and Bluetooth consume the same Common Feel state.
3. Current trigger force has one internal unit: `0..48`.
4. Official eight-level trigger quantization exists only in the final HID adapter.
5. Protocol-v40 trigger formats are decoded only by `legacy_trigger_compat.go`; protocol v41 never falls back to them.
6. Haptic PCM is never reduced to a trigger-force integer scale.
7. LEDs and adaptive triggers do not pass through the PCM pipeline.
8. Transport compensation is centralized and narrow: global PCM adaptation stays in `haptic_pipeline.go`, while explicitly measured effect-family corrections live only in `transport_calibration.go`; no per-material transport compensation.
9. USB remains the canonical unfiltered haptic reference.
10. RGB has exactly one runtime writer: BeamNG `Device.setRGB()` on both USB and Bluetooth. The Bridge explicitly keeps lightbar-valid fields clear during normal runtime and announces ownership hand-back on startup.
11. User-owned settings survive Bridge/mod updates through explicit schema migrations.
12. Runtime transport adaptation is single-path: USB never executes Bluetooth decimation/filtering and Bluetooth never builds an unused USB PCM copy.
13. Protocol-v41 settings are stateful and sparse: a missing `userSettings` field means “retain the last settings”; BeamNG resends settings on change and periodically as a recovery heartbeat.
14. High-volume diagnostic/event detail is event-scoped or diagnostics-scoped rather than serialized/calculated on every normal frame.
15. Protocol generation numbers are monotonic and immutable: increment only for breaking wire changes; never reuse an older number.
16. Compatibility detection is local-only. Network access starts only after the user manually runs the updater or accepts the incompatibility prompt.
17. Release selection is compatibility-based, not simply latest-version-based: keep historical stable entries in `BRIDGE_COMPATIBILITY.json`.

## Transport parity

Gameplay synthesis remains shared and USB remains the unchanged reference. `transport_calibration.go` contains only physical transport compensation. V1.3 keeps USB at 1.00 and applies one global Bluetooth calibration of 0.80 before the Bluetooth-only anti-alias filtering, 48 kHz -> 3 kHz conversion and HID packetization. No effect family has a transport-specific gain. Trigger encoding remains transport-neutral. Runtime RGB ownership stays in BeamNG for both transports, while the Bridge keeps its HID state neutral with respect to the lightbar.
