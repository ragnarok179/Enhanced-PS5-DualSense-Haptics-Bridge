package main

import (
	"math"
	"time"
)

func absPedalPulseHz(t telemetry) float64 {
	// BeamNG absHz is the hydraulic controller rate. The v0.35 mechanical cue is
	// exactly twice the v0.34 mapping: 22-28 Hz, preserving vehicle differences.
	if t.Raw != nil && t.Raw.ABSControlHz > 0 {
		native := math.Max(20, math.Min(140, t.Raw.ABSControlHz))
		return 22.0 + math.Sqrt((native-20)/120)*6.0
	}
	return 25.6
}

type absPulseState struct {
	cycleStart    time.Time
	holdUntil     time.Time
	lastStrong    bool
	lastLock      bool
	lastControlHz float64
}

// applyABSHybridPulsePair preserves the validated ABS rhythm: a short release,
// a strong kick and a light hydraulic base. Gameplay forces remain normalized;
// only the DualSense output adapter converts them to a concrete HID encoding.
// The ABS setting controls the kick while the L2 start force controls the base.
func applyABSHybridPulsePair(t telemetry, pair triggerPair, now time.Time, state *absPulseState) (triggerPair, bool, string, int, int, int, float64) {
	if state == nil {
		return pair, false, "off", 0, 0, 0, 0
	}
	rawActive := t.Raw != nil && (t.Raw.ABS || t.Raw.ABSRaw) && pair.L2.Kind == triggerVibration
	if rawActive {
		if state.cycleStart.IsZero() || now.After(state.holdUntil) {
			state.cycleStart = now
		}
		state.holdUntil = now.Add(92 * time.Millisecond)
		state.lastStrong = t.Raw.ABSSeverity >= 0.35 || t.Raw.ABSWheelCount >= 1 || pair.L2.Amplitude.Float64() >= 0.5
		state.lastLock = t.Raw.LockHaptic || t.Raw.Lock || t.Raw.LockedWheelCount > 0
		state.lastControlHz = t.Raw.ABSControlHz
	}
	active := rawActive || (!state.holdUntil.IsZero() && now.Before(state.holdUntil))
	if !active {
		state.cycleStart = time.Time{}
		state.holdUntil = time.Time{}
		state.lastLock = false
		state.lastControlHz = 0
		return pair, false, "off", 0, 0, 0, 0
	}
	if state.cycleStart.IsZero() {
		state.cycleStart = now
	}

	frequencyTelemetry := t
	if frequencyTelemetry.Raw == nil {
		frequencyTelemetry.Raw = &rawTelemetry{}
	}
	if frequencyTelemetry.Raw.ABSControlHz <= 0 {
		frequencyTelemetry.Raw.ABSControlHz = state.lastControlHz
	}
	cadenceHz := absPedalPulseHz(frequencyTelemetry)
	period := time.Duration(float64(time.Second) / cadenceHz)
	if period < 34*time.Millisecond {
		period = 34 * time.Millisecond
	}

	user := currentUserSettings()
	settings := user.AdaptiveTriggers
	if !settings.ABSEnabled {
		state.cycleStart = time.Time{}
		state.holdUntil = time.Time{}
		state.lastStrong = false
		state.lastLock = false
		state.lastControlHz = 0
		return pair, false, "disabled", 0, 0, 0, 0
	}

	base := configuredL2StartForce(user)
	kick := settingForce(true, settings.ABSStrength)
	elapsed := now.Sub(state.cycleStart)
	phaseElapsed := elapsed % period
	releaseDuration := 5 * time.Millisecond
	kickDuration := 13 * time.Millisecond
	if elapsed < period {
		releaseDuration = 6 * time.Millisecond
		kickDuration = 16 * time.Millisecond
	}

	phase := "base"
	current := base
	switch {
	case phaseElapsed < releaseDuration:
		if state.lastLock && base > 0 {
			pair.L2 = resistanceTrigger(0, base.Float64(), base.Float64())
			phase = "lock_base"
			current = base
		} else {
			pair.L2 = offTrigger()
			phase = "release_off"
			current = 0
		}
	case phaseElapsed < releaseDuration+kickDuration:
		pair.L2 = resistanceTrigger(0, kick.Float64(), kick.Float64())
		phase = "kick"
		current = kick
	default:
		if base > 0 {
			pair.L2 = resistanceTrigger(0, base.Float64(), base.Float64())
		} else {
			pair.L2 = offTrigger()
		}
	}
	return pair, true, phase, base.Level(), kick.Level(), current.Level(), cadenceHz
}

type adaptiveThrottleTriggerState struct {
	currentPosition float64 // normalized 0..1
	currentStrength float64 // normalized 0..1
	lastTime        time.Time
	lastLog         time.Time
}

func applyAdaptiveThrottleTriggerPair(t telemetry, pair triggerPair, now time.Time, state *adaptiveThrottleTriggerState, shiftTorqueCut bool) (triggerPair, bool, float64, float64, float64) {
	if state == nil {
		return pair, false, 0, 0, 0
	}
	if t.Raw == nil || !t.Active || !t.Raw.EngineRunning {
		state.currentPosition, state.currentStrength = 0, 0
		state.lastTime = now
		pair.R2 = offTrigger()
		return pair, false, 0, 0, 0
	}

	settings := currentUserSettings()
	if !settings.AdaptiveTriggers.R2EffectsEnabled {
		state.currentPosition, state.currentStrength = 0, 0
		state.lastTime = now
		pair.R2 = normalR2Effect(settings)
		return pair, false, 0, 0, 0
	}

	cfg := feelProfile().Triggers.R2
	if t.Raw.Airborne {
		state.currentPosition = unit(cfg.AirbornePosition).Float64()
		state.currentStrength = unit(cfg.AirborneForce).Float64()
		state.lastTime = now
		pair.R2 = fineTrigger(state.currentPosition, state.currentStrength)
		return pair, true, 0, state.currentPosition, state.currentPosition
	}

	if shiftTorqueCut {
		state.currentPosition = unit(cfg.ShiftPosition).Float64()
		state.currentStrength = unit(cfg.ShiftForce).Float64()
		state.lastTime = now
		pair.R2 = fineTrigger(state.currentPosition, state.currentStrength)
		return pair, true, 0, state.currentPosition, state.currentPosition
	}

	if t.Raw.TCS || t.Raw.TCSRaw || t.Raw.RevLimiter || pair.R2.Kind == triggerVibration {
		state.lastTime = now
		if pair.R2.Kind == triggerVibration {
			calibratedStrongJolt := unitFromPercent(r2DynamicReferencePercent).Float64()
			ratio := clamp(pair.R2.Amplitude.Float64()/calibratedStrongJolt, 0, 1)
			peak := settingForce(true, settings.AdaptiveTriggers.R2EffectsStrength)
			pair.R2.Amplitude = force48(peak.Float64() * ratio)
			if pair.R2.Amplitude <= 0 {
				pair.R2 = normalR2Effect(settings)
			}
		}
		return pair, false, 0, state.currentPosition, state.currentPosition
	}

	severity := 0.0
	slipActive := false
	targetPosition := 0.0
	targetStrength := 0.0
	if t.Raw.Wheelspin && !t.Raw.TCS && !t.Raw.TCSRaw {
		const slipStart = 3.2
		const slipFull = 20.0
		severity = clamp01((t.Raw.DrivenSlip - slipStart) / (slipFull - slipStart))
		if t.Raw.DrivenSlip >= slipStart {
			slipActive = true
			targetStrength = cfg.WheelspinStartForce + (cfg.WheelspinEndForce-cfg.WheelspinStartForce)*severity
			targetPosition = cfg.WheelspinStartPosition + (cfg.WheelspinEndPosition-cfg.WheelspinStartPosition)*severity
		}
	}

	dt := now.Sub(state.lastTime).Seconds()
	if state.lastTime.IsZero() || dt < 0 || dt > 0.25 {
		dt = 0.016
	}
	state.lastTime = now
	if slipActive {
		posAlpha := 1 - math.Exp(-dt/0.028)
		forceAlpha := 1 - math.Exp(-dt/0.035)
		state.currentPosition += (targetPosition - state.currentPosition) * posAlpha
		state.currentStrength += (targetStrength - state.currentStrength) * forceAlpha
		pair.R2 = fineTrigger(state.currentPosition, state.currentStrength)
		return pair, true, severity, targetPosition, state.currentPosition
	}

	state.currentPosition, state.currentStrength = 0, 0
	pair.R2 = resistanceTrigger(0, cfg.NormalStartForce, cfg.NormalEndForce)
	return pair, false, 0, 0, 0
}
