# BeamNG gameplay compatibility protocol

The gameplay protocol is a monotonic compatibility generation used to decide
whether the installed Bridge can safely consume the running mod.

| Mod release | Gameplay protocol | Bridge V1.4 |
| --- | ---: | --- |
| V1.0 | 40 | Supported |
| V1.1 | 41 | Supported |
| V1.2 | 42 | Supported |

Protocol 40 is the original flat trigger representation. Genuine V1.0 packets
are converted from official 1–8 trigger strengths at decode time.

Protocol 41 introduced semantic `l2Effect` / `r2Effect` objects and schema-based
settings telemetry for V1.1.

Protocol 42 is the V1.2 compatibility generation. Its trigger objects remain
semantic, but V1.2 requires Bridge V1.4 behaviour and therefore must not advertise
itself as protocol 41. Mod V1.2 does not publish a legacy v40 mirror.

Bridge V1.4 decodes 40, 41 and 42. A future protocol 43 is rejected until a later
Bridge explicitly declares support. The runtime then creates a compatibility
update request rather than guessing the new contract.
