package main

// legacyV40Effect is the only decoder for pre-v41 trigger packets. Those old
// packets used 0..8 for Official Feedback/Vibration force, 0..48 for Fine
// Feedback force, and byte precision for Fine position/frequency. Values are
// converted immediately into the current 0..48 trigger-force model.
func legacyV40Effect(mode, position, start, end, amplitude, hz int) triggerEffect {
	switch mode {
	case 1:
		return resistanceTrigger(
			float64(clampInt(position, 0, 9))/9.0,
			force48FromOfficialStep(start).Float64(),
			force48FromOfficialStep(end).Float64(),
		)
	case 2:
		return vibrationTrigger(
			float64(clampInt(position, 0, 9))/9.0,
			force48FromOfficialStep(amplitude).Float64(),
			float64(clampInt(hz, 0, 255)),
		)
	case 3:
		return fineTrigger(
			unitFromPositionByte(position).Float64(),
			force48FromLevel(start).Float64(),
		)
	default:
		return offTrigger()
	}
}
