# Enhanced PS5 DualSense Haptics Bridge

Bring the full DualSense experience to **BeamNG.drive** with **HD stereo haptics, adaptive triggers and dynamic LED lighting**.

This is the companion Windows Bridge for the [Enhanced PS5 DualSense Haptics](https://www.beamng.com/resources/enhanced-ps5-dualsense-haptics.38997/) BeamNG.drive mod.

Designed to be simple and automatic, it does **not require large third-party controller software such as DSX or DS4Windows**. Instead, this lightweight Bridge is made specifically for BeamNG.drive, communicates directly with the DualSense and automatically handles USB or Bluetooth.

Once installed, simply connect the controller and start the Bridge.


## Main features

- **HD stereo haptics:** surfaces, suspension, bumps, collisions, landings, tyres, transmission, etc.
- **Adaptive triggers:** L2 braking/ABS/wheel lock and R2 throttle/wheelspin/TCS/shift feedback and airborne mode.
- **Dynamic lighting:** RPM-reactive DualSense lightbar and rev-limiter behavior.


## Installation

1) Download and install [Enhanced PS5 DualSense Haptics](https://www.beamng.com/resources/enhanced-ps5-dualsense-haptics.38997/) through the BeamNG Repository.
2) Download the `Enhanced_PS5_DualSense_Haptics_Bridge.zip` asset from the latest GitHub Release (not GitHub's automatically generated Source code ZIP). Extract the **complete Bridge folder** anywhere on Windows. Do not run the Bridge directly from the ZIP and do not move individual files out of the extracted folder.
3) Start BeamNG.drive and **disable BeamNG's native controller vibration** while using this mod.
4) Connect the DualSense through USB or Bluetooth.
5) Run `START_BRIDGE.exe`, or use `START_BRIDGE_AND_BEAMNG.exe` to automatically start both the Bridge and BeamNG.drive.

Notes:
The Bridge can be launched at any time, before or after BeamNG.drive, as long as the DualSense is connected to the PC. No specific launch order is required.

If you disconnect the DualSense or switch between USB and Bluetooth, reconnect the controller and restart the Bridge.

For a more precise and spatialized experience, it is recommended to use the DualSense controller via wired USB rather than Bluetooth. Bluetooth uses a lower-bandwidth haptic transport and cannot reproduce the full HD haptic detail available over USB.

The USB path uses a low-latency WASAPI queue and avoids keeping the Windows audio buffer permanently filled with silence. This reduces the delay that could be felt in earlier USB builds.


## Controller settings

Controller feedback can be configured directly inside BeamNG.drive. Open **Pause -> Mods -> DualSense Haptics Settings**, or use the **DualSense Haptics Settings** action in **Options -> Controls**. A default `O` binding is included and can be remapped.

The in-game page exposes percentage-based controls with explicit semantics:

- **Haptic output level**: a true global output multiplier. **100%** preserves the calibrated mix. This is also the truthful ceiling for the group because hard impacts can already reach full-scale PCM.
- **Road surface master**: a true global surface multiplier. **100%** preserves the calibrated material mix.
- **Advanced surfaces**: independent **Rolling power** and **Slip power** values for every detected material. Rolling defaults preserve the calibrated renderer. Slip defaults now show the measured incremental slip power versus the strongest calibrated suspension bump; the displayed default still maps to runtime gain 1.0, while higher values provide extra headroom.
- **L2 start/end resistance**: two independent endpoints for the normal brake-trigger resistance; the hardware interpolates between them.
- **R2 start/end resistance**: two independent endpoints for the normal throttle-trigger resistance. Equal values give a constant trigger; different values create an increasing or decreasing resistance curve.
- **Bumps / impacts**: remains at **100%** by default because the strongest calibrated suspension/collision events can already reach the final PCM ceiling; weaker events still scale dynamically with physical severity.
- **L2 brake resistance** and **L2 ABS kick**: percentage controls converted to normalized trigger forces before gameplay processing.
- **R2 base resistance**: normal constant accelerator resistance only.
- **R2 jolt strength**: TCS/rev-limiter vibration peak only. The calibrated strong-TCS peak is **25%**; 100% requests the maximum trigger-vibration amplitude. The dynamic-effects switch also disables wheelspin/shift/airborne unload cues.
- **LED lighting**: the calibrated lightbar brightness is **86%**; 100% uses the remaining RGB output headroom.

Settings are stored by BeamNG.drive in the user settings folder and survive normal mod updates. The Bridge console settings menu is diagnostic-only and is not started during normal use.

The Bridge uses one transport-neutral trigger model. BeamNG authors semantic effects (`resistance`, `vibration` or `fine`) with normalized `0.0..1.0` values. Trigger force is then represented internally by one common **0..48 force lattice**, matching the highest-resolution Fine Feedback force mode used by the project. There is no active `0..255` trigger-force scale. Fine Feedback writes the 0..48 force directly; Official Feedback/Vibration are reduced to their eight physical strength levels only while the final HID report is packed. Genuine non-force byte fields such as Fine Feedback position, frequency and RGB remain byte-sized where the DualSense protocol requires them.

The calibrated L2 progression, strong ABS rhythm, weak fine-feedback cues and R2 dynamic behavior keep the same intended output levels; only the internal representation and protocol boundaries are simplified. Haptic grip output remains a floating-point/PCM audio path and is never forced through the trigger-force lattice.


## Bridge compatibility and updater

The BeamNG mod announces its mod version and wire-protocol generation over the existing localhost telemetry. This check is entirely local and does **not** contact GitHub. Bridge V1.3 accepts protocol generations 40 and 41.

For the V1.0/V1.2 -> V1.1/V1.3 rollout, compatibility is intentionally two-way during the transition: Bridge V1.3 continues to run the old Mod V1.0 protocol-40 packets, while Mod V1.1 additionally publishes a marked protocol-40 mirror so an installed Bridge V1.2 keeps working until the user runs its updater. Bridge V1.3 ignores that marked mirror and consumes only the canonical protocol-41 packet, preventing duplicated bumps, triggers or haptics. New V1.1 settings that exist only in the protocol-41 settings payload take full effect after Bridge V1.3 is installed; the legacy lane exists to keep the controller functional during the migration window.

If the mod uses an unsupported protocol, the Bridge explains the mismatch and asks whether to install a compatible Bridge. GitHub is contacted only after the user explicitly answers **Yes**. `UPDATE_BRIDGE.exe` then reads `BRIDGE_COMPATIBILITY.json` from a published stable GitHub Release, selects the newest published stable Bridge that supports the detected protocol, verifies the downloaded package with `SHA256SUMS.txt`, installs it transactionally and restarts the Bridge. If no compatible release is available yet or GitHub cannot be reached, nothing is installed and the updater asks the user to try again later.

`UPDATE_BRIDGE.exe` can still be launched manually at any time. Manual launch itself is the explicit request to check GitHub. The updater installs only official `Enhanced_PS5_DualSense_Haptics_Bridge.zip` release assets and never installs executable files from the development branch.

Protocol numbers are compatibility generations, not public release numbers. They only increase for breaking Mod/Bridge wire changes (`41 -> 42 -> 43...`) and an older number is never reused. A Bridge release may support several generations at once.

> The Bridge must be closed while files are replaced. Compatibility mode handles that handoff automatically; BeamNG.drive can stay open.

> Maintainers: push the complete repository to `main` first so legacy V1.1 users can migrate, then publish the stable GitHub Release with two assets named exactly `Enhanced_PS5_DualSense_Haptics_Bridge.zip` and `BRIDGE_COMPATIBILITY.json`. `SHA256SUMS.txt` is inside the official ZIP.


## Troubleshooting

**Mod installed but no feedback?** Make sure the mod is enabled in BeamNG's Mod Manager, then reload the current vehicle with CTRL+R. If it still does not initialize, restart BeamNG.drive.
- Do not run DSX, DS4Windows or another application that controls DualSense haptics or LEDs at the same time.
- If the DualSense does not work correctly in BeamNG, disable Steam Input for BeamNG.drive in Steam > Properties > Controller.
- Make sure Microsoft GameInput 3.0 or newer is installed. GameInput can be installed or updated from an administrator terminal with: `winget install Microsoft.GameInput`
- Updating the DualSense firmware through PlayStation Accessories is recommended.


## Diagnostic tools

The Bridge includes additional troubleshooting and testing tools under:

`Tools and diagnostics\Diagnostics\`

Available tools include:

- Installation/package verification
- Combined USB/Bluetooth hardware and audio detection
- USB stereo testing
- Bluetooth stereo testing
- USB diagnostic logging
- Bluetooth diagnostic logging
- Bridge log collection

These tools are mainly intended for troubleshooting and bug reports. Normal Bridge launches keep live telemetry/status logging disabled to avoid unnecessary console I/O; the diagnostic log launchers enable it explicitly.


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
- Airborne mode


### R2 — Throttle

R2 provides dynamic throttle feedback including:

- Normal throttle resistance
- Wheelspin feedback
- TCS feedback
- Gear-shift feedback
- Airborne mode


## Dynamic lighting

The DualSense lightbar reacts dynamically to engine RPM like BeamNG in 0.39.

Its behavior follows the engine rev range and provides dedicated feedback when the vehicle reaches the rev limiter.


## Code signing policy

Free code signing provided by [SignPath.io](https://signpath.io/), certificate by [SignPath Foundation](https://signpath.org/).

### Team roles

- Committer and reviewer: [Ragnarok179](https://github.com/ragnarok179)
- Approver: [Ragnarok179](https://github.com/ragnarok179)

### Privacy

This program will not transfer any information to other networked systems unless specifically requested by the user or the person installing or operating it.

BeamNG.drive vehicle telemetry is received and processed locally to generate controller feedback. No gameplay telemetry or personal user data is sent to the developer or to analytics services.

Network access to GitHub occurs only for actions explicitly requested by the user, such as manually running the updater or accepting the compatibility-update prompt. The local Mod/Bridge protocol check itself performs no network request.

## License

MIT.

See `LICENSE` and `THIRD_PARTY_NOTICES.md`.


## Recent additions

- BeamNG can now send persistent in-game user settings to the Bridge.
- Added support for advanced per-surface haptic strength overrides from the BeamNG settings menu.
