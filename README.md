# Enhanced PS5 DualSense Haptics

**Enhanced PS5 DualSense Haptics** brings the full DualSense immersive experience to BeamNG.

Designed to be simple and automatic, the mod **does NOT require DSX or DS4Windows.** It uses a **lightweight dedicated Bridge** made specifically for BeamNG, which communicates directly with the DualSense and automatically handles **USB or Bluetooth**.


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
  - Mute button (Button / Toggle modes)


# Installation

1. Download both ZIP files from the [**latest GitHub Release**](https://github.com/ragnarok179/Enhanced-PS5-DualSense-Haptics-Bridge/releases):
   - `Enhanced_PS5_DualSense_Haptics_Vx.xx.zip`
   - `Enhanced_PS5_DualSense_Haptics_Bridge_Vx.xx.zip`
   - Do **not** download GitHub's automatically generated **Source code** archives or `BRIDGE_COMPATIBILITY.json`.
2. Place the **BeamNG mod ZIP** in your BeamNG mods folder.
3. Extract the complete **Bridge ZIP** anywhere on Windows.
   - Do not move individual files out of the extracted Bridge folder.
4. Connect your DualSense through **USB or Bluetooth**.
5. Start BeamNG and **disable BeamNG's native controller vibration** in the game settings.
6. Run `START_BRIDGE.exe`.
   - Or use `START_BRIDGE_AND_BEAMNG.exe` to automatically start both the Bridge and BeamNG.
   - You can create a shortcut to either launcher on your desktop or taskbar if you wish.

### Main settings

- **Adaptive triggers:** adjust the **Start / end deadzone** for L2 and R2 (default: 0–100%), plus the **Resistance strength at start/end of travel**.
- **R2 dynamic effects:** Jolt strength now scales the rev-limiter/TCS feedback relative to the calibrated 25% reference; Wheelspin pulse strength has its own independent control. Driven-wheel slip can pulse during TCS intervention or a stationary burnout (alternating with TCS feedback, not stacking HID effects). Normal throttle resistance, ABS, shift and airborne release are unchanged.
- **Touchpad mouse:** saving a new touchpad binding automatically turns **Mouse Off**. You can turn the mouse back on at any time through the UI or BeamNG shortcut without deleting saved bindings. Other controller bindings do not change this setting.

### Notes

- No specific launch order is required.
- If the DualSense is disconnected, the Bridge stays open and waits for it. Reconnecting through USB or Bluetooth is detected automatically.

#### Compatibility

The mod is compatible with [**Local Split Screen**](https://www.beamng.com/resources/local-split-screen.39292/).


# Controls, bindings and settings

## Opening the settings

Open the mod settings using the default shortcut: **O**

Or manually through: **Pause > Mods > DualSense Haptics Settings**


## Binding an input

Touchpad, motion and accelerometer inputs all use the same binding workflow:

1. Open the mod settings.
2. Enable and configure the input you want to use.
3. Open BeamNG's **Controls** menu.
4. Select the game function you want to control.
5. Move, tilt, swipe or activate the corresponding DualSense input.

Once configured, BeamNG can save it like any other controller binding.


### Mic Mute

In **Extended Inputs**, choose **Button** for a normal press-and-release input or **Toggle** to switch the virtual input on and off with each press. Toggle also provides a 0–100% virtual axis for BeamNG's **Set ... Mode** controls.

**Invert Mute LED state** (Toggle mode only) reverses which virtual input state corresponds to the orange light. The light is an indicator; this feature does **not** mute the real microphone.

For recognized vehicle functions with a readable state (including certain differential locks, 4WD and transfer range), Toggle follows the actual in-game state during driving and after changing vehicles, loading a level, resetting or repairing a vehicle. The mod confirms new readings before changing the LED, ignores outdated responses and does not operate vehicle controls just to synchronize the indicator. Other or ambiguous bindings keep the simple manual Toggle behavior; their indicator may need manual adjustment.

# Touchpad

## Mouse mode

With BeamNG running, the touchpad works as a **Windows mouse** automatically when it has no active BeamNG touchpad bindings. **Touchpad taps, swipes and analog inputs are available for the very first binding**, even if no touchpad bindings have been saved yet. A newly saved touchpad binding switches **Mouse Off** automatically; BeamNG takes priority during binding capture. You can turn the mouse back on manually without deleting the binding, and turn it off again to use the bound actions. Bindings for other controller buttons do not change the mouse mode.

Use **Mouse On/Off** under **Mod settings > Inputs > Touchpad**, or bind **“Toggle DualSense Touchpad Mouse Mode”** in BeamNG. **On** forces Windows mouse control, temporarily suspending saved touchpad actions. **Off** disables Windows mouse movement/clicks and restores those actions. The bindings are never deleted, and the binding editor always takes priority. Vehicle-specific actions are available only when their vehicle or part is loaded.

If BeamNG closes or the Bridge stops receiving the game's mouse ownership updates, the Windows mouse is disabled instead of being unexpectedly re-enabled. Restart BeamNG after installing both the updated mod and Bridge; use the same V1.50 packages together.

## Touchpad input types

Each horizontal or vertical channel can be configured independently for one- or two-finger input:

- **Off**: disables the channel.
- **Swipe**: sends a digital input when swiping, similar to a button.
- **Relative**: creates an analog axis relative to where the gesture started; suited to steering, camera movement or other centered controls.
- **Absolute**: maps the finger's physical position on the touchpad directly to an analog axis.


# Motion

Motion inputs can generally be exposed as:

- **Axis**: continuous analog control.
- **Button**: triggers an action after a configured threshold is reached.
- **Off**: disables the input when available.

## Motion types

- **Rotation angle**:
  - Measures how far the controller has rotated from its centered position. Suited to steering-like controls.
  - **Auto calibration** can detect the physical axis you naturally rotate around.
  - **Steering end stops** hold the bound axis at the configured per-side angle (90°, 135°, 180°, etc.), even if you rotate past ±180°. Turning back toward the center releases the stop smoothly; recentering resets the rotation history.
- **Rotation speed**:
  - Measures how quickly the controller is rotating.
  - Returns toward zero when the controller stops moving.
- **Tilt**:
  - Measures the controller's lean using gravity. Useful when physical tilt itself should act as an analog input.
- **Orientation**:
  - Tracks the controller's orientation relative to its centered position. Useful when the current orientation itself should determine the input.

Modes that use a center reference can be recentered directly from the mod settings.


# Accelerometer

**Acceleration X, Y and Z** measure the DualSense's linear acceleration and react to movements, pushes and shakes.

Each direction can be configured as:

- **Off**: disables that direction.
- **Axis**: outputs a continuous analog value.
- **Button**: triggers a BeamNG action when acceleration exceeds the configured threshold.


# Troubleshooting

**Mod installed but no feedback?** Make sure the mod is enabled in BeamNG's Mod Manager, then reload Lua with **Ctrl+L**. If it still does not initialize, restart BeamNG.

- Do not run DSX, DS4Windows or another application that controls DualSense haptics or LEDs at the same time.
- **USB audio:** Make sure Windows does not select the DualSense as the default audio output, otherwise all Windows and game sounds will play through the controller. Go to **Windows Settings > System > Sound > Output** and select your usual speakers or headphones.
- If the DualSense does not work correctly in BeamNG, disable Steam Input for BeamNG in **Steam > Properties > Controller**.
- Make sure Microsoft GameInput 3.0 or newer is installed. GameInput can be installed or updated from an administrator terminal using: `winget install Microsoft.GameInput`
- Updating the DualSense firmware through PlayStation Accessories is recommended.
- Run `UPDATE_DUALSENSE.exe` to check for updates and follow the displayed instructions.


# Architecture

```text
BeamNG
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
│  ├─ dualsensePhysicsDirectRPMLights.lua
│  │  └─ engine RPM / rev-limiter lighting ──► DualSense RGB
│  ├─ touchpad / settings / status channel ── UDP 127.0.0.1:6978 ──► Bridge
│  │
│  ├─ dualsenseExtendedInputs.lua ◄────────── UDP 127.0.0.1:6977 ── Bridge
│  ├─ dualsenseMotionInputs.lua   ◄────────── UDP 127.0.0.1:6979 ── Bridge
│  ├─ dualsenseSuite/virtualInputDevice.lua
│  │  └─ one BeamNG virtual device for touchpad + motion bindings
│  │
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

# Updates

Run `UPDATE_DUALSENSE.exe` to check for updates and follow the displayed instructions.

The updater keeps the Bridge and BeamNG mod synchronized and guides you through any required installation steps.


# Feedback, suggestions and bug reports

If you encounter unexpected behaviour, compatibility problems or something that does not feel right, feel free to report it.

Suggestions are welcome too. If there is another DualSense feature that you think would work particularly well in BeamNG, feel free to share your idea.

The BeamNG forum thread can also be used for discussion, troubleshooting and suggestions:

[**Enhanced PS5 DualSense Haptics — BeamNG forum thread**](https://www.beamng.com/threads/enhanced-ps5-dualsense-haptics-feedback-support-suggestions.111320/)

[**Community discussion on r/BeamNG**](https://www.reddit.com/r/BeamNG/s/dfnxC5vECj)


# License

This project is released under the **MIT License**.

See [`LICENSE`](LICENSE) for details.


## Release integrity and source builds

The GitHub source repository does not track generated executables or ZIPs. For a
release, use Python 3.11+ and Go 1.27.4 to build from the same source checkout:

```sh
python Source/scripts/package_release.py --repo-root .
```

The script validates version/compatibility metadata, rebuilds the Windows USB
and Bluetooth Bridge executables, launcher and updater, and packages the mod
and Bridge in `Release/`, together with SHA-256 manifests and a build provenance note. The build process
does not upload or publish anything and does not touch the BeamNG installation.
For an official release, build and verify both ZIPs from the same source with a
**maintained Go toolchain** (Go 1.27.4 in the optional CI). GitHub Actions is
not required: you may upload the verified ZIPs manually to GitHub Releases.
Local archives built using another Go toolchain may differ.
Publish the two matching `V1.50` ZIPs as assets of a complete stable GitHub
release tagged `V1.50`. A draft/prerelease or missing asset will be ignored by
the V1.43 updater. Updating the Bridge does not automatically replace the
BeamNG mod ZIP: follow the manual installation guide produced by the updater.

Back up any **custom edits** made to `Config/feel_profile_v1.json` before
updating from older Bridge versions: this file ships with the Bridge runtime
and can be replaced by the legacy updater. BeamNG's saved controls and
settings are not part of that runtime file.


## Mod Manager activation/deactivation test

This release explicitly unloads the mod's persistent Lua extensions only when
BeamNG disables **this** mod. Switching maps, resetting vehicles or switching
vehicles must not unload them. Vue route and pause-button lifecycle operations
are serialized. See [`Release/MOD_ACTIVATION_TEST_V1.50.md`](Release/MOD_ACTIVATION_TEST_V1.50.md)
for the manual in-game regression test.
