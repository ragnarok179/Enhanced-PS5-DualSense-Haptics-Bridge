# Source layout

The repository separates maintainable source code from the public runtime package.

```text
/
├─ Source/          Go source, tests, docs and release scripts
├─ Bridge/          built USB/Bluetooth runtime executables
├─ Config/          shipped runtime configuration
├─ Diagnostics/     explicit support diagnostics
├─ START_BRIDGE.exe
├─ START_BRIDGE_AND_BEAMNG.exe
├─ UPDATE_DUALSENSE.exe
├─ UPDATE_BRIDGE.exe  compatibility alias
├─ README.md
├─ LICENSE
├─ THIRD_PARTY_NOTICES.md
└─ SHA256SUMS.txt
```

`Source/` and Git metadata belong to the GitHub repository. They are not included
in the public Bridge ZIP.

## Shared runtime

USB and Bluetooth share telemetry decoding, Common Feel, gameplay interpretation,
adaptive-trigger logic, user settings and haptic synthesis. Transport-specific I/O
stays in explicitly named USB/Bluetooth files.

## Versioning

- Bridge public application version: centralized in `version.go`.
- Supported gameplay protocol range: centralized in `protocol.go`.
- Bridge V1.41 accepts legacy/migration wire generations 40-43. From V1.41 onward normal updates are release-version based and UPDATE_DUALSENSE.exe updates the Bridge and downloads the matching BeamNG mod ZIP for manual installation.
- Common Feel profile version is independent from the Bridge application version;
  it changes only when the calibrated profile itself changes.

See `docs/BEAMNG_PROTOCOL.md` for the mod compatibility matrix.

## User settings

User-facing runtime settings are owned by the BeamNG in-game UI. The Bridge does
not expose a second console settings menu. Any local runtime state such as
`Config/user_settings.json`, when present for migration/support, is user-owned and
must remain excluded from release manifests and updater overwrite operations.

## Public launcher

`Source/launcher/main.go` builds `START_BRIDGE.exe`. The same binary is copied to
`START_BRIDGE_AND_BEAMNG.exe`; it selects behavior from its own filename. The
launcher detects the connected transport and starts the matching executable from
`Bridge/`.

## Public updater

`Source/updater/` builds `UPDATE_DUALSENSE.exe`. The same V1.41 binary is also
shipped as `UPDATE_BRIDGE.exe` so old installations and shortcuts keep working.

From V1.41 onward the updater is manual and release-based. It accepts only stable
GitHub Releases containing both matching versioned ZIP assets, downloads and
verifies both packages before installation, then updates the BeamNG mod and
Bridge with backup and rollback, while the BeamNG mod ZIP is staged for manual installation.
The updater writes and opens one common version-matched `..._INSTALL_INSTRUCTIONS.txt` tutorial next to the staged mod ZIP. When V1.41 is reached through an old Bridge-only updater, the running Bridge prints only a short one-time migration message directing the user to `UPDATE_DUALSENSE.exe`.

The legacy compatibility-index code remains in the source because pre-V1.41
`UPDATE_BRIDGE.exe` binaries already installed on users' PCs need
`BRIDGE_COMPATIBILITY.json` to land on Bridge V1.41. V1.41 is the migration hub;
once there, normal updates use the unified updater instead of protocol-based
release selection.

## Release manifest

`Source/scripts/generate_sha256sums.ps1` hashes only the public runtime package
surface. It intentionally excludes `Source/`, Git metadata, user settings and
runtime log files. This prevents source files from accidentally becoming updater-
managed content.
