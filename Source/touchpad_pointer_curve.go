package main

import "math"

const (
	touchPointerPrecisionSpeed = 55.0 // touch-surface pixels/s
	touchPointerNormalSpeed    = 160.0
	touchPointerFastSpeed      = 420.0
	touchPointerPrecisionGain  = 0.14
	touchPointerNormalGain     = 0.32
	touchPointerFastGain       = 0.76
)

const touchPointerVelocityTimeConstant = 0.030 // seconds

type touchPointerVelocityFilter struct {
	speed float64
}

func (f *touchPointerVelocityFilter) Reset() {
	f.speed = 0
}

// Update smooths velocity across HID reports instead of treating each
// quantized report delta as a complete speed measurement. This mirrors the
// multi-event velocity tracking used by desktop touchpad stacks and keeps a
// one-pixel hardware step from looking like a fast swipe.
func (f *touchPointerVelocityFilter) Update(rawDX, rawDY, dtSeconds float64) float64 {
	if dtSeconds <= 0 {
		return f.speed
	}
	instant := touchSurfaceSpeed(rawDX, rawDY, dtSeconds)
	alpha := 1 - math.Exp(-dtSeconds/touchPointerVelocityTimeConstant)
	f.speed += alpha * (instant - f.speed)
	return f.speed
}

func interpolatePointerGain(value, inMin, inMax, outMin, outMax float64) float64 {
	if value <= inMin {
		return outMin
	}
	if value >= inMax {
		return outMax
	}
	t := (value - inMin) / (inMax - inMin)
	return outMin + (outMax-outMin)*t
}

// touchPointerGainForSpeed adds the low-speed deceleration expected from a
// touchpad while deliberately capping fast-swipe cursor speed as well. The input
// speed is measured on the physical DualSense touch surface, not in Windows
// cursor pixels, so USB and Bluetooth use the same curve.
func touchPointerGainForSpeed(surfacePixelsPerSecond float64) float64 {
	if surfacePixelsPerSecond <= touchPointerPrecisionSpeed {
		return touchPointerPrecisionGain
	}
	if surfacePixelsPerSecond <= touchPointerNormalSpeed {
		return interpolatePointerGain(
			surfacePixelsPerSecond,
			touchPointerPrecisionSpeed, touchPointerNormalSpeed,
			touchPointerPrecisionGain, touchPointerNormalGain,
		)
	}
	if surfacePixelsPerSecond <= touchPointerFastSpeed {
		return interpolatePointerGain(
			surfacePixelsPerSecond,
			touchPointerNormalSpeed, touchPointerFastSpeed,
			touchPointerNormalGain, touchPointerFastGain,
		)
	}
	return touchPointerFastGain
}

func touchSurfaceSpeed(rawDX, rawDY, dtSeconds float64) float64 {
	if dtSeconds <= 0 {
		return touchPointerFastSpeed
	}
	dx := rawDX * float64(dualSenseTouchpadWidth-1)
	dy := rawDY * float64(dualSenseTouchpadHeight-1)
	return math.Hypot(dx, dy) / dtSeconds
}
