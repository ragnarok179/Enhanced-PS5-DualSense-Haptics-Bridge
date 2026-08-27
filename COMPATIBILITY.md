# Compatibility

## Public gameplay generations

| BeamNG mod | Gameplay protocol | Required / compatible Bridge |
| --- | ---: | --- |
| V1.0 | 40 | V1.3 or V1.4 |
| V1.1 | 41 | V1.3 or V1.4 |
| V1.2 | 42 | V1.4 |

Protocol numbers are monotonic **compatibility generations**. They must change
whenever a mod release requires a newer Bridge runtime contract or introduces an
incompatible gameplay-wire change. A number is never reused for a different
contract.

Mod V1.2 deliberately uses protocol 42 so Bridge V1.3 cannot silently run it.
Bridge V1.3 officially supports only 40 and 41; receiving V1.2/42 therefore
activates its compatibility updater. Mod V1.2 does not emit the old v40 migration
mirror.

Bridge V1.4 accepts 40, 41 and 42. This means V1.4 may safely be released before
the BeamNG V1.2 mod: users on V1.0/V1.1 continue working, and V1.2 works as soon as
it is later installed.

Speaker, Extended Inputs, Motion and settings side protocols are versioned
independently from the gameplay compatibility generation.

## GitHub release index

Each stable Bridge release publishes `BRIDGE_COMPATIBILITY.json`. The index maps
a GitHub tag to its supported gameplay protocols and runtime ZIP asset. The
updater chooses the newest stable published release that explicitly supports the
requested protocol and refuses downgrades.

For V1.4, publish the runtime asset as `Enhanced_PS5_DualSense_Haptics_Bridge.zip`. This preserves the asset name already consumed by the published V1.3 updater; V1.4 continues to resolve the asset name through `BRIDGE_COMPATIBILITY.json`.

The compatibility index points to the historical `...Haptics_Bridge.zip` name so
Bridge V1.3's already-published updater can transition to V1.4 without an asset
name mismatch.
