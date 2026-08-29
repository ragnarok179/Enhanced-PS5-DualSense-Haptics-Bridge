# Enhanced PS5 DualSense Haptics

**Enhanced PS5 DualSense Haptics** brings a more complete DualSense experience to **BeamNG.drive**, using stereo haptics, adaptive triggers, controller-speaker audio, touchpad inputs, motion controls and additional DualSense buttons.

Designed to be simple and automatic, the project **does not require DSX or DS4Windows**. It uses a lightweight dedicated Windows Bridge made specifically for BeamNG.drive, which communicates directly with the DualSense and automatically handles USB or Bluetooth.

The project is made of two matching parts:

- **Enhanced PS5 DualSense Haptics** — BeamNG.drive mod
- **Enhanced PS5 DualSense Haptics Bridge** — Windows Bridge

Starting with **V1.41**, both components always use the **same release version** and are published together.

---

## Main features

### Stereo haptics

- Multiple road surfaces
- Suspension feedback
- Bumps and road irregularities
- Impacts
- Collisions
- Landings
- Tyre behaviour
- Wheelspin
- ABS
- Wheel lock
- Traction control
- Gear shifts
- Rev limiter
- Airborne feedback

### Adaptive triggers

**L2 — Brake**

- Dynamic braking resistance
- ABS feedback
- Wheel-lock feedback
- Airborne behaviour

**R2 — Throttle**

- Dynamic throttle resistance
- Wheelspin feedback
- TCS feedback
- Gear-shift feedback
- Airborne behaviour

### Dynamic lighting

- Engine RPM
- Rev-limiter behaviour

### Controller speaker

- BeamNG collision sounds through the DualSense speaker
- USB and Bluetooth support
- Controller-only output
- Controller + PC output

### Bindable DualSense touchpad inputs

- One-finger touch and movement
- Two-finger touch and movement
- Swipe inputs
- Relative analog inputs
- Absolute analog inputs
- Windows mouse mode

### Bindable motion inputs

- Rotation angle
- Rotation speed / gyroscope
- Tilt
- Orientation
- Accelerometer X / Y / Z
- Axis and button modes
- Calibration and centering
- Deadzones and sensitivity options

### Additional bindable DualSense buttons

- Create button
- PlayStation button *(Steam Input must be disabled when required)*
- Mute button

---

## Installation

Because the advanced DualSense features require the dedicated Windows Bridge, the complete project is distributed through **GitHub Releases** rather than the BeamNG Repository.

Download **both ZIP files from the same release**:

- `Enhanced_PS5_DualSense_Haptics_Vx.xx.zip` — BeamNG mod
- `Enhanced_PS5_DualSense_Haptics_Bridge_Vx.xx.zip` — Windows Bridge

Do **not** download GitHub's automatically generated **Source code** archives or `BRIDGE_COMPATIBILITY.json` for a normal installation.

1. Place the **BeamNG mod ZIP** in your BeamNG `mods` folder. Do not extract it.
2. Extract the complete **Bridge ZIP** anywhere on Windows.
3. Connect the DualSense through USB or Bluetooth.
4. Start BeamNG.drive and disable BeamNG's native controller vibration while using the mod.
5. Run `START_BRIDGE.exe`, or use `START_BRIDGE_AND_BEAMNG.exe` to start both the Bridge and BeamNG.drive.

Do not run the Bridge directly from its ZIP and do not move individual files out of the extracted Bridge folder.

You can create a shortcut to `START_BRIDGE.exe` or `START_BRIDGE_AND_BEAMNG.exe` on the desktop or taskbar if you wish.

### Notes

- The Bridge can be launched before or after BeamNG.drive. No specific launch order is required.
- If you disconnect the DualSense or switch between USB and Bluetooth, reconnect the controller and restart the Bridge.
- USB is recommended for the highest-quality and most precise haptic feedback.
- Official public Bridge executables are signed through SignPath before publication.

---

## Architecture

```text
                    BeamNG.drive
                         |
              vehicle physics + events
                         |
                         v
       Enhanced PS5 DualSense Haptics Mod
                         |
            settings + local telemetry
                         |
                         v
                Windows Bridge
          +--------------+--------------+
          |              |              |
          v              v              v
       Haptics         Triggers       Speaker
          |              |              |
          +--------------+--------------+
                         |
                         v
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

The BeamNG mod decides what happens in the game and sends the required local data to the Bridge. The Bridge handles the DualSense-specific Windows communication required for the advanced controller features.

Gameplay telemetry and controller data are processed locally on the PC.

---

## Controls, bindings and settings

Open the mod settings with the default shortcut **O**, or manually through:

**Pause > Mods > Enhanced PS5 DualSense Settings**

The settings interface contains separate controls for haptics, adaptive triggers, controller speaker, touchpad, motion inputs and Bluetooth idle behaviour.

### Binding an input

Touchpad, motion and accelerometer inputs use the normal BeamNG binding workflow:

1. Open the DualSense mod settings.
2. Enable and configure the input you want to use.
3. Open BeamNG's **Controls** menu.
4. Select the game function you want to control.
5. Move, tilt, swipe or activate the corresponding DualSense input.

Once configured, BeamNG can save it like any other controller binding.

---

## Touchpad

### Mouse mode

When no touchpad input is bound in BeamNG, the touchpad can work as a Windows mouse, including outside the game.

When touchpad bindings are active, the touchpad is used as a BeamNG input device instead.

To manually switch between mouse mode and your saved touchpad bindings:

- Bind **Toggle DualSense Touchpad Mouse Mode** in BeamNG's **Controls** menu. It can be assigned to a keyboard key or controller button.
- The **Mouse** button in the mod settings can also force mouse mode manually.
- Existing BeamNG bindings are preserved when switching between both modes.

### Touchpad input types

Each horizontal or vertical channel can be configured independently for one- or two-finger input:

- **Off:** disables the channel.
- **Swipe:** sends a digital input when swiping, similar to a button.
- **Relative:** creates an analog axis relative to where the gesture started; useful for steering, camera movement or other centered controls.
- **Absolute:** maps the finger's physical position on the touchpad directly to an analog axis.

---

## Motion

Motion inputs can generally be exposed as:

- **Axis:** continuous analog control.
- **Button:** triggers an action after the configured threshold is reached.
- **Off:** disables the input when available.

### Motion types

**Rotation angle**

Measures how far the controller has rotated from its centered position. This is useful for steering-like controls. **Auto calibration** can detect the physical axis you naturally rotate around.

**Rotation speed**

Measures how quickly the controller is rotating and returns toward zero when movement stops.

**Tilt**

Measures the controller's lean using gravity. This is useful when the physical tilt itself should act as an analog input.

**Orientation**

Tracks the controller's orientation relative to its centered position. This is useful when the current orientation itself should determine the input.

Modes that use a center reference can be recentered directly from the mod settings.

---

## Accelerometer

**Acceleration X, Y and Z** measure the DualSense's linear acceleration and react to movements, pushes and shakes.

Each direction can be configured as:

- **Off:** disables that direction.
- **Axis:** outputs a continuous analog value.
- **Button:** triggers a BeamNG action when acceleration exceeds the configured threshold.

---

## Updating

Starting with **V1.41**, the BeamNG mod and Windows Bridge always use the same release version and are updated as one project release.

Updates are **manual**. Run:

`UPDATE_DUALSENSE.exe`

If an update is available, confirm it in the updater. When it finishes, follow the **single BeamNG installation tutorial** that opens with the downloaded mod ZIP.

### If your old mod came from the BeamNG Repository

The installation tutorial guides you through these steps:

1. Open BeamNG.drive.
2. Open **Mod Manager**.
3. Select **Enhanced PS5 DualSense Haptics**.
4. Open its **Repository page** from the mod entry.
5. Unsubscribe and remove the old Repository version.
6. Close BeamNG.drive.
7. Move the downloaded `Enhanced_PS5_DualSense_Haptics_Vx.xx.zip` into **your own BeamNG mods folder**.
8. Do not extract the ZIP.
9. Start BeamNG.drive and make sure the mod is enabled.

### If your old mod was installed manually

1. Close BeamNG.drive.
2. Remove the old Enhanced PS5 DualSense Haptics ZIP.
3. Move the newly downloaded mod ZIP into your BeamNG `mods` folder.
4. Do not extract the ZIP.
5. Start BeamNG.drive and make sure the mod is enabled.

The updater does not need to know where your BeamNG user folder is located. This also works with moved or custom BeamNG user folders.

### One-time migration from older Bridge updaters

Older `UPDATE_BRIDGE.exe` versions can update only the Bridge. They remain able to reach **Bridge V1.41** through the legacy compatibility system.

If you arrive at V1.41 through an older updater, Bridge V1.41 displays a short one-time migration message:

1. Close the Bridge.
2. Run `UPDATE_DUALSENSE.exe` from the Bridge folder.
3. Follow the update instructions shown by `UPDATE_DUALSENSE.exe`.

After this one-time migration, future updates use the normal V1.41+ update flow above.

`UPDATE_BRIDGE.exe` is temporarily kept as an alias of `UPDATE_DUALSENSE.exe` for old shortcuts and installations.

---

## Troubleshooting

**Mod installed but no feedback?** Make sure the mod is enabled in BeamNG's Mod Manager, then reload Lua with **Ctrl+L**. If it still does not initialize, restart BeamNG.drive.

- Do not run DSX, DS4Windows or another application controlling DualSense haptics or LEDs at the same time.
- If the DualSense does not work correctly in BeamNG, disable Steam Input under **Steam > BeamNG.drive > Properties > Controller**.
- Make sure Microsoft GameInput 3.0 or newer is installed. It can be installed or updated from an administrator terminal with:

```text
winget install Microsoft.GameInput
```

- Updating the DualSense firmware through PlayStation Accessories is recommended.
- Run `UPDATE_DUALSENSE.exe` manually to check for a new complete release.

---

## Features currently under development

- DualSense Edge extended inputs
- Further haptic improvements
- UI/UX improvements
- Compatibility and reliability improvements
- Additional uses for DualSense hardware where possible

---

## V1.41 release changes

- Unified the BeamNG mod and Windows Bridge under the same release version.
- Added the new manual `UPDATE_DUALSENSE.exe` update flow for complete Mod + Bridge releases.
- Added the one-time migration path from older Bridge-only updaters.
- Kept `UPDATE_BRIDGE.exe` temporarily as a compatibility alias.
- Added a single user-facing BeamNG installation tutorial for the normal update flow.
- Kept the validated haptics, adaptive triggers, controller speaker, touchpad, motion, lighting and USB/Bluetooth behaviour unchanged.

---

## Feedback, suggestions and discussion

Questions, suggestions and bug reports are welcome on the BeamNG forum thread:

[**Enhanced PS5 DualSense Haptics — Feedback, Support & Suggestions**](https://www.beamng.com/threads/enhanced-ps5-dualsense-haptics-feedback-support-suggestions.111320/)

If you encounter unexpected behaviour or have an idea for another DualSense feature that would work well in BeamNG.drive, feel free to report it there.

### Development note

This is a community project. AI-assisted tools have been used for some complex development tasks, especially low-level byte conversions and packet handling. Changes are tested before public release, and feedback or corrections are welcome.

---

## Code signing

Free code signing is provided by [SignPath.io](https://signpath.io/), with a certificate provided by the [SignPath Foundation](https://signpath.org/).

### Team roles

- Committer and reviewer: [Ragnarok179](https://github.com/ragnarok179)
- Approver: [Ragnarok179](https://github.com/ragnarok179)

---

## Privacy

BeamNG.drive gameplay telemetry and controller data are processed locally on the PC. No gameplay telemetry or personal user data is sent to the developer or to analytics services.

Network access is used for user-requested actions such as checking GitHub Releases through the manual updater.

---

## Requirements

- Windows 10/11
- BeamNG.drive
- Sony DualSense controller
- Microsoft GameInput 3.0 or newer

---

## License

MIT. See `LICENSE` and `THIRD_PARTY_NOTICES.md`.
