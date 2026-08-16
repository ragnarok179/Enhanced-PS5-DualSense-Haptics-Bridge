# Tools and diagnostics

- `Bridge`: prebuilt Windows executables used by the root launchers.
- `Config`: Common Feel Engine calibration plus locally generated user preferences.
- `Diagnostics`: focused hardware, stereo and log-capture tools.
- `Source`: Go source code, developer documentation, build scripts and tests.

Normal users should start from one of the two `.exe` launchers at the repository root:

- `START_BRIDGE.exe`: Bridge only.
- `START_BRIDGE_AND_BEAMNG.exe`: BeamNG.drive + Bridge.

## Controller settings

Configure feedback from BeamNG.drive through **Pause -> Mods -> DualSense Haptics** or the **DualSense Haptics Settings** action in **Options -> Controls**. Settings are saved by BeamNG and sent locally to the Bridge.

## Diagnostics

Only useful end-user diagnostics are kept:

- `CHECK_INSTALLATION.bat`: validates the package and Common Feel profile.
- `LIST_HARDWARE.bat`: USB probe, Bluetooth probe, DualSense audio endpoints and Windows PnP entries.
- `TEST_USB_STEREO.bat`: USB left/right/center haptic test.
- `TEST_BLUETOOTH_STEREO.bat`: Bluetooth left/right/center haptic test.
- `START_USB_DIAGNOSTIC_LOG.bat`: detailed USB diagnostic session saved to `Logs`.
- `START_BLUETOOTH_DIAGNOSTIC_LOG.bat`: detailed Bluetooth diagnostic session saved to `Logs`.
- `OPEN_LOGS_FOLDER.bat`: opens the diagnostic log folder.

Redundant standalone audio/profile tools and the development-only Bluetooth bump-carrier test were removed. Hardware listing already includes audio enumeration, while installation checking already prints the active Feel profile.

## Updater

`UPDATE_BRIDGE.exe` is the current updater. Normal Bridge startup performs only a local Mod/Bridge protocol check. GitHub is contacted only after an explicit manual updater launch or after the user accepts a compatibility update prompt.

The updater reads compatibility metadata from a **published stable GitHub Release**, selects the newest compatible stable Bridge, verifies `SHA256SUMS.txt`, performs a transactional install with rollback, updates itself through a temporary worker when necessary, and can restart the Bridge after compatibility updates.

The repository keeps the old `UPDATE_BRIDGE.bat` and `Tools and diagnostics/Updater/Update-Bridge.ps1` only so V1.1 users can migrate to the current EXE updater. They are repository migration files, not part of normal modern installations.
