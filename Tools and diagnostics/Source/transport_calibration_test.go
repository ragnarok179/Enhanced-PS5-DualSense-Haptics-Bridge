package main

import (
	"math"
	"testing"
)

func TestTransportCalibration(t *testing.T) {
	profile := feelProfile().Transport
	if math.Abs(transportGain(profile.USBOutputGain)-1.0) > 1e-9 {
		t.Fatalf("USB output gain = %.6f, want reference 1.0", profile.USBOutputGain)
	}
	if math.Abs(transportGain(profile.BluetoothOutputGain)-0.80) > 1e-9 {
		t.Fatalf("Bluetooth output gain = %.6f, want calibrated 0.80", profile.BluetoothOutputGain)
	}
}

func TestBluetoothLowFrequencyAmplitudeUsesCalibratedGain(t *testing.T) {
	const frames = 4800
	main := make([]int8, frames*2)
	for i := 0; i < frames; i++ {
		v := int8(math.Round(math.Sin(2*math.Pi*92*float64(i)/canonicalHapticSampleRate) * 70))
		main[i*2], main[i*2+1] = v, v
	}
	stream := newCanonicalPCMStream()
	out := stream.processBluetooth(main)
	if len(out) == 0 {
		t.Fatal("Bluetooth conversion produced no PCM")
	}
	var sum float64
	peak := 0.0
	for _, x := range out {
		v := math.Abs(float64(x) / 127.0)
		sum += v * v
		if v > peak {
			peak = v
		}
	}
	rms := math.Sqrt(sum / float64(len(out)))
	inputRMS := (70.0 / 127.0) / math.Sqrt2
	targetRMS := inputRMS * 0.80
	if math.Abs(rms-targetRMS) > inputRMS*0.03 {
		t.Fatalf("Bluetooth low-frequency calibrated rms=%.4f, want %.4f (+/-3%% input)", rms, targetRMS)
	}
	targetPeak := (70.0 / 127.0) * 0.80
	if math.Abs(peak-targetPeak) > (70.0/127.0)*0.04 {
		t.Fatalf("Bluetooth low-frequency calibrated peak=%.4f, want %.4f (+/-4%% input)", peak, targetPeak)
	}
}
