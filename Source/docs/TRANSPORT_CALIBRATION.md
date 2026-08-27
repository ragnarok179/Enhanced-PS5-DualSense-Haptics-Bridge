# USB / Bluetooth transport calibration

## Why equal PCM does not guarantee equal physical strength

The V1 engine generates one common 48 kHz source signal. USB is the canonical
reference and receives it directly. Bluetooth derives its 3 kHz stream from that
same source after wireless-only anti-alias filtering. Event timing, stereo pan
and relative effect balance therefore remain shared without altering the validated
USB waveform to satisfy Bluetooth transport constraints.

The final physical path is still different:

- USB streams the waveform through the Windows GameInput/WASAPI haptic audio
  endpoint.
- Bluetooth transmits a 3 kHz signed stereo representation in DualSense audio-
  haptic packets.

Microsoft documents that WASAPI shared-mode audio passes through the Windows
audio engine. Effective shared-mode level can be influenced by stream, session
and policy volume factors. Bluetooth `0x36` haptic packets do not use that same
Windows shared-audio path.

Relevant Microsoft documentation:

- https://learn.microsoft.com/windows/win32/api/audioclient/nn-audioclient-iaudioclient
- https://learn.microsoft.com/windows/win32/api/audioclient/nn-audioclient-iaudiostreamvolume
- https://learn.microsoft.com/gaming/gdk/docs/features/common/input/hardware/input-hardware-haptics

DS5Dongle independently uses the same general Bluetooth conversion principle:
48 kHz haptic channels are resampled to 3 kHz and converted to signed 8-bit
stereo before Bluetooth transport.

- https://github.com/awalol/DS5Dongle/blob/master/src/audio.cpp

## V1 calibration policy

V1 uses no per-material or per-effect transport compensation. The USB path is
also intentionally unfiltered at the transport boundary so future Bluetooth
changes cannot silently change the wired reference feel.

The only compensation is:

- USB gain: `1.00`
- Bluetooth gain: `0.80`

The Bluetooth value restores the V1.3 physically validated USB/Bluetooth A/B reference. Because
shared-mode endpoint/session settings can vary by machine, it should be treated
as a practical default rather than a universal physical constant.

## Recommended calibration procedure

For a new controller/PC:

1. Set the DualSense haptic audio endpoint and Windows session volume consistently.
2. Run the USB stereo diagnostic at fixed amplitude.
3. Run the Bluetooth stereo diagnostic with the same source effect.
4. Compare physical strength (preferably with a repeatable accelerometer/IMU
   measurement rather than perception alone).
5. Change only `bluetooth_output_gain` or `usb_output_gain`.
6. Do not change individual surface, bump or collision gains to compensate for
   transport differences.
