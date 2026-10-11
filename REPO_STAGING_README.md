# V1.43 offline repository integration kit

**This is not a complete clone of the upstream GitHub repository.** The full `Source/` tree and `.git` history cannot be retrieved into this workspace through the connected source interface.

Contents:
- Original-style distribution root (`Bridge/`, `Config/`, `Diagnostics/`, launchers, updater, manifests)
- `BeamNG-Mod/`: complete new BeamNG mod source tree (31 files)
- `Release/`: mod ZIP and release notes
- `Source/patches/V1.42_R2_semantic_strengths.patch`: Go R2 fix to apply to the full upstream source
- `Source/patches/V1.43_buildinfo.patch`: Go version bump
- `Source/patches/V1.43_mod_settings_and_volume.patch`: changes from working V1.42 mod to V1.43
- `Source/internal/buildinfo/buildinfo.go`: version source file for reference

To update the real source repository, first clone/pull the actual `ragnarok179/Enhanced-PS5-DualSense-Haptics-Bridge` repository on Windows, apply the patches to the matching upstream source revision, rebuild launchers, updaters and both Bridge executables from the patched Go sources, run the test suite, then replace the distribution assets. The supplied distribution executables have a five-byte equal-length `V1.42`->`V1.43` version-string change; they were not freshly compiled. Do not confuse this release integration kit with a reproducible full-source release.

No GitHub files, branches, tags, releases or CI workflows were changed.
