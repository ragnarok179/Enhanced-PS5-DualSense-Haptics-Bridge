# BeamNG telemetry protocol

The BeamNG mod and bridge communicate over localhost UDP using JSON telemetry.
The schema is intentionally transport-neutral: USB and Bluetooth consume the
same decoded `telemetry` state.

Current bridge protocol version: `40`.

Important categories include:

- vehicle speed / RPM / limiter state;
- ABS, TCS, wheelspin and grounded/airborne state;
- left/right surface material, roughness, excitation and slip;
- discrete body events (suspension bump, secondary axle, landing, collision);
- adaptive-trigger state and RPM-light ownership flags.

The bridge ignores packets with a different protocol version. Protocol version
changes are independent from the public product version, which remains `V1`.
