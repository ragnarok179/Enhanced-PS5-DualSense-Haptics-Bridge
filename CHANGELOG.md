# Changelog

## V1.41

- Unifies the BeamNG mod and Windows Bridge under one shared release version.
- Replaces separate Bridge-only updates with the manual `UPDATE_DUALSENSE.exe` updater, which updates the matching Bridge and downloads the matching BeamNG mod ZIP for manual installation.
- Downloads and verifies both release assets before changing the Bridge, while leaving BeamNG mod placement under user control.
- Keeps `UPDATE_BRIDGE.exe` as a temporary compatibility alias for older installations and shortcuts.
- Adds a short one-time legacy migration message and one common BeamNG installation tutorial for the normal unified update flow.
- Adds a one-time migration path so old V1.3/V1.4 updaters can first install Bridge V1.41, then hand over to the unified updater for the BeamNG mod.
- Adds wire generation 43 only as the V1.41 migration generation while retaining legacy generations 40-42 for old installations.
- Keeps previously validated haptics, adaptive triggers, controller speaker, extended inputs, motion, lighting and USB/Bluetooth behaviour unchanged unless required by the V1.41 migration/update architecture.
