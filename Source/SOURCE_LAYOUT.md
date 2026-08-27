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
├─ UPDATE_BRIDGE.exe
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
- Current Bridge V1.4 supports BeamNG gameplay protocols 40, 41 and 42.
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

`Source/updater/main.go` builds `UPDATE_BRIDGE.exe`. It reads the compatibility
index published with stable GitHub Releases, selects the newest non-downgrade
release that explicitly supports the detected BeamNG gameplay protocol, and then
downloads the asset named by that release entry. The package is verified with its
embedded `SHA256SUMS.txt` before installation.

The updater still knows the names of a few files from the old V1.1 layout solely
so it can remove them during migration. Those files are not part of the modern
runtime package or manifest.

## Release manifest

`Source/scripts/generate_sha256sums.ps1` hashes only the public runtime package
surface. It intentionally excludes `Source/`, Git metadata, user settings and
runtime log files. This prevents source files from accidentally becoming updater-
managed content.
