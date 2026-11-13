package main

import (
	"testing"
	"time"
)

func TestV150WheelspinPulseWithoutTractionControl(t *testing.T) {
	state := &adaptiveThrottleTriggerState{}
	cmd := telemetry{
		Active:   true,
		R2Mode:   1,
		R2Effect: &triggerEffectWire{Kind: "resistance"},
		Raw:      &rawTelemetry{EngineRunning: true, Wheelspin: true, DrivenSlip: 19, TCS: false, TCSRaw: false},
		UserSettings: &telemetryUserSettings{
			Schema: 13, R2EffectsEnabled: true,
			R2WheelspinPulseEnabled: true, R2WheelspinPulseStrength: 36,
		},
	}
	pulse, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, time.Unix(0, 0), state, false)
	if !active || pulse.R2Mode != 2 || pulse.R2Amplitude < 96 || pulse.R2Hz < 18 {
		t.Fatalf("expected substantial slip pulse without traction control, got %+v active=%t", pulse, active)
	}
	release, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, time.Unix(0, int64(100*time.Millisecond)), state, false)
	if !active || release.R2Mode != 3 || release.R2StartStrength > 3 {
		t.Fatalf("expected low-force grip release between pulses, got %+v active=%t", release, active)
	}
}

func TestV150WheelspinPulseCanBeDisabledIndependently(t *testing.T) {
	cmd := telemetry{
		Active: true, R2Mode: 1,
		Raw:          &rawTelemetry{EngineRunning: true, Wheelspin: true, DrivenSlip: 20},
		UserSettings: &telemetryUserSettings{Schema: 13, R2EffectsEnabled: true, R2WheelspinPulseStrength: 48, R2WheelspinPulseEnabled: false},
	}
	got, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
	if !active || got.R2Mode != 3 || got.R2Amplitude != 0 {
		t.Fatalf("wheelspin should only release resistance when pulsing disabled, got %+v", got)
	}
	cmd.UserSettings.R2EffectsEnabled = false
	cmd.R2Mode, cmd.R2StartStrength = 1, 15
	got, active, _, _, _ = applyAdaptiveThrottleTrigger(cmd, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
	if active || got.R2Mode != 1 || got.R2StartStrength != 15 {
		t.Fatalf("disabling dynamic effects must preserve normal throttle, got %+v", got)
	}
}

func TestV150AdaptiveTriggerStartZoneConversion(t *testing.T) {
	got := telemetry{L2Effect: &triggerEffectWire{Kind: "resistance", StartPosition: 0.45, StartForce: 0.2, EndForce: 0.6}, R2Effect: &triggerEffectWire{Kind: "resistance", StartPosition: 0.70, StartForce: 0.1, EndForce: 0.25}}
	normalizeSemanticTriggerEffects(&got)
	if got.L2Mode != 1 || got.L2StartZone != 4 || got.R2Mode != 1 || got.R2StartZone != 6 {
		t.Fatalf("start positions not reflected in HID zones: L2=%d R2=%d", got.L2StartZone, got.R2StartZone)
	}
}

func TestV150SparseSettingsHeartbeatControlsEveryWheelspinFrame(t *testing.T) {
	original := currentUserSettings()
	defer func() {
		userSettingsState.mu.Lock()
		userSettingsState.data = original
		userSettingsState.mu.Unlock()
	}()
	applyBeamNGUserSettings(telemetryUserSettings{
		Schema: 13, MasterEnabled: true, R2ThrottleEnabled: true,
		R2EffectsEnabled: true, R2WheelspinPulseEnabled: true,
		R2WheelspinPulseStrength: 48,
	})
	// No userSettings object on normal gameplay frames after the heartbeat.
	cmd := telemetry{
		Active: true, Raw: &rawTelemetry{EngineRunning: true, Wheelspin: true, DrivenSlip: 20},
		R2Mode: 1, R2StartStrength: 10,
	}
	state := &adaptiveThrottleTriggerState{}
	pulse, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, time.Unix(0, 0), state, false)
	if !active || pulse.R2Mode != 2 || pulse.R2Amplitude != 255 {
		t.Fatalf("cached wheelspin configuration missing after settings heartbeat: %+v", pulse)
	}
	applyBeamNGUserSettings(telemetryUserSettings{
		Schema: 13, MasterEnabled: true, R2ThrottleEnabled: true,
		R2EffectsEnabled: true, R2WheelspinPulseEnabled: false,
	})
	release, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
	if !active || release.R2Mode != 3 || release.R2Amplitude != 0 {
		t.Fatalf("disabling wheelspin pulse must persist across gameplay frames: %+v", release)
	}
	applyBeamNGUserSettings(telemetryUserSettings{Schema: 12, R2EffectsEnabled: true})
	legacy, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, time.Unix(0, 0), &adaptiveThrottleTriggerState{}, false)
	if !active || legacy.R2Mode != 3 || legacy.R2Amplitude != 0 {
		t.Fatalf("legacy settings must not enable V1.50 pulse: %+v", legacy)
	}
}
