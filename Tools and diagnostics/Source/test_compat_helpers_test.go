package main

import "time"

// Test-only adapters keep assertions readable without shipping duplicate
// telemetry-wrapper paths in the runtime binaries.
func applyABSHybridPulse(t telemetry, now time.Time, state *absPulseState) (telemetry, bool, string, int, int, int, float64) {
	pair := triggerPairFromTelemetry(t)
	pair, active, phase, base, kick, current, hz := applyABSHybridPulsePair(t, pair, now, state)
	writeTriggerState(&t, pair)
	return t, active, phase, base, kick, current, hz
}

func applyAdaptiveThrottleTrigger(t telemetry, now time.Time, state *adaptiveThrottleTriggerState, shiftTorqueCut bool) (telemetry, bool, float64, float64, float64) {
	pair := triggerPairFromTelemetry(t)
	pair, active, severity, target, current := applyAdaptiveThrottleTriggerPair(t, pair, now, state, shiftTorqueCut)
	writeTriggerState(&t, pair)
	return t, active, severity, target, current
}

func applyNormalL2Settings(t telemetry, settings userSettings) telemetry {
	pair := triggerPairFromTelemetry(t)
	pair.L2 = normalL2Effect(t, settings)
	writeTriggerState(&t, pair)
	return t
}

func applyNormalR2Settings(t telemetry, settings userSettings) telemetry {
	pair := triggerPairFromTelemetry(t)
	pair.R2 = normalR2Effect(settings)
	writeTriggerState(&t, pair)
	return t
}

func applyUserTriggerPreferences(t telemetry, absActive bool) telemetry {
	pair := triggerPairFromTelemetry(t)
	pair = applyUserTriggerPreferencesPair(t, pair, absActive)
	writeTriggerState(&t, pair)
	return t
}

func continuousSurfaceStrength(profile string, roughness, speed, excitation, slip float64) float64 {
	rolling, sliding := surfaceStrengthComponents(profile, roughness, speed, excitation, slip)
	return min(0.44, rolling+sliding)
}

func (q *sharedPCMQueue) push(samples []int8) {
	q.pushAtRate(samples, canonicalHapticSampleRate)
}

func (q *sharedPCMQueue) renderHold(dstFrames, outputRate int) (left, right []float64) {
	return q.render(dstFrames, outputRate)
}

func (c *lightbarController) isBlinking() bool {
	return c.blinkActive
}
