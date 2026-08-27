# Enhanced PS5 DualSense Haptics Bridge

Bring the DualSense experience to **BeamNG.drive** with stereo haptics, adaptive triggers, dynamic lighting, controller-speaker audio and extended touchpad/motion inputs.

This is the companion Windows Bridge for the **Enhanced PS5 DualSense Haptics** BeamNG.drive mod. It is designed specifically for BeamNG.drive and supports a DualSense connected through USB or Bluetooth without requiring DSX or DS4Windows.

## Main features

- **Stereo haptics** for road surfaces, suspension, bumps, impacts, landings, tyres and other vehicle feedback.
- **Adaptive triggers** for braking, ABS, wheel lock, throttle, wheelspin, TCS, shifts and airborne feedback.
- **Dynamic lighting** driven by engine RPM and the rev limiter.
- **Controller speaker** support for BeamNG collision sounds.
- **Touchpad inputs** with automatic mouse fallback and configurable BeamNG controls.
- **Motion inputs** using rotation, gyroscope, tilt, orientation and acceleration.

## Installation

1. Install **Enhanced PS5 DualSense Haptics** through the BeamNG Repository.
2. Download `Enhanced_PS5_DualSense_Haptics_Bridge.zip` from the latest GitHub Release. Do not use GitHub's automatically generated source-code ZIP.
3. Extract the complete Bridge folder anywhere on Windows. Do not run it directly from the ZIP and do not move individual files out of the folder.
4. In BeamNG.drive, disable the game's native controller vibration while using this mod.
5. Connect the DualSense through USB or Bluetooth.
6. Run `START_BRIDGE.exe`, or use `START_BRIDGE_AND_BEAMNG.exe` to start both the Bridge and BeamNG.drive.

The Bridge can be launched before or after BeamNG.drive. If the controller is disconnected or switched between USB and Bluetooth, reconnect it and restart the Bridge.

For the most detailed and spatial haptic feedback, USB is recommended. Bluetooth has a lower-bandwidth haptic transport than wired USB.

## Controller settings

All user settings are available directly inside BeamNG.drive:

- **Pause -> Mods -> Enhanced PS5 DualSense Settings**
- Default shortcut: **O**

The settings are grouped by function: haptics, adaptive triggers, speaker, touchpad and motion inputs. They are saved by BeamNG.drive and persist through normal mod updates.

## Architecture

```text
                              BeamNG.drive
                     vehicle physics + sound events
                                  |
                                  v
                 Enhanced PS5 DualSense Haptics mod
              settings / telemetry / input configuration
                                  |
                                  v
                         Local Windows Bridge
                                  |
                  +---------------+---------------+
                  |                               |
                  v                               v
               USB path                      Bluetooth path
                  |                               |
        +---------+---------+           +---------+---------+
        |         |         |           |         |         |
        v         v         v           v         v         v
     Haptics   Triggers   Speaker     Haptics   Triggers   Speaker
     48 kHz      HID      WASAPI      adapted      HID       Opus
        |         |         |           |         |         |
        +---------+---------+           +---------+---------+
                  |                               |
                  +---------------+---------------+
                                  |
                                  v
                               DualSense
                                  |
                    touchpad + motion sensors
                                  |
                                  v
                  DualSense Extended Inputs
                     virtual BeamNG device
                                  |
                                  v
                       BeamNG control bindings
```

The haptic/trigger logic is shared before the final USB or Bluetooth transport. Touchpad and motion data travel in the opposite direction: from the DualSense through the Bridge to the virtual **DualSense Extended Inputs** device used by BeamNG's normal control-binding system.

Gameplay telemetry and controller data are processed locally on the PC.

## Touchpad controls

### Mouse mode

When **no touchpad control is bound in BeamNG**, the touchpad automatically works as a Windows mouse. No manual setup is required for this default behavior.

As soon as at least one touchpad input is successfully bound in BeamNG, the automatic mouse fallback stops and the touchpad is used for those BeamNG bindings instead. Existing bindings are kept when changing modes.

If you want to switch manually between your saved touchpad bindings and mouse control, assign the BeamNG action:

**Toggle DualSense Touchpad Mouse Mode**

You can find it in BeamNG's **Controls** options and bind it to a normal keyboard key or controller button. The **Mouse** button in the mod settings can also force mouse mode manually.

### Choosing what the touchpad sends

Open the mod settings with **O**, then go to the Touchpad section. Each horizontal/vertical channel for one or two fingers can be configured independently:

- **Off** — disables that channel.
- **Swipe** — sends a short digital input when a swipe direction is detected; useful for actions that normally use a button.
- **Relative** — creates an analog axis relative to the point where the gesture started; useful for steering, camera movement or other centered controls.
- **Absolute** — maps the physical position on the pad directly to an analog axis.

After choosing the type you want, open BeamNG's **Controls** menu, select the game function you want to control and perform the corresponding touchpad gesture. BeamNG will see it through **DualSense Extended Inputs** and can save it like any other controller binding.

## Motion and accelerometer controls

Motion controls use the same two-step workflow:

1. Open the mod settings with **O** and choose what the DualSense sensor should output.
2. Open BeamNG's **Controls** menu and bind the newly exposed axis or button to the game function you want.

A motion slot can be exposed as an **Axis** for continuous control or as a **Button** for threshold-based actions.

### Motion types

- **Rotation angle** — measures how far the controller has rotated away from its centered position around the selected axis. This is suited to steering-like controls or a camera controlled by controller angle.
- **Rotation speed** — measures how fast the controller is currently rotating. The output returns toward zero when you stop moving it, making it useful for camera/look rate or gesture-like controls.
- **Tilt** — measures left/right or forward/back lean using gravity. It is useful when you want the controller's physical tilt to act like a centered analog control.
- **Orientation** — tracks roll, pitch or yaw relative to the centered orientation. It is useful when the controller's orientation itself should determine the input.

For **Rotation angle**, Auto calibration can learn the physical axis you naturally rotate around. Modes that use a center reference can also be recentered from the mod UI.

### Accelerometer

Acceleration X, Y and Z use the DualSense's **linear acceleration**: they react to movement, pushes and shakes rather than the controller simply being held at an angle.

For each acceleration direction, choose:

- **Axis** for a continuous analog value.
- **Button** to trigger a BeamNG action when acceleration passes the configured threshold.
- **Off** when that channel is not needed.

Once the mode is enabled, bind it from BeamNG's normal **Controls** menu through **DualSense Extended Inputs**.

## Simulated feedback

The system can react to vehicle physics and events including:

- Asphalt and other road textures
- Wet and slippery surfaces
- Gravel, dirt, cobblestone and rumble strips
- Sand, mud, grass, snow, ice and rock
- Suspension movement and individual wheel impacts
- Bumps, collisions and landings
- Wheelspin and traction-control intervention
- ABS and wheel lock
- Gear shifts and rev limiter
- Vehicle airborne state
- Engine RPM

## Stereo haptics

Road textures and vehicle events are generated from BeamNG's live physics rather than generic vibration presets. Stereo feedback allows impacts and road effects to correspond more closely to what happens on the left and right sides of the vehicle.

## Adaptive triggers

### L2 — Brake

L2 can provide normal braking resistance, ABS pulses, locked-wheel feedback and airborne behavior.

### R2 — Throttle

R2 can provide throttle resistance, wheelspin, TCS, gear-shift feedback and airborne behavior.

## Dynamic lighting

The DualSense lightbar reacts to engine RPM and provides dedicated behavior near the rev limiter.

## Troubleshooting

**Mod installed but no feedback?** Make sure the mod is enabled in BeamNG's Mod Manager, then reload the current vehicle with `CTRL+R`. If it still does not initialize, restart BeamNG.drive.

- Do not run DSX, DS4Windows or another application that controls DualSense haptics or LEDs at the same time.
- If the DualSense does not work correctly in BeamNG, disable Steam Input for BeamNG.drive in Steam -> Properties -> Controller.
- Make sure Microsoft GameInput 3.0 or newer is installed. It can be installed or updated from an administrator terminal with `winget install Microsoft.GameInput`.
- Updating the DualSense firmware through PlayStation Accessories is recommended.

## Code signing

Free code signing is provided by [SignPath.io](https://signpath.io/), with a certificate provided by the [SignPath Foundation](https://signpath.org/).

### Team roles

- Committer and reviewer: [Ragnarok179](https://github.com/ragnarok179)
- Approver: [Ragnarok179](https://github.com/ragnarok179)

### Privacy

BeamNG.drive gameplay telemetry and controller data are processed locally. No gameplay telemetry or personal user data is sent to the developer or to analytics services.

Network access is used only for actions explicitly requested by the user, such as update actions through GitHub.

## License

MIT.

See `LICENSE` and `THIRD_PARTY_NOTICES.md`.
