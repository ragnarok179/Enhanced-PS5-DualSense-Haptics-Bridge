package main

// Test-only helper for diagnostics that still exercise the compact trigger
// mirror written by writeTriggerState. Its strength fields are current force48
// levels; it is not a gameplay or protocol encoding.
func fillTrigger(dst []byte, mode, startZone, startStrength, endStrength, amplitude, hz int) {
	switch mode {
	case 1:
		fillOfficialResistance(dst, resistanceTrigger(
			float64(clampInt(startZone, 0, 9))/9.0,
			force48FromLevel(startStrength).Float64(),
			force48FromLevel(endStrength).Float64(),
		))
	case 2:
		fillOfficialVibrationEffect(dst, vibrationTrigger(
			float64(clampInt(startZone, 0, 9))/9.0,
			force48FromLevel(amplitude).Float64(), float64(hz),
		))
	case 3:
		fillFineFeedbackEffect(dst, fineTrigger(
			unitFromPositionByte(startZone).Float64(),
			force48FromLevel(startStrength).Float64(),
		))
	default:
		clearTrigger(dst)
	}
}
