# Enhanced PS5 DualSense Haptics — V1.43

## Changes since V1.42
- New/default controller-speaker volume is **100%** (previously 70%).
- **No automatic volume migration**. An existing saved volume, including 70%, stays unchanged. The setting remains adjustable from the interface.
- The working V1.42 speaker settings snapshot (`dseSpeakerSettingsV2`) is retained without introducing a V3 key or modifying persisted values.
- Retains the user-validated speaker persistence fix and normal L2/R2 strength fixes from the latest local V1.42 package.
- Gameplay wire protocol remains 43 (legacy bridge compatibility 41–43).

## Installation
1. Remove any other active Enhanced PS5 DualSense Haptics mod ZIP from BeamNG's mods folder.
2. Install `Enhanced_PS5_DualSense_Haptics_V1.43.zip` without extracting it.
3. Install `Enhanced_PS5_DualSense_Haptics_Bridge_V1.43.zip` into the bridge folder.
4. Restart BeamNG and the bridge. Previous user-selected speaker volume is preserved; to use 100% with an existing setting, set 100% in the UI or invoke its Reset to defaults action.

## Release status and limitations
- Bridge binaries were compiled from the V1.43 Go sources with the R2 fix integrated. The CI unit, race, vet and cross-compilation checks passed.
- Hardware Bluetooth and USB tests are still required for the packaged V1.43 build.
- Selective Windows collision-sound suppression for all native BeamNG audio paths is not established.
- The user's separate BeamNG log mentions an invalid ETK JBeam, the unrelated `fullVehicleStructureDump` extension, and an unrelated Custom Engine Lab UI exception.
