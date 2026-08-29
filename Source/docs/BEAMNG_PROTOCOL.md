# BeamNG wire generations

The project used explicit gameplay protocol generations while the BeamNG mod and Bridge were distributed and updated separately.

| Generation | Historical release | Purpose in V1.41 |
| ---: | --- | --- |
| 40 | V1.0 | Legacy migration support |
| 41 | V1.1 | Legacy migration support |
| 42 | V1.2 | Legacy migration support |
| 43 | V1.41 | One-time migration generation |

Bridge V1.41 accepts generations 40-43 so older public installations can reach the unified V1.41 release.

From V1.41 onward, normal update compatibility is release-based: the BeamNG mod and Bridge use the same project version and `UPDATE_DUALSENSE.exe` requires both matching assets from one complete GitHub Release, updates the Bridge and stages the BeamNG mod ZIP for manual installation. Wire generations are no longer incremented merely because a project release number changes.

A future wire-format breaking change may still introduce a new schema/generation if technically necessary, but that is separate from normal release versioning.
