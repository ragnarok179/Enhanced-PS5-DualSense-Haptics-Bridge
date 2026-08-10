package main

import "math"

// Common Feel is generated once at 48 kHz for every transport.
// USB is the canonical reference and receives that stream directly, without
// transport filtering. Bluetooth derives its 3 kHz payload from the same source
// through an anti-alias low-pass and 16:1 decimation. This keeps the validated
// USB feel byte-stable while making Bluetooth a deterministic adapter of it.
const (
	commonFeelLowPassHz = 1000.0
	bluetoothDecimation = canonicalHapticSampleRate / bluetoothHapticSampleRate
)

type biquadLowPass struct {
	b0, b1, b2 float64
	a1, a2     float64
	z1, z2     float64
}

func newBiquadLowPass(sampleRate, cutoff, q float64) biquadLowPass {
	omega := 2 * math.Pi * cutoff / sampleRate
	c, sn := math.Cos(omega), math.Sin(omega)
	alpha := sn / (2 * q)
	a0 := 1 + alpha
	return biquadLowPass{
		b0: (1 - c) / 2 / a0,
		b1: (1 - c) / a0,
		b2: (1 - c) / 2 / a0,
		a1: (-2 * c) / a0,
		a2: (1 - alpha) / a0,
	}
}

func (f *biquadLowPass) process(x float64) float64 {
	y := f.b0*x + f.z1
	f.z1 = f.b1*x - f.a1*y + f.z2
	f.z2 = f.b2*x - f.a2*y
	return y
}

type stereoLowPass struct {
	left  [2]biquadLowPass
	right [2]biquadLowPass
}

func newStereoLowPass() stereoLowPass {
	// Q values for a 4th-order Butterworth split into two biquad sections.
	const q1 = 0.541196100146197
	const q2 = 1.306562964876377
	return stereoLowPass{
		left: [2]biquadLowPass{
			newBiquadLowPass(canonicalHapticSampleRate, commonFeelLowPassHz, q1),
			newBiquadLowPass(canonicalHapticSampleRate, commonFeelLowPassHz, q2),
		},
		right: [2]biquadLowPass{
			newBiquadLowPass(canonicalHapticSampleRate, commonFeelLowPassHz, q1),
			newBiquadLowPass(canonicalHapticSampleRate, commonFeelLowPassHz, q2),
		},
	}
}

func (f *stereoLowPass) process(left, right float64) (float64, float64) {
	for i := range f.left {
		left = f.left[i].process(left)
		right = f.right[i].process(right)
	}
	return left, right
}

type transportPCMBlock struct {
	USB48k      []int8
	Bluetooth3k []int8
}

// canonicalPCMStream is the sole PCM transport boundary. USB is intentionally
// not filtered here: it is the validated 48 kHz reference. Bluetooth alone is
// band-limited before decimation. Transport gains are global calibration only;
// no material or gameplay effect receives a transport-specific correction.
type canonicalPCMStream struct {
	lowPass        stereoLowPass
	bluetoothPhase int
}

func newCanonicalPCMStream() *canonicalPCMStream {
	return &canonicalPCMStream{lowPass: newStereoLowPass()}
}

func canonicalFramesForBluetoothFrames(frames int) int {
	if frames <= 0 {
		return 0
	}
	return frames * bluetoothDecimation
}

func quantizeHaptic(v float64) int8 {
	return int8(math.Round(clamp(v, -0.99, 0.99) * 127))
}

func (s *canonicalPCMStream) process(samples []int8) transportPCMBlock {
	if s == nil || len(samples) < 2 {
		return transportPCMBlock{}
	}
	frames := len(samples) / 2
	usb := make([]int8, frames*2)
	bt := make([]int8, 0, (frames/bluetoothDecimation+1)*2)

	profile := feelProfile()
	usbGain := profile.Transport.USBOutputGain
	btGain := profile.Transport.BluetoothOutputGain
	if usbGain <= 0 {
		usbGain = 1.0
	}
	if btGain <= 0 {
		btGain = 1.0
	}

	for i := 0; i < frames; i++ {
		rawLeft := float64(samples[i*2]) / 127.0
		rawRight := float64(samples[i*2+1]) / 127.0

		// USB stays byte-equivalent to the validated canonical renderer when
		// usb_output_gain is 1.0. Do not place Bluetooth transport filtering here.
		usb[i*2] = quantizeHaptic(rawLeft * usbGain)
		usb[i*2+1] = quantizeHaptic(rawRight * usbGain)

		// Bluetooth requires band-limiting before the 48 kHz -> 3 kHz reduction.
		filteredLeft, filteredRight := s.lowPass.process(rawLeft, rawRight)
		s.bluetoothPhase++
		if s.bluetoothPhase == bluetoothDecimation {
			bt = append(bt, quantizeHaptic(filteredLeft*btGain), quantizeHaptic(filteredRight*btGain))
			s.bluetoothPhase = 0
		}
	}
	return transportPCMBlock{USB48k: usb, Bluetooth3k: bt}
}
