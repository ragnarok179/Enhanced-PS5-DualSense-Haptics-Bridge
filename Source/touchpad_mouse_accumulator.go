package main

import "math"

// accumulateWholePixels preserves fractional displacement between HID reports
// and returns only the integral pixels that can be passed to Win32 SendInput.
func accumulateWholePixels(accum *float64, delta float64) int32 {
	*accum += delta
	whole := math.Trunc(*accum)
	*accum -= whole
	return int32(whole)
}
