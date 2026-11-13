package main

import (
	"math"
	"testing"
	"time"
)

func TestR2JoltSliderControlsVibrationAmplitude(t *testing.T) {
	original := currentUserSettings()
	defer func() { userSettingsState.mu.Lock(); userSettingsState.data = original; userSettingsState.mu.Unlock() }()
	base := telemetry{Active: true, R2Mode: 2, R2Amplitude: 32, R2Hz: 22}
	for _, slider := range []int{1, 25, 50, 100} {
		// The Lua settings packet uses a 0..48 force lattice.
		wire := int(math.Floor(float64(slider)*48/100 + 0.5))
		applyBeamNGUserSettings(telemetryUserSettings{Schema: 13, R2EffectsEnabled: true, R2EffectsStrength: wire})
		got := applyUserTriggerPreferences(base, false, false)
		// The on-wire scale is quantized; compare against the exact transmitted level.
		exact := clampInt(int(math.Round(32*float64(wire)/12)), 0, 255)
		if got.R2Amplitude != exact {
			t.Fatalf("jolt=%d wire=%d: amplitude %d, want %d", slider, wire, got.R2Amplitude, exact)
		}
	}
}

func TestWheelspinSliderControlsPulsesIndependentlyOfJolt(t *testing.T) {
	original := currentUserSettings()
	defer func() { userSettingsState.mu.Lock(); userSettingsState.data = original; userSettingsState.mu.Unlock() }()
	t1 := telemetry{Active: true, R2Mode: 1, R2StartStrength: 15, Raw: &rawTelemetry{EngineRunning: true, Wheelspin: true, DrivenSlip: 20}}
	for _, tc := range []struct {
		pulseWire    int
		minAmplitude int
		maxAmplitude int
	}{
		{1, 1, 10}, {24, 110, 140}, {48, 240, 255},
	} {
		applyBeamNGUserSettings(telemetryUserSettings{Schema: 13, R2EffectsEnabled: true, R2EffectsStrength: 1, R2WheelspinPulseEnabled: true, R2WheelspinPulseStrength: tc.pulseWire})
		raw, active, _, _, _ := applyAdaptiveThrottleTrigger(t1, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
		if !active || raw.R2Mode != 2 {
			t.Fatalf("wire %d: wheelspin pulse inactive: %+v", tc.pulseWire, raw)
		}
		got := applyUserTriggerPreferences(raw, false, true)
		if got.R2Amplitude < tc.minAmplitude || got.R2Amplitude > tc.maxAmplitude {
			t.Fatalf("wire %d: amplitude %d outside %d..%d", tc.pulseWire, got.R2Amplitude, tc.minAmplitude, tc.maxAmplitude)
		}
	}
}

func TestWheelspinPulseCanCoexistWithTractionControl(t *testing.T) {
	original := currentUserSettings()
	defer func() { userSettingsState.mu.Lock(); userSettingsState.data = original; userSettingsState.mu.Unlock() }()
	applyBeamNGUserSettings(telemetryUserSettings{Schema: 13, R2EffectsEnabled: true, R2EffectsStrength: 12, R2WheelspinPulseEnabled: true, R2WheelspinPulseStrength: 48})
	base := telemetry{Active: true, R2Mode: 2, R2Amplitude: 30, R2Hz: 22, Raw: &rawTelemetry{EngineRunning: true, Wheelspin: true, DrivenSlip: 20, TCS: true}}
	got, active, _, _, _ := applyAdaptiveThrottleTrigger(base, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
	if !active || got.R2Mode != 2 || got.R2Amplitude <= base.R2Amplitude {
		t.Fatalf("TCS should leave room for physical wheelspin pulse: %+v active=%v", got, active)
	}
	later, active, _, _, _ := applyAdaptiveThrottleTrigger(base, time.Unix(0, int64(100*time.Millisecond)), &adaptiveThrottleTriggerState{}, false)
	if active || later.R2Amplitude != base.R2Amplitude {
		t.Fatalf("TCS should keep its normal feedback outside pulse windows: %+v", later)
	}
}

func TestR2JoltAndWheelspinPreserveLimiterAndNormalThrottle(t *testing.T) {
	original := currentUserSettings()
	defer func() { userSettingsState.mu.Lock(); userSettingsState.data = original; userSettingsState.mu.Unlock() }()
	applyBeamNGUserSettings(telemetryUserSettings{Schema: 13, R2ThrottleEnabled: true, R2EffectsEnabled: true, R2EffectsStrength: 48, R2WheelspinPulseEnabled: true, R2WheelspinPulseStrength: 48})
	normal := telemetry{Active: true, R2Mode: 1, R2StartStrength: 12, R2EndStrength: 24}
	got := applyUserTriggerPreferences(normal, false, false)
	if got.R2StartStrength != 12 || got.R2EndStrength != 24 {
		t.Fatalf("Jolt slider must not alter normal throttle resistance: %+v", got)
	}
	limiter := telemetry{Active: true, R2Mode: 2, R2Amplitude: 32, Raw: &rawTelemetry{EngineRunning: true, RevLimiter: true, Wheelspin: true, DrivenSlip: 20}}
	unchanged, active, _, _, _ := applyAdaptiveThrottleTrigger(limiter, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
	if active || unchanged.R2Amplitude != 32 {
		t.Fatalf("rev limiter must have priority over wheelspin pulses: %+v", unchanged)
	}
	tcs := telemetry{Active: true, R2Mode: 2, R2Amplitude: 30, Raw: &rawTelemetry{EngineRunning: true, TCS: true}}
	unchanged, active, _, _, _ = applyAdaptiveThrottleTrigger(tcs, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
	if active || unchanged.R2Amplitude != 30 {
		t.Fatalf("TCS without physical wheelspin must be unchanged: %+v", unchanged)
	}
}
