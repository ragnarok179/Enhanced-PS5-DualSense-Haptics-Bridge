# V1.50 — Touchpad ownership, Mic Mute responsiveness and gyro steering

- **Gyro steering end stops (issue #1):** Rotation Angle no longer changes steering direction when the controller crosses ±180°. Rotation is accumulated continuously per Motion input and clamped to the configured per-side steering range. Returning from beyond the stop releases it smoothly, without recentering. Auto-calibration, recenter and stream reconnection reset the history; legacy stored settings and normal gyro/tilt/orientation modes are unchanged.

- **New touchpad bindings:** A newly saved touchpad click/tap/swipe/axis binding automatically selects Mouse Off (including after the binding-capture popup closes); bindings on ordinary buttons or motion sensors do not. A later manual Mouse On remains available without deleting bindings.
- **Touchpad:** The Mouse On/Off UI control and BeamNG shortcut now switch between Windows mouse and existing BeamNG touchpad bindings without deleting bindings. Explicit Mouse On wins over a saved binding; the binding editor temporarily wins over mouse so you can still create new bindings.
- **Mic Mute:** The Bridge reacts to the physical Mic Mute press immediately in Toggle mode, before the next in-game state packet. Late pre-press packets are ignored briefly; the actual vehicle state can still correct the indicator.
- **Feedback mode removed:** The entire Inputs only option and its custom runtime branching have been removed from the settings UI, Lua and Bridge. Individual haptics, triggers, lighting and speaker settings remain.
- Trigger start/end dual sliders and their resistance settings remain unchanged, including the increased spacing.
- **Mic Mute lifecycle:** A new read-only observation follows vehicle switching, vehicle replacement (including reused IDs), Ctrl+R/reset/repair and level changes. The LED uses two consistent observations to reject transient states; no virtual action is sent merely for synchronization. Unsupported actions still use manual Toggle.
- Version and telemetry protocol remain **V1.50** / protocol 43. Replace both the mod ZIP and the Bridge files manually; the updater does not distinguish revisions sharing the same version number.

Automated tests and Windows builds cannot replace a real DualSense USB/Bluetooth test in BeamNG.

## R2 Jolt / Wheelspin slider correction (same V1.50)

- Fixed the Bridge discarding the user-defined **Jolt strength** and forcing a constant 100% intensity. The default 25% slider setting continues to reproduce the previous calibrated output; other slider values scale R2 limiter/TCS vibrations up or down.
- Fixed the **Wheelspin pulse strength** being masked by TCS priority. Actual driven-wheel slip now remains detectable during TCS intervention and at very low vehicle speed; brief wheelspin pulses can alternate with TCS feedback. Rev-limiter feedback retains priority.
- The two sliders are independent, and wheelspin pulses do not receive a second Jolt gain. Normal throttle resistance, ABS, shift and airborne release remain separate.
- Changes affect both USB and Bluetooth through the shared telemetry and Common Feel Engine; in-game device validation is still required.


## Release and upgrade verification

- Release tag must be exactly `V1.50` with the two ZIPs `Enhanced_PS5_DualSense_Haptics_V1.50.zip` and `Enhanced_PS5_DualSense_Haptics_Bridge_V1.50.zip` attached.
- The V1.43 updater searches for a complete stable release and downloads both assets. It replaces the Bridge after verification, but the BeamNG mod ZIP must be installed manually using the displayed instructions.
- `BRIDGE_COMPATIBILITY.json` must be published in the repository root so legacy compatibility checks can locate the V1.50 target.
- Runtime binaries are rebuilt by `Source/scripts/package_release.py`; use the GitHub Actions build with **Go 1.27.4** for the official release, and do not substitute EXEs from previous test builds.
- If `Config/feel_profile_v1.json` was manually edited, back it up before running the V1.43 updater. The old updater treats it as a managed runtime file.
- Hardware validation is completed by users with a DualSense; offline CI tests cannot prove every USB/Bluetooth hardware behavior.


## Issue #2: Mod Manager enable/disable identity and lifecycle (same V1.50)

- **Root cause confirmed by `beamng.log` (2026-11-13):** BeamNG logged `deactivateModId: mod not existing ENHANCEDPS5DUALSENSEHAPTICS`. The mod had a `mod_info` ID but no `modData.tagid`. BeamNG's `getModNameFromID()` therefore could not match the UI-supplied ID to the filename-derived ZIP record (especially names with ` (7)` suffix). The same lookup is used for activation.
- Added the explicit, stable `tagid` in `mod_info/EnhancedPS5DualSenseHaptics/info.json`; the identity is independent of the installed ZIP filename. Release validation and CI now reject future missing/mismatched IDs.

- The mod removes its **own** manual-lifetime GE extension registrations and unloads them when BeamNG emits `onModDeactivated` for this mod. Ownership is identified by BeamNG's actual `modData.tagid`, including when the ZIP was renamed with a `(7)` suffix. It does **not** unload extensions during normal level changes, vehicle changes or Ctrl+R.
- Cleanup releases the shared virtual input producers and UDP sockets through their existing `onExtensionUnloaded` handlers. On reactivation, the normal modScript path re-registers extensions and BeamNG's `loadManualUnloadExtensions()` loads them again.
- The Vue settings page serializes asynchronous route/button registration and removal so a slow registration cannot reappear after a disable. Failed registrations are cleaned up and may be retried.
- Automated lifecycle regression tests cover unrelated mods, repeated deactivation, manual registration removal, reactivation, and overlapping UI operations. The user has confirmed the original Mod Manager loop is resolved by the `tagid` fix; the deeper cleanup/device recreation cases still require hardware regression checks. See `Release/MOD_ACTIVATION_TEST_V1.50.md`.
- No Bridge wire protocol or Windows executable functionality changes are required by this fix.

Manual release procedure is unchanged: publish the two ZIPs with their exact V1.50 names as release assets. GitHub Actions is optional.
