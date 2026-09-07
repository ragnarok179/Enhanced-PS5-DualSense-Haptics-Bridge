# Enhanced PS5 DualSense Haptics

**Enhanced PS5 DualSense Haptics** brings the full DualSense immersive experience to BeamNG.drive.

Designed to be simple and automatic, the mod **does NOT require DSX or DS4Windows.** It uses a **lightweight dedicated Bridge** made specifically for BeamNG.drive, which communicates directly with the DualSense and automatically handles **USB or Bluetooth**.

---

## Main features

- **Stereo haptics:**
  - Multiple road surfaces
  - Suspension
  - Bumps
  - Impacts
  - Tyres
  - Airborne feedback
- **Adaptive triggers:**
  - Braking
  - ABS
  - Wheel lock
  - Throttle
  - Wheelspin
  - TCS
  - Shifts
  - Airborne feedback
- **Dynamic lighting:**
  - Engine RPM
  - Rev limiter
- **Controller speaker:**
  - Collision sounds
- **Bindable DualSense touchpad inputs:**
  - One-finger touch and movement
  - Two-finger touch and movement
  - Touchpad **mouse mode**
- **Bindable motion inputs:**
  - Rotation
  - Gyroscope
  - Tilt
  - Orientation
  - Acceleration
- **Additional DualSense bindable buttons:**
  - Create / Share button
  - PlayStation button *(Steam Input disabled)*
  - Mute button
- **Local Split Screen compatibility:**
  - Designed to work alongside the **Local Split Screen** mod for multi-controller local play.

---


# Installation

1. Download both ZIP files from the [**latest GitHub Release**](https://github.com/ragnarok179/Enhanced-PS5-DualSense-Haptics-Bridge/releases):
   - `Enhanced_PS5_DualSense_Haptics_Vx.xx.zip`
   - `Enhanced_PS5_DualSense_Haptics_Bridge_Vx.xx.zip`
   - Do **not** download GitHub's automatically generated **Source code** archives or `BRIDGE_COMPATIBILITY.json`.
2. Place the **BeamNG mod ZIP** in your BeamNG mods folder.
3. Extract the complete **Bridge ZIP** anywhere on Windows.
   - Do not move individual files out of the extracted Bridge folder.
4. Connect your DualSense through **USB or Bluetooth**.
5. Start BeamNG.drive and **disable BeamNG's native controller vibration** while using this mod.
6. Run `START_BRIDGE.exe`.
   - Or use `START_BRIDGE_AND_BEAMNG.exe` to automatically start both the Bridge and BeamNG.drive.
   - You can create a shortcut to either launcher on your desktop or taskbar if you wish.

### Notes

- The Bridge can be launched before or after BeamNG.drive, as long as the DualSense is connected to the PC.
- No specific launch order is required.
- If you disconnect the DualSense or switch between USB and Bluetooth, reconnect the controller and restart the Bridge.
- Do not run DSX, DS4Windows or another application controlling DualSense haptics or LEDs at the same time.
- **Local Split Screen:** controller assignment remains managed by BeamNG / Local Split Screen while this Bridge handles the DualSense-specific features.


## Legacy update path (V1.3+)

Supported legacy migration starts at **V1.3**.

- A **V1.3** installation can update its Bridge **directly to V1.42** through `BRIDGE_COMPATIBILITY.json`.
- A **V1.4-era** installation can also update **directly to V1.42** the same way.
- After the Bridge reaches **V1.42**, run `UPDATE_DUALSENSE.exe` to download the matching BeamNG mod ZIP and follow the installation tutorial it opens.
- Versions older than **V1.3** are intentionally outside the supported migration range.

V1.41 remains a historical release, but it is no longer a required migration hub for supported V1.3+ updates.

---

# Controls, bindings and settings

## Opening the settings

Open the mod settings using the default shortcut: **O**

Or manually through: **Pause > Mods > DualSense Haptics Settings**

---

## Binding an input

Touchpad, motion and accelerometer inputs all use the same binding workflow:

1. Open the mod settings.
2. Enable and configure the input you want to use.
3. Open BeamNG's **Controls** menu.
4. Select the game function you want to control.
5. Move, tilt, swipe or activate the corresponding DualSense input.

Once configured, BeamNG can save it like any other controller binding.

---

# Touchpad

## Mouse mode

When no touchpad input is bound in BeamNG, the touchpad automatically works as a **Windows mouse**, including outside the game.

As soon as at least one touchpad input is bound, automatic mouse mode is disabled and the touchpad is used for BeamNG bindings instead.

To manually switch between mouse mode and your saved touchpad bindings:

- Bind the BeamNG action **“Toggle DualSense Touchpad Mouse Mode”** in the BeamNG **Controls** menu.
  - It can be assigned to a keyboard key or controller button.
- The **Mouse** button in the mod settings can also force mouse mode manually.
- Existing bindings are preserved when switching between both modes.

## Touchpad input types

Each horizontal or vertical channel can be configured independently for one- or two-finger input:

- **Off**: disables the channel.
- **Swipe**: sends a digital input when swiping, similar to a button.
- **Relative**: creates an analog axis relative to where the gesture started; suited to steering, camera movement or other centered controls.
- **Absolute**: maps the finger's physical position on the touchpad directly to an analog axis.

---

# Motion

Motion inputs can generally be exposed as:

- **Axis**: continuous analog control.
- **Button**: triggers an action after a configured threshold is reached.
- **Off**: disables the input when available.

## Motion types

- **Rotation angle**:
  - Measures how far the controller has rotated from its centered position. Suited to steering-like controls.
  - **Auto calibration** can detect the physical axis you naturally rotate around.
- **Rotation speed**:
  - Measures how quickly the controller is rotating.
  - Returns toward zero when the controller stops moving.
- **Tilt**:
  - Measures the controller's lean using gravity. Useful when physical tilt itself should act as an analog input.
- **Orientation**:
  - Tracks the controller's orientation relative to its centered position. Useful when the current orientation itself should determine the input.

Modes that use a center reference can be recentered directly from the mod settings.

---

# Accelerometer

**Acceleration X, Y and Z** measure the DualSense's linear acceleration and react to movements, pushes and shakes.

Each direction can be configured as:

- **Off**: disables that direction.
- **Axis**: outputs a continuous analog value.
- **Button**: triggers a BeamNG action when acceleration exceeds the configured threshold.

---

# Troubleshooting

**Mod installed but no feedback?** Make sure the mod is enabled in BeamNG's Mod Manager, then reload Lua with **Ctrl+L**. If it still does not initialize, restart BeamNG.drive.

- Do not run DSX, DS4Windows or another application that controls DualSense haptics or LEDs at the same time.
- If the DualSense does not work correctly in BeamNG, disable Steam Input for BeamNG in **Steam > Properties > Controller**.
- Make sure Microsoft GameInput 3.0 or newer is installed. GameInput can be installed or updated from an administrator terminal using: `winget install Microsoft.GameInput`
- Updating the DualSense firmware through PlayStation Accessories is recommended.
- Run `UPDATE_DUALSENSE.exe` to check for updates and follow the displayed instructions.

---

# Architecture

```text
BeamNG.drive
│
├─ Vehicle Lua
│  ├─ dualsensePhysicsTelemetry.lua
│  │  ├─ gameplay / physics telemetry ── UDP 127.0.0.1:6972 ──► USB Bridge
│  │  └─ gameplay / physics telemetry ── UDP 127.0.0.1:6974 ──► Bluetooth Bridge
│  │
│  └─ dualsenseCollisionSoundProbe.lua
│     └─ exact BeamNG collision sound calls
│        └─ queueGameEngineLua()
│
├─ GE Lua
│  ├─ dualsenseCollisionSoundProbeGE.lua
│  ├─ dualsenseControllerSpeaker.lua
│  ├─ touchpad / settings / status channel ── UDP 127.0.0.1:6978 ──► Bridge
│  │
│  ├─ dualsenseExtendedInputs.lua ◄────────── UDP 127.0.0.1:6977 ── Bridge
│  ├─ dualsenseMotionInputs.lua   ◄────────── UDP 127.0.0.1:6979 ── Bridge
│  ├─ dualsenseSuite/virtualInputDevice.lua
│  │  └─ one BeamNG virtual device for touchpad + motion bindings
│  │
│  └─ dualsensePhysicsDirectRPMLights.lua
│     └─ BeamNG-owned DualSense RGB / RPM lighting
│
└─────────────────────────────────────────────────────────────────────────────
                                      │
                                      ▼
                           Windows Bridge (Go)
                                      │
                     ┌────────────────┴────────────────┐
                     │                                 │
                     ▼                                 ▼
          Gameplay telemetry decoder          DualSense HID input
       supported wire generations 41-43      touchpad / buttons / IMU
                     │                                 │
                     ▼                                 ├─► UDP 6977 ─► extended inputs
             Common Feel Engine                        └─► UDP 6979 ─► motion inputs
                     │
          ┌──────────┼───────────┐
          │          │           │
          ▼          ▼           ▼
     Haptic mixer  Triggers  Controller speaker
          │          │           │
          │          │     exact BeamNG collision events
          │          │     + local BeamNG sample decode
          │          │     + overlapping voice mixer
          │          │           │
          └──────────┴───────────┘
                     │
          Canonical Common Feel state
                     │
          ┌──────────┴────────────────────┐
          │                               │
          ▼                               ▼
     USB transport                  Bluetooth transport
  48 kHz WASAPI haptics          48 kHz source -> 3 kHz haptics
  + USB HID trigger state        + single 0x36 HID writer
  + controller speaker           + triggers + speaker packets
          │                               │
          └───────────────┬───────────────┘
                          ▼
                       DualSense
```

### Technical boundaries

- **BeamNG Vehicle Lua** is the source of gameplay/physics telemetry and the authority for collision sound calls.
- **BeamNG GE Lua** handles settings, RGB/RPM lighting and the virtual input layer used by BeamNG bindings.
- **The Bridge** owns DualSense haptics, adaptive triggers, controller-speaker playback and raw DualSense input decoding.
- Gameplay effects are interpreted by a **transport-neutral Common Feel Engine** before the USB/Bluetooth split. USB is the canonical 48 kHz reference path; Bluetooth derives its 3 kHz haptic stream from the same source.
- Bluetooth uses a **single runtime `0x36` HID writer** for haptics, triggers and controller-speaker packets.
- **RGB/RPM lighting stays BeamNG-owned**; the Bridge does not run a competing RGB writer.
- Controller-speaker collision audio is triggered only by **exact BeamNG sound calls**. The Bridge reads the matching BeamNG audio assets locally and does not synthesize replacement collision events.
- All Mod ↔ Bridge communication is local to `127.0.0.1`.

For deeper implementation details, see [`ARCHITECTURE.md`](https://github.com/ragnarok179/Enhanced-PS5-DualSense-Haptics-Bridge/blob/main/Source/docs/ARCHITECTURE.md), [`BEAMNG_PROTOCOL.md`](https://github.com/ragnarok179/Enhanced-PS5-DualSense-Haptics-Bridge/blob/main/Source/docs/BEAMNG_PROTOCOL.md) and [`TRANSPORT_CALIBRATION.md`](https://github.com/ragnarok179/Enhanced-PS5-DualSense-Haptics-Bridge/blob/main/Source/docs/TRANSPORT_CALIBRATION.md).

---

# Updates

Run `UPDATE_DUALSENSE.exe` to check for updates and follow the displayed instructions.

Legacy migration support starts at **V1.3**. Supported V1.3/V1.4-era installations can use their legacy updater and `BRIDGE_COMPATIBILITY.json` to update the **Bridge directly to V1.42**. Once Bridge V1.42 is installed, run `UPDATE_DUALSENSE.exe` to download the matching BeamNG mod ZIP and follow the manual installation instructions.

V1.41 installations already use the unified updater and can update normally to V1.42.

---

# Features currently under development

- DualSense Edge extended inputs
- Further haptic improvements
- UI/UX improvements
- Compatibility and reliability improvements
- Additional uses for DualSense hardware where possible

---

# Feedback, suggestions and bug reports

If you encounter unexpected behaviour, compatibility problems or something that does not feel right, feel free to report it.

Suggestions are welcome too. If there is another DualSense feature that you think would work particularly well in BeamNG.drive, feel free to share your idea.

The BeamNG forum thread can also be used for discussion, troubleshooting and suggestions:

[**Enhanced PS5 DualSense Haptics — BeamNG forum thread**](https://www.beamng.com/threads/enhanced-ps5-dualsense-haptics-feedback-support-suggestions.111320/)

---

# License

This project is released under the **MIT License**.

See [`LICENSE`](LICENSE) for details.

