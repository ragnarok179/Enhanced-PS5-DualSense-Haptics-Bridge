# Enhanced PS5 DualSense Haptics Bridge

Bring the full DualSense experience to **BeamNG.drive** with **HD stereo haptics, adaptive triggers and dynamic LED lighting**.

This is the companion Windows Bridge for the [Enhanced PS5 DualSense Haptics](https://www.beamng.com/resources/enhanced-ps5-dualsense-haptics.38997/) BeamNG.drive mod.

Designed to be simple and automatic, it does **not require large third-party controller software such as DSX or DS4Windows**. Instead, this lightweight Bridge is made specifically for BeamNG.drive, communicates directly with the DualSense and automatically handles USB or Bluetooth.

Once installed, simply connect the controller and start the Bridge.


## Main features

- **HD stereo haptics:** surfaces, suspension, bumps, collisions, landings, tyres, transmission, etc.
- **Adaptive triggers:** L2 braking/ABS/wheel lock and R2 throttle/wheelspin/TCS/shift feedback and airborn mode.
- **Dynamic lighting:** RPM-reactive DualSense lightbar and rev-limiter behavior.


## Installation

1) Download and install [Enhanced PS5 DualSense Haptics](https://www.beamng.com/resources/enhanced-ps5-dualsense-haptics.38997/) through the BeamNG Repository.
2) Download the latest Bridge release from GitHub. Extract the **complete Bridge folder** anywhere on Windows. Do not run the Bridge directly from the ZIP and do not move individual files out of the extracted folder.
3) Start BeamNG.drive and **disable BeamNG's native controller vibration** while using this mod.
4) Connect the DualSense through USB or Bluetooth.
5) Run START_BRIDGE.bat, or use START_BRIDGE_AND_BEAMNG.bat to automatically start both the Bridge and BeamNG.drive.

Notes :
The Bridge can be launched at any time, before or after BeamNG.drive, as long as the DualSense is connected to the PC. No specific launch order is required.

If you disconnect the DualSense or switch between USB and Bluetooth, reconnect the controller and restart the Bridge.

For a more precise and spatialized experience, it is recommended to use the DualSense controller via wired USB rather than Bluetooth. Bluetooth uses a lower-bandwidth haptic transport and cannot reproduce the full HD haptic detail available over USB.


## Troubleshooting

**Mod installed but no feedback?** Make sure the mod is enabled in BeamNG's Mod Manager, then reload the current vehicle with CTRL+R. If it still does not initialize, restart BeamNG.drive.
- Do not run DSX, DS4Windows or another application that controls DualSense haptics or LEDs at the same time.
- If the DualSense does not work correctly in BeamNG, disable Steam Input for BeamNG.drive in Steam > Properties > Controller.
- Microsoft GameInput 3.0 or newer is installed. GameInput can be easily installed or updated from an administrator terminal using the command: winget install Microsoft.GameInput
- Updating the DualSense firmware through PlayStation Accessories is recommended.


## Diagnostic tools

The Bridge includes additional troubleshooting and testing tools under:

`Tools and diagnostics\Diagnostics\`

Available tools include:

- Hardware detection
- Audio-output detection
- USB stereo testing
- Bluetooth stereo testing
- Bluetooth bump-carrier testing
- USB diagnostic logging
- Bluetooth diagnostic logging
- Bridge log collection

These tools are mainly intended for troubleshooting and bug reports.


## How it works

BeamNG.drive sends local vehicle telemetry to the Bridge.

The Bridge interprets this telemetry using a shared physics-driven haptic engine:

```text
BeamNG.drive vehicle physics
            |
            v
      Local telemetry
            |
            v
     Common Feel Engine
            |
            v
 Canonical 48 kHz stereo haptics
            |
      +-----+-----+
      |           |
      v           v
     USB       Bluetooth
   48 kHz       adapted
 HD haptics     transport
```


## Simulated feedback

The system can react to vehicle physics and events including:

- Asphalt and other road textures
- Wet and slippery surfaces
- Gravel
- Dirt
- Cobblestone
- Rumble strips
- Sand
- Mud
- Grass
- Snow
- Ice
- Rock
- Suspension movement
- Individual wheel impacts
- Bumps and road irregularities
- Collisions
- Landings
- Wheelspin
- Traction-control intervention
- ABS
- Wheel lock
- Gear shifts
- Rev limiter
- Vehicle airborne state
- Engine RPM


## HD stereo haptics

Road textures and vehicle events are generated from BeamNG's live physics rather than using generic vibration presets.

The stereo haptic system allows physical events to be spatialized across the DualSense, making impacts and road feedback correspond more closely to what happens on the left and right sides of the vehicle.

The haptic engine includes feedback for road surfaces, suspension movement, wheel impacts, collisions, landings, tyre behavior, transmission events and other vehicle dynamics.


## Adaptive triggers

### L2 — Brake

L2 provides dynamic brake feedback including:

- Normal braking resistance
- ABS pulse feedback
- Locked-wheel behavior
- Airborn mode


### R2 — Throttle

R2 provides dynamic throttle feedback including:

- Normal throttle resistance
- Wheelspin feedback
- TCS feedback
- Gear-shift feedback
- Airborn mode


## Dynamic lighting

The DualSense lightbar reacts dynamically to engine RPM like BeamNG in 0.39.

Its behavior follows the engine rev range and provides dedicated feedback when the vehicle reaches the rev limiter.


## License

MIT.

See `LICENSE` and `THIRD_PARTY_NOTICES.md`.
