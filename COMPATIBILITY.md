# Compatibility and V1.41 migration

V1.41 is the first unified project release: the BeamNG mod and Windows Bridge use the same release version and are published together.

## Normal updates from V1.41 onward

`UPDATE_DUALSENSE.exe` is manual. It accepts only complete GitHub Releases containing both matching assets:

- `Enhanced_PS5_DualSense_Haptics_Vx.y.zip`
- `Enhanced_PS5_DualSense_Haptics_Bridge_Vx.y.zip`

Both packages are downloaded and verified before the Bridge is changed. The Bridge is updated automatically. The matching BeamNG mod ZIP is copied to the user's Downloads folder for manual installation. If Downloads is unavailable, the updater falls back to Desktop and then to the Bridge folder. After the update, the updater creates and opens one version-matched `..._INSTALL_INSTRUCTIONS.txt` tutorial next to the downloaded mod ZIP. This is the single normal update tutorial shown to the user.

The updater never searches for, edits, deletes or guesses the location of the BeamNG user folder. The user remains in control of the BeamNG mod installation, which also works with moved/custom BeamNG user folders.

`UPDATE_BRIDGE.exe` is retained as a compatibility alias of the same V1.41 updater.

## Migration from older versions

Pre-V1.41 updaters understand only `BRIDGE_COMPATIBILITY.json` and can update only the Bridge. The V1.41 release therefore keeps the legacy compatibility index and protocol generations 40-43.

An old updater can install `Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip`. That package installs the new updater and a migration notice. Bridge V1.41 continues to accept old mod generations long enough to display the one-time migration instruction.

The user then runs `UPDATE_DUALSENSE.exe` once. Bridge V1.41 shows a short one-time migration message that only tells the user to run `UPDATE_DUALSENSE.exe`. The updater then uses the same single BeamNG installation tutorial as every normal V1.41+ update. It downloads and verifies the complete V1.41 release, keeps/updates Bridge V1.41, saves `Enhanced_PS5_DualSense_Haptics_V1.41.zip` in Downloads and opens the installation tutorial. If the old mod came from the BeamNG Repository, the tutorial directs the user through BeamNG.drive > Mod Manager > the mod entry > its Repository page, then unsubscribe/remove, close BeamNG.drive, and place the downloaded ZIP in the user's own BeamNG `mods` folder. No BeamNG path detection is performed.

Do not remove the V1.41 entry from `BRIDGE_COMPATIBILITY.json` in future releases while migration from old public updaters is still supported: V1.41 is the permanent landing point from the old updater system.
