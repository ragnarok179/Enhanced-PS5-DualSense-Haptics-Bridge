# BeamNG wire generations

The supported legacy migration floor is Bridge/Mod V1.3.

| Generation | Historical origin | Purpose in V1.42 |
| ---: | --- | --- |
| 41 | V1.1 | Retained because the supported V1.3 generation still uses protocol 41 |
| 42 | V1.2 | Retained for later pre-V1.41 releases such as V1.4 |
| 43 | V1.41 | Unified wire generation used by V1.41 and V1.42 |

Versions older than V1.3 are intentionally outside the supported migration range even when they share an older wire generation. `BRIDGE_COMPATIBILITY.json` uses `modMin`/`modMax` in addition to protocol numbers so V1.1/V1.2 are not selected accidentally.

Supported V1.3 and V1.4-era installations use the compatibility index to reach Bridge V1.42 directly. Once Bridge V1.42 is installed, `UPDATE_DUALSENSE.exe` selects the matching complete release, verifies both assets and stages the BeamNG mod ZIP for manual installation. V1.41 onward also uses this unified updater path normally.

Wire generations are not incremented merely because a project release number changes. A future wire-format breaking change may introduce a new schema/generation if technically necessary.
