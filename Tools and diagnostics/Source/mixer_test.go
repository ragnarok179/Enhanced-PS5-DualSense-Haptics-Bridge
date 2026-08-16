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

func TestReport32StructureAndCRC(t *testing.T) {
	s := make([]int8, 64)
	s[0], s[1] = 100, -50
	r := buildBluetoothHapticReport32(3, 8, s, 547)
	if len(r) != 142 || r[0] != 0x32 || r[1] != 0x00 || r[2] != 0x91 || r[3] != 7 || r[4] != 0xFE || r[10] != 8 || r[11] != 0x92 || r[12] != 64 {
		t.Fatalf("bad 0x32 header %v", r[:16])
	}
	if int8(r[13]) != 100 || int8(r[14]) != -50 {
		t.Fatal("0x32 stereo payload lost")
	}
	got := binary.LittleEndian.Uint32(r[138:142])
	want := sonyBluetoothCRC(r, bluetoothHaptic32ProtoSize)
	if got != want || got == 0 {
		t.Fatalf("0x32 crc got=%08x want=%08x", got, want)
	}
}

func TestReport39StructureAndCRC(t *testing.T) {
	s := make([]int8, 128)
	s[0], s[1] = 100, -50
	r := buildBluetoothHapticReport39(3, 8, s, 547)
	if len(r) != 547 || r[0] != 0x39 || r[1] != 0x30 || r[2] != 0x91 || r[3] != 6 || r[4] != 0x7E || r[5] != 64 || r[6] != 64 || r[7] != 64 || r[8] != 64 || r[9] != 8 || r[10] != 0xD2 || r[11] != 64 {
		t.Fatalf("bad 0x39 header %v", r[:16])
	}
	if int8(r[12]) != 100 || int8(r[13]) != -50 {
		t.Fatal("0x39 stereo payload lost")
	}
	got := binary.LittleEndian.Uint32(r[543:547])
	want := sonyBluetoothCRC(r, 547)
	if got != want || got == 0 {
		t.Fatalf("0x39 crc got=%08x want=%08x", got, want)
	}
}

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

func TestControlReportTriggersAndCRC(t *testing.T) {
	input := telemetry{
		Version: 40, Active: true,
		L2Mode: 1, L2StartZone: 0, L2StartStrength: 2, L2EndStrength: 6,
		R2Mode: 3, R2StartZone: 0, R2StartStrength: 1,
	}
	r := buildBluetoothTriggerControlReport(5, input, 547)
	if len(r) != 78 || r[0] != 0x31 || r[1] != 0x50 || r[2] != 0x10 {
		t.Fatalf("bad control header %v", r[:8])
	}
	common := r[3:50]
	if common[0] != 0x0C || common[1] != 0 || common[38] != 0 {
		t.Fatalf("bad trigger-only valid flags %02x %02x %02x", common[0], common[1], common[38])
	}
	if common[10] != 0x01 || common[21] != 0x21 {
		t.Fatalf("trigger modes R2=%02x L2=%02x", common[10], common[21])
	}
	if common[44] != 0 || common[45] != 0 || common[46] != 0 {
		t.Fatalf("trigger-only report carries RGB: %02x%02x%02x", common[44], common[45], common[46])
	}
	got := binary.LittleEndian.Uint32(r[74:78])
	want := sonyBluetoothCRC(r, 78)
	if got != want || got == 0 {
		t.Fatalf("control crc got=%08x want=%08x", got, want)
	}
}

func TestExactGamepadCoreTransportLengthsAndState(t *testing.T) {
	c := buildBluetoothTriggerControlReport(0, telemetry{}, 547)
	h := buildBluetoothHapticReport32(9, 1, make([]int8, 64), 547)
	c2 := buildBluetoothTriggerControlReport(1, telemetry{}, 547)
	if len(c) != 78 || len(h) != 142 {
		t.Fatalf("wrong protocol lengths control=%d haptic=%d", len(c), len(h))
	}
	if c[1] != 0x00 || c2[1] != 0x10 || c[2] != 0x10 || c2[2] != 0x10 || h[1] != 0x00 {
		t.Fatalf("wrong standard BT header: %02x/%02x %02x/%02x h=%02x", c[1], c[2], c2[1], c2[2], h[1])
	}
	if c[3+38] != 0 || c2[3+38] != 0 {
		t.Fatalf("undocumented valid_flag2 bits must remain clear: %02x %02x", c[41], c2[41])
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

func TestTriggerControlDoesNotTouchUnrelatedGroups(t *testing.T) {
	state := telemetry{Version: 40, Active: true, L2Mode: 1, L2StartZone: 1, L2StartStrength: 2, L2EndStrength: 4, R2Mode: 1, R2StartZone: 1, R2StartStrength: 2, R2EndStrength: 4}
	report := buildBluetoothTriggerControlReport(0, state, 547)
	common := report[3:50]
	if common[0] != 0x0C || common[1] != 0 || common[38] != 0 {
		t.Fatalf("trigger mask leaked into unrelated groups: %02x %02x %02x", common[0], common[1], common[38])
	}
	if common[44] != 0 || common[45] != 0 || common[46] != 0 {
		t.Fatal("trigger-only packet rewrote the lightbar")
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
	state.Raw.RPM = 0.499 * state.Raw.MaxRPM
	if got := c.update(state, base); got != ([3]byte{}) {
		t.Fatalf("USB LED should be off below 50%%: %v", got)
	}
	state.Raw.RPM = 0.50 * state.Raw.MaxRPM
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

func TestRuntimeControlNeverSelectsCompatibleRumbleAudioOrLED(t *testing.T) {
	state := telemetry{Version: protocolVersion, Active: true, L2Effect: wireEffect(resistanceTrigger(0, 0.25, 0.75)), R2Effect: wireEffect(resistanceTrigger(0, 0.125, 0.25))}
	report := buildBluetoothTriggerControlReport(0, state, 547)
	if report[3] != 0x0C || report[4] != 0x00 {
		t.Fatalf("trigger mask=%02x/%02x want 0c/00", report[3], report[4])
	}
	if report[5] != 0 || report[6] != 0 {
		t.Fatalf("compatible rumble bytes changed: %02x %02x", report[5], report[6])
	}
	if report[47] != 0 || report[48] != 0 || report[49] != 0 {
		t.Fatalf("RGB bytes changed: %02x%02x%02x", report[47], report[48], report[49])
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
	cmd := telemetry{Active: true, L2Mode: 2, L2Amplitude: 5, Raw: &rawTelemetry{ABS: true, ABSSeverity: .8, ABSWheelCount: 1, ABSControlHz: 100, Brake: .75}}
	got, active, phase, _, _, _, _ := applyABSHybridPulse(cmd, base, &state)
	if !active || phase != "release_off" || got.L2Mode != 0 || got.L2StartStrength != 0 || got.L2EndStrength != 0 {
		t.Fatalf("ABS release mismatch active=%t phase=%s state=%+v", active, phase, got)
	}
	got, active, phase, _, _, _, _ = applyABSHybridPulse(cmd, base.Add(8*time.Millisecond), &state)
	if !active || phase != "kick" || got.L2Mode != 1 || got.L2StartStrength != 36 || got.L2EndStrength != 36 {
		t.Fatalf("ABS kick mismatch %+v %s", got, phase)
	}
	got, active, phase, _, _, _, _ = applyABSHybridPulse(cmd, base.Add(25*time.Millisecond), &state)
	if !active || phase != "base" || got.L2StartStrength != 6 || got.L2EndStrength != 6 {
		t.Fatalf("ABS base mismatch %+v %s", got, phase)
	}
}

func TestABSUserStrengthAndDisable(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()

	setUserSettingValue(5, 28)
	var state absPulseState
	baseTime := time.Unix(6500, 0)
	cmd := telemetry{Active: true, L2Mode: 2, Raw: &rawTelemetry{ABS: true, ABSSeverity: .8, ABSWheelCount: 1, ABSControlHz: 100, Brake: .7}}
	_, _, _, _, kick, _, _ := applyABSHybridPulse(cmd, baseTime.Add(8*time.Millisecond), &state)
	if kick != 28 {
		t.Fatalf("custom ABS strength=%d want 28/48", kick)
	}

	setUserSettingEnabled(5, false)
	got, active, phase, _, _, _, _ := applyABSHybridPulse(cmd, baseTime.Add(16*time.Millisecond), &state)
	if active || phase != "disabled" {
		t.Fatalf("disabled ABS remained active: active=%t phase=%s state=%+v", active, phase, got)
	}
}

func TestNormalL2UsesRawUserStrengthAndCanDisable(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()

	setUserSettingValue(4, 14)
	settings := currentUserSettings()
	got := applyNormalL2Settings(telemetry{Active: true, L2Mode: 1, Raw: &rawTelemetry{Brake: .5}}, settings)
	if got.L2Mode != 1 || got.L2EndStrength != 14 || got.L2StartStrength < 1 || got.L2StartStrength > 14 {
		t.Fatalf("raw L2 setting not applied: %+v", got)
	}

	setUserSettingEnabled(3, false)
	got = applyNormalL2Settings(got, currentUserSettings())
	if got.L2Mode != 0 || got.L2StartStrength != 0 || got.L2EndStrength != 0 {
		t.Fatalf("disabled L2 still active: %+v", got)
	}
}

func TestNormalTriggerStartEndRanges(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	s := currentUserSettings()
	s.AdaptiveTriggers.L2BrakeStartStrength = 6
	s.AdaptiveTriggers.L2BrakeEndStrength = 24
	s.AdaptiveTriggers.R2ThrottleStartStrength = 6
	s.AdaptiveTriggers.R2ThrottleEndStrength = 30
	userSettingsState.mu.Lock()
	userSettingsState.data = s
	userSettingsState.mu.Unlock()

	l2 := applyNormalL2Settings(telemetry{Active: true, Raw: &rawTelemetry{Brake: .7}}, currentUserSettings())
	if l2.L2StartStrength != 6 || l2.L2EndStrength != 24 {
		t.Fatalf("L2 range mismatch: %+v", l2)
	}
	r2 := applyNormalR2Settings(telemetry{Active: true, Raw: &rawTelemetry{Throttle: .7}}, currentUserSettings())
	if r2.R2StartStrength != 6 || r2.R2EndStrength != 30 {
		t.Fatalf("R2 range mismatch: %+v", r2)
	}

	buf := make([]byte, 11)
	fillTrigger(buf, 1, 0, r2.R2StartStrength, r2.R2EndStrength, 0, 0)
	if buf[0] != 0x21 {
		t.Fatalf("progressive R2 did not build official feedback report: %v", buf)
	}
}

func TestSharedR2BaselineAndSlip(t *testing.T) {
	var state adaptiveThrottleTriggerState
	now := time.Unix(7000, 0)
	cmd := telemetry{Active: true, R2Mode: 1, R2StartZone: 0, R2StartStrength: 32, R2EndStrength: 64, Raw: &rawTelemetry{EngineRunning: true, DrivenSlip: 0}}
	got, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, now, &state, false)
	if active || got.R2Mode != 1 || got.R2StartStrength != 6 || got.R2EndStrength != 6 {
		t.Fatalf("R2 baseline mismatch %+v", got)
	}
	cmd.Raw.Wheelspin = true
	cmd.Raw.DrivenSlip = 20
	got, active, _, _, _ = applyAdaptiveThrottleTrigger(cmd, now.Add(16*time.Millisecond), &state, false)
	if !active || got.R2Mode != 3 || got.R2StartZone < 1 || got.R2StartStrength < 1 || got.R2StartStrength > 3 {
		t.Fatalf("R2 slip mismatch %+v", got)
	}
}

func TestSharedR2AirborneIsExactOneOver48(t *testing.T) {
	var state adaptiveThrottleTriggerState
	now := time.Unix(7050, 0)
	cmd := telemetry{Active: true, Raw: &rawTelemetry{EngineRunning: true, Airborne: true, GroundedWheels: 0, Wheelspin: true, DrivenSlip: 20, TCS: true, RevLimiter: true}}
	got, active, _, _, _ := applyAdaptiveThrottleTrigger(cmd, now, &state, true)
	if !active || got.R2Mode != 3 || got.R2StartZone != 0 || got.R2StartStrength != 1 || got.R2EndStrength != 1 || got.R2Amplitude != 0 || got.R2Hz != 0 {
		t.Fatalf("airborne R2 must be exact 1/48: %+v", got)
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
	state := telemetry{Version: 40, Active: true, L2Mode: 1, L2StartZone: 3, L2StartStrength: 5, L2EndStrength: 6, R2Mode: 2, R2StartZone: 4, R2Amplitude: 4, R2Hz: 30}
	setState := buildBluetoothSetStateData63(state)
	if len(setState) != 63 {
		t.Fatalf("state len=%d", len(setState))
	}
	if setState[0] != 0xFD || setState[1] != 0x00 {
		t.Fatalf("state flags=%02x/%02x", setState[0], setState[1])
	}
	if setState[6] != 0x40 || setState[7] != 0x09 || setState[9] != 0x00 || setState[38] != 0x00 {
		t.Fatalf("persistent state mismatch mic=%02x audio=%02x powerSave=%02x valid3=%02x", setState[6], setState[7], setState[9], setState[38])
	}
	if setState[41] != 0x00 || setState[44] != 0 || setState[45] != 0 || setState[46] != 0 {
		t.Fatalf("runtime 0x36 state owns LED fields: setup=%02x rgb=%02x%02x%02x", setState[41], setState[44], setState[45], setState[46])
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
	if r[13] != 0xFD || r[14] != 0x00 {
		t.Fatalf("SetStateData offset is wrong: %x", r[13:16])
	}
	if r[57] != 0 || r[58] != 0 || r[59] != 0 {
		t.Fatalf("combined packet carries RGB: %02x%02x%02x", r[57], r[58], r[59])
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

func TestAllInOneCarriesTriggersAndHapticsWithoutLED(t *testing.T) {
	state := telemetry{Version: 40, Active: true, L2Mode: 1, L2StartZone: 2, L2StartStrength: 4, L2EndStrength: 6, R2Mode: 1, R2StartZone: 2, R2StartStrength: 3, R2EndStrength: 5}
	ss := buildBluetoothSetStateData63(state)
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
	if r[57] != 0 || r[58] != 0 || r[59] != 0 {
		t.Fatalf("combined report must not own LED state")
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

func TestSharedFeelShiftR2ExactOneOver48(t *testing.T) {
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
	if p.ProfileVersion == "" || math.Abs(p.Triggers.ABS.KickForce-36.0/48.0) > 1e-9 || math.Abs(p.Triggers.L2.NormalEndForce-24.0/48.0) > 1e-9 || p.Triggers.R2.ShiftDurationMS != 150 || p.ShiftHaptic.Priority != 25 {
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
		t.Fatalf("shift cut did not start with exact 1/48: %+v", frame.Control)
	}
	// Raw.Shifting stays true deliberately: it must not extend the 150 ms event window.
	state.Raw.Shifting = true
	frame = e.step(state, base.Add(170*time.Millisecond), base.Add(170*time.Millisecond), canonicalFramesForBluetoothFrames(32))
	if frame.Control.R2Mode != 1 || frame.Control.R2StartStrength != 6 || frame.Control.R2EndStrength != 6 {
		t.Fatalf("shift cut was extended beyond 150 ms by Raw.Shifting: %+v", frame.Control)
	}
}

func TestSharedFeelLEDNoFlickerAroundFirstThreshold(t *testing.T) {
	c := &lightbarController{}
	base := time.Unix(13000, 0)
	state := telemetry{Version: 40, Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{EngineRunning: true, MaxRPM: 7000}}
	state.Raw.RPM = 3600 // 51.4%, activate
	first := c.update(state, base)
	if first == ([3]byte{}) || c.status() != "progress" {
		t.Fatalf("LED did not enter progress phase: %v %s", first, c.status())
	}
	// Oscillate around the 50% threshold for one second. It must never go black.
	for i := 1; i <= 40; i++ {
		if i%2 == 0 {
			state.Raw.RPM = 3465
		} else {
			state.Raw.RPM = 3535
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
	state := telemetry{Version: 40, Active: true, ShiftLEDsInUse: true, Raw: &rawTelemetry{EngineRunning: true, MaxRPM: 7000, RPM: 3600}}
	_ = c.update(state, base)
	state.Raw.RPM = 2800 // 40%, below 45% off hysteresis
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

func TestBluetoothRuntimeNeverOwnsLightbar(t *testing.T) {
	state := buildBluetoothSetStateData63(telemetry{Active: true})
	if state[1] != 0 || state[38] != 0 || state[41] != 0 {
		t.Fatalf("runtime state validates LED-adjacent fields: valid1=%02x valid2=%02x setup=%02x", state[1], state[38], state[41])
	}
	if state[44] != 0 || state[45] != 0 || state[46] != 0 {
		t.Fatalf("runtime state carries RGB: %02x%02x%02x", state[44], state[45], state[46])
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
	r := buildBluetoothAllInOneReport36(1, 2, samples, buildBluetoothSetStateData63(telemetry{}))
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
		state := buildBluetoothSetStateData63(telemetry{Active: false})
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

func TestBluetoothSetStateDoesNotRequestHapticPowerSave(t *testing.T) {
	state := buildBluetoothSetStateData63(telemetry{Version: protocolVersion, Active: true})
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

func TestBluetoothRuntimeStateOwnsNoSecondaryLEDFlags(t *testing.T) {
	state := buildBluetoothSetStateData63(telemetry{Active: true})
	if len(state) != 63 {
		t.Fatalf("runtime state len = %d", len(state))
	}
	if state[1] != 0 {
		t.Fatalf("runtime valid_flag1 must be zero, got 0x%02x", state[1])
	}
	if state[44] != 0 || state[45] != 0 || state[46] != 0 {
		t.Fatalf("runtime state must not carry stale RGB: %v", state[44:47])
	}
}

func TestUSBTriggerStateOwnsNoLEDFields(t *testing.T) {
	common := make([]byte, 47)
	fillUSBTriggerStateCommon(common, telemetry{Version: protocolVersion, Active: true})
	if common[0] != 0x0C || common[1] != 0 {
		t.Fatalf("USB trigger flags=%02X/%02X want 0C/00", common[0], common[1])
	}
	if common[44] != 0 || common[45] != 0 || common[46] != 0 {
		t.Fatalf("USB trigger state carries RGB: %02X%02X%02X", common[44], common[45], common[46])
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

func TestCanonicalForce48MapsToFinalHIDModes(t *testing.T) {
	// Fine Feedback consumes the canonical force level directly.
	fine := make([]byte, 11)
	fillFineFeedbackEffect(fine, fineTrigger(0.25, force48FromLevel(1).Float64()))
	if fine[0] != 0x01 || fine[2] != 1 {
		t.Fatalf("fine force48 level 1 did not reach HID unchanged: %v", fine[:3])
	}
	fillFineFeedbackEffect(fine, fineTrigger(0.25, force48FromLevel(48).Float64()))
	if fine[2] != 48 {
		t.Fatalf("fine force48 max did not reach HID unchanged: %d", fine[2])
	}

	// Official Feedback has only eight physical force levels. The reduction from
	// the common 48-level force occurs here, at the HID boundary, and nowhere in
	// gameplay/settings code.
	checks := []struct {
		force48 int
		want8   int
	}{{1, 1}, {6, 1}, {12, 2}, {24, 4}, {36, 6}, {48, 8}}
	for _, tc := range checks {
		if got := force48FromLevel(tc.force48).officialStep(); got != tc.want8 {
			t.Fatalf("force48=%d -> official=%d want %d", tc.force48, got, tc.want8)
		}
	}

	buf := make([]byte, 11)
	fillOfficialResistance(buf, resistanceTrigger(0, force48FromLevel(6).Float64(), force48FromLevel(24).Float64()))
	if buf[0] != 0x21 {
		t.Fatalf("unexpected feedback mode %02x", buf[0])
	}
	packed := uint32(buf[3]) | uint32(buf[4])<<8 | uint32(buf[5])<<16 | uint32(buf[6])<<24
	zone0 := int(packed&7) + 1
	zone9 := int((packed>>(3*9))&7) + 1
	if zone0 != 1 || zone9 != 4 {
		t.Fatalf("force48 gradient quantized incorrectly: start=%d end=%d", zone0, zone9)
	}
}

func TestProtocol41NormalizedTriggersQuantizeOnceToForce48(t *testing.T) {
	packet := []byte(`{"v":41,"active":true,"l2Effect":{"kind":"resistance","startPosition":0.1,"startForce":0.13,"endForce":0.5,"amplitude":0,"frequencyHz":0},"r2Effect":{"kind":"fine","startPosition":0.2,"startForce":0.020833333333333332,"endForce":0.020833333333333332,"amplitude":0,"frequencyHz":0}}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("protocol 41 normalized trigger packet was rejected")
	}
	pair := triggerPairFromTelemetry(got)
	if pair.L2.StartForce.Level() != 6 || pair.L2.EndForce.Level() != 24 {
		t.Fatalf("v41 L2 force48 mismatch: %+v", pair.L2)
	}
	if pair.R2.StartForce.Level() != 1 {
		t.Fatalf("v41 fine force should quantize to 1/48, got %d/48", pair.R2.StartForce.Level())
	}
}

func TestProtocol41NeverFallsBackToLegacyTriggerIntegers(t *testing.T) {
	packet := []byte(`{"v":41,"active":true,"l2Mode":1,"l2StartStrength":8,"l2EndStrength":8,"r2Mode":3,"r2StartStrength":48}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("protocol 41 packet was rejected")
	}
	pair := triggerPairFromTelemetry(got)
	if pair.L2.Kind != triggerOff || pair.R2.Kind != triggerOff {
		t.Fatalf("v41 leaked historical trigger fields into runtime: %+v", pair)
	}
}

func TestDecodeTelemetrySchema11UsesForce48Directly(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	packet := []byte(`{"v":41,"active":true,"userSettings":{"schema":11,"triggerForceScale":48,"masterEnabled":true,"masterStrength":255,"surfaceEnabled":true,"surfaceStrength":255,"impactEnabled":true,"impactStrength":255,"l2BrakeEnabled":true,"l2BrakeStartStrength":6,"l2BrakeEndStrength":24,"absEnabled":true,"absStrength":36,"r2ThrottleEnabled":true,"r2ThrottleStartStrength":6,"r2ThrottleEndStrength":6,"r2EffectsEnabled":true,"r2EffectsStrength":12,"lightingEnabled":true,"lightingBrightness":219}}`)
	if _, ok := decodeTelemetry(packet); !ok {
		t.Fatal("schema11 protocol41 telemetry rejected")
	}
	s := currentUserSettings()
	if s.AdaptiveTriggers.L2BrakeStartStrength != 6 || s.AdaptiveTriggers.L2BrakeEndStrength != 24 ||
		s.AdaptiveTriggers.ABSStrength != 36 || s.AdaptiveTriggers.R2ThrottleStartStrength != 6 ||
		s.AdaptiveTriggers.R2ThrottleEndStrength != 6 || s.AdaptiveTriggers.R2EffectsStrength != 12 {
		t.Fatalf("schema11 force48 settings changed in transit: %+v", s.AdaptiveTriggers)
	}
}

func TestLegacyTelemetryNormalizesAtTriggerAdapter(t *testing.T) {
	packet := []byte(`{"v":40,"active":true,"l2Mode":1,"l2StartStrength":1,"l2EndStrength":4,"r2Mode":2,"r2Amplitude":2,"raw":{"engineRunning":true}}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("legacy telemetry packet was rejected")
	}
	pair := triggerPairFromTelemetry(got)
	if math.Abs(pair.L2.StartForce.Float64()-1.0/8.0) > 1e-9 ||
		math.Abs(pair.L2.EndForce.Float64()-4.0/8.0) > 1e-9 ||
		math.Abs(pair.R2.Amplitude.Float64()-2.0/8.0) > 1e-9 {
		t.Fatalf("legacy trigger adapter mismatch: %+v", pair)
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

func TestDecodeTelemetrySchema10TriggerRanges(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	packet := []byte(`{"v":40,"active":true,"userSettings":{"schema":10,"masterEnabled":true,"masterStrength":255,"surfaceEnabled":true,"surfaceStrength":255,"impactEnabled":true,"impactStrength":255,"l2BrakeEnabled":true,"l2BrakeStartStrength":40,"l2BrakeEndStrength":150,"absEnabled":true,"absStrength":192,"r2ThrottleEnabled":true,"r2ThrottleStartStrength":30,"r2ThrottleEndStrength":120,"r2EffectsEnabled":true,"r2EffectsStrength":64,"lightingEnabled":true,"lightingBrightness":219}}`)
	if _, ok := decodeTelemetry(packet); !ok {
		t.Fatal("schema10 telemetry rejected")
	}
	s := currentUserSettings()
	if s.AdaptiveTriggers.L2BrakeStartStrength != 8 || s.AdaptiveTriggers.L2BrakeEndStrength != 28 {
		t.Fatalf("schema10 L2 range lost: %+v", s.AdaptiveTriggers)
	}
	if s.AdaptiveTriggers.R2ThrottleStartStrength != 6 || s.AdaptiveTriggers.R2ThrottleEndStrength != 23 {
		t.Fatalf("schema10 R2 range lost: %+v", s.AdaptiveTriggers)
	}
}

func TestDecodeTelemetryAppliesBeamNGUserSettings(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()

	packet := []byte(`{"v":40,"active":true,"userSettings":{"schema":8,"masterEnabled":true,"masterStrength":255,"surfaceEnabled":false,"surfaceStrength":255,"impactEnabled":true,"impactStrength":123,"surfaceRollingStrengths":{"asphalt":74,"gravel":255},"surfaceSlipStrengths":{"asphalt":255,"gravel":255},"l2BrakeEnabled":true,"l2BrakeStrength":96,"absEnabled":true,"absStrength":211,"r2ThrottleEnabled":false,"r2ThrottleStrength":27,"r2EffectsEnabled":true,"r2EffectsStrength":64,"lightingEnabled":false,"lightingBrightness":219}}`)
	decoded, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("telemetry packet with BeamNG user settings was rejected")
	}
	if decoded.UserSettings == nil {
		t.Fatal("BeamNG user settings were not decoded")
	}

	got := currentUserSettings()
	if !got.Haptics.MasterEnabled || got.Haptics.MasterStrength != 255 ||
		got.Haptics.SurfaceEnabled || got.Haptics.SurfaceStrength != 255 ||
		!got.Haptics.ImpactEnabled || got.Haptics.ImpactStrength != 123 ||
		math.Abs(surfaceRollingGain(got, "asphalt")-1.0) > 0.05 || math.Abs(surfaceRollingGain(got, "gravel")-1.0) > 0.05 ||
		math.Abs(surfaceSlipGain(got, "asphalt")-1.0) > 0.20 || math.Abs(surfaceSlipGain(got, "gravel")-1.0) > 0.40 ||
		!got.AdaptiveTriggers.L2BrakeEnabled || got.AdaptiveTriggers.L2BrakeEndStrength != 18 || got.AdaptiveTriggers.L2BrakeStartStrength != 5 ||
		!got.AdaptiveTriggers.ABSEnabled || got.AdaptiveTriggers.ABSStrength != 40 ||
		got.AdaptiveTriggers.R2ThrottleEnabled || got.AdaptiveTriggers.R2ThrottleStartStrength != 5 || got.AdaptiveTriggers.R2ThrottleEndStrength != 5 ||
		!got.AdaptiveTriggers.R2EffectsEnabled || got.AdaptiveTriggers.R2EffectsStrength != 12 ||
		got.Lighting.Enabled || got.Lighting.Brightness != 219 {
		t.Fatalf("BeamNG schema8 settings were not applied: %+v", got)
	}
}

func TestDecodeTelemetryLegacyPercentSettingsStillMigrate(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	packet := []byte(`{"v":40,"active":true,"userSettings":{"masterPercent":85,"surfacePercent":0,"impactPercent":125,"l2BrakePercent":80,"absPercent":140,"r2ThrottlePercent":65,"r2EffectsPercent":110}}`)
	if _, ok := decodeTelemetry(packet); !ok {
		t.Fatal("legacy percentage settings packet was rejected")
	}
	got := currentUserSettings()
	if !got.Haptics.MasterEnabled || hapticMasterGain(true, got.Haptics.MasterStrength) < 0.83 || hapticMasterGain(true, got.Haptics.MasterStrength) > 0.87 {
		t.Fatalf("legacy master migration mismatch: %+v gain=%f", got.Haptics, hapticMasterGain(true, got.Haptics.MasterStrength))
	}
	if got.Haptics.SurfaceEnabled {
		t.Fatal("legacy 0% surface setting must migrate to disabled")
	}
	if got.AdaptiveTriggers.L2BrakeEndStrength != 19 || got.AdaptiveTriggers.L2BrakeStartStrength != 5 || got.AdaptiveTriggers.ABSStrength != 48 {
		t.Fatalf("legacy trigger migration mismatch: %+v", got.AdaptiveTriggers)
	}
}

func TestDecodeTelemetryWithoutBeamNGSettingsKeepsCurrentPreferences(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	setUserSettingValue(0, 73)
	setUserSettingEnabled(1, false)
	if _, ok := decodeTelemetry([]byte(`{"v":40,"active":true}`)); !ok {
		t.Fatal("legacy telemetry packet was rejected")
	}
	got := currentUserSettings()
	if got.Haptics.MasterStrength != 73 || got.Haptics.SurfaceEnabled {
		t.Fatalf("legacy telemetry unexpectedly replaced local settings: %+v", got.Haptics)
	}
}

func TestHapticMasterIsTrueAttenuationScale(t *testing.T) {
	ref := percentToStrength255(hapticReferencePercent)
	if got := hapticMasterGain(true, ref); math.Abs(got-1.0) > 0.0001 {
		t.Fatalf("calibrated %g%% gain=%f want 1", hapticReferencePercent, got)
	}
	if got := hapticMasterGain(true, percentToStrength255(50)); got < 0.49 || got > 0.51 {
		t.Fatalf("50%% gain=%f want 0.5", got)
	}
	if got := hapticMasterGain(false, 255); got != 0 {
		t.Fatalf("disabled gain=%f want 0", got)
	}
}

func TestR2DynamicEffectsDisabledSettlesToBaseResistance(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	userSettingsState.mu.Lock()
	settings := userSettingsState.data
	settings.AdaptiveTriggers.R2EffectsEnabled = false
	settings.AdaptiveTriggers.R2ThrottleEnabled = true
	settings.AdaptiveTriggers.R2ThrottleStrength = 6
	userSettingsState.data = settings
	userSettingsState.mu.Unlock()

	cmd := telemetry{Active: true, R2Mode: 2, R2Amplitude: 8, R2Hz: 24, Raw: &rawTelemetry{EngineRunning: true, RevLimiter: true, Throttle: 1}}
	got, _, _, _, _ := applyAdaptiveThrottleTrigger(cmd, time.Now(), &adaptiveThrottleTriggerState{}, false)
	got = applyUserTriggerPreferences(got, false)
	if got.R2Mode != 1 || got.R2StartStrength != 6 || got.R2EndStrength != 6 || got.R2Amplitude != 0 || got.R2Hz != 0 {
		t.Fatalf("dynamic-off R2 = mode%d strength=%d/%d amp=%d hz=%d; want constant mode1 6/6", got.R2Mode, got.R2StartStrength, got.R2EndStrength, got.R2Amplitude, got.R2Hz)
	}
}

func TestSurfaceDefaultsPreserveCalibratedEngine(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	s := currentUserSettings()
	if math.Abs(surfaceMasterGain(s)-1.0) > 0.001 {
		t.Fatalf("surface master gain=%f want 1 at calibrated %g%%", surfaceMasterGain(s), surfaceMasterReferencePercent)
	}
	for _, profile := range knownSurfaceProfiles {
		if math.Abs(surfaceRollingGain(s, profile)-1.0) > 0.001 {
			t.Fatalf("%s rolling gain=%f want 1 at its calibrated peak reference", profile, surfaceRollingGain(s, profile))
		}
		if math.Abs(surfaceSlipGain(s, profile)-1.0) > 0.001 {
			t.Fatalf("%s slip gain=%f want 1 at calibrated relative contribution", profile, surfaceSlipGain(s, profile))
		}
	}
}

func TestSurfaceRollingAndSlipAreIndependent(t *testing.T) {
	profile := "gravel"
	roughness, speed := 1.0, 15.0
	rollingExcitation, legacyExcitation, slip := 0.10, 0.24, 1.0
	rollingRaw, slipRaw := splitSurfaceRawComponents(profile, roughness, speed, rollingExcitation, legacyExcitation, slip)
	legacyRolling, legacySlip := surfaceStrengthComponents(profile, roughness, speed, legacyExcitation, slip)
	if rollingRaw <= 0 || slipRaw <= 0 {
		t.Fatalf("expected independent gravel components, rolling=%f slip=%f", rollingRaw, slipRaw)
	}
	if math.Abs((rollingRaw+slipRaw)-(legacyRolling+legacySlip)) > 0.000001 {
		t.Fatalf("split changed calibrated raw total: split=%f legacy=%f", rollingRaw+slipRaw, legacyRolling+legacySlip)
	}

	// Changing the rolling-only excitation must not alter the dedicated slip input.
	rollingRaw2, slipRaw2 := splitSurfaceRawComponents(profile, roughness, speed, 0.20, legacyExcitation, slip)
	if math.Abs(rollingRaw2-rollingRaw) < 0.0001 {
		t.Fatal("rolling-only excitation did not change the rolling component")
	}
	if slipRaw2 >= slipRaw {
		// Increasing rolling can legitimately absorb part of the old slip-leakage
		// delta, but it must never create additional Slip power.
		t.Fatalf("rolling excitation unexpectedly increased slip component: %f -> %f", slipRaw, slipRaw2)
	}
}

func TestSurfaceSplitPreservesCalibratedAudibleTotal(t *testing.T) {
	profiles := []string{"asphalt", "asphalt_wet", "dirt", "gravel", "snow", "rock", "rumble_strip"}
	for _, profile := range profiles {
		rollingRaw, slipRaw := splitSurfaceRawComponents(profile, .55, 18, .12, .24, .8)
		rolling, slip, _ := splitCalibratedSurfaceStrength(profile, rollingRaw, slipRaw, .35, .60)
		legacyRolling, legacySlip := surfaceStrengthComponents(profile, .55, 18, .24, .8)
		legacyBase := tactileSurfaceStrength(profile, math.Min(.44, legacyRolling+legacySlip))
		legacyTotal := audibleSurfaceStrength(profile, legacyBase, .35, .60)
		if math.Abs((rolling+slip)-legacyTotal) > 0.000001 {
			t.Fatalf("%s split changed stock audible total: split=%f legacy=%f", profile, rolling+slip, legacyTotal)
		}
	}
}

func TestSurfaceAbsolutePowerControlsHaveRealHeadroom(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	s := currentUserSettings()
	profile := "asphalt"
	if got := surfaceRollingGain(s, profile); math.Abs(got-1) > .001 {
		t.Fatalf("default asphalt gain=%f want 1", got)
	}
	ref := referenceFor(profile, surfaceRollingReferencePercent, 100)
	if math.Abs(ref-10) > .001 {
		t.Fatalf("asphalt displayed calibrated power=%f want 10", ref)
	}
	s.Haptics.SurfaceRollingStrengths[profile] = 255
	maxGain := surfaceRollingGain(s, profile)
	if maxGain < 11.5 {
		t.Fatalf("asphalt 100%% gain=%f does not expose enough full-power headroom", maxGain)
	}

	rollingRaw, slipRaw := splitSurfaceRawComponents(profile, 1, 32, 1, 1, 0)
	rollingStock, _, rollingBase := splitCalibratedSurfaceStrength(profile, rollingRaw, slipRaw, 0, 0)
	if got := applyBoostOnlyAsphaltBed(profile, rollingStock, rollingBase, 1); got != 0 {
		t.Fatalf("stock smooth asphalt unexpectedly gained a continuous bed: %f", got)
	}
	boostedBed := applyBoostOnlyAsphaltBed(profile, rollingStock, rollingBase, maxGain)
	if boostedBed <= .10 {
		t.Fatalf("boosted smooth asphalt still has no useful rolling response: %f", boostedBed)
	}
}

func TestSlipAbsoluteControlHasLargeUpwardRange(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	s := currentUserSettings()
	for _, profile := range []string{"asphalt", "gravel", "snow"} {
		stock := surfaceSlipGain(s, profile)
		if math.Abs(stock-1) > .01 {
			t.Fatalf("%s stock slip gain=%f want 1", profile, stock)
		}
		s.Haptics.SurfaceSlipStrengths[profile] = 255
		maximum := surfaceSlipGain(s, profile)
		if maximum < 25 {
			t.Fatalf("%s 100%% slip gain=%f should expose obvious headroom", profile, maximum)
		}
	}
}

func TestSchema9AsphaltSliderReallyChangesRuntimeGain(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	ref := percentToStrength255(surfaceRollingReferencePercent["asphalt"])
	slipRef := percentToStrength255(surfaceSlipReferencePercent["asphalt"])
	packetRef := []byte(fmt.Sprintf(`{"v":40,"active":true,"userSettings":{"schema":9,"masterEnabled":true,"masterStrength":255,"surfaceEnabled":true,"surfaceStrength":255,"impactEnabled":true,"impactStrength":255,"surfaceRollingStrengths":{"asphalt":%d},"surfaceSlipStrengths":{"asphalt":%d},"l2BrakeEnabled":true,"l2BrakeStrength":128,"absEnabled":true,"absStrength":192,"r2ThrottleEnabled":true,"r2ThrottleStrength":32,"r2EffectsEnabled":true,"r2EffectsStrength":64,"lightingEnabled":true,"lightingBrightness":219}}`, ref, slipRef))
	if _, ok := decodeTelemetry(packetRef); !ok {
		t.Fatal("schema9 calibrated asphalt packet rejected")
	}
	calibrated := surfaceRollingGain(currentUserSettings(), "asphalt")
	packetMax := []byte(`{"v":40,"active":true,"userSettings":{"schema":9,"masterEnabled":true,"masterStrength":255,"surfaceEnabled":true,"surfaceStrength":255,"impactEnabled":true,"impactStrength":255,"surfaceRollingStrengths":{"asphalt":255},"surfaceSlipStrengths":{"asphalt":255},"l2BrakeEnabled":true,"l2BrakeStrength":128,"absEnabled":true,"absStrength":192,"r2ThrottleEnabled":true,"r2ThrottleStrength":32,"r2EffectsEnabled":true,"r2EffectsStrength":64,"lightingEnabled":true,"lightingBrightness":219}}`)
	if _, ok := decodeTelemetry(packetMax); !ok {
		t.Fatal("schema9 max asphalt packet rejected")
	}
	maximum := surfaceRollingGain(currentUserSettings(), "asphalt")
	if calibrated < .98 || calibrated > 1.02 || maximum < 11.5 {
		t.Fatalf("asphalt slider did not alter runtime gain: calibrated=%f max=%f", calibrated, maximum)
	}
}

func TestSchema8SurfaceSettingsMigrateWithoutChangingFeel(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	old := defaultUserSettings()
	old.Schema = 8
	old.Haptics.SurfaceStrength = 255
	old.Haptics.SurfaceRollingStrengths = map[string]int{"asphalt": percentToStrength255(29)}
	old.Haptics.SurfaceSlipStrengths = map[string]int{"asphalt": 255}
	migrated := migrateSchema8UserSettings(old)
	if got := surfaceMasterGain(migrated); math.Abs(got-1) > .01 {
		t.Fatalf("schema8 surface master migrated gain=%f want 1", got)
	}
	if got := surfaceRollingGain(migrated, "asphalt"); math.Abs(got-1) > .05 {
		t.Fatalf("schema8 asphalt migrated gain=%f want ~1", got)
	}
	if got := surfaceSlipGain(migrated, "asphalt"); math.Abs(got-1) > .20 {
		t.Fatalf("schema8 asphalt slip migrated gain=%f want ~1", got)
	}
}

func measureSurfacePowerForTest(profile string, material int, roughness float64, rollingPercent, slipPercent float64, slip bool) float64 {
	resetUserSettings()
	settings := currentUserSettings()
	settings.Haptics.SurfaceRollingStrengths[profile] = percentToStrength255(rollingPercent)
	settings.Haptics.SurfaceSlipStrengths[profile] = percentToStrength255(slipPercent)
	userSettingsState.mu.Lock()
	userSettingsState.data = settings
	userSettingsState.mu.Unlock()

	m := newCanonicalHapticMixer()
	base := time.Unix(64000, 0)
	legacyExcitation := 1.0
	rollingExcitation := 1.0
	roadSlip := 0.0
	if slip {
		rollingExcitation = .10
		legacyExcitation = .24
		roadSlip = 1
	}
	state := telemetry{Version: protocolVersion, Active: true, Raw: &rawTelemetry{
		Speed: 32, GroundedWheels: 4,
		SurfaceMaterialL: material, SurfaceMaterialR: material,
		SurfaceRoughnessL: roughness, SurfaceRoughnessR: roughness,
		RoadExcitationL: legacyExcitation, RoadExcitationR: legacyExcitation,
		RoadRollingExcitationValid: true,
		RoadRollingExcitationL:     rollingExcitation, RoadRollingExcitationR: rollingExcitation,
		RoadSlipL: roadSlip, RoadSlipR: roadSlip,
	}}
	m.update(state, base)
	ss, n := 0.0, 0
	for i := 0; i < 160; i++ {
		now := base.Add(time.Duration(i) * 10667 * time.Microsecond)
		m.update(state, now)
		_, status := m.render(512, now)
		for _, sample := range status.surfacePCM {
			v := float64(sample) / 127.0
			ss += v * v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return math.Sqrt(ss / float64(n))
}

func TestEverySurface100PercentHasFullPowerHeadroom(t *testing.T) {
	defer resetUserSettings()
	cases := []struct {
		profile   string
		material  int
		roughness float64
	}{
		{"asphalt", 10, .03}, {"asphalt_wet", 11, .06}, {"slippery", 12, .045}, {"ice", 21, .04},
		{"dirt", 15, .30}, {"dusty_dirt", 14, .34}, {"sandy_road", 17, .30}, {"sand", 16, .22},
		{"mud", 18, .28}, {"gravel", 19, .48}, {"grass", 20, .25}, {"snow", 22, .24},
		{"rock", 13, .58}, {"cobblestone", 30, .78}, {"rumble_strip", 29, 1.0},
	}
	for _, tc := range cases {
		rollingRMS := measureSurfacePowerForTest(tc.profile, tc.material, tc.roughness, 100, referenceFor(tc.profile, surfaceSlipReferencePercent, 1), false)
		// The calibration target is the ~0.625 active RMS of a strongest primary
		// bump. Scheduler windows vary by material, so use a small tolerance while
		// rejecting the old failure mode where 100% could still be very weak.
		if rollingRMS < .50 {
			t.Fatalf("%s 100%% rolling RMS=%f is still far below the full-power reference", tc.profile, rollingRMS)
		}
	}
}

func TestSlip100PercentIsAudibleAndIndependent(t *testing.T) {
	defer resetUserSettings()
	for _, tc := range []struct {
		profile   string
		material  int
		roughness float64
	}{{"asphalt", 10, .03}, {"gravel", 19, .48}, {"snow", 22, .24}} {
		refRolling := referenceFor(tc.profile, surfaceRollingReferencePercent, 100)
		refSlip := referenceFor(tc.profile, surfaceSlipReferencePercent, 1)
		stock := measureSurfacePowerForTest(tc.profile, tc.material, tc.roughness, refRolling, refSlip, true)
		maxSlip := measureSurfacePowerForTest(tc.profile, tc.material, tc.roughness, refRolling, 100, true)
		if maxSlip < stock+.12 {
			t.Fatalf("%s slip slider has too little audible range: stock=%f max=%f", tc.profile, stock, maxSlip)
		}
	}
}

func TestR2DynamicPercentControlsPeakNotBase(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	now := time.Now()
	cmd := telemetry{Active: true, R2Mode: 2, R2Amplitude: 2, R2Hz: 23, Raw: &rawTelemetry{EngineRunning: true, TCS: true, Throttle: 1}}

	settings := currentUserSettings()
	settings.AdaptiveTriggers.R2ThrottleStrength = 6
	settings.AdaptiveTriggers.R2EffectsStrength = percentToTriggerForce48(r2DynamicReferencePercent)
	userSettingsState.mu.Lock()
	userSettingsState.data = settings
	userSettingsState.mu.Unlock()
	got, _, _, _, _ := applyAdaptiveThrottleTrigger(cmd, now, &adaptiveThrottleTriggerState{}, false)
	got = applyUserTriggerPreferences(got, false)
	if got.R2Amplitude != 12 {
		t.Fatalf("default dynamic peak=%d want 12/48", got.R2Amplitude)
	}

	settings = currentUserSettings()
	settings.AdaptiveTriggers.R2EffectsStrength = 48
	userSettingsState.mu.Lock()
	userSettingsState.data = settings
	userSettingsState.mu.Unlock()
	got, _, _, _, _ = applyAdaptiveThrottleTrigger(cmd, now, &adaptiveThrottleTriggerState{}, false)
	got = applyUserTriggerPreferences(got, false)
	if got.R2Amplitude != 48 {
		t.Fatalf("100%% dynamic peak=%d want 48/48", got.R2Amplitude)
	}

	base := telemetry{Active: true, R2Mode: 1, Raw: &rawTelemetry{EngineRunning: true, Throttle: 0.5}}
	got, _, _, _, _ = applyAdaptiveThrottleTrigger(base, now, &adaptiveThrottleTriggerState{}, false)
	got = applyUserTriggerPreferences(got, false)
	if got.R2StartStrength != 6 || got.R2EndStrength != 6 {
		t.Fatalf("dynamic peak setting changed base resistance: %d/%d", got.R2StartStrength, got.R2EndStrength)
	}
}

func TestR2DynamicReportBytesChangeWithSlider(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	now := time.Now()
	cmd := telemetry{Active: true, R2Mode: 2, R2Amplitude: 2, R2Hz: 23, Raw: &rawTelemetry{EngineRunning: true, TCS: true}}

	setPeak := func(percent float64) []byte {
		s := currentUserSettings()
		s.AdaptiveTriggers.R2EffectsStrength = percentToTriggerForce48(percent)
		userSettingsState.mu.Lock()
		userSettingsState.data = s
		userSettingsState.mu.Unlock()
		got, _, _, _, _ := applyAdaptiveThrottleTrigger(cmd, now, &adaptiveThrottleTriggerState{}, false)
		got = applyUserTriggerPreferences(got, false)
		buf := make([]byte, 11)
		fillTrigger(buf, got.R2Mode, got.R2StartZone, got.R2StartStrength, got.R2EndStrength, got.R2Amplitude, got.R2Hz)
		return buf
	}

	lowBytes := setPeak(25)
	highBytes := setPeak(100)
	if bytes.Equal(lowBytes, highBytes) {
		t.Fatalf("R2 slider did not change final trigger report: %v", lowBytes)
	}
}

func TestR2DynamicStrengthDoesNotScaleFineFeedbackMode(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	settings := currentUserSettings()
	settings.AdaptiveTriggers.R2EffectsEnabled = true
	settings.AdaptiveTriggers.R2EffectsStrength = 48
	userSettingsState.mu.Lock()
	userSettingsState.data = settings
	userSettingsState.mu.Unlock()
	cmd := telemetry{Active: true, R2Mode: 3, R2StartZone: 26, R2StartStrength: 3, R2EndStrength: 3, Raw: &rawTelemetry{EngineRunning: true, Wheelspin: true, DrivenSlip: 4.0}}
	got := applyUserTriggerPreferences(cmd, false)
	if got.R2Mode != 3 || got.R2StartStrength != 3 || got.R2EndStrength != 3 {
		t.Fatalf("fine-feedback mode changed: mode=%d strength=%d/%d", got.R2Mode, got.R2StartStrength, got.R2EndStrength)
	}
}

func TestUserLEDSettingsCanDisableAndScale(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	userSettingsState.mu.Lock()
	s := userSettingsState.data
	s.Lighting.Enabled = false
	s.Lighting.Brightness = 255
	userSettingsState.data = s
	userSettingsState.mu.Unlock()
	if got := applyUserLEDSettings([3]byte{220, 110, 55}); got != [3]byte{} {
		t.Fatalf("disabled LED output=%v want off", got)
	}

	// 86% is the calibrated 220/255 lightbar ceiling and must preserve it.
	userSettingsState.mu.Lock()
	s = userSettingsState.data
	s.Lighting.Enabled = true
	s.Lighting.Brightness = percentToStrength255(ledReferencePercent)
	userSettingsState.data = s
	userSettingsState.mu.Unlock()
	if got := applyUserLEDSettings([3]byte{220, 110, 55}); got[0] < 219 || got[0] > 221 {
		t.Fatalf("calibrated LED reference changed output: %v", got)
	}

	// 100% exposes the remaining byte-range headroom.
	userSettingsState.mu.Lock()
	s = userSettingsState.data
	s.Lighting.Brightness = 255
	userSettingsState.data = s
	userSettingsState.mu.Unlock()
	got := applyUserLEDSettings([3]byte{220, 110, 55})
	if got[0] != 255 || got[1] <= 110 {
		t.Fatalf("100%% LED did not use headroom: %v", got)
	}
}

func TestSchema5SettingsMigrateToCalibratedMultipliers(t *testing.T) {
	resetUserSettings()
	defer resetUserSettings()
	packet := []byte(`{"v":40,"active":true,"userSettings":{"schema":6,"masterEnabled":true,"masterStrength":255,"surfaceEnabled":true,"surfaceStrength":255,"impactEnabled":true,"impactStrength":255,"surfaceRollingStrengths":{"asphalt":255,"gravel":128},"surfaceSlipStrengths":{"asphalt":255,"gravel":128},"l2BrakeEnabled":true,"l2BrakeStrength":128,"absEnabled":true,"absStrength":192,"r2ThrottleEnabled":true,"r2ThrottleStrength":32,"r2EffectsEnabled":true,"r2EffectsStrength":64,"lightingEnabled":true,"lightingBrightness":255}}`)
	if _, ok := decodeTelemetry(packet); !ok {
		t.Fatal("schema6 packet rejected")
	}
	got := currentUserSettings()
	if math.Abs(hapticMasterGain(true, got.Haptics.MasterStrength)-1.0) > 0.01 {
		t.Fatalf("schema6 master migration gain=%f want calibrated 1", hapticMasterGain(true, got.Haptics.MasterStrength))
	}
	if math.Abs(surfaceMasterGain(got)-1.0) > 0.01 {
		t.Fatalf("schema6 surface master migration gain=%f want calibrated 1", surfaceMasterGain(got))
	}
	if math.Abs(surfaceRollingGain(got, "asphalt")-1.0) > 0.03 {
		t.Fatalf("asphalt migration gain=%f want ~1", surfaceRollingGain(got, "asphalt"))
	}
	if surfaceRollingGain(got, "gravel") < 0.48 || surfaceRollingGain(got, "gravel") > 0.53 {
		t.Fatalf("gravel migration gain=%f want ~0.5", surfaceRollingGain(got, "gravel"))
	}
	if got.Lighting.Brightness != percentToStrength255(ledReferencePercent) {
		t.Fatalf("schema6 LED 100%% gain should migrate to calibrated %g%%, got %d", ledReferencePercent, got.Lighting.Brightness)
	}
}

func TestNormalizedTriggerPayloadOverridesLegacyMirror(t *testing.T) {
	packet := []byte(`{"v":40,"active":true,"l2Effect":{"kind":"resistance","startPosition":0.2,"startForce":0.17,"endForce":0.61,"amplitude":0,"frequencyHz":0},"l2Mode":1,"l2StartStrength":8,"l2EndStrength":8}`)
	got, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("normalized trigger packet was rejected")
	}
	pair := triggerPairFromTelemetry(got)
	if pair.L2.Kind != triggerResistance || math.Abs(pair.L2.StartPosition.Float64()-0.2) > 1e-9 || math.Abs(pair.L2.StartForce.Float64()-force48(0.17).Float64()) > 1e-9 || math.Abs(pair.L2.EndForce.Float64()-force48(0.61).Float64()) > 1e-9 {
		t.Fatalf("normalized payload was not authoritative: %+v", pair.L2)
	}
}

func TestFineFeedbackKeepsWeakNormalizedCommandAtHIDBoundary(t *testing.T) {
	buf := make([]byte, 11)
	fillTriggerEffect(buf, fineTrigger(0.37, 0.020833333333333332))
	if buf[0] != 0x01 {
		t.Fatalf("fine feedback mode=%02x want 01", buf[0])
	}
	if buf[2] != 1 {
		t.Fatalf("minimum fine feedback command encoded as %d want 1", buf[2])
	}
}
