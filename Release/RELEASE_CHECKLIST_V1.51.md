# V1.51 release checklist

## Source and automated verification

- [x] Source tree based on the latest R2 Jolt / Wheelspin and touchpad fixes; isolated issue #1 steering end-stop and issue #2 activation lifecycle corrections, with no unrelated gameplay refactor.
- [x] Build and source version `1.51.0`, display `V1.51`, protocol 43 only, compatibility metadata generated automatically from buildinfo.
- [x] Diagnostic script prints V1.51 rather than V1.42; architecture/protocol documents updated.
- [x] Deterministic ZIP builder and checksum validator added to `Source/scripts`.
- [x] Source builds of USB, Bluetooth, launcher and updater reproduced byte-for-byte with Go 1.23.2.
- [x] `go test ./...`, `go test -race ./...`, `go vet ./...`, Lua contract tests (steering, Mic Mute, touchpad, mod lifecycle), Node UI lifecycle tests, Windows USB/Bluetooth test compilation.
- [x] V1.43 and V1.50 published updater source inspected: both select complete stable releases without a hardcoded V1.50 ceiling; V1.51 selection is covered by regression tests.
- [x] Offline V1.50 → V1.51 Bridge installation exercised using the checksum-matching published V1.50 ZIP and the V1.51 candidate. Verified 9 managed changes, no unexpected removals, and preserved unmanaged `Config/user_settings.json`.
- [ ] Perform an end-to-end V1.43 → V1.51 installation test using the published V1.43 Windows updater and a stable V1.51 GitHub Release. Source/API compatibility is verified but a real V1.43 installation was not independently reproduced during this audit.
- [x] Two release ZIPs, SHA manifests and full installation bundle verified internally.
- [ ] Publish binaries built by the configured GitHub CI with maintained Go 1.27.4, after checking all CI tests and checksums. The reproducible local candidate is built with legacy Go 1.23.2 and should not be mistaken for the security-updated CI build.
- [ ] Windows physical DualSense sanity test with **both USB and Bluetooth** against the CI-built Bridge.
- [x] Stable `mod_info.tagid` present; Mod Manager ID tests and release archive validation cover the previous failed UI lookup (`deactivateModId: mod not existing`).
- [ ] Execute `Release/MOD_ACTIVATION_TEST_V1.51.md` in BeamNG: three disable/enable cycles from the Mod Manager, then repeat inside a level; ensure no duplicates, stuck loading, or residual vinput entries.
- [ ] In BeamNG, bind Rotation Angle to steering and test 90° / 135° / 180° limits: exceed ±180° on both sides, hold at each end stop, return toward center, recenter, then switch Motion modes. Verify no inversion or sudden jump.
- [ ] Real update V1.43 → V1.51 **and** V1.50 → V1.51 on Windows using published GitHub Releases and the updater included with each older Bridge.

## Publication procedure

1. Review and commit the source-only repository. Generated EXEs and ZIPs must not be committed.
2. Push source to the GitHub repository, ensure `BRIDGE_COMPATIBILITY.json` on the default branch selects V1.51.
3. Prepare the two ZIPs from the same verified source (e.g. `python Source/scripts/package_release.py --repo-root .`) or use the matching CI build if desired. GitHub Actions is optional; publishing is always manual.
4. Verify the two ZIPs and `RELEASE_SHA256SUMS.txt`, and perform the Windows USB/Bluetooth and Mod Manager smoke tests.
5. Create a **stable** GitHub Release with tag `V1.51` and upload *both* exactly named ZIPs:
   - `Enhanced_PS5_DualSense_Haptics_V1.51.zip`
   - `Enhanced_PS5_DualSense_Haptics_Bridge_V1.51.zip`
6. Upgrade separate V1.43 and V1.50 installations using their original `UPDATE_DUALSENSE.exe` after the V1.51 release is public. Confirm the Bridge is installed, unmanaged user settings are preserved, and the matching mod ZIP is staged for manual installation.

## Constraints

The updater never installs the BeamNG mod automatically. It stages the matching ZIP and prints instructions.
Backup custom edits to the Bridge-provided `Config/feel_profile_v1.json` before updating;
the V1.43 updater treats that file as managed and may replace it.
No code changes were pushed or releases published by this preparation.

## Nettoyage V1.43+

- [x] L'updater `UPDATE_DUALSENSE.exe` continue de gérer V1.43+ et les releases futures.
- [x] Suppression de l’ancienne implémentation de `UPDATE_BRIDGE.exe`, du guide de migration et des chemins V1.42 et antérieurs. Le fichier `UPDATE_BRIDGE.exe` reste en alias identique de l’updater actuel pour permettre V1.43 → V1.51.
- [x] Retrait des protocoles 41/42 dans l'index et le décodeur.
- [x] Console utilisateur sans messages internes de propriété du touchpad (diagnostic seulement).
- [x] Génération automatique du JSON depuis la version compilée et sa copie dans l'artefact CI.
