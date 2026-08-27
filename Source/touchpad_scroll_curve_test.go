package main

import (
	"math"
	"testing"
)

func TestTouchScrollGainPrecisionAndFast(t *testing.T) {
	if got := touchScrollGainForSpeed(10); math.Abs(got-touchScrollPrecisionGain) > 1e-9 {
		t.Fatalf("low-speed scroll gain = %.4f, want %.4f", got, touchScrollPrecisionGain)
	}
	if got := touchScrollGainForSpeed(touchScrollFastSpeed + 100); math.Abs(got-touchScrollFastGain) > 1e-9 {
		t.Fatalf("fast scroll gain = %.4f, want %.4f", got, touchScrollFastGain)
	}
}

func TestTouchScrollGainMonotonic(t *testing.T) {
	previous := 0.0
	for speed := 0.0; speed <= 800; speed += 5 {
		gain := touchScrollGainForSpeed(speed)
		if gain < previous {
			t.Fatalf("scroll gain decreased at speed %.1f: %.4f < %.4f", speed, gain, previous)
		}
		if gain < touchScrollPrecisionGain || gain > touchScrollFastGain {
			t.Fatalf("scroll gain %.4f outside expected range", gain)
		}
		previous = gain
	}
}

func TestTouchScrollIsMuchSlowerThanLegacyAtPrecisionSpeed(t *testing.T) {
	// The previous uncalibrated curve produced 120 wheel units every 0.045 normalized travel. At low
	// speed the calibrated curve should produce far less wheel motion for the same gesture.
	legacy := 120.0 / 0.045
	current := touchScrollWheelUnitsPerSurface * touchScrollPrecisionGain
	if current >= legacy*0.2 {
		t.Fatalf("precision scroll still too fast: current %.1f units/surface legacy %.1f", current, legacy)
	}
}

func TestTouchScrollFastRateIsCalibrated(t *testing.T) {
	dev31Fast := 960.0
	currentFast := touchScrollWheelUnitsPerSurface * touchScrollFastGain
	if currentFast >= dev31Fast*0.60 {
		t.Fatalf("fast scroll still too fast: current %.1f previous %.1f", currentFast, dev31Fast)
	}
}
