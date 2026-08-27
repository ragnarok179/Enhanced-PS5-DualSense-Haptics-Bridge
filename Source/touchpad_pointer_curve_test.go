package main

import (
	"math"
	"testing"
)

func TestTouchPointerGainPreservesPrecisionAtLowSpeed(t *testing.T) {
	if got := touchPointerGainForSpeed(10); math.Abs(got-touchPointerPrecisionGain) > 1e-9 {
		t.Fatalf("low-speed gain = %.4f, want %.4f", got, touchPointerPrecisionGain)
	}
	if got := touchPointerGainForSpeed(touchPointerFastSpeed + 100); math.Abs(got-touchPointerFastGain) > 1e-9 {
		t.Fatalf("fast gain = %.4f, want %.4f", got, touchPointerFastGain)
	}
}

func TestTouchPointerGainIsMonotonic(t *testing.T) {
	previous := 0.0
	for speed := 0.0; speed <= 600; speed += 5 {
		gain := touchPointerGainForSpeed(speed)
		if gain < previous {
			t.Fatalf("gain decreased at speed %.1f: %.4f < %.4f", speed, gain, previous)
		}
		if gain < touchPointerPrecisionGain || gain > touchPointerFastGain {
			t.Fatalf("gain %.4f outside expected range", gain)
		}
		previous = gain
	}
}

func TestTouchSurfaceSpeedUsesPadDimensions(t *testing.T) {
	oneXPixel := 1.0 / float64(dualSenseTouchpadWidth-1)
	if got := touchSurfaceSpeed(oneXPixel, 0, 0.01); math.Abs(got-100) > 1e-6 {
		t.Fatalf("speed = %.6f, want 100", got)
	}
}

func TestTouchPointerVelocityFilterRejectsSingleQuantizedStep(t *testing.T) {
	var f touchPointerVelocityFilter
	dx := 1.0 / float64(dualSenseTouchpadWidth-1)
	speed := f.Update(dx, 0, 0.001)
	if speed >= touchPointerPrecisionSpeed {
		t.Fatalf("single 1-pixel HID step should remain precision-speed, got %.2f", speed)
	}
}

func TestTouchPointerVelocityFilterReachesFastSwipe(t *testing.T) {
	var f touchPointerVelocityFilter
	dx := 10.0 / float64(dualSenseTouchpadWidth-1)
	var speed float64
	for i := 0; i < 20; i++ {
		speed = f.Update(dx, 0, 0.001)
	}
	if speed <= touchPointerFastSpeed {
		t.Fatalf("sustained fast motion should reach fast-speed region, got %.2f", speed)
	}
}

func TestTouchPointerVelocityFilterDecaysDuringPause(t *testing.T) {
	var f touchPointerVelocityFilter
	dx := 8.0 / float64(dualSenseTouchpadWidth-1)
	for i := 0; i < 10; i++ {
		f.Update(dx, 0, 0.002)
	}
	before := f.speed
	for i := 0; i < 40; i++ {
		f.Update(0, 0, 0.005)
	}
	if f.speed >= before || f.speed > touchPointerPrecisionSpeed {
		t.Fatalf("velocity should decay into precision region: before=%.2f after=%.2f", before, f.speed)
	}
}

func TestTouchPointerFastGainIsNoticeablyReduced(t *testing.T) {
	if touchPointerFastGain >= 0.82 {
		t.Fatalf("fast gain %.3f is unexpectedly high", touchPointerFastGain)
	}
	if touchPointerPrecisionGain >= 0.20 {
		t.Fatalf("precision gain %.3f is not reduced enough", touchPointerPrecisionGain)
	}
}
