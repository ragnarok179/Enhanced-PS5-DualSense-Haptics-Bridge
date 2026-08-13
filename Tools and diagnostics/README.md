# Tools and diagnostics

- `Bridge`: prebuilt Windows executables used by the root launchers.
- `Config`: shared Common Feel Engine profile and transport calibration.
- `Diagnostics`: hardware probes, stereo tests and bridge console log capture.
- `Source`: Go source code, developer documentation, build scripts and tests.

Normal users should start from one of the two signed-ready `.exe` launchers at the repository root:

- `START_BRIDGE.exe`: starts the Bridge only.
- `START_BRIDGE_AND_BEAMNG.exe`: starts BeamNG.drive and then the Bridge.

Both files are built from the same launcher source. The second file is an identical copy; the launcher selects its mode from its filename, so there is no duplicated launcher implementation.


The optional manual updater is available at the repository root as `UPDATE_BRIDGE.exe`. It checks the latest stable GitHub Release, downloads the official `Enhanced_PS5_DualSense_Haptics_Bridge.zip` asset and uses its own temporary worker copy during installation so it can update itself safely without a separate helper executable.
