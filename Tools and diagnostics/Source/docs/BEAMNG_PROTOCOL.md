# BeamNG telemetry protocol

The BeamNG mod and Bridge communicate over localhost UDP using JSON. USB and Bluetooth consume the same transport-neutral telemetry.

Current protocol version: `41`.
Legacy trigger compatibility accepted by the Bridge: `40`.

## Main payload groups

- speed, RPM, engine and limiter state;
- ABS, TCS, wheelspin, lock, shift and airborne state;
- left/right surface material, roughness, rolling excitation and slip;
- discrete suspension, landing and collision events;
- semantic adaptive-trigger intents;
- optional persistent user settings.

## Compatibility envelope

Current packets also include small additive metadata fields: `protocolId: "DPH"`, `modVersion`, `protocolMin`, and `protocolMax`. `v` remains the **actual encoding used by that packet** and is the authoritative value for decoding. During the V1.0/V1.2 -> V1.1/V1.3 migration only, Mod V1.1 also emits a marked `legacyCompat: true` protocol-40 mirror immediately after the canonical protocol-41 packet. Bridge V1.2 ignores v41 and consumes that mirror; Bridge V1.3 ignores the marked mirror so effects are never processed twice.

The Bridge inspects only this local UDP envelope before decoding. The current Bridge accepts protocol generations 40 and 41. If a future mod sends an unsupported generation, the Bridge asks the user whether to run the updater; no GitHub request occurs before that approval.

Protocol numbers are immutable, monotonic compatibility generations. A breaking wire change increments the number (`41 -> 42 -> 43...`); a historical number is never reassigned to a different format. Non-breaking additive fields remain on the existing protocol.

`protocolMin`/`protocolMax` describe the mod's declared protocol-generation capability for diagnostics/future negotiation. They do not override `v`: the Bridge decodes the actual protocol named by `v`.

## Trigger payload — protocol 41

The current mod sends `l2Effect` and `r2Effect` objects. Their semantic kinds are `off`, `resistance`, `vibration` and `fine`.

```json
{
  "kind": "resistance",
  "startPosition": 0.0,
  "startForce": 0.125,
  "endForce": 0.5,
  "amplitude": 0.0,
  "frequencyHz": 0.0
}
```

Position, force and amplitude are normalized `0.0..1.0` on the wire. The BeamNG trigger model already snaps force/amplitude to the common 48-step lattice; the Bridge snaps once again on decode to make malformed/third-party packets deterministic.

Inside the Bridge, trigger force is never represented as `0..255`: it is `triggerForce` `0..48`.

At the final HID boundary:

- Fine Feedback force: `0..48` is written directly;
- Official Feedback resistance: `0..48` is reduced to the controller's eight available strength levels;
- Official Vibration amplitude: `0..48` is reduced to the controller's eight available strength levels;
- Fine Feedback position is converted separately to its genuine `0..255` position byte;
- frequency remains a separate physical/logical quantity and is encoded only at output.

Protocol 41 does **not** emit the historical `l2Mode/l2StartStrength/...` trigger mirrors. A v41 packet also never falls back to those fields if they are injected accidentally.

## Protocol 40 compatibility

Bridge V1.3 distinguishes two protocol-40 sources:

- real Mod V1.0 traffic has no `legacyCompat` marker and is consumed normally through the legacy trigger adapter;
- the temporary Mod V1.1 migration mirror carries `legacyCompat: true` and is ignored by Bridge V1.3 because the same state has already been sent canonically as protocol 41. Older Bridge V1.2 ignores the unknown marker and consumes the v40 mirror normally.

This transition lane can be removed from a later mod once V1.2 migration is no longer required. It does not create a second haptic engine or second trigger model.


The Bridge still accepts v40 packets so an older BeamNG mod can be diagnosed/migrated. Historical trigger fields are handled exclusively by `legacy_trigger_compat.go` and are converted immediately into the current force48 model. They never become an active unit in the Common Feel Engine.

## Sparse runtime payload

Protocol 41 remains wire-compatible; the current mod avoids repeating unchanged state:

- `userSettings` is sent immediately when its normalized settings revision changes and periodically (about once per second by default) as a recovery heartbeat; when omitted, the Bridge retains its last settings snapshot.
- the normal `raw` object contains only vehicle state actually consumed by the current Bridge runtime; the old per-wheel raw diagnostic table is no longer built/serialized on every sample.
- `bodyEvent` remains present so event sequencing is reliable, while detailed body-event source/score fields are attached only to the packet carrying a new body event.

This is an efficiency change, not a protocol-version change: existing v41 receivers already treat absent optional settings/event detail as unchanged/default state.

## User settings — schema 11

Current adaptive-trigger strength settings are transported and stored as `0..48`, with `triggerForceScale: 48` declaring the unit explicitly. BeamNG's UI remains percentage-based; percentages are converted to the nearest force48 level before transmission.

Non-trigger settings keep the representation appropriate to their own subsystem. For example, some haptic/surface calibration fields and light output use byte-scaled storage. Those values are not trigger force and are not converted to force48.

Schemas 10 and older are migration inputs only. Their historical trigger values may use old byte-like storage; `user_settings_schema.go` converts them once into schema 11 / force48.

Rolling and Slip remain independent controls. Each displayed default is a calibration label whose runtime gain is exactly 1.0, preserving the validated stock surface feel.

Public mod/Bridge release versions are independent from protocol version `41`.
