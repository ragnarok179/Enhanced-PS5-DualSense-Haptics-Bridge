# V1.43 — offline release validation report (default-only revision)

## Base archives
- `Enhanced_PS5_DualSense_Haptics_V1.42_SPEAKER_ATOMIC_PERSISTENCE_FIX.zip` (user-validated working mod).
- `Enhanced_PS5_DualSense_Haptics_Bridge_V1.42_R2_FIX.zip` (user-validated working bridge).

## Changes
- Mod defaults for speaker volume changed from 70 to 100 in exactly two locations: Lua GE extension and Vue default settings module.
- No conversion of saved 70% values; no volume migration; no V3 key. The existing `dseSpeakerSettingsV2` key and atomic persistence are retained.
- V1.43 version metadata updated in gameplay compatibility config and mod_info.
- All other 27 mod files, including the Speaker Vue component and L2/R2 physics code, are byte-for-byte identical to the validated V1.42 base.
- Bridge is unchanged from the prior prepared V1.43 candidate.

## Automated checks for this revision
- Complete 31-file mod archive and unchanged entries verified by exact byte comparison.
- Asserted V2 persistence key on Lua and JS sides; asserted absent V3 migration logic.
- Confirmed both new fallback defaults are 100, preserving existing saved object handling.
- Updated repository staging mod tree, generated source-diff patch and release manifests.
- ZIP integrity and distribution checksums verified.

## Publication gate
- Hardware testing (Bluetooth/USB) for the packaged V1.43 remains required.
- Full Go source rebuild and CI are not included in this offline preparation.
- Third-party errors in the user's BeamNG log are outside the DualSense package.
