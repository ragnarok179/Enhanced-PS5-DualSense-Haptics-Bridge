# Changelog
## V1.4 terminal cleanup

- Removed the routine touchpad-mouse SendInput success confirmation from normal terminal output.
- Moved touchpad ownership, speaker setting-change and Bluetooth idle-configuration details to diagnostic mode only.
- Moved one-time Windows HID report-padding validation to diagnostic mode only.
- Kept actionable errors, controller/mod connection state and protocol compatibility/update messages in the normal terminal.
- Restored `Diagnostics/Logs/.gitkeep` in the complete repository snapshot.


## V1.4

- adds official gameplay compatibility for Mod V1.0/protocol 40,
  V1.1/protocol 41 and V1.2/protocol 42;
- makes Mod V1.2 protocol 42 intentionally require Bridge V1.4, so Bridge V1.3
  requests an update instead of silently running the newer mod;
- keeps V1.4 backward-compatible with protocols 40/41 so the Bridge can be
  published before Mod V1.2;
- restores decoding of the V1.2 schema-12 settings packet so haptic and adaptive-
  trigger gains are not replaced by zero after connection;
- restores the validated transport-specific haptic adapters and V1.3 Bluetooth
  haptic calibration (`USB=1.00`, `Bluetooth=0.80`);
- USB Bridge no longer writes lightbar-valid/RGB fields; BeamNG is the sole RGB
  writer;
- fixes FFmpeg 8 `AVCodecParameters` offsets used by Bluetooth collision audio;
- fixes Speaker output routing so `Controller only` suppresses BeamNG system collision audio and `Controller + PC` does not duplicate the PC sound;
- changes Bluetooth collision audio from a FIFO of pre-encoded complete cues to
  the same overlapping PCM voice model used by USB;
- BeamNG reset/reload events now flush active Bluetooth speaker voices instead of
  leaving queued collision sounds to play later;
- Bluetooth speaker output calibration is applied at the DualSense hardware
  volume field (`80`) in both setup and continuous `0x36` state; collision/event
  gain itself is unchanged;
- Bluetooth speaker timing follows 512 source samples -> 480 Opus samples at the
  `0x36` 10.667 ms cadence;
- automatic launcher no longer probes and restarts the selected runtime process;
- diagnostic log launchers avoid the previous long-lived PowerShell `Tee-Object`
  pipeline;
- keeps the production single-writer Bluetooth `0x36` architecture;
- normal USB/Bluetooth terminals now share V1.3-style lifecycle messages for
  controller state, BeamNG/mod detection, connection loss/recovery and protocol updates;
- keeps verbose state behind explicit diagnostics and source/tests outside the
  runtime ZIP;
- restores the public mod name **Enhanced PS5 DualSense Haptics** and removes
  routine extended-input success lines from normal/diagnostic telemetry output;
- documents public touchpad mouse/custom mappings, motion/accelerometer inputs
  and Bluetooth sleep options.
