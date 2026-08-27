package main

// Windows wheel input accepts high-resolution deltas below WHEEL_DELTA (120).
// Use that capability for touchpad-like two-finger scrolling: a short
// activation threshold prevents accidental scrolls, then small finger motion
// produces small wheel deltas instead of full 120-unit notches.
const (
	touchScrollStartSlop            = 0.0100 // normalized axis travel before scroll engages
	touchScrollWheelUnitsPerSurface = 720.0  // calibrated high-resolution scroll scale

	touchScrollPrecisionSpeed = 60.0 // touch-surface pixels/s
	touchScrollNormalSpeed    = 200.0
	touchScrollFastSpeed      = 560.0
	touchScrollPrecisionGain  = 0.18
	touchScrollNormalGain     = 0.40
	touchScrollFastGain       = 0.68
)

func touchScrollGainForSpeed(surfacePixelsPerSecond float64) float64 {
	if surfacePixelsPerSecond <= touchScrollPrecisionSpeed {
		return touchScrollPrecisionGain
	}
	if surfacePixelsPerSecond <= touchScrollNormalSpeed {
		return interpolatePointerGain(
			surfacePixelsPerSecond,
			touchScrollPrecisionSpeed, touchScrollNormalSpeed,
			touchScrollPrecisionGain, touchScrollNormalGain,
		)
	}
	if surfacePixelsPerSecond <= touchScrollFastSpeed {
		return interpolatePointerGain(
			surfacePixelsPerSecond,
			touchScrollNormalSpeed, touchScrollFastSpeed,
			touchScrollNormalGain, touchScrollFastGain,
		)
	}
	return touchScrollFastGain
}
