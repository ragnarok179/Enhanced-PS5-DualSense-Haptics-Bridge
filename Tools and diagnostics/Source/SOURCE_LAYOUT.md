# Source layout

The Bridge uses one Go `main` package for both Windows transports. Gameplay interpretation is shared; USB and Bluetooth differ only at the transport/output boundary.

## Core flow

- `telemetry.go`: BeamNG protocol schema and JSON decoding. Current v41 trigger intents are semantic normalized objects.
- `protocol_compatibility.go`: local protocol-envelope inspection, compiled multi-generation support and the user-approved updater handoff. It performs no network requests.
- `internal/compatibility`: shared compatibility-index model and release-selection rules used by the Bridge and updater.
- `trigger_model.go`: semantic trigger effects plus the single canonical `triggerForce` type (`0..48`). Position remains normalized; frequency remains Hz.
- `legacy_trigger_compat.go`: isolated protocol-v40 trigger decoder. This is the only production adapter for historical trigger integer formats.
- `common_feel_engine.go`: transport-neutral orchestration and the single trigger-processing path.
- `trigger_effects.go`: ABS, TCS, rev-limiter, wheelspin, shift and airborne behavior using force48 trigger effects.
- `surface_effects.go`: material classification, rolling/slip strength, surface scheduler and synthetic asperities.
- `body_effects.go`: suspension, collision, landing, shift and tyre/body haptic cues.
- `haptic_waveforms.go`: pure surface/body waveform signatures and profile gains.
- `haptic_mixer.go`: voice scheduling, spatial isolation and canonical 48 kHz stereo mixing. Runtime buffers are reused and diagnostic-only statistics are gated.
- `transport_calibration.go`: transport boundary policy. USB is the 1.00 reference; Bluetooth uses one global physical calibration (0.80 in V1.3) and Bluetooth-only filtering/decimation is isolated here. Trigger/LED logic does not use PCM transport gain.
- `haptic_pipeline.go`: final PCM transport adaptation. USB stays the canonical 48 kHz reference; Bluetooth derives its lower-bandwidth stream here. Runtime calls are transport-specific so the inactive adaptation path is never computed.
- `led_controller.go`: shared RPM color-state logic used for diagnostics. Runtime lightbar output is owned by BeamNG `Device.setRGB()` on both USB and Bluetooth; the Bridge keeps LED-valid HID fields clear during normal operation.

## Output boundaries

- `dualsense_reports.go`: final DualSense HID encoding only. Fine force48 is written directly; Official modes reduce force48 to their eight physical force levels here and nowhere else. Fine position/frequency use their own hardware fields and are not force values.
- `usb_main_windows.go`, `usb_hid_windows.go`, `usb_audio_shared_windows.go`, `usb_pcm_buffer.go`: USB HID/WASAPI adapter. `usb_pcm_buffer.go` is a fixed 50 ms stereo ring buffer in steady state.
- `bluetooth_main_windows.go`, `bluetooth_audio_packets.go`, `hid_windows.go`: Bluetooth adapter and report framing. The normal 0x36 path reuses report/state storage, computes Sony CRC incrementally, and leaves RGB/lightbar validity clear so BeamNG remains the sole runtime LED writer.

## Settings

- `user_settings_schema.go`: defaults, calibration references and migrations. Schema 11 stores adaptive-trigger strengths as `0..48`; older trigger byte-like formats exist only in migration code.
- `user_settings.go`: persistence and runtime accessors. Current adaptive-trigger accessors return `triggerForce` directly.
- `console_settings_windows.go`: optional diagnostic console editor; values are displayed as percentages even though trigger settings use force48 internally.

## Trigger invariant

Current gameplay never reasons in `X/255` or `X/8` trigger-force units. It uses normalized semantic values at authoring/wire boundaries and one `0..48` force lattice inside the Bridge. The Official eight-level format appears only while building the final DualSense HID report.

Protocol v41 sends only normalized `l2Effect` / `r2Effect` objects for trigger behavior. Protocol v40 compatibility is kept in the isolated legacy adapter and is not emitted by the current mod. Public protocol numbers are monotonic compatibility generations: increment only for a breaking wire change and never reuse an older number.

## Update tooling

- `launcher/main.go`: source for `START_BRIDGE.exe` and `START_BRIDGE_AND_BEAMNG.exe`.
- `updater/main.go`: stable GitHub Release updater. Both compatibility metadata and install ZIPs come from published Release assets; compatibility mode selects the newest published release supporting the detected mod protocol, while manual mode remains explicitly user initiated.
- `BRIDGE_COMPATIBILITY.json`: stable release-to-protocol compatibility index. Keep historical compatible releases listed so older/newer mod generations can select the newest valid Bridge.
- `SHA256SUMS.txt`: managed-file manifest. `user_settings.json`, diagnostics logs and migration-only legacy updater files are intentionally excluded where applicable.

## Runtime efficiency

- Normal BeamNG packets carry only runtime fields consumed by the Bridge. Persistent settings are revision-cached and transmitted on change plus a low-rate recovery heartbeat.
- Lua trigger effects/signatures reuse numeric state instead of constructing formatted strings/tables every control tick.
- The Common Feel Engine owns reusable PCM scratch buffers. USB and Bluetooth each invoke only their own transport adapter.
- USB audio buffering and Bluetooth report framing are fixed-storage/reusable in steady state.
- Diagnostic RMS/peak/surface metrics are not part of the normal hot path unless runtime diagnostics are enabled.
