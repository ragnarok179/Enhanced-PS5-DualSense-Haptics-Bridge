# V1.50 — Mod Manager activation/deactivation regression test

Test the **identity-fixed and lifecycle-audited** V1.50 mod ZIP, with no older DualSense mod ZIP installed alongside it. Do not publish the release until these checks pass inside BeamNG. No GitHub Actions or release upload is needed for the test.

1. Close BeamNG, replace the existing `Enhanced_PS5_DualSense_Haptics_V1.50.zip` with the candidate in the BeamNG user mods directory, and restart BeamNG. Keep a backup of the old archive. Verify there is only one copy installed.
2. On the main menu, open **Mod Manager**, **deactivate** the DualSense mod, and verify the game leaves the loading screen and the mod shows as disabled. **Reactivate** it and verify the game returns to the menu. Repeat this **three times** without restarting BeamNG.
3. Open a Free Roam level, then open the pause menu. Confirm **Enhanced PS5 DualSense Settings** appears exactly once and opens correctly. Check that saved controls, Touchpad Mouse mode, Motion settings and Mic Mute mode remain unchanged.
4. While in Free Roam, disable the mod from Mod Manager. Confirm the loading screen ends, the custom settings entry disappears, and **DualSense Extended Inputs** is no longer registered as a virtual input device. If the device list does not refresh immediately, leave/reopen the Input menu rather than restarting the game.
5. Re-enable the mod without leaving Free Roam. Confirm the settings entry returns **once**, and that **one** DualSense Extended Inputs device is present. Test a touchpad gesture and gyro steering to ensure bindings respond as before. Repeat the disable/enable operation **three times**.
6. With the mod enabled, change vehicle, use **Ctrl+R**, repair/respawn, leave Free Roam for the main menu, and enter a different level. Verify the mod stays enabled, the UI button stays present, and no duplicate virtual input device appears.
7. Repeat a disable/enable cycle with the Windows Bridge **not running**; the mod manager must not hang. Then start the Bridge and verify it reconnects in USB or Bluetooth as appropriate.
8. Close BeamNG normally. Inspect `beamng.log` for `DualSenseModLifecycle`, `unload failed`, `Failed to execute`, `Lua error`, or repeated route errors, and inspect the Bridge log for reconnect and device discovery.

**Expected:** no infinite spinner, no stale UI button, no duplicate virtual device, no residual touchpad/gyro actions while disabled, and saved bindings/settings remain intact.

**If it fails:** record which step failed and attach `beamng.log` from that session, plus the Bridge log if it was running. Avoid repeatedly activating both the old and new ZIP in the same test session.

**Scope:** automated Lua/JS tests check event and cleanup ordering; they do not simulate BeamNG's full GUI or hardware.

**Extra check:** With a copied test ZIP renamed with a suffix like ` (7)` (and no other copy installed), repeat a deactivation/re-activation cycle; cleanup must still recognize this mod by `modData.tagid`.

**Specific regression:** The previous `beamng.log` contained `deactivateModId| mod not existing ENHANCEDPS5DUALSENSEHAPTICS`. The corrected ZIP must no longer produce that line after a deactivate request. An updated BeamNG log must instead show the mod changing state and the UI returning.
