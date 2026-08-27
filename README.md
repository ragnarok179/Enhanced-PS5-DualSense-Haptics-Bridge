# Enhanced PS5 DualSense Haptics Bridge

Bring the full DualSense experience to **BeamNG.drive** with stereo haptics, adaptive triggers, dynamic lighting, controller-speaker audio and extended DualSense inputs.

This is the companion Windows Bridge for the **Enhanced PS5 DualSense Haptics** BeamNG.drive mod. It is designed specifically for BeamNG.drive and automatically supports a DualSense connected through USB or Bluetooth without requiring DSX or DS4Windows.

## Main features

- **Stereo haptics:** road surfaces, suspension movement, bumps, impacts, landings, tyres and other vehicle feedback.
- **Adaptive triggers:** braking, ABS, wheel lock, throttle, wheelspin, TCS, gear shifts and airborne feedback.
- **Dynamic lighting:** RPM-reactive DualSense lightbar and rev-limiter behavior.
- **Controller speaker:** BeamNG collision sounds can be reproduced through the DualSense speaker.
- **Touchpad inputs:** mouse control or configurable BeamNG inputs using one or two fingers.
- **Motion inputs:** gyroscope, orientation and accelerometer channels can be mapped to BeamNG controls.

## Installation

1. Install **Enhanced PS5 DualSense Haptics** through the BeamNG Repository.
2. Download `Enhanced_PS5_DualSense_Haptics_Bridge.zip` from the latest GitHub Release. Do not use GitHub's automatically generated source-code ZIP.
3. Extract the complete Bridge folder anywhere on Windows. Do not run it directly from the ZIP and do not move individual files out of the folder.
4. In BeamNG.drive, disable the game's native controller vibration while using this mod.
5. Connect the DualSense through USB or Bluetooth.
6. Run `START_BRIDGE.exe`, or use `START_BRIDGE_AND_BEAMNG.exe` to start both the Bridge and BeamNG.drive.

The Bridge can be launched before or after BeamNG.drive. If the controller is disconnected or switched between USB and Bluetooth, reconnect it and restart the Bridge.

For the most detailed and spatial haptic feedback, USB is recommended. Bluetooth uses a lower-bandwidth haptic transport and therefore cannot reproduce the same level of detail as wired USB.

## Controller settings

All user settings are available directly inside BeamNG.drive:

- **Pause -> Mods -> Enhanced PS5 DualSense Settings**
- Default shortcut: **O**

The settings interface includes separate pages for haptics, adaptive triggers, speaker output, touchpad and motion inputs. Settings are saved by BeamNG.drive and persist through normal mod updates.

## Architecture

```text
                 BeamNG.drive
                      |
          vehicle physics + events
                      |
                      v
      Enhanced PS5 DualSense Haptics
                      |
       local settings / telemetry
                      |
                      v
             Windows Bridge
          +-----------+-----------+
          |           |           |
          v           v           v
       Haptics     Triggers     Speaker
          |           |           |
          +-----------+-----------+
                      |
                 DualSense
                USB / Bluetooth
                      |
          +-----------+-----------+
          |                       |
          v                       v
       Touchpad               Motion sensors
          |                       |
          +-----------+-----------+
                      |
                      v
          BeamNG extended inputs
```

Gameplay telemetry and controller data are processed locally on the PC.

## Touchpad and motion inputs

The Bridge exposes extra DualSense controls to BeamNG through a virtual device named **DualSense Extended Inputs**. The physical DualSense remains the normal game controller; the virtual device is used only for additional touchpad and motion channels.

### Touchpad

The touchpad can be used as:

- **Mouse mode** for controlling the Windows cursor.
- **BeamNG input mode** with configurable one-finger and two-finger horizontal/vertical controls.

Touch channels can be configured as **Off**, **Swipe**, **Relative** or **Absolute**, then assigned through BeamNG's normal bindings interface.

### Motion sensors

Configurable motion inputs can use the DualSense gyroscope and orientation as rotation angle, rotation speed, tilt or orientation. They can be exposed as axes or buttons and adjusted with options such as inversion, calibration, centering, deadzone and sensitivity.

The accelerometer X/Y/Z channels can also be mapped as BeamNG axes or buttons for custom controls such as steering, camera movement or head-look.

Bluetooth idle timeout can also be configured from the Inputs page.

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

L2 can provide:

- Normal braking resistance
- ABS pulse feedback
- Locked-wheel behavior
- Airborne behavior

### R2 — Throttle

R2 can provide:

- Normal throttle resistance
- Wheelspin feedback
- TCS feedback
- Gear-shift feedback
- Airborne behavior

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
