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

USB is the physical reference and is not altered to imitate Bluetooth.

V1.3 keeps USB as the physical reference. Follow-up physical A/B testing found the later 0.86-0.875 Bluetooth values still perceptually too strong, so the requested global calibration is:

- USB global PCM gain: `1.00`
- Bluetooth global PCM gain: `0.80`

There is no suspension-bump-specific transport multiplier. Bumps,
collisions, landings, surfaces, shifts and generic engine/tyre haptics all use
the same canonical effect balance; only the final transport representation
differs.

Triggers are not PCM and do not use this calibration. USB and Bluetooth use the
same semantic trigger model and the same final trigger-effect encoder. LEDs do
not use PCM transport gains. BeamNG `Device.setRGB()` is the sole runtime lightbar writer on both transports; the Bridge keeps LED-valid HID fields clear.

## Recommended calibration procedure

For a new controller/PC:

1. Set the DualSense haptic audio endpoint and Windows session volume consistently.
2. Run the USB stereo diagnostic at fixed amplitude.
3. Run the Bluetooth stereo diagnostic with the same source effect.
4. Compare physical strength (preferably with a repeatable accelerometer/IMU
   measurement rather than perception alone).
5. Use the global transport gains only for a transport-wide mismatch.
6. If a repeatable physical mismatch is isolated to one effect family, keep the
   correction centralized in `transport_calibration.go`; do not scatter per-material
   or per-effect transport multipliers through gameplay code.
