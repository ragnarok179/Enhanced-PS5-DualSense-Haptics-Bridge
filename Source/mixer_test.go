package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func TestLeftRightRemainSeparate(t *testing.T) {
	m := newHapticMixer()
	m.update(telemetry{Version: 40, Active: true, BodyEvent: 1, Raw: &rawTelemetry{GroundedWheels: 4}}, time.Now())
	m.update(telemetry{Version: 40, Active: true, BodyEvent: 2, BodyProfile: "collision", BodyLeftStrength: .9, BodyRightStrength: 0, BodyDurationMS: 100, Raw: &rawTelemetry{GroundedWheels: 4}}, time.Now())
	s, _ := m.render(canonicalFramesForBluetoothFrames(32), time.Now())
	la, ra := 0, 0
	for i := 0; i < len(s); i += 2 {
		if s[i] < 0 {
			la -= int(s[i])
		} else {
			la += int(s[i])
		}
		if s[i+1] < 0 {
			ra -= int(s[i+1])
		} else {
			ra += int(s[i+1])
		}
	}
	if la == 0 || ra > la/20 {
		t.Fatalf("left=%d right=%d", la, ra)
	}
}

func TestSharedSurfaceStrengthIncludesSlipLikeUSB(t *testing.T) {
	withoutSlip := continuousSurfaceStrength("gravel", .8, 15, .4, 0)
	withSlip := continuousSurfaceStrength("gravel", .8, 15, .4, 1)
	if withoutSlip <= 0 || withSlip <= withoutSlip {
		t.Fatalf("USB reference surface formula mismatch noSlip=%.6f slip=%.6f", withoutSlip, withSlip)
	}
	// Locked numerical regression values from the validated USB formula.
	if math.Abs(withoutSlip-0.16752820) > 0.000001 || math.Abs(withSlip-0.19132820) > 0.000001 {
		t.Fatalf("USB surface calibration drifted: %.8f %.8f", withoutSlip, withSlip)
	}
}

func TestSharedWatchdog1200MS(t *testing.T) {
	now := time.Now()
	m := newHapticMixer()
	m.update(telemetry{Version: 40, Active: true, Raw: &rawTelemetry{Speed: 12, GroundedWheels: 4, SurfaceMaterialL: 19, SurfaceMaterialR: 19, SurfaceRoughnessL: 1, SurfaceRoughnessR: 1}}, now.Add(-time.Second))
	_, st := m.render(canonicalFramesForBluetoothFrames(32), now)
	if st.stale {
		t.Fatal("USB parity watchdog must still hold telemetry at 1000 ms")
	}

	m.lastPacket = now.Add(-1300 * time.Millisecond)
	_, st = m.render(canonicalFramesForBluetoothFrames(32), now)
	if !st.stale {
		t.Fatal("USB parity watchdog must be stale after 1200 ms")
	}
	if m.left.target != 0 || m.right.target != 0 {
		t.Fatalf("stale watchdog did not release surface targets: %.6f %.6f", m.left.target, m.right.target)
	}
	// WASAPI USB smooths a released surface over 10 ms rather than hard-cutting
	// the sample. Verify the same short decay eventually reaches digital silence.
	var tail []int8
	for i := 1; i <= 50; i++ {
		tail, _ = m.render(canonicalFramesForBluetoothFrames(32), now.Add(time.Duration(i)*11*time.Millisecond))
	}
	for _, v := range tail {
		if v != 0 {
			t.Fatal("stale decay did not reach silence")
		}
	}
}

func TestControlReportTriggersLEDAndCRC(t *testing.T) {
	input := telemetry{
		Version: 40, Active: true, ShiftLEDsInUse: true,
		L2Mode: 1, L2StartZone: 0, L2StartStrength: strength8To255(2), L2EndStrength: strength8To255(6),
		R2Mode: 3, R2StartZone: 0, R2StartStrength: 1,
		Raw: &rawTelemetry{RPM: 6800, MaxRPM: 7000, EngineRunning: true},
	}
	rgb := (&lightbarController{}).update(input, time.Unix(1000, 0))
	r := buildBluetoothControlReport(5, input, rgb, 547)
	if len(r) != 78 || r[0] != 0x31 || r[1] != 0x50 || r[2] != 0x10 {
		t.Fatalf("bad control header %v", r[:8])
	}
	common := r[3:50]
	if common[0] != 0x0C || common[1] != 0x04 || common[38] != 0 {
		t.Fatalf("bad minimal valid flags %02x %02x %02x", common[0], common[1], common[38])
	}
	if common[10] != 0x01 || common[21] != 0x21 {
		t.Fatalf("trigger modes R2=%02x L2=%02x", common[10], common[21])
	}
	if common[44] == 0 && common[45] == 0 && common[46] == 0 {
		t.Fatal("expected RPM light color")
	}
	got := binary.LittleEndian.Uint32(r[74:78])
	want := sonyBluetoothCRC(r, 78)
	if got != want || got == 0 {
		t.Fatalf("control crc got=%08x want=%08x", got, want)
	}
}

func TestSharedSurfaceSpeedGate(t *testing.T) {
	if got := continuousSurfaceStrength("cobblestone", 1, 0.12, .02, 0); got != 0 {
		t.Fatalf("USB reference should be silent below 0.30 m/s, got %.6f", got)
	}
	if got := continuousSurfaceStrength("cobblestone", 1, 0.31, .02, 0); got <= 0 {
		t.Fatal("USB reference did not activate above its 0.30 m/s gate")
	}
}

func TestSharedNoArtificialMaterialDropoutHold(t *testing.T) {
	m := newHapticMixer()
	now := time.Now()
	base := telemetry{Version: 40, Active: true, Raw: &rawTelemetry{Speed: 8, GroundedWheels: 4, SurfaceMaterialL: 19, SurfaceMaterialR: 19, SurfaceRoughnessL: 1, SurfaceRoughnessR: 1, RoadExcitationL: .3, RoadExcitationR: .3}}
	m.update(base, now)
	_, _ = m.render(canonicalFramesForBluetoothFrames(32), now)
	base.Raw.SurfaceMaterialL, base.Raw.SurfaceMaterialR = -1, -1
	base.Raw.SurfaceRoughnessL, base.Raw.SurfaceRoughnessR = 0, 0
	base.Raw.RoadExcitationL, base.Raw.RoadExcitationR = 0, 0
	m.update(base, now.Add(20*time.Millisecond))
	_, held := m.render(canonicalFramesForBluetoothFrames(32), now.Add(20*time.Millisecond))
	if held.surfaceL != 0 || held.surfaceR != 0 {
		t.Fatalf("Bluetooth added a material hold absent from USB: %.3f %.3f", held.surfaceL, held.surfaceR)
	}
}

func TestBumpStrengthPreservesDynamicRange(t *testing.T) {
	m := newHapticMixer()
	now := time.Now()
	m.update(telemetry{Version: 40, Active: true, BodyEvent: 1, Raw: &rawTelemetry{GroundedWheels: 4}}, now)
	m.update(telemetry{Version: 40, Active: true, BodyEvent: 2, BodyProfile: "suspension_bump", BodySide: -1, BodyStrength: 0.30, BodyLeftStrength: 0.30, BodyDurationMS: 58, Raw: &rawTelemetry{GroundedWheels: 4}}, now)
	if len(m.voices) != 1 || m.voices[0].left < 0.34 || m.voices[0].left > 0.40 || m.voices[0].right != 0 || m.voices[0].durationSamples != 58*canonicalHapticSampleRate/1000 {
		t.Fatalf("weak bump voice lost dynamic range: %+v", m.voices)
	}
	weak := m.voices[0].left
	m.update(telemetry{Version: 40, Active: true, BodyEvent: 3, BodyProfile: "suspension_bump", BodySide: -1, BodyStrength: 0.95, BodyLeftStrength: 0.95, BodyDurationMS: 110, Raw: &rawTelemetry{GroundedWheels: 4}}, now.Add(50*time.Millisecond))
	if len(m.voices) < 2 || m.voices[len(m.voices)-1].left <= weak+0.35 {
		t.Fatalf("strong bump should be materially stronger than a seam: weak=%.3f voices=%+v", weak, m.voices)
	}
}

func TestSharedDoesNotApplyBluetoothGammaBoost(t *testing.T) {
	// The USB path clamps/mixes linearly. The shared engine must not put the former
	// Bluetooth gamma boost back into the render path.
	src, err := os.ReadFile("haptic_mixer.go")
	if err != nil {
		t.Fatal(err)
	}
	renderStart := bytes.Index(src, []byte("func (m *hapticMixer) render"))
	if renderStart < 0 {
		t.Fatal("render function missing")
	}
	if bytes.Contains(src[renderStart:], []byte("bluetoothPerceptualCurve(l)")) {
		t.Fatal("Bluetooth gamma compensation still applied in render path")
	}
}

func TestMaskedControlDoesNotTouchUnrelatedGroups(t *testing.T) {
	state := telemetry{Version: 40, Active: true, L2Mode: 1, L2StartZone: 1, L2StartStrength: strength8To255(2), L2EndStrength: strength8To255(4), R2Mode: 1, R2StartZone: 1, R2StartStrength: strength8To255(2), R2EndStrength: strength8To255(4)}
	triggerOnly := buildBluetoothControlReportMasked(0, state, [3]byte{200, 30, 10}, 547, true, false)
	common := triggerOnly[3:50]
	if common[0] != 0x0C || common[1] != 0 || common[38] != 0 {
		t.Fatalf("trigger mask leaked into unrelated groups: %02x %02x %02x", common[0], common[1], common[38])
	}
	if common[44] != 0 || common[45] != 0 || common[46] != 0 {
		t.Fatal("trigger-only packet rewrote the lightbar")
	}
	ledOnly := buildBluetoothControlReportMasked(0, state, [3]byte{200, 30, 10}, 547, false, true)
	common = ledOnly[3:50]
	if common[0] != 0 || common[1] != 0x04 || common[38] != 0 {
		t.Fatalf("LED mask leaked into unrelated groups: %02x %02x %02x", common[0], common[1], common[38])
	}
	if common[10] != 0 || common[21] != 0 {
		t.Fatal("LED-only packet rewrote adaptive triggers")
	}
}

func TestRPMLEDBlinksOnlyAtLimiter(t *testing.T) {
	controller := &lightbarController{}
	state := telemetry{Version: 40, Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{RPM: 6900, MaxRPM: 7000, EngineRunning: true, RevLimiter: true}}
	base := time.Unix(1000, 0)
	seenOn, seenOff := false, false
	for i := 0; i < 20; i++ {
		got := controller.update(state, base.Add(time.Duration(i)*35*time.Millisecond))
		if got == ([3]byte{}) {
			seenOff = true
		} else {
			seenOn = true
		}
		if !controller.isBlinking() {
			t.Fatal("limiter did not enable blink state")
		}
	}
	if !seenOn || !seenOff {
		t.Fatalf("limiter did not alternate on/off: on=%t off=%t", seenOn, seenOff)
	}
}

func TestSharedRPMLEDThresholds(t *testing.T) {
	c := &lightbarController{}
	base := time.Unix(2000, 0)
	state := telemetry{Version: 40, Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{MaxRPM: 7000, EngineRunning: true}}
	state.Raw.RPM = 0.699 * state.Raw.MaxRPM
	if got := c.update(state, base); got != ([3]byte{}) {
		t.Fatalf("USB LED should be off below 70%%: %v", got)
	}
	state.Raw.RPM = 0.70 * state.Raw.MaxRPM
	if got := c.update(state, base); got != ([3]byte{0, 48, 0}) {
		t.Fatalf("USB first LED mismatch: %v", got)
	}
	state.Raw.RPM = 0.95 * state.Raw.MaxRPM
	if got := c.update(state, base); got != ([3]byte{220, 0, 0}) {
		t.Fatalf("USB redline mismatch: %v", got)
	}
}

func TestSharedContinuousSurfaceOverSpatialWindow(t *testing.T) {
	m := newHapticMixer()
	baseTime := time.Unix(5000, 0)
	state := telemetry{Version: 40, Active: true, Raw: &rawTelemetry{Speed: 15, GroundedWheels: 4, SurfaceMaterialL: 30, SurfaceMaterialR: 30, SurfaceRoughnessL: .78, SurfaceRoughnessR: .78, RoadExcitationL: .35, RoadExcitationR: .35}}
	nonSilent := 0
	totalRMS := 0.0
	for i := 0; i < 120; i++ {
		now := baseTime.Add(time.Duration(i) * 11 * time.Millisecond)
		m.update(state, now)
		samples, st := m.render(canonicalFramesForBluetoothFrames(32), now)
		if countNonSilent(samples) > 0 {
			nonSilent++
		}
		totalRMS += st.blockRMS
	}
	if nonSilent < 45 {
		t.Fatalf("USB spatial scheduler too silent: %d/120 blocks", nonSilent)
	}
	if totalRMS/120 < 0.04 {
		t.Fatalf("USB parity average RMS too low: %.3f", totalRMS/120)
	}
}

func TestRoughSurfaceGapKeepsMaterialBed(t *testing.T) {
	if got := audibleSurfaceStrength("gravel", .20, 0, 0); got <= 0 {
		t.Fatalf("gravel scheduler gap became silent: %.6f", got)
	}
	if got := audibleSurfaceStrength("cobblestone", .20, 0, 0); got <= 0 {
		t.Fatalf("cobblestone scheduler gap became silent: %.6f", got)
	}
	if got := audibleSurfaceStrength("asphalt", .20, 0, 0); got != 0 {
		t.Fatalf("smooth asphalt must keep a zero material bed: %.6f", got)
	}
}

func TestSuspensionBumpKeepsImpactedSideSurfaceLayer(t *testing.T) {
	m := newHapticMixer()
	base := time.Unix(5050, 0)
	state := telemetry{Version: protocolVersion, Active: true, Raw: &rawTelemetry{
		Speed: 12, GroundedWheels: 4,
		SurfaceMaterialL: 30, SurfaceMaterialR: 30,
		SurfaceRoughnessL: .78, SurfaceRoughnessR: .78,
		RoadExcitationL: .45, RoadExcitationR: .45,
	}}
	// Warm up the continuous surface lanes first.
	for i := 0; i < 12; i++ {
		now := base.Add(time.Duration(i) * 11 * time.Millisecond)
		m.update(state, now)
		m.render(canonicalFramesForBluetoothFrames(32), now)
	}

	bump := state
	bump.BodyEvent = 1
	bump.BodyKind = "wheel"
	bump.BodyProfile = "suspension_bump"
	bump.BodySide = -1
	bump.BodyStrength = .90
	bump.BodyLeftStrength = .90
	bump.BodyRightStrength = 0
	bump.BodyDurationMS = 105
	now := base.Add(150 * time.Millisecond)
	m.update(bump, now)

	leftSurface, rightSurface := 0, 0
	// Stay inside the 105 ms bump/isolation window. After it expires the normal
	// bilateral road bed is expected to return.
	for i := 0; i < 8; i++ {
		samples, st := m.render(canonicalFramesForBluetoothFrames(32), now.Add(time.Duration(i)*11*time.Millisecond))
		_ = samples
		for j := 0; j+1 < len(st.surfacePCM); j += 2 {
			leftSurface += absInt(int(st.surfacePCM[j]))
			rightSurface += absInt(int(st.surfacePCM[j+1]))
		}
	}
	if leftSurface == 0 {
		t.Fatal("directional suspension bump erased impacted-side surface")
	}
	if rightSurface != 0 {
		t.Fatalf("directional suspension bump leaked opposite-side surface: L=%d R=%d", leftSurface, rightSurface)
	}
}

func TestRuntimeControlNeverSelectsCompatibleRumbleOrAudioRoute(t *testing.T) {
	state := telemetry{Version: protocolVersion, Active: true, L2Mode: 1, L2StartZone: 0, L2StartStrength: strength8To255(2), L2EndStrength: strength8To255(6), R2Mode: 1, R2StartZone: 0, R2StartStrength: strength8To255(1), R2EndStrength: strength8To255(2)}
	triggers := buildBluetoothControlReportMasked(0, state, [3]byte{}, 547, true, false)
	if triggers[3] != 0x0C || triggers[4] != 0x00 {
		t.Fatalf("trigger mask=%02x/%02x want 0c/00", triggers[3], triggers[4])
	}
	if triggers[5] != 0 || triggers[6] != 0 {
		t.Fatalf("compatible rumble bytes changed: %02x %02x", triggers[5], triggers[6])
	}
	led := buildBluetoothControlReportMasked(0, state, [3]byte{64, 120, 0}, 547, false, true)
	if led[3] != 0 || led[4] != 0x04 {
		t.Fatalf("LED mask=%02x/%02x want 00/04", led[3], led[4])
	}
}

func TestRPMLEDDoesNotBlinkAwayFromLimiter(t *testing.T) {
	c := &lightbarController{}
	now := time.Unix(1000, 0)
	for i := 0; i < 40; i++ {
		rpm := 5200.0 + float64(i%3)*8
		state := telemetry{Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{EngineRunning: true, RPM: rpm, MaxRPM: 7000}}
		_ = c.update(state, now.Add(time.Duration(i)*25*time.Millisecond))
		if c.isBlinking() {
			t.Fatal("LED blinked without rev limiter")
		}
	}
}

func TestSharedBodyPanCollision(t *testing.T) {
	state := telemetry{BodyKind: "collision", BodyProfile: "collision", BodySide: -1, BodyStrength: .7, BodyLeftStrength: .7, BodyRightStrength: .12, BodyDurationMS: 180}
	profile, left, right, dur := bodyStereo(state)
	if profile != "collision" || dur != 180 {
		t.Fatalf("profile/duration mismatch %s %d", profile, dur)
	}
	if left < .94 || right != 0 {
		t.Fatalf("USB collision pan mismatch left=%.3f right=%.3f", left, right)
	}
}

func TestSharedShiftCue(t *testing.T) {
	level, dur := shiftCue(telemetry{ShiftStrength: .40, ShiftDurationMS: 70})
	if math.Abs(level-.15) > 1e-9 || dur != 70 {
		t.Fatalf("USB shift cue mismatch level=%.6f dur=%d", level, dur)
	}
	level, dur = shiftCue(telemetry{ShiftStrength: .10, ShiftDurationMS: 20})
	if math.Abs(level-.11) > 1e-9 || dur != 68 {
		t.Fatalf("USB shift floor mismatch level=%.6f dur=%d", level, dur)
	}
}

func TestSharedABSHybridPulse(t *testing.T) {
	var state absPulseState
	base := time.Unix(6000, 0)
	cmd := telemetry{Active: true, L2Mode: 2, L2Amplitude: strength8To255(5), Raw: &rawTelemetry{ABS: true, ABSSeverity: .8, ABSWheelCount: 1, ABSControlHz: 100, Brake: .75}}
	got, active, phase, _, _, _, _ := applyABSHybridPulse(cmd, base, &state)
	if !active || phase != "release_off" || got.L2Mode != 0 || got.L2StartStrength != 0 || got.L2EndStrength != 0 {
		t.Fatalf("ABS release mismatch active=%t phase=%s state=%+v", active, phase, got)
	}
	got, active, phase, _, _, _, _ = applyABSHybridPulse(cmd, base.Add(8*time.Millisecond), &state)
	if !active || phase != "kick" || got.L2Mode != 1 || got.L2StartStrength != 224 || got.L2EndStrength != 224 {
		t.Fatalf("ABS kick mismatch %+v %s", got, phase)
	}
	got, active, phase, _, _, _, _ = applyABSHybridPulse(cmd, base.Add(25*time.Millisecond), &state)
	if !active || phase != "base" || got.L2StartStrength != 32 || got.L2EndStrength != 32 {
		t.Fatalf("ABS base mismatch %+v %s", got, phase)
	}
}

func TestSharedR2BaselineAndSlip(t *testing.T) {
	var state adaptiveThrottleTriggerState
	now := time.Unix(7000, 0)
	cmd := telemetry{Active: true, R2Mode: 1, R2StartZone: 0, R2StartStrength: 32, R2EndStrength: 64, Raw: &rawTelemetry{EngineRunning: true, DrivenSlip: 0}}
	got, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, now, &state, false)
	if active || got.R2Mode != 1 || got.R2StartStrength != 32 || got.R2EndStrength != 32 {
		t.Fatalf("R2 baseline mismatch %+v", got)
	}
	cmd.Raw.Wheelspin = true
	cmd.Raw.DrivenSlip = 20
	got, active, _, _, _ = applyAdaptiveThrottleTrigger(cmd, now.Add(16*time.Millisecond), &state, false)
	if !active || got.R2Mode != 3 || got.R2StartZone < 1 || got.R2StartStrength < 1 || got.R2StartStrength > 3 {
		t.Fatalf("R2 slip mismatch %+v", got)
	}
}

func TestSharedR2AirborneIsExactOneOver255(t *testing.T) {
	var state adaptiveThrottleTriggerState
	now := time.Unix(7050, 0)
	cmd := telemetry{Active: true, Raw: &rawTelemetry{EngineRunning: true, Airborne: true, GroundedWheels: 0, Wheelspin: true, DrivenSlip: 20, TCS: true, RevLimiter: true}}
	got, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, now, &state, true)
	if !active || got.R2Mode != 3 || got.R2StartZone != 0 || got.R2StartStrength != 1 || got.R2EndStrength != 1 || got.R2Amplitude != 0 || got.R2Hz != 0 {
		t.Fatalf("airborne R2 must be exact 1/255: %+v", got)
	}
}

func TestSharedLEDOnlyBlinksOnRevLimiter(t *testing.T) {
	c := &lightbarController{}
	state := telemetry{Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{EngineRunning: true, RPM: 6950, MaxRPM: 7000}}
	base := time.Unix(8000, 0)
	first := c.update(state, base)
	for i := 1; i < 20; i++ {
		rgb := c.update(state, base.Add(time.Duration(i)*35*time.Millisecond))
		if c.isBlinking() || rgb != first {
			t.Fatalf("LED changed/blinked without limiter first=%v got=%v blink=%t", first, rgb, c.isBlinking())
		}
	}
	state.Raw.RevLimiter = true
	seenOn, seenOff := false, false
	for i := 0; i < 20; i++ {
		rgb := c.update(state, base.Add(time.Second+time.Duration(i)*35*time.Millisecond))
		if rgb == ([3]byte{}) {
			seenOff = true
		} else {
			seenOn = true
		}
		if !c.isBlinking() {
			t.Fatal("rev-limiter did not enable LED blink")
		}
	}
	if !seenOn || !seenOff {
		t.Fatalf("limiter blink missing on=%t off=%t", seenOn, seenOff)
	}
}

func TestAllInOne36Layout(t *testing.T) {
	state := telemetry{Version: 40, Active: true, L2Mode: 1, L2StartZone: 32, L2StartStrength: strength8To255(5), L2EndStrength: strength8To255(6), R2Mode: 2, R2StartZone: 40, R2Amplitude: strength8To255(4), R2Hz: 30}
	rgb := [3]byte{16, 88, 200}
	setState := buildBluetoothSetStateData63(state, rgb)
	if len(setState) != 63 {
		t.Fatalf("state len=%d", len(setState))
	}
	if setState[0] != 0xFD || setState[1] != 0x84 {
		t.Fatalf("state flags=%02x/%02x", setState[0], setState[1])
	}
	if setState[6] != 0x40 || setState[7] != 0x09 || setState[9] != 0x00 || setState[38] != 0x00 {
		t.Fatalf("persistent state mismatch mic=%02x audio=%02x powerSave=%02x valid3=%02x", setState[6], setState[7], setState[9], setState[38])
	}
	if setState[41] != 0x00 {
		t.Fatalf("runtime lightbar setup must stay clear, got %02x", setState[41])
	}
	if setState[44] != rgb[0] || setState[45] != rgb[1] || setState[46] != rgb[2] {
		t.Fatalf("RGB shifted in SetStateData")
	}
	// Critical regression guard: no USB report-id 0x02 may prefix SetStateData.
	if setState[0] == 0x02 {
		t.Fatal("SetStateData incorrectly prefixed with USB report ID")
	}
	samples := make([]int8, 64)
	samples[0], samples[1] = 70, -70
	r := buildBluetoothAllInOneReport36(3, 9, samples, setState)
	if len(r) != 398 {
		t.Fatalf("report len=%d", len(r))
	}
	if r[0] != 0x36 || r[1] != 0x30 || r[2] != 0x91 || r[3] != 7 || r[4] != 0xFE {
		t.Fatalf("bad header %x", r[:13])
	}
	if r[11] != 0x90 || r[12] != 63 {
		t.Fatalf("missing SetStateData sized packet")
	}
	if r[13] != 0xFD || r[14] != 0x84 {
		t.Fatalf("SetStateData offset is wrong: %x", r[13:16])
	}
	if r[57] != rgb[0] || r[58] != rgb[1] || r[59] != rgb[2] {
		t.Fatalf("RGB wrong in combined packet")
	}
	if r[76] != 0x92 || r[77] != 64 || int8(r[78]) != 70 || int8(r[79]) != -70 {
		t.Fatalf("haptic offset wrong")
	}
	want := sonyBluetoothCRC(r, 398)
	got := binary.LittleEndian.Uint32(r[394:398])
	if got != want {
		t.Fatalf("crc=%08x want=%08x", got, want)
	}
}

func TestAllInOneCarriesTriggersLEDAndHapticsTogether(t *testing.T) {
	state := telemetry{Version: 40, Active: true, L2Mode: 1, L2StartZone: 20, L2StartStrength: strength8To255(4), L2EndStrength: strength8To255(6), R2Mode: 1, R2StartZone: 18, R2StartStrength: strength8To255(3), R2EndStrength: strength8To255(5)}
	rgb := [3]byte{224, 32, 0}
	ss := buildBluetoothSetStateData63(state, rgb)
	samples := make([]int8, 64)
	for i := range samples {
		if i%2 == 0 {
			samples[i] = 40
		} else {
			samples[i] = -30
		}
	}
	r := buildBluetoothAllInOneReport36(1, 2, samples, ss)
	if r[23] == 0x05 || r[34] == 0x05 {
		t.Fatalf("trigger state missing from combined report")
	}
	if r[57] != 224 || r[58] != 32 || r[59] != 0 {
		t.Fatalf("LED missing from combined report")
	}
	if countNonSilent(samples) != 64 {
		t.Fatalf("haptic payload unexpectedly silent")
	}
}

func TestSharedVoicePrioritiesAndDucks(t *testing.T) {
	cases := []struct {
		profile  string
		priority int
		surface  float64
		global   float64
	}{
		{"collision", 50, .50, .10},
		{"landing", 40, .68, .42},
		{"suspension_bump", 30, .60, .78},
		{"shift", 25, .76, .76},
		{"cobblestone", 12, 1.0, 1.0},
	}
	for _, tc := range cases {
		if got := profilePriority(tc.profile); got != tc.priority {
			t.Fatalf("priority %s=%d want=%d", tc.profile, got, tc.priority)
		}
		s, g := ducksForPriority(tc.priority)
		if math.Abs(s-tc.surface) > 1e-9 || math.Abs(g-tc.global) > 1e-9 {
			t.Fatalf("duck %s got %.2f/%.2f want %.2f/%.2f", tc.profile, s, g, tc.surface, tc.global)
		}
	}
}

func TestSharedSurfaceLanePhaseSurvivesSchedulerGap(t *testing.T) {
	lane := surfaceLane{}
	setSurfaceLane(&lane, "asphalt", .12, 10, 0xA341316C)
	if lane.profile != "asphalt" || lane.noise != 0xA341316C {
		t.Fatalf("initial lane mismatch: %+v", lane)
	}
	lane.t = 1.25
	setSurfaceLane(&lane, "none", 0, 10, 0xA341316C)
	if lane.profile != "asphalt" || lane.t != 1.25 || lane.target != 0 {
		t.Fatalf("USB gap must preserve profile phase: %+v", lane)
	}
	setSurfaceLane(&lane, "asphalt", .10, 10, 0xA341316C)
	if lane.t != 1.25 {
		t.Fatalf("same material restarted phase after gap: %.3f", lane.t)
	}
	setSurfaceLane(&lane, "gravel", .10, 10, 0xA341316C)
	if lane.profile != "gravel" || lane.t != 0 {
		t.Fatalf("real material change must reset phase: %+v", lane)
	}
}

func TestSharedVoiceMergeAndDirection(t *testing.T) {
	m := newHapticMixer()
	now := time.Unix(9000, 0)
	m.enqueueVoice("suspension_bump", .50, 0, 80, now, 1)
	// A duplicate observation of the same hit within 20 ms should merge.
	m.enqueueVoice("suspension_bump", .70, 0, 82, now.Add(20*time.Millisecond), 2)
	if len(m.voices) != 1 {
		t.Fatalf("same-side duplicate inside short merge window should merge: %d", len(m.voices))
	}
	v := m.voices[0]
	if math.Abs(v.left-.70) > 1e-9 || v.durationSamples != 82*canonicalHapticSampleRate/1000 || v.pos != 0 {
		t.Fatalf("merged voice mismatch: %+v", v)
	}
	// A genuine next bumper 100 ms later must remain a separate impulse.
	m.enqueueVoice("suspension_bump", .60, 0, 70, now.Add(100*time.Millisecond), 3)
	if len(m.voices) != 2 {
		t.Fatalf("distinct same-side bumpers must remain separate, voices=%d", len(m.voices))
	}
	m.enqueueVoice("suspension_bump", 0, .60, 70, now.Add(120*time.Millisecond), 4)
	if len(m.voices) != 3 {
		t.Fatalf("opposite-side impacts must remain separate, voices=%d", len(m.voices))
	}
}

func TestSharedTransientDurationClampsLikeWASAPI(t *testing.T) {
	m := newHapticMixer()
	m.enqueueVoice("collision", 1, 1, 280, time.Unix(9100, 0), 1)
	if len(m.voices) != 1 {
		t.Fatal("collision voice missing")
	}
	want := 240 * canonicalHapticSampleRate / 1000
	if m.voices[0].durationSamples != want {
		t.Fatalf("duration=%d want USB PlayStereo clamp=%d", m.voices[0].durationSamples, want)
	}
}

func TestSharedSurfaceLeftRightIsolation(t *testing.T) {
	m := newHapticMixer()
	base := time.Unix(9200, 0)
	state := telemetry{Version: protocolVersion, Active: true, Raw: &rawTelemetry{
		Speed: 12, GroundedWheels: 4,
		SurfaceMaterialL: 19, SurfaceMaterialR: -1,
		SurfaceRoughnessL: .85, SurfaceRoughnessR: 0,
		RoadExcitationL: .35, RoadExcitationR: 0,
	}}
	leftEnergy, rightEnergy := 0, 0
	for i := 0; i < 160; i++ {
		now := base.Add(time.Duration(i) * 11 * time.Millisecond)
		m.update(state, now)
		samples, _ := m.render(canonicalFramesForBluetoothFrames(32), now)
		for j := 0; j < len(samples); j += 2 {
			leftEnergy += int(math.Abs(float64(samples[j])))
			rightEnergy += int(math.Abs(float64(samples[j+1])))
		}
	}
	if leftEnergy == 0 {
		t.Fatal("left surface produced no haptic energy")
	}
	if rightEnergy != 0 {
		t.Fatalf("left-only USB surface leaked into right grip: L=%d R=%d", leftEnergy, rightEnergy)
	}
}

func TestSharedCenteredCollisionRemainsCentered(t *testing.T) {
	profile, left, right, _ := bodyStereo(telemetry{
		BodyKind: "collision", BodyProfile: "collision", BodySide: 0,
		BodyStrength: .75, BodyLeftStrength: .70, BodyRightStrength: .62, BodyDurationMS: 180,
	})
	if profile != "collision" || math.Abs(left-right) > 1e-9 {
		t.Fatalf("center collision must remain centered, profile=%s L=%.6f R=%.6f", profile, left, right)
	}
}

func TestSharedFeelExactLowSpeedAmplitudeFixtures(t *testing.T) {
	cases := []struct{ speed, want float64 }{
		{0.25, 0.16},
		{0.42709120351498, 0.24746277051108626},
		{0.68539270949422, 0.31695239105149914},
		{0.8086580622486, 0.34456108426854004},
		{1.624313, 0.49132197},
		{6.0, 1.0},
	}
	for _, tc := range cases {
		got := lowSpeedSurfaceScale(tc.speed)
		if math.Abs(got-tc.want) > 2e-6 {
			t.Fatalf("low-speed scale %.6f = %.9f want %.9f", tc.speed, got, tc.want)
		}
	}
}

func TestSharedFeelLowSpeedCadence(t *testing.T) {
	c025, k025 := lowSpeedCadenceScales(.25)
	c8, k8 := lowSpeedCadenceScales(8)
	_, k20 := lowSpeedCadenceScales(20)
	if !(c025 < c8 && k025 < k8) {
		t.Fatalf("low speed not slowed: %.3f/%.3f vs %.3f/%.3f", c025, k025, c8, k8)
	}
	if math.Abs(c8-1) > 1e-9 {
		t.Fatalf("cadence at 8m/s = %.4f", c8)
	}
	if k20 <= k8 {
		t.Fatalf("high-speed carrier did not rise: %.3f <= %.3f", k20, k8)
	}
}

func TestCommonFeelProfileHasNoTransportSpecificCompensation(t *testing.T) {
	src, err := os.ReadFile("feel_profile.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(src, []byte("TransportCompensation")) || bytes.Contains(src, []byte("bluetoothSurfaceTransportGain")) {
		t.Fatal("transport-specific feel compensation leaked back into the Common Feel profile")
	}
}

func TestSharedFeelSyntheticCooldownSlowsAtLowSpeed(t *testing.T) {
	if got := surfaceCueCooldownAt("asphalt", 0); got != 800*time.Millisecond {
		t.Fatalf("asphalt slow cooldown=%v", got)
	}
	if got := surfaceCueCooldownAt("asphalt", 6); got != 140*time.Millisecond {
		t.Fatalf("asphalt normal cooldown=%v", got)
	}
	if got := surfaceCueCooldownAt("rumble_strip", 0); got != 240*time.Millisecond {
		t.Fatalf("rumble slow cooldown=%v", got)
	}
	if got := surfaceCueCooldownAt("rumble_strip", 6); got != 35*time.Millisecond {
		t.Fatalf("rumble normal cooldown=%v", got)
	}
}

func TestSharedFeelShiftR2ExactOneOver255(t *testing.T) {
	var st adaptiveThrottleTriggerState
	tm := time.Unix(9000, 0)
	cmd := telemetry{Active: true, Raw: &rawTelemetry{EngineRunning: true}}
	got, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, tm, &st, true)
	if !active || got.R2Mode != 3 || got.R2StartZone != 0 || got.R2StartStrength != 1 || got.R2EndStrength != 1 {
		t.Fatalf("shift R2 mismatch %+v", got)
	}
}

func TestSharedFeelBodyHardIsolation(t *testing.T) {
	m := newHapticMixer()
	now := time.Unix(10000, 0)
	m.updateBodyIsolation(telemetry{BodyKind: "collision", BodyProfile: "collision", BodySide: -1}, now)
	hardSamples := int(math.Round(0.032 * m.rate()))
	for i := 0; i < hardSamples; i++ {
		l, r := m.bodyIsolationGains()
		if l != 1 || r != 0 {
			t.Fatalf("collision hard attack sample %d gains %.3f/%.3f", i, l, r)
		}
	}
	l, r := m.bodyIsolationGains()
	if l != 1 || !(r >= 0 && r < 0.05) {
		t.Fatalf("collision release did not start near zero %.3f/%.3f", l, r)
	}
	releaseSamples := int(math.Round(0.080 * m.rate()))
	for i := 0; i < releaseSamples; i++ {
		l, r = m.bodyIsolationGains()
	}
	l, r = m.bodyIsolationGains()
	if l != 1 || r != 1 {
		t.Fatalf("collision release did not finish %.3f/%.3f", l, r)
	}
}

func TestSharedFeelBodyIsolationIgnoresWallClockStall(t *testing.T) {
	m := newHapticMixer()
	now := time.Unix(10500, 0)
	m.updateBodyIsolation(telemetry{BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1}, now)
	// Simulate a 50 ms WriteFile stall. No samples have been rendered yet, so the
	// first delivered sample must still be in the hard-isolation attack.
	_ = now.Add(50 * time.Millisecond)
	l, r := m.bodyIsolationGains()
	if l != 1 || r != 0 {
		t.Fatalf("wall-clock stall consumed bump isolation %.3f/%.3f", l, r)
	}
}

func TestSharedFeelOppositeImpactsMergeBilateral(t *testing.T) {
	m := newHapticMixer()
	now := time.Unix(11000, 0)
	m.updateBodyIsolation(telemetry{BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1}, now)
	m.updateBodyIsolation(telemetry{BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: 1}, now.Add(4*time.Millisecond))
	l, r := m.bodyIsolationGains()
	if l != 1 || r != 1 {
		t.Fatalf("opposite impacts within merge window stayed unilateral %.3f/%.3f", l, r)
	}
}

func TestSharedFeelProfileLoadsDefaults(t *testing.T) {
	p := feelProfile()
	if p.ProfileVersion == "" || p.Triggers.ABS.KickStrength255 != 224 || p.Triggers.R2.ShiftDurationMS != 150 || p.ShiftHaptic.Priority != 25 {
		t.Fatalf("shared profile mismatch %+v", p)
	}
}

func TestSharedFeelEngineShiftCutIsExactlyEventTimed(t *testing.T) {
	m := newHapticMixer()
	e := newSharedFeelEngine(m)
	base := time.Unix(12000, 0)
	state := telemetry{Version: 40, Active: true, ShiftEvent: 1, Raw: &rawTelemetry{EngineRunning: true, GroundedWheels: 4}}
	_ = e.step(state, base, base, canonicalFramesForBluetoothFrames(32)) // synchronize counter
	state.ShiftEvent = 2
	frame := e.step(state, base.Add(10*time.Millisecond), base.Add(10*time.Millisecond), canonicalFramesForBluetoothFrames(32))
	if frame.Control.R2Mode != 3 || frame.Control.R2StartZone != 0 || frame.Control.R2StartStrength != 1 {
		t.Fatalf("shift cut did not start with exact 1/255: %+v", frame.Control)
	}
	// Raw.Shifting stays true deliberately: it must not extend the 150 ms event window.
	state.Raw.Shifting = true
	frame = e.step(state, base.Add(170*time.Millisecond), base.Add(170*time.Millisecond), canonicalFramesForBluetoothFrames(32))
	if frame.Control.R2Mode != 1 || frame.Control.R2StartStrength != 32 || frame.Control.R2EndStrength != 32 {
		t.Fatalf("shift cut was extended beyond 150 ms by Raw.Shifting: %+v", frame.Control)
	}
}

func TestSharedFeelLEDNoFlickerAroundFirstThreshold(t *testing.T) {
	c := &lightbarController{}
	base := time.Unix(13000, 0)
	state := telemetry{Version: 40, Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{EngineRunning: true, MaxRPM: 7000}}
	state.Raw.RPM = 5000 // 71.4%, activate
	first := c.update(state, base)
	if first == ([3]byte{}) || c.status() != "progress" {
		t.Fatalf("LED did not enter progress phase: %v %s", first, c.status())
	}
	// Oscillate around the 70% threshold for one second. It must never go black.
	for i := 1; i <= 40; i++ {
		if i%2 == 0 {
			state.Raw.RPM = 4865
		} else {
			state.Raw.RPM = 4935
		}
		got := c.update(state, base.Add(time.Duration(i)*25*time.Millisecond))
		if got == ([3]byte{}) {
			t.Fatalf("LED flickered off around first threshold at step %d", i)
		}
	}
}

func TestSharedFeelLEDTurnsOffOnlyAfterSustainedLowRPM(t *testing.T) {
	c := &lightbarController{}
	base := time.Unix(14000, 0)
	state := telemetry{Version: 40, Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{EngineRunning: true, MaxRPM: 7000, RPM: 5200}}
	_ = c.update(state, base)
	state.Raw.RPM = 4200 // 60%, below 65% off hysteresis
	if got := c.update(state, base.Add(100*time.Millisecond)); got == ([3]byte{}) {
		t.Fatal("LED turned off before hold elapsed")
	}
	if got := c.update(state, base.Add(450*time.Millisecond)); got != ([3]byte{}) {
		t.Fatalf("LED did not turn off after sustained low RPM: %v", got)
	}
}

func TestSharedFeelLimiterBlinkSurvivesRawLimiterDropout(t *testing.T) {
	c := &lightbarController{}
	base := time.Unix(15000, 0)
	state := telemetry{Version: 40, Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{EngineRunning: true, MaxRPM: 7000, RPM: 6800, RevLimiter: true}}
	_ = c.update(state, base)
	if !c.isBlinking() {
		t.Fatal("limiter did not arm blink")
	}
	state.Raw.RevLimiter = false
	_ = c.update(state, base.Add(60*time.Millisecond))
	if !c.isBlinking() {
		t.Fatal("single limiter dropout stopped blink immediately")
	}
	_ = c.update(state, base.Add(250*time.Millisecond))
	if c.isBlinking() {
		t.Fatal("limiter latch did not expire")
	}
}

func TestUSBTransportPreservesCommonStereoPan(t *testing.T) {
	var q sharedPCMQueue
	samples := make([]int8, 64*2)
	for i := 0; i < 64; i++ {
		samples[i*2] = int8(40 + i%40)
	}
	q.push(samples)
	l, r := q.renderHold(64, canonicalHapticSampleRate)
	if len(l) != 64 || len(r) != 64 {
		t.Fatal("USB pass-through length mismatch")
	}
	for i := range r {
		if r[i] != 0 {
			t.Fatalf("left-only common frame leaked to right USB channel at %d: %.6f", i, r[i])
		}
	}
	if l[0] <= 0 || l[31] <= 0 || l[63] <= 0 {
		t.Fatalf("left USB channel lost canonical samples: %.3f %.3f %.3f", l[0], l[31], l[63])
	}
}

func TestUSBTransportRightMirror(t *testing.T) {
	var leftQ, rightQ sharedPCMQueue
	left := make([]int8, 32*2)
	right := make([]int8, 32*2)
	for i := 0; i < 32; i++ {
		v := int8(45 + i%20)
		left[i*2] = v
		right[i*2+1] = v
	}
	leftQ.push(left)
	rightQ.push(right)
	ll, lr := leftQ.renderHold(32, canonicalHapticSampleRate)
	rl, rr := rightQ.renderHold(32, canonicalHapticSampleRate)
	for i := range ll {
		if lr[i] != 0 || rl[i] != 0 || math.Abs(ll[i]-rr[i]) > 1e-12 {
			t.Fatalf("USB left/right mirror mismatch at %d: L %.6f/%.6f R %.6f/%.6f", i, ll[i], lr[i], rl[i], rr[i])
		}
	}
}

func TestUSBTransportUnderrunDoesNotSkipFutureFrame(t *testing.T) {
	var q sharedPCMQueue
	l, r := q.renderHold(96, 48000)
	for i := range l {
		if l[i] != 0 || r[i] != 0 {
			t.Fatal("empty USB queue was not silent")
		}
	}
	q.push([]int8{100, -100})
	l, r = q.renderHold(1, 48000)
	if l[0] <= .7 || r[0] >= -.7 {
		t.Fatalf("post-underrun frame was skipped: %.3f %.3f", l[0], r[0])
	}
}

func TestSharedBodyPanIsExactLeftRightMirror(t *testing.T) {
	leftState := telemetry{BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1, BodyStrength: .42, BodyLeftStrength: .42, BodyRightStrength: .08, BodyDurationMS: 90}
	rightState := leftState
	rightState.BodySide = 1
	rightState.BodyLeftStrength, rightState.BodyRightStrength = leftState.BodyRightStrength, leftState.BodyLeftStrength
	lp, ll, lr, ld := bodyStereo(leftState)
	rp, rl, rr, rd := bodyStereo(rightState)
	if lp != rp || ld != rd {
		t.Fatalf("mirrored bump metadata differs %s/%s %d/%d", lp, rp, ld, rd)
	}
	if math.Abs(ll-rr) > 1e-12 || math.Abs(lr-rl) > 1e-12 {
		t.Fatalf("mirrored bump pan differs L %.6f/%.6f R %.6f/%.6f", ll, lr, rl, rr)
	}
	if lr != 0 || rl != 0 {
		t.Fatalf("unilateral bump attack leaks to wrong side: %.6f %.6f", lr, rl)
	}
}

func TestBluetoothRuntimeNeverRetriggersLightbarSetup(t *testing.T) {
	state := buildBluetoothSetStateData63(telemetry{Active: true}, [3]byte{64, 32, 0})
	if state[38] != 0 {
		t.Fatalf("runtime valid_flag2=%02x; setup/compatible-vibration2 must be clear", state[38])
	}
	if state[41] != 0 {
		t.Fatalf("runtime lightbar_setup=%02x; setup must not repeat", state[41])
	}
	if state[1]&0x04 == 0 {
		t.Fatalf("runtime RGB control bit missing in valid_flag1=%02x", state[1])
	}
}

func TestUSBTransportPreservesCanonicalReference(t *testing.T) {
	stream := newCanonicalPCMStream()
	frames := canonicalFramesForBluetoothFrames(hapticFramesPerReport36)
	in := make([]int8, frames*2)
	for i := 0; i < frames; i++ {
		phase := 2 * math.Pi * 180 * float64(i) / canonicalHapticSampleRate
		in[i*2] = int8(math.Round(math.Sin(phase) * 90))
		in[i*2+1] = int8(math.Round(math.Cos(phase) * 55))
	}
	out := stream.process(in)
	if len(out.USB48k) != len(in) {
		t.Fatalf("USB block length=%d want=%d", len(out.USB48k), len(in))
	}
	profile := feelProfile()
	usbGain := profile.Transport.USBOutputGain
	if usbGain <= 0 {
		usbGain = 1
	}
	for i, sample := range in {
		expected := quantizeHaptic((float64(sample) / 127.0) * usbGain)
		if out.USB48k[i] != expected {
			t.Fatalf("USB transport modified canonical sample %d: got=%d want=%d", i, out.USB48k[i], expected)
		}
	}
	if len(out.Bluetooth3k) != hapticFramesPerReport36*2 {
		t.Fatalf("BT block length=%d want=%d", len(out.Bluetooth3k), hapticFramesPerReport36*2)
	}
}

func TestBluetoothAdapterPreservesDesignedBandAndRejectsAliases(t *testing.T) {
	rmsFor := func(hz float64) float64 {
		stream := newCanonicalPCMStream()
		frames := canonicalHapticSampleRate
		in := make([]int8, frames*2)
		for i := 0; i < frames; i++ {
			v := int8(math.Round(math.Sin(2*math.Pi*hz*float64(i)/canonicalHapticSampleRate) * 100))
			in[i*2], in[i*2+1] = v, v
		}
		out := stream.process(in).Bluetooth3k
		start := bluetoothHapticSampleRate / 10
		ss := 0.0
		n := 0
		for i := start; i < len(out)/2; i++ {
			v := float64(out[i*2]) / 127
			ss += v * v
			n++
		}
		return math.Sqrt(ss / float64(n))
	}
	base := rmsFor(180)
	high := rmsFor(6000)
	if base < 0.30 {
		t.Fatalf("designed 180 Hz Bluetooth cue was over-attenuated: rms=%.3f", base)
	}
	if high > base*0.08 {
		t.Fatalf("Bluetooth anti-alias filter leaves too much 6 kHz energy: base=%.3f high=%.3f", base, high)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestSymmetricSurfaceHasNoRightBias(t *testing.T) {
	m := newCanonicalHapticMixer()
	base := time.Unix(12100, 0)
	state := telemetry{Version: protocolVersion, Active: true, Raw: &rawTelemetry{
		EngineRunning: true, Speed: 10, GroundedWheels: 4,
		SurfaceMaterialL: 10, SurfaceMaterialR: 10,
		SurfaceRoughnessL: .03, SurfaceRoughnessR: .03,
		RoadExcitationL: .16, RoadExcitationR: .16,
	}}
	m.update(state, base)
	m.update(state, base.Add(time.Millisecond))
	left, right := 0.0, 0.0
	for i := 0; i < 120; i++ {
		s, _ := m.render(canonicalFramesForBluetoothFrames(32), base.Add(time.Duration(i)*10667*time.Microsecond))
		for j := 0; j < len(s); j += 2 {
			left += math.Abs(float64(s[j]))
			right += math.Abs(float64(s[j+1]))
		}
	}
	// Independent noise seeds mean instantaneous equality is not expected, but
	// the long-window energy must not show the former double-smoothing right bias.
	ratio := right / math.Max(left, 1)
	if ratio < .80 || ratio > 1.25 {
		t.Fatalf("symmetric surface energy biased L=%.0f R=%.0f ratio=%.3f", left, right, ratio)
	}
}

func TestCanonicalBumpTimingIsSingleSourceForBothTransports(t *testing.T) {
	m := newCanonicalHapticMixer()
	now := time.Unix(12200, 0)
	m.enqueueVoice("suspension_bump", .75, 0, 86, now, 1)
	want := int(math.Round(.086 * canonicalHapticSampleRate))
	if m.voices[0].durationSamples != want {
		t.Fatalf("canonical bump duration=%d want=%d", m.voices[0].durationSamples, want)
	}
	if canonicalFramesForBluetoothFrames(32) != 512 {
		t.Fatalf("one BT report must advance exactly 512 canonical frames")
	}
}

func TestBluetooth36PreservesStereoByteOrder(t *testing.T) {
	samples := make([]int8, 64)
	for i := 0; i < 32; i++ {
		samples[i*2] = 60
		samples[i*2+1] = -20
	}
	r := buildBluetoothAllInOneReport36(1, 2, samples, buildBluetoothSetStateData63(telemetry{}, [3]byte{}))
	for i := 0; i < 32; i++ {
		if int8(r[78+i*2]) != 60 || int8(r[79+i*2]) != -20 {
			t.Fatalf("stereo byte order changed at frame %d: %d/%d", i, int8(r[78+i*2]), int8(r[79+i*2]))
		}
	}
}

func TestBluetoothMenuFramesForceStableBlackLED(t *testing.T) {
	c := &lightbarController{}
	for i := 0; i < 120; i++ {
		now := time.Unix(13000, 0).Add(time.Duration(i) * 11 * time.Millisecond)
		rgb := c.update(telemetry{Active: false}, now)
		if rgb != ([3]byte{}) || c.isBlinking() {
			t.Fatalf("menu LED changed at frame %d rgb=%v blink=%t", i, rgb, c.isBlinking())
		}
		state := buildBluetoothSetStateData63(telemetry{Active: false}, rgb)
		if state[38] != 0 || state[41] != 0 || state[44] != 0 || state[45] != 0 || state[46] != 0 {
			t.Fatalf("menu 0x36 state can retrigger LED frame=%d flags=%02x setup=%02x rgb=%02x%02x%02x", i, state[38], state[41], state[44], state[45], state[46])
		}
	}
}

func TestBumpIsolationCoversWholeTypicalBump(t *testing.T) {
	m := newHapticMixerAtRate(3000)
	now := time.Unix(12000, 0)
	m.updateBodyIsolation(telemetry{BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1}, now)
	for i := 0; i < int(0.110*3000); i++ {
		l, r := m.bodyIsolationGains()
		if l < .999 || r != 0 {
			t.Fatalf("bump leaked before 110ms at %d: %.3f/%.3f", i, l, r)
		}
	}
}

func TestBumpOppositeMergeWindowIsShort(t *testing.T) {
	v := feelProfile().BodyIsolation.SuspensionBump.OppositeMergeWindowMS
	if v <= 0 || v >= 15 {
		t.Fatalf("bump merge window=%d", v)
	}
}

func TestBluetoothLightbarOwnershipIsEventDrivenWithoutSetupRetrigger(t *testing.T) {
	tel := telemetry{Version: 40, Active: true}
	steady := buildBluetoothSetStateData63WithFlags(tel, [3]byte{10, 20, 30}, false, false)
	changed := buildBluetoothSetStateData63WithFlags(tel, [3]byte{10, 20, 30}, true, false)
	if steady[1]&0x04 != 0 || changed[1]&0x04 == 0 {
		t.Fatalf("lightbar validation not event-driven: %02x/%02x", steady[1], changed[1])
	}
	if steady[38] != 0 || steady[41] != 0 {
		t.Fatal("runtime retriggers lightbar setup")
	}
	release := buildBluetoothSetStateData63WithFlags(tel, [3]byte{}, false, true)
	if release[1] != 0x88 || release[38] != 0 || release[41] != 0 {
		t.Fatalf("one-shot RELEASE_LEDS must not combine LIGHT_OUT: flags=%02x valid3=%02x setup=%02x", release[1], release[38], release[41])
	}
}

func TestBluetoothLightbarUpdateCadenceIsCoalesced(t *testing.T) {
	now := time.Unix(15000, 0)
	last := [3]byte{10, 20, 30}
	if bluetoothLightbarUpdateDue(true, last, last, now.Add(-time.Second), now, false) {
		t.Fatal("identical RGB requested another lightbar write")
	}
	changed := [3]byte{40, 20, 30}
	if bluetoothLightbarUpdateDue(true, last, changed, now.Add(-40*time.Millisecond), now, false) {
		t.Fatal("steady RPM lightbar update bypassed 120 ms coalescing")
	}
	if !bluetoothLightbarUpdateDue(true, last, changed, now.Add(-130*time.Millisecond), now, false) {
		t.Fatal("meaningful steady RGB change was not eventually sent")
	}
	if bluetoothLightbarUpdateDue(true, last, changed, now.Add(-30*time.Millisecond), now, true) {
		t.Fatal("limiter blink exceeded 20 Hz transport cap")
	}
	if !bluetoothLightbarUpdateDue(true, last, changed, now.Add(-60*time.Millisecond), now, true) {
		t.Fatal("limiter blink update was over-coalesced")
	}
}

func TestBluetoothSteadyLightbarDoesNotValidateEveryHapticFrame(t *testing.T) {
	tel := telemetry{Version: protocolVersion, Active: true}
	rgb := [3]byte{80, 120, 0}
	if buildBluetoothSetStateData63WithFlags(tel, rgb, false, false)[1]&0x04 != 0 {
		t.Fatal("steady lightbar validated")
	}
	if buildBluetoothSetStateData63WithFlags(tel, rgb, true, false)[1]&0x04 == 0 {
		t.Fatal("changed lightbar not validated")
	}
	release := buildBluetoothSetStateData63WithFlags(tel, [3]byte{}, false, true)
	if release[1] != 0x88 || release[38] != 0 || release[41] != 0 {
		t.Fatal("release packet unexpectedly enables lightbar setup")
	}
}

func TestBluetoothSteadySetStateDoesNotOwnLEDAdjacentState(t *testing.T) {
	state := buildBluetoothSetStateData63WithFlags(telemetry{Version: protocolVersion, Active: true}, [3]byte{20, 40, 60}, false, false)
	if state[1] != 0x80 {
		t.Fatalf("steady valid_flag1=%02x want 80 (audio_control2 only)", state[1])
	}
	const forbidden = byte(0x01 | 0x02 | 0x04 | 0x08 | 0x10)
	if state[1]&forbidden != 0 {
		t.Fatalf("steady frame revalidates LED/power/player state: %02x", state[1])
	}
}

func TestBumpCenterEchoSuppressed(t *testing.T) {
	m := newHapticMixerAtRate(3000)
	now := time.Unix(9000, 0)
	left := telemetry{BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1, BodyLeftStrength: .68, BodyStrength: .68}
	if m.suppressBumpEcho(left, now) {
		t.Fatal("first suppressed")
	}
	center := telemetry{BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: 0, BodyLeftStrength: .68, BodyRightStrength: .68, BodyStrength: .68}
	if !m.suppressBumpEcho(center, now.Add(105*time.Millisecond)) {
		t.Fatal("center echo passed")
	}
}

func TestBumpOppositeEchoFilter(t *testing.T) {
	m := newHapticMixerAtRate(3000)
	now := time.Unix(9100, 0)
	left := telemetry{
		BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1, BodyLeftStrength: .68, BodyStrength: .68,
		Raw: &rawTelemetry{CandidateL: .72, CandidateR: .04, PeakImpulseL: .78, PeakImpulseR: .03},
	}
	if m.suppressBumpEcho(left, now) {
		t.Fatal("first left bump suppressed")
	}

	// Body amplitude alone is not side evidence. A centered/road-texture echo that
	// happens to be labelled right must stay suppressed inside the cluster.
	echo := telemetry{
		BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: 1, BodyRightStrength: .95, BodyStrength: .95,
		Raw: &rawTelemetry{CandidateL: 0, CandidateR: 0, PeakImpulseL: 0, PeakImpulseR: 0},
	}
	if !m.suppressBumpEcho(echo, now.Add(40*time.Millisecond)) {
		t.Fatal("opposite echo without wheel evidence passed")
	}

	// A genuine opposite-wheel hit is allowed only when that wheel has clearly
	// stronger transient candidate/impulse evidence of its own.
	genuineRight := telemetry{
		BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: 1, BodyRightStrength: .80, BodyStrength: .80,
		Raw: &rawTelemetry{CandidateL: .08, CandidateR: .70, PeakImpulseL: .09, PeakImpulseR: .76},
	}
	if m.suppressBumpEcho(genuineRight, now.Add(80*time.Millisecond)) {
		t.Fatal("genuine opposite-wheel transient suppressed")
	}
}

func TestBluetoothSetStateDoesNotRequestHapticPowerSave(t *testing.T) {
	state := buildBluetoothSetStateData63WithFlags(telemetry{Version: protocolVersion, Active: true}, [3]byte{20, 40, 60}, false, false)
	if state[9] != 0 {
		t.Fatalf("power-save byte must stay clear during haptics, got %02x", state[9])
	}
	if state[6] > 0x40 {
		t.Fatalf("microphone volume exceeds DualSense documented range: %02x", state[6])
	}
}

func TestBeamNGBaseLowSpeedScalesAmplitudeNotOnlyCadence(t *testing.T) {
	base := rawTelemetry{GroundedWheels: 4, NativeRumbleBaseForce: .12}
	fast := base
	fast.Speed = 8.0
	fastLow, fastHigh := tactileBeamNGBase(&fast)
	if fastHigh <= 0 || fastLow <= 0 {
		t.Fatalf("expected non-zero fast tactile base: %.4f/%.4f", fastLow, fastHigh)
	}

	slow := base
	slow.Speed = 0.55
	slowLow, slowHigh := tactileBeamNGBase(&slow)
	if slowHigh <= 0 || slowLow <= 0 {
		t.Fatalf("walking-speed tactile base unexpectedly vanished: %.4f/%.4f", slowLow, slowHigh)
	}
	if slowHigh >= fastHigh*.75 || slowLow >= fastLow*.75 {
		t.Fatalf("low-speed amplitude did not attenuate enough: slow=%.4f/%.4f fast=%.4f/%.4f", slowLow, slowHigh, fastLow, fastHigh)
	}

	stopped := base
	stopped.Speed = .30
	stopLow, stopHigh := tactileBeamNGBase(&stopped)
	if stopLow != 0 || stopHigh != 0 {
		t.Fatalf("sub-threshold tactile base should be silent: %.4f/%.4f", stopLow, stopHigh)
	}
}

func TestSuspensionBumpCenterUsesEqualEnergy(t *testing.T) {
	_, l, r, _ := bodyStereo(telemetry{
		BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: 0,
		BodyStrength: 0.30, BodyLeftStrength: 0.30, BodyRightStrength: 0.30, BodyDurationMS: 55,
	})
	if l <= 0 || r <= 0 || math.Abs(l-r) > 1e-9 {
		t.Fatalf("center bump not symmetric: %.6f %.6f", l, r)
	}
	_, one, _, _ := bodyStereo(telemetry{
		BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1,
		BodyStrength: 0.30, BodyLeftStrength: 0.30, BodyRightStrength: 0, BodyDurationMS: 55,
	})
	centerEnergy := l*l + r*r
	oneEnergy := one * one
	if math.Abs(centerEnergy-oneEnergy) > 0.015 {
		t.Fatalf("center energy drift: center=%.6f one=%.6f L/R=%.6f/%.6f one=%.6f", centerEnergy, oneEnergy, l, r, one)
	}
}

func TestSuspensionBumpClearsPriorIsolation(t *testing.T) {
	m := newHapticMixer()
	now := time.Now()
	m.eventSynced = true
	m.bodyEvent = 0
	m.isolation.side = -1
	m.isolation.hardSamplesRemaining = 999
	m.isolation.releaseSamplesRemaining = 999
	m.isolation.releaseSamplesTotal = 999
	m.isolation.lastSide = -1
	m.update(telemetry{
		Version: 40, Active: true, BodyEvent: 1, BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: 0,
		BodyStrength: 0.30, BodyLeftStrength: 0.212132, BodyRightStrength: 0.212132, BodyDurationMS: 55,
		Raw: &rawTelemetry{GroundedWheels: 4},
	}, now)
	if m.isolation.side != 0 || m.isolation.hardSamplesRemaining != 0 || m.isolation.releaseSamplesRemaining != 0 {
		t.Fatalf("suspension bump inherited prior isolation: %+v", m.isolation)
	}
}

func TestExternalRGBStateOwnsNoSecondaryLEDFlags(t *testing.T) {
	state := buildBluetoothSetStateData63ExternalRGB(telemetry{Active: true})
	if len(state) != 63 {
		t.Fatalf("external RGB state len = %d", len(state))
	}
	if state[1] != 0 {
		t.Fatalf("external RGB valid_flag1 must be zero, got 0x%02x", state[1])
	}
	if state[44] != 0 || state[45] != 0 || state[46] != 0 {
		t.Fatalf("external RGB payload must not carry stale RGB: %v", state[44:47])
	}
}

func TestSuspensionBumpHardPanMutesOppositeBackground(t *testing.T) {
	m := newHapticMixerAtRate(3000)
	now := time.Unix(17000, 0)
	// First sync counters with a live packet.
	m.update(telemetry{Version: 40, Active: true, BodyEvent: 1, Raw: &rawTelemetry{GroundedWheels: 4}}, now)
	// Seed centered/global and opposite-lane background so the test proves the
	// isolation applies after every layer has been mixed, not merely to the bump voice.
	m.globalLowCurrent = 0.30
	m.globalHighCurrent = 0.20
	m.right.current = 0.35
	m.right.target = 0.35
	m.right.profile = "asphalt"
	m.right.speed = 12

	m.update(telemetry{
		Version: 40, Active: true, BodyEvent: 2,
		BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1,
		BodyStrength: .30, BodyLeftStrength: .30, BodyRightStrength: 0, BodyDurationMS: 60,
		Raw: &rawTelemetry{GroundedWheels: 4, Speed: 12},
	}, now.Add(10*time.Millisecond))

	s, _ := m.render(180, now.Add(11*time.Millisecond)) // exactly 60 ms at 3 kHz
	leftEnergy, rightEnergy := 0, 0
	for i := 0; i+1 < len(s); i += 2 {
		leftEnergy += int(math.Abs(float64(s[i])))
		rightEnergy += int(math.Abs(float64(s[i+1])))
	}
	if leftEnergy == 0 {
		t.Fatal("directional bump produced no left-channel energy")
	}
	if rightEnergy != 0 {
		t.Fatalf("hard-panned LEFT bump leaked background into right channel: L=%d R=%d", leftEnergy, rightEnergy)
	}
}

func waveformMeanAbsDifference(profile string, durationA, durationB, window float64) float64 {
	const rate = 3000.0
	frames := int(math.Min(math.Min(durationA, durationB), window) * rate)
	if frames < 1 {
		return 0
	}
	seedA := uint32(0x12345678)
	seedB := uint32(0x12345678)
	sum := 0.0
	for i := 0; i < frames; i++ {
		tm := float64(i) / rate
		a := profileWave(profile, tm, durationA, &seedA)
		b := profileWave(profile, tm, durationB, &seedB)
		sum += math.Abs(a - b)
	}
	return sum / float64(frames)
}

func TestSafeImpactSignaturesStayDistinct(t *testing.T) {
	// These tests protect only renderer timbre. Timing, strength mapping and
	// spatial pan are intentionally untouched and covered by other regressions.
	const window = 0.050
	cases := []struct {
		profile string
		a, b    float64
		minDiff float64
	}{
		{"suspension_bump", 0.060, 0.110, 0.20},
		{"collision", 0.140, 0.220, 0.20},
		{"landing", 0.130, 0.175, 0.15},
	}
	for _, tc := range cases {
		d := waveformMeanAbsDifference(tc.profile, tc.a, tc.b, window)
		if d < tc.minDiff {
			t.Fatalf("%s severity signatures converged: diff=%.4f want>=%.4f", tc.profile, d, tc.minDiff)
		}
	}
}

func waveformRMS(profile string, duration, strength float64) float64 {
	const rate = 3000.0
	frames := int(duration * rate)
	if frames < 1 {
		return 0
	}
	seed := uint32(0xCAFEBABE)
	amp := math.Min(0.99, math.Max(0, strength*profileGain(profile)))
	sum := 0.0
	for i := 0; i < frames; i++ {
		v := profileWave(profile, float64(i)/rate, duration, &seed) * amp
		v = clamp(v, -1, 1)
		sum += v * v
	}
	return math.Sqrt(sum / float64(frames))
}

func TestSafeImpactSignaturesKeepSeverityEnergyOrdering(t *testing.T) {
	checks := []struct {
		profile                      string
		lightDuration, lightStrength float64
		heavyDuration, heavyStrength float64
	}{
		{"suspension_bump", 0.060, 0.45, 0.110, 0.95},
		{"collision", 0.140, 0.60, 0.220, 1.00},
		{"landing", 0.130, 0.35, 0.175, 0.55},
	}
	for _, c := range checks {
		light := waveformRMS(c.profile, c.lightDuration, c.lightStrength)
		heavy := waveformRMS(c.profile, c.heavyDuration, c.heavyStrength)
		if heavy <= light*1.10 {
			t.Fatalf("%s heavy impact lost energy ordering: light=%.4f heavy=%.4f", c.profile, light, heavy)
		}
	}
}

func TestOfficialStrengthUsesNormalized255Internally(t *testing.T) {
	for strength := 1; strength <= 8; strength++ {
		normalized := strength8To255(strength)
		if got := strength255To8(normalized); got != strength {
			t.Fatalf("%d/8 -> %d/255 -> %d/8", strength, normalized, got)
		}
	}
	if strength8To255(4) != 128 {
		t.Fatalf("4/8 must map to 128/255, got %d", strength8To255(4))
	}
	buf := make([]byte, 11)
	fillOfficialFeedback(buf, 0, strength8To255(1), strength8To255(4))
	if buf[0] != 0x21 {
		t.Fatalf("unexpected feedback mode %02x", buf[0])
	}
	packed := uint32(buf[3]) | uint32(buf[4])<<8 | uint32(buf[5])<<16 | uint32(buf[6])<<24
	zone0 := int(packed&7) + 1
	zone9 := int((packed>>(3*9))&7) + 1
	if zone0 != 1 || zone9 != 4 {
		t.Fatalf("normalized gradient quantized incorrectly: start=%d end=%d", zone0, zone9)
	}
}

func TestLegacyTelemetryConvertsOfficialTriggerStrengthTo255(t *testing.T) {
	packet := []byte(`{"v":40,"active":true,"l2Mode":1,"l2StartStrength":1,"l2EndStrength":4,"r2Mode":2,"r2Amplitude":2,"raw":{"engineRunning":true}}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("legacy telemetry packet was rejected")
	}
	if got.L2StartStrength != 32 || got.L2EndStrength != 128 || got.R2Amplitude != 64 {
		t.Fatalf("legacy trigger normalization mismatch: %+v", got)
	}
}

func TestUSBPCMQueueReportsOnlyRealQueuedFrames(t *testing.T) {
	var q sharedPCMQueue
	samples := make([]int8, 512*2)
	for i := range samples {
		samples[i] = 12
	}
	q.pushAtRate(samples, 48000)
	if got := q.availableOutputFrames(48000); got != 512 {
		t.Fatalf("queued output frames=%d want 512", got)
	}
	left, right := q.render(512, 48000)
	if len(left) != 512 || len(right) != 512 || q.availableOutputFrames(48000) != 0 {
		t.Fatalf("queue did not consume exactly the available PCM")
	}
}

func TestDecodeTelemetryAppliesBeamNGUserSettings(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()

	packet := []byte(`{"v":40,"active":true,"userSettings":{"masterPercent":85,"surfacePercent":70,"impactPercent":125,"l2BrakePercent":80,"absPercent":140,"r2ThrottlePercent":65,"r2EffectsPercent":110}}`)
	decoded, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("telemetry packet with BeamNG user settings was rejected")
	}
	if decoded.UserSettings == nil {
		t.Fatal("BeamNG user settings were not decoded")
	}

	got := currentUserSettings()
	if got.Haptics.MasterPercent != 85 || got.Haptics.SurfacePercent != 70 || got.Haptics.ImpactPercent != 125 ||
		got.AdaptiveTriggers.L2BrakePercent != 80 || got.AdaptiveTriggers.ABSPercent != 140 ||
		got.AdaptiveTriggers.R2ThrottlePercent != 65 || got.AdaptiveTriggers.R2EffectsPercent != 110 {
		t.Fatalf("BeamNG settings were not applied: %+v", got)
	}
}

func TestDecodeTelemetryAppliesModV12Schema12SettingsWithoutMutingFeedback(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()

	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.2","protocolMin":42,"protocolMax":42,"v":42,"active":true,"userSettings":{"schema":12,"triggerForceScale":48,"masterEnabled":true,"masterStrength":255,"surfaceEnabled":true,"surfaceStrength":255,"impactEnabled":true,"impactStrength":255,"l2BrakeEnabled":true,"l2BrakeStartStrength":12,"l2BrakeEndStrength":36,"l2BrakeStrength":36,"absEnabled":true,"absStrength":42,"r2ThrottleEnabled":true,"r2ThrottleStartStrength":6,"r2ThrottleEndStrength":12,"r2ThrottleStrength":6,"r2EffectsEnabled":true,"r2EffectsStrength":48,"lightingEnabled":true,"lightingBrightness":255},"l2Effect":{"kind":"resistance","startPosition":0,"startForce":0.25,"endForce":0.75},"r2Effect":{"kind":"resistance","startPosition":0,"startForce":0.125,"endForce":0.25},"raw":{"engineRunning":true,"groundedWheels":4}}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("V1.2 schema-12 telemetry was rejected")
	}
	settings := currentUserSettings()
	if settings.Haptics.MasterPercent != 100 || settings.Haptics.SurfacePercent != 100 || settings.Haptics.ImpactPercent != 100 {
		t.Fatalf("schema-12 haptic settings muted feedback: %+v", settings.Haptics)
	}
	if settings.AdaptiveTriggers.L2BrakePercent != 100 || settings.AdaptiveTriggers.R2ThrottlePercent != 100 || settings.AdaptiveTriggers.R2EffectsPercent != 100 {
		t.Fatalf("schema-12 trigger settings unexpectedly rescaled semantic effects: %+v", settings.AdaptiveTriggers)
	}
	if settings.AdaptiveTriggers.ABSPercent < 85 {
		t.Fatalf("schema-12 ABS strength collapsed: %d%%", settings.AdaptiveTriggers.ABSPercent)
	}
	if got.L2Mode != 1 || got.L2StartStrength < 60 || got.L2EndStrength < 185 {
		t.Fatalf("semantic L2 force lost after schema-12 decode: %+v", got)
	}
	if got.R2Mode != 1 || got.R2StartStrength < 30 || got.R2EndStrength < 60 {
		t.Fatalf("semantic R2 force lost after schema-12 decode: %+v", got)
	}
}

func TestSchema12SettingsDoNotMuteSuspensionBumpPCM(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	applyBeamNGUserSettings(telemetryUserSettings{
		Schema: 12, MasterEnabled: true, MasterStrength: 255,
		SurfaceEnabled: true, SurfaceStrength: 255,
		ImpactEnabled: true, ImpactStrength: 255,
		L2BrakeEnabled: true, L2BrakeStartStrength: 12, L2BrakeEndStrength: 36,
		ABSEnabled: true, ABSStrength: 42,
		R2ThrottleEnabled: true, R2ThrottleStartStrength: 6, R2ThrottleEndStrength: 12,
		R2EffectsEnabled: true, R2EffectsStrength: 48,
	})

	m := newHapticMixer()
	now := time.Unix(100, 0)
	base := telemetry{Version: 42, Active: true, Raw: &rawTelemetry{EngineRunning: true, GroundedWheels: 4}}
	m.update(base, now)
	event := base
	event.BodyEvent = 1
	event.BodyProfile = "suspension_bump"
	event.BodyKind = "wheel"
	event.BodySide = 1
	event.BodyRightStrength = 0.99
	event.BodyStrength = 0.99
	m.update(event, now.Add(time.Millisecond))
	pcm, status := m.render(512, now.Add(2*time.Millisecond))
	if status.nonSilent < 100 || status.blockPeak < 0.45 || status.blockRMS < 0.10 {
		t.Fatalf("schema-12 settings muted bump PCM: nz=%d rms=%.3f peak=%.3f", status.nonSilent, status.blockRMS, status.blockPeak)
	}
	if len(pcm) != 1024 {
		t.Fatalf("unexpected PCM size: %d", len(pcm))
	}
}

func TestDecodeTelemetryWithoutBeamNGSettingsKeepsCurrentPreferences(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	setUserSettingValue(0, 73)

	if _, ok := decodeTelemetry([]byte(`{"v":40,"active":true}`)); !ok {
		t.Fatal("legacy telemetry packet was rejected")
	}
	if got := currentUserSettings().Haptics.MasterPercent; got != 73 {
		t.Fatalf("legacy telemetry unexpectedly replaced local settings: got %d want 73", got)
	}
}

func TestDecodeTelemetryAcceptsModV10Protocol40(t *testing.T) {
	packet := []byte(`{"v":40,"active":true,"l2Mode":1,"l2StartZone":0,"l2StartStrength":2,"l2EndStrength":6,"r2Mode":3,"r2StartZone":0,"r2StartStrength":1,"r2EndStrength":1}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("mod V1.0 protocol 40 packet was rejected")
	}
	if got.L2StartStrength != strength8To255(2) || got.L2EndStrength != strength8To255(6) {
		t.Fatalf("V1.0 trigger conversion mismatch: %+v", got)
	}
	if got.R2Mode != 3 || got.R2StartStrength != 1 {
		t.Fatalf("V1.0 fine trigger semantics changed: %+v", got)
	}
}

func TestDecodeTelemetryAcceptsModV11Protocol41Semantic(t *testing.T) {
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.1","protocolMin":40,"protocolMax":41,"v":41,"active":true,"l2Effect":{"kind":"resistance","startPosition":0,"startForce":0.25,"endForce":0.75},"r2Effect":{"kind":"fine","startPosition":0.5,"startForce":0.0208333333},"raw":{"engineRunning":true}}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("mod V1.1 protocol 41 packet was rejected")
	}
	if got.L2Mode != 1 || got.L2StartZone != 0 || got.L2StartStrength != 64 || got.L2EndStrength != 191 {
		t.Fatalf("V1.1 semantic resistance conversion mismatch: %+v", got)
	}
	if got.R2Mode != 3 || got.R2StartZone != 128 || got.R2StartStrength != 1 || got.R2EndStrength != 1 {
		t.Fatalf("V1.1 semantic fine conversion mismatch: %+v", got)
	}
}

func TestDecodeTelemetryAcceptsModV12OnProtocol42(t *testing.T) {
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.2","protocolMin":42,"protocolMax":42,"v":42,"active":true,"l2Effect":{"kind":"resistance","startPosition":0,"startForce":0.25,"endForce":0.75},"r2Effect":{"kind":"resistance","startPosition":0,"startForce":0.125,"endForce":0.25},"raw":{"engineRunning":true}}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("mod V1.2 protocol 42 packet was rejected")
	}
	if got.L2Mode != 1 || got.L2StartStrength != 64 || got.L2EndStrength != 191 {
		t.Fatalf("V1.2 L2 semantic conversion mismatch: %+v", got)
	}
	if got.R2Mode != 1 || got.R2StartStrength != 32 || got.R2EndStrength != 64 {
		t.Fatalf("V1.2 R2 semantic conversion mismatch: %+v", got)
	}
	if summary := telemetryConnectionSummary(got); summary != "BeamNG.drive connected - mod V1.2 / Bridge V1.41." {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestMarkedV40MigrationMirrorIsIgnored(t *testing.T) {
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.2","protocolMin":40,"protocolMax":41,"legacyCompat":true,"v":40,"active":true,"bodyEvent":99,"l2Mode":1,"l2StartStrength":8,"l2EndStrength":8}`)
	if _, ok := decodeTelemetry(packet); ok {
		t.Fatal("marked v40 migration mirror must be ignored by Bridge V1.4")
	}
	if msg := telemetryCompatibilityError(packet); msg != "" {
		t.Fatalf("migration mirror must not request update: %q", msg)
	}
}

func TestProtocol42BodyEventProducesStrongStereoPCM(t *testing.T) {
	initialJSON := []byte(`{"protocolId":"DPH","modVersion":"V1.2","protocolMin":42,"protocolMax":42,"v":42,"active":true,"seq":1,"l2Effect":{"kind":"resistance","startPosition":0,"startForce":0.25,"endForce":0.75},"r2Effect":{"kind":"resistance","startPosition":0,"startForce":0.125,"endForce":0.25},"bodyEvent":0,"raw":{"speed":8,"groundedWheels":4,"surfaceMaterialL":0,"surfaceMaterialR":0}}`)
	bumpJSON := []byte(`{"protocolId":"DPH","modVersion":"V1.2","protocolMin":42,"protocolMax":42,"v":42,"active":true,"seq":2,"l2Effect":{"kind":"resistance","startPosition":0,"startForce":0.25,"endForce":0.75},"r2Effect":{"kind":"resistance","startPosition":0,"startForce":0.125,"endForce":0.25},"bodyEvent":1,"bodyStrength":1,"bodyLeftStrength":1,"bodyRightStrength":0,"bodyDurationMS":116,"bodyKind":"bump","bodySide":-1,"bodyProfile":"suspension_bump","bodySourceReason":"test","bodySourceConfidence":1,"bodySourceLeftScore":1,"bodySourceLeftContact":1,"bodySourceLeftPeak":1,"bodySourceLeftEnergy":1,"bodySourceLeftWheel":1,"bodySourceLeftAxle":1,"raw":{"speed":8,"groundedWheels":4,"surfaceMaterialL":0,"surfaceMaterialR":0}}`)
	initial, ok := decodeTelemetry(initialJSON)
	if !ok {
		t.Fatal("initial protocol42 packet rejected")
	}
	bump, ok := decodeTelemetry(bumpJSON)
	if !ok {
		t.Fatal("bump protocol42 packet rejected")
	}
	m := newHapticMixer()
	now := time.Now()
	m.update(initial, now)
	m.update(bump, now.Add(time.Millisecond))
	samples, status := m.render(512, now.Add(2*time.Millisecond))
	peak := 0.0
	for _, sample := range samples {
		v := math.Abs(float64(sample)) / 127.0
		if v > peak {
			peak = v
		}
	}
	if peak < 0.5 || status.blockRMS < 0.1 || status.nonSilent == 0 {
		t.Fatalf("protocol42 bump produced weak/no PCM peak=%.3f rms=%.3f nz=%d", peak, status.blockRMS, status.nonSilent)
	}
}

func TestProtocol43AcceptsUnifiedV15Release(t *testing.T) {
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.41","protocolMin":43,"protocolMax":43,"v":43,"active":true}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("V1.41 wire generation 43 packet was rejected")
	}
	if summary := telemetryConnectionSummary(got); summary != "BeamNG.drive connected - release V1.41 synchronized." {
		t.Fatalf("unexpected V1.41 summary: %q", summary)
	}
	if msg := telemetryCompatibilityError(packet); msg != "" {
		t.Fatalf("V1.41 wire generation 43 unexpectedly requested update: %q", msg)
	}
}

func TestDecodeTelemetryRejectsUnsupportedGameplayProtocols(t *testing.T) {
	for _, version := range []int{39, 44, 45} {
		packet := []byte(fmt.Sprintf(`{"protocolId":"DPH","v":%d,"active":true}`, version))
		if _, ok := decodeTelemetry(packet); ok {
			t.Fatalf("unsupported protocol %d was accepted", version)
		}
	}
}
