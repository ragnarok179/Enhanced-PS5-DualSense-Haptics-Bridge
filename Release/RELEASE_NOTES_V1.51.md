# V1.51 — Mic Mute vehicle-state synchronization

- Keep the Mic Mute toggle and LED state across Ctrl+R, even when BeamNG follows the reset with `onVehicleSpawned` for the same object.
- Track the last manually selected toggle per live vehicle. Switching A → B → A restores A’s previous LED state; switching between two ON vehicles never emits an intermediate OFF status.
- Read supported vehicle actions as before and let confirmed observations correct the cached toggle; custom/unrecognized toggle bindings retain their last Mic Mute state for their own vehicle. Arbitrary external changes cannot be inferred without a state reader.
- Avoid synthetic `button2` onDown calls when returning to a vehicle bound to an unrecognized onDown-only action; a real physical press still emits the command.
- On replacement (including reuse of the same numeric ID), forget the replaced vehicle’s state; no toggle commands are sent by lifecycle events. On actual vehicle destruction, discard its cached state.
- Protocol 43 is unchanged. Both Windows Bridge and BeamNG mod are versioned V1.51, and should be installed together. No upload or release is performed automatically.

## Verification

Lua lifecycle, known and custom button toggle regressions; Go, UI and compatibility tests; Windows USB/Bluetooth builds; archive and checksum validation. On-device testing in BeamNG remains necessary.

## Mic Mute maintenance (V1.51, no protocol change)

- Removed the unused cached action name and unavailable-state flag from the vehicle sync module, together with its unconsumed `waitingForState` and `active` methods.
- Scan candidate `button2` / `axis10` bindings first; query BeamNG's full action catalogue only when one unambiguous, valid binding exists. The detection of readable actions and unknown custom onDown toggles is unchanged.
- Moved the Go LED state snapshot helper out of the Windows runtime code and into the unit-test file.
- Added a regression contract for the action-catalogue lookup and kept the existing Ctrl+R, focus-loss, A→B→A, custom-toggle and disconnection tests.

This is a maintenance-only refactor. Mic Mute state, LED packets (DMB1/DMB2), polling cadence and vehicle lifecycle behavior are unchanged.
