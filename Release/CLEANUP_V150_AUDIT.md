# V1.50 — update-path cleanup and console audit

The source of truth is the user-provided `Enhanced-PS5-DualSense-Haptics-Bridge-main (1).zip`.
No change was pushed to GitHub. Publishing releases remains manual.

## Removed paths used only by V1.42 or earlier

- Legacy `runLegacyWorker`, old compatibility-index GitHub download/selection, and obsolete updater CLI options.
- Separate legacy update guide `IMPORTANT_GUIDE_-_RUN_UPDATE_DUALSENSE_TO_FINISH_UPDATE.txt` and its packaging/manifest entries.
- Gameplay protocol 41/42 decoding and their legacy migration tests; current wire protocol is 43.
- Old compatibility-index Go selection data models. The shared version comparison and generic pending compatibility state remain for V1.43+.

## Important V1.43 compatibility exception

The published **V1.43** updater verifies that the downloaded Bridge ZIP contains
`UPDATE_BRIDGE.exe`, even though the current updater is `UPDATE_DUALSENSE.exe`.
Therefore the new ZIP retains `UPDATE_BRIDGE.exe` as a **byte-identical copy**
of `UPDATE_DUALSENSE.exe`. It contains no old migration implementation.
Removing the alias would break the V1.43 -> V1.50 update process.

## Console and diagnostics

- Normal Bridge windows keep the familiar controller/BeamNG connection status.
- Touchpad state/ownership transitions (`bound`, `capture`, `policy`, etc.) are printed
  **only when explicit touchpad-binding diagnostics are enabled**.
- Input and haptic operation are unchanged by this logging change.

## CI compatibility JSON

The workflow **generates** `BRIDGE_COMPATIBILITY.json` from the current source
`DisplayVersion` and gameplay protocol 43, then includes it in the CI artifact and
Bridge ZIP. It **does not** read GitHub's latest published release, modify GitHub,
commit the JSON, or publish a release. When updating the repository manually,
copy the generated JSON from the CI artifact to repository root.

For a future V1.51 with the same wire protocol, changing DisplayVersion in code
is sufficient for the generated JSON to use V1.51 and its matching Bridge asset.
A new gameplay protocol requires a manual compatibility review.

## Verification

- Go test, race detector, vet: pass.
- Lua and Vue/JavaScript integration tests: pass.
- Python metadata and generated-index tests: pass.
- Windows USB and Bluetooth test-binary cross-compilation: pass.
- Mod ZIP content is byte-for-byte identical to the supplied BeamNG-Mod source.
- Release archive manifests and SHA-256 sums: verified.
- Actual Windows HID hardware run and Go 1.27.4 GitHub Actions run: not executed here.

Local Bridge binaries were compiled using the available local Go toolchain.
Use the CI artifacts (Go 1.27.4) for the final published release after successful
GitHub Actions verification, followed by a hardware smoke test.
