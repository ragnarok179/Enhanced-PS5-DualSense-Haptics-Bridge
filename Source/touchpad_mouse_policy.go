package main

import "time"

const (
	// A short settle window prevents the small finger displacement caused by a
	// tap from moving the pointer. A deliberate movement can escape immediately
	// by travelling beyond the larger early-move threshold.
	touchMouseTapSettleTime       = 110 * time.Millisecond
	touchMousePointerStartSlop    = 0.0065
	touchMouseEarlyMoveSlop       = 0.024
	touchMouseTapMaxTravel        = touchTapMaxTravel
	touchMouseTapMaxInjectedPixel = 6.0
)

func touchMouseShouldArmMotion(contactAge time.Duration, maxTravel float64) (arm, deliberate bool) {
	if maxTravel > touchMouseEarlyMoveSlop {
		return true, true
	}
	if contactAge >= touchMouseTapSettleTime && maxTravel > touchMousePointerStartSlop {
		return true, false
	}
	return false, false
}

func touchMouseTapEligible(maxTravel, injectedPixels float64, deliberateMove bool) bool {
	return !deliberateMove && maxTravel <= touchMouseTapMaxTravel && injectedPixels <= touchMouseTapMaxInjectedPixel
}
