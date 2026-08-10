package main

import (
	"math"
	"time"
)

func continuousSurfaceStrength(profile string, roughness, speed, excitation, slip float64) float64 {
	speedNorm := clamp01((speed - 0.35) / 32.0)
	excitation = clamp01(excitation)
	slip = clamp01(slip)
	if profile == "none" || speed < feelProfile().Surface.LowSpeed.MinSpeedMS {
		return 0
	}
	// Keep amplitude secondary to temporal/spectral character. The ground model
	// defines whether a surface is deformable, fluid-like or hard; the bridge
	// maps that to a bounded compatibility-rumble envelope. Smooth asphalt sits
	// near the perceptual threshold, while hard discrete surfaces retain more
	// headroom than soft soils.
	base, speedGain, excitationGain, slipGain := 0.0, 0.0, 0.0, 0.0
	switch profile {
	case "asphalt":
		base, speedGain, excitationGain, slipGain = 0.0020, 0.0025, 0.024, 0.006
	case "asphalt_wet":
		base, speedGain, excitationGain, slipGain = 0.0060, 0.0060, 0.030, 0.016
	case "slippery":
		base, speedGain, excitationGain, slipGain = 0.0030, 0.0030, 0.020, 0.018
	case "ice":
		base, speedGain, excitationGain, slipGain = 0.0015, 0.0015, 0.015, 0.016
	case "sand":
		base, speedGain, excitationGain, slipGain = 0.090, 0.020, 0.035, 0.008
	case "mud":
		base, speedGain, excitationGain, slipGain = 0.110, 0.018, 0.045, 0.010
	case "dirt", "dusty_dirt":
		base, speedGain, excitationGain, slipGain = 0.105, 0.038, 0.050, 0.018
	case "sandy_road":
		base, speedGain, excitationGain, slipGain = 0.085, 0.030, 0.040, 0.012
	case "gravel":
		base, speedGain, excitationGain, slipGain = 0.130, 0.048, 0.060, 0.025
	case "grass":
		base, speedGain, excitationGain, slipGain = 0.065, 0.022, 0.040, 0.012
	case "snow":
		base, speedGain, excitationGain, slipGain = 0.050, 0.016, 0.030, 0.010
	case "rock":
		base, speedGain, excitationGain, slipGain = 0.145, 0.040, 0.065, 0.018
	case "rumble_strip":
		base, speedGain, excitationGain, slipGain = 0.205, 0.050, 0.055, 0.010
	case "cobblestone":
		base, speedGain, excitationGain, slipGain = 0.145, 0.045, 0.060, 0.015
	default:
		return 0
	}
	roughGain := 0.76 + clamp01(roughness)*0.24
	return math.Min(0.44, (base+speedGain*speedNorm+excitationGain*excitation+slipGain*slip)*roughGain)
}

func spatialHash(seed uint64, cell int64) float64 {
	x := uint64(cell) + seed + 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) / float64(uint64(1)<<53)
}

func surfaceProfileSeed(profile string, now time.Time) uint64 {
	h := uint64(1469598103934665603)
	for i := 0; i < len(profile); i++ {
		h ^= uint64(profile[i])
		h *= 1099511628211
	}
	return h ^ uint64(now.UnixNano())
}

func smooth01(x float64) float64 {
	x = clamp01(x)
	return x * x * (3 - 2*x)
}

func smoothNoise1D(x float64, seed uint64) float64 {
	i := int64(math.Floor(x))
	f := x - float64(i)
	a := spatialHash(seed, i)
	b := spatialHash(seed, i+1)
	u := smooth01(f)
	return a + (b-a)*u
}

func cellLocal(x, length float64) (int64, float64) {
	if length <= 0 {
		return 0, 0
	}
	q := x / length
	i := int64(math.Floor(q))
	return i, q - float64(i)
}

func softPulse(local, center, halfWidth float64) float64 {
	if halfWidth <= 0 {
		return 0
	}
	d := math.Abs(local - center)
	if d >= halfWidth {
		return 0
	}
	return smooth01(1 - d/halfWidth)
}

func impactPulse(local, center, attack, decay float64) float64 {
	if local < center {
		if attack <= 0 || local < center-attack {
			return 0
		}
		return smooth01((local - (center - attack)) / attack)
	}
	if decay <= 0 || local > center+decay {
		return 0
	}
	x := 1 - (local-center)/decay
	return x * x
}

func gateWindow(local, start, end, edge float64) float64 {
	if end <= start || local <= start || local >= end {
		return 0
	}
	if edge <= 0 {
		return 1
	}
	up := smooth01((local - start) / edge)
	down := smooth01((end - local) / edge)
	return math.Min(up, down)
}

func singleOwnerSurfaceGain(profile string) float64 {
	// Peak scaling only. Temporal density, gaps and attack/release behaviour are
	// produced by the stateful stochastic scene below. Soft/deformable media
	// deliberately stay far below hard discrete surfaces.
	switch profile {
	case "asphalt":
		return 0.025
	case "asphalt_wet":
		return 0.10
	case "slippery":
		return 0.07
	case "ice":
		return 0.045
	case "sand":
		return 0.32
	case "mud":
		return 0.38
	case "dirt":
		return 0.70
	case "dusty_dirt":
		return 0.64
	case "sandy_road":
		return 0.42
	case "gravel":
		return 0.82
	case "grass":
		return 0.42
	case "snow":
		return 0.30
	case "rock":
		return 1.00
	case "cobblestone":
		return 0.88
	case "rumble_strip":
		return 1.10
	default:
		return 0
	}
}

func lowSpeedSurfaceScale(speed float64) float64 {
	cfg := feelProfile().Surface.LowSpeed
	if speed >= cfg.FullSpeedMS {
		return 1.0
	}
	if speed <= cfg.MinSpeedMS {
		return cfg.MinAmplitudeScale
	}
	x := clamp01((speed - cfg.MinSpeedMS) / math.Max(0.001, cfg.FullSpeedMS-cfg.MinSpeedMS))
	return cfg.MinAmplitudeScale + (1-cfg.MinAmplitudeScale)*math.Pow(x, cfg.AmplitudeExponent)
}

func lowSpeedCadenceScales(speed float64) (cadenceScale, carrierScale float64) {
	cfg := feelProfile().Surface.LowSpeed
	full := math.Max(0.001, cfg.CadenceFullSpeedMS)
	p := math.Pow(clamp01(math.Max(0, speed)/full), cfg.AmplitudeExponent)
	cadenceScale = cfg.CadenceMinScale + cfg.CadenceSpan*p
	if speed < full {
		carrierScale = cfg.CarrierMinScale + cfg.CarrierSpan*p
	} else {
		carrierScale = math.Min(cfg.HighSpeedCarrierMax, cfg.HighSpeedCarrierBase+speed/math.Max(0.001, cfg.HighSpeedCarrierDivisor))
	}
	return cadenceScale, carrierScale
}

func surfaceCueCooldownAt(profile string, speed float64) time.Duration {
	cfg := feelProfile().Surface.SyntheticCooldownMS
	pair, ok := cfg[profile]
	if !ok {
		pair = cfg["default"]
	}
	x := smooth01(clamp01(math.Max(0, speed) / 6.0))
	ms := float64(pair[0]) + (float64(pair[1])-float64(pair[0]))*x
	return time.Duration(math.Round(ms)) * time.Millisecond
}

func surfaceSpeedCap(profile string) float64 {
	// Events are defined in metres, not in seconds, so temporal cadence rises
	// naturally with vehicle speed. The cap only prevents the two compatibility
	// bands from collapsing into an indistinguishable buzz at extreme speed.
	switch profile {
	case "sand", "mud":
		return 22
	case "grass", "snow", "sandy_road":
		return 25
	case "dirt", "dusty_dirt", "rock":
		return 30
	case "gravel", "cobblestone", "rumble_strip":
		return 35
	default:
		return 32
	}
}

type surfaceSegmentKind int

const (
	surfaceSegmentGap surfaceSegmentKind = iota
	surfaceSegmentSmooth
	surfaceSegmentImpact
	surfaceSegmentCluster
)

type surfacePatternState struct {
	profile  string
	lastTime time.Time
	rng      uint64
	serial   uint64

	segmentKind   surfaceSegmentKind
	segmentLength float64
	segmentPos    float64

	startLow, startHigh   float64
	targetLow, targetHigh float64
	peakLow, peakHigh     float64
	attackFrac            float64
	releaseFrac           float64

	pulseCount int
	pulsePos   [10]float64
	pulseWidth [10]float64
	pulseAmp   [10]float64

	currentLow, currentHigh float64
}

func (s *surfacePatternState) nextRand() float64 {
	if s.rng == 0 {
		s.rng = 0x9e3779b97f4a7c15
	}
	// xorshift64*: a practical period of 2^64-1, so there is no short scene loop.
	x := s.rng
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	s.rng = x
	return float64((x*2685821657736338717)>>11) / float64(uint64(1)<<53)
}

func (s *surfacePatternState) randRange(lo, hi float64) float64 {
	return lo + (hi-lo)*s.nextRand()
}

func (s *surfacePatternState) clearPulses() {
	s.pulseCount = 0
	for i := range s.pulsePos {
		s.pulsePos[i], s.pulseWidth[i], s.pulseAmp[i] = 0, 0, 0
	}
}

func (s *surfacePatternState) beginGap(length float64) {
	s.segmentKind = surfaceSegmentGap
	s.segmentLength = math.Max(0.02, length)
	s.segmentPos = 0
	s.startLow, s.startHigh = s.currentLow, s.currentHigh
	s.targetLow, s.targetHigh = 0, 0
	s.clearPulses()
}

func (s *surfacePatternState) beginSmooth(length, low, high float64) {
	s.segmentKind = surfaceSegmentSmooth
	s.segmentLength = math.Max(0.02, length)
	s.segmentPos = 0
	s.startLow, s.startHigh = s.currentLow, s.currentHigh
	s.targetLow, s.targetHigh = clamp01(low), clamp01(high)
	s.clearPulses()
}

func (s *surfacePatternState) beginImpact(length, low, high, attackFrac, releaseFrac float64) {
	s.segmentKind = surfaceSegmentImpact
	s.segmentLength = math.Max(0.02, length)
	s.segmentPos = 0
	s.peakLow, s.peakHigh = clamp01(low), clamp01(high)
	s.attackFrac = math.Max(0.02, math.Min(0.45, attackFrac))
	s.releaseFrac = math.Max(0.10, math.Min(0.90, releaseFrac))
	s.clearPulses()
}

func (s *surfacePatternState) beginCluster(length float64, count int, low, high float64) {
	s.segmentKind = surfaceSegmentCluster
	s.segmentLength = math.Max(0.02, length)
	s.segmentPos = 0
	s.peakLow, s.peakHigh = clamp01(low), clamp01(high)
	s.clearPulses()
	s.pulseCount = clampInt(count, 1, len(s.pulsePos))
	cursor := s.randRange(0.04, 0.12)
	for i := 0; i < s.pulseCount; i++ {
		remaining := float64(s.pulseCount - i)
		step := (0.92 - cursor) / math.Max(remaining, 1)
		s.pulsePos[i] = math.Min(0.94, cursor+s.randRange(0.05, 0.65)*step)
		s.pulseWidth[i] = s.randRange(0.025, 0.095)
		s.pulseAmp[i] = s.randRange(0.35, 1.0)
		cursor += step
	}
}

func (s *surfacePatternState) beginNext(profile string) {
	s.serial++
	r := s.nextRand()
	switch profile {
	case "asphalt":
		// Fine rolling texture is synthesized even on a geometrically flat road.
		// Short quiet windows and rare joints keep it subtle rather than constant.
		if r < 0.24 {
			s.beginGap(s.randRange(0.35, 1.35))
		} else if r < 0.90 {
			s.beginSmooth(s.randRange(0.55, 2.40), s.randRange(0.04, 0.12), s.randRange(0.34, 0.78))
		} else {
			s.beginImpact(s.randRange(0.18, 0.55), s.randRange(0.02, 0.08), s.randRange(0.45, 0.92), 0.12, 0.72)
		}
	case "asphalt_wet":
		s.beginSmooth(s.randRange(1.2, 4.8), s.randRange(0.005, 0.025), s.randRange(0.08, 0.24))
	case "slippery":
		if r < 0.78 {
			s.beginGap(s.randRange(1.5, 6.0))
		} else {
			s.beginImpact(s.randRange(0.20, 0.65), 0.01, s.randRange(0.12, 0.32), 0.12, 0.72)
		}
	case "ice":
		if r < 0.86 {
			s.beginGap(s.randRange(2.0, 8.0))
		} else {
			s.beginSmooth(s.randRange(0.5, 1.8), 0, s.randRange(0.04, 0.15))
		}
	case "sand":
		// No discrete grains: only very slow, rounded pressure changes.
		s.beginSmooth(s.randRange(1.8, 6.5), s.randRange(0.18, 0.62), s.randRange(0.0, 0.008))
	case "mud":
		// Viscous pushes and rests, both with rounded transitions.
		if r < 0.30 {
			s.beginSmooth(s.randRange(1.0, 3.8), s.randRange(0.0, 0.10), 0)
		} else {
			s.beginSmooth(s.randRange(1.8, 5.5), s.randRange(0.22, 0.72), s.randRange(0.0, 0.012))
		}
	case "sandy_road":
		if r < 0.18 {
			s.beginGap(s.randRange(0.6, 2.2))
		} else {
			s.beginSmooth(s.randRange(1.2, 4.2), s.randRange(0.16, 0.52), s.randRange(0.0, 0.035))
		}
	case "grass":
		if r < 0.48 {
			s.beginGap(s.randRange(0.45, 2.8))
		} else {
			s.beginImpact(s.randRange(0.35, 1.25), s.randRange(0.18, 0.58), s.randRange(0.015, 0.10), 0.32, 0.80)
		}
	case "snow":
		if r < 0.62 {
			s.beginGap(s.randRange(0.8, 3.8))
		} else {
			s.beginSmooth(s.randRange(0.7, 2.5), s.randRange(0.10, 0.38), s.randRange(0.0, 0.012))
		}
	case "dirt", "dusty_dirt":
		if r < 0.24 {
			s.beginGap(s.randRange(0.22, 1.35))
		} else {
			count := 2 + int(s.nextRand()*4)
			high := s.randRange(0.18, 0.55)
			if profile == "dusty_dirt" {
				high *= 1.18
			}
			s.beginCluster(s.randRange(0.55, 1.85), count, s.randRange(0.28, 0.78), high)
		}
	case "gravel":
		if r < 0.34 {
			s.beginGap(s.randRange(0.12, 0.80))
		} else {
			s.beginCluster(s.randRange(0.28, 1.05), 3+int(s.nextRand()*7), s.randRange(0.03, 0.30), s.randRange(0.38, 0.98))
		}
	case "rock":
		if r < 0.52 {
			s.beginGap(s.randRange(0.45, 2.60))
		} else {
			s.beginImpact(s.randRange(0.16, 0.58), s.randRange(0.45, 0.92), s.randRange(0.48, 1.0), 0.08, 0.82)
		}
	case "cobblestone":
		// Naturally quasi-regular, but every joint has random pitch, height and
		// occasional missing/settled stones, preventing a repeating loop.
		if r < 0.11 {
			s.beginGap(s.randRange(0.32, 0.90))
		} else {
			s.beginImpact(s.randRange(0.42, 0.78), s.randRange(0.32, 0.75), s.randRange(0.18, 0.62), 0.18, 0.72)
		}
	case "rumble_strip":
		if r < 0.08 {
			s.beginGap(s.randRange(0.25, 0.75))
		} else {
			s.beginImpact(s.randRange(0.32, 0.66), s.randRange(0.55, 0.92), s.randRange(0.68, 1.0), 0.10, 0.68)
		}
	default:
		s.beginGap(2.0)
	}
}

func (s *surfacePatternState) finishSegment() {
	switch s.segmentKind {
	case surfaceSegmentSmooth:
		s.currentLow, s.currentHigh = s.targetLow, s.targetHigh
	default:
		s.currentLow, s.currentHigh = 0, 0
	}
}

func (s *surfacePatternState) output() (low, high float64) {
	if s.segmentLength <= 0 {
		return 0, 0
	}
	p := clamp01(s.segmentPos / s.segmentLength)
	switch s.segmentKind {
	case surfaceSegmentGap:
		// Rounded release, then a true white interval.
		u := smooth01(math.Min(1, p/0.28))
		return s.startLow * (1 - u), s.startHigh * (1 - u)
	case surfaceSegmentSmooth:
		u := smooth01(p)
		return s.startLow + (s.targetLow-s.startLow)*u,
			s.startHigh + (s.targetHigh-s.startHigh)*u
	case surfaceSegmentImpact:
		env := 0.0
		if p < s.attackFrac {
			env = smooth01(p / s.attackFrac)
		} else {
			decayP := (p - s.attackFrac) / math.Max(0.01, 1-s.attackFrac)
			env = math.Pow(math.Max(0, 1-decayP), 1.2+s.releaseFrac*2.2)
		}
		return s.peakLow * env, s.peakHigh * env
	case surfaceSegmentCluster:
		lo, hi := 0.0, 0.0
		for i := 0; i < s.pulseCount; i++ {
			d := math.Abs(p - s.pulsePos[i])
			w := math.Max(0.01, s.pulseWidth[i])
			if d < w {
				x := 1 - d/w
				env := x * x * (3 - 2*x)
				amp := s.pulseAmp[i] * env
				lo += s.peakLow * amp
				hi += s.peakHigh * amp
			}
		}
		return math.Min(1, lo), math.Min(1, hi)
	default:
		return 0, 0
	}
}

func (s *surfacePatternState) reset(profile string, now time.Time) {
	if s == nil {
		return
	}
	*s = surfacePatternState{
		profile:  profile,
		lastTime: now,
		rng:      surfaceProfileSeed(profile, now),
	}
	s.beginNext(profile)
}

func continuousSurfaceScene(profile string, strength, speed float64, now time.Time, state *surfacePatternState) (low, high float64) {
	if state == nil {
		return 0, 0
	}
	if state.profile != profile || state.lastTime.IsZero() {
		state.reset(profile, now)
	}
	dt := now.Sub(state.lastTime).Seconds()
	if dt < 0 || dt > 0.20 {
		dt = 0
	}
	state.lastTime = now
	cadenceScale, _ := lowSpeedCadenceScales(speed)
	dx := math.Min(math.Max(speed, 0), surfaceSpeedCap(profile)) * cadenceScale * dt
	for dx > 0 {
		remaining := state.segmentLength - state.segmentPos
		if remaining <= 1e-6 {
			state.finishSegment()
			state.beginNext(profile)
			continue
		}
		if dx < remaining {
			state.segmentPos += dx
			dx = 0
		} else {
			state.segmentPos = state.segmentLength
			dx -= remaining
			state.finishSegment()
			state.beginNext(profile)
		}
	}
	nLow, nHigh := state.output()
	return strength * nLow, strength * nHigh
}

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

// applyABSHybridPulse restores the earlier hydraulic sensation: a very short
// 0/8 release lets the trigger mechanism move, followed by the shared USB-reference kick and
// a 1/8 holding force for the remainder of the cycle. Raw interventions are
// latched briefly so a one-frame ABS event still creates a complete sequence.
func applyABSHybridPulse(t telemetry, now time.Time, state *absPulseState) (telemetry, bool, string, int, int, int, float64) {
	if state == nil {
		return t, false, "off", 0, 0, 0, 0
	}
	rawActive := t.Raw != nil && (t.Raw.ABS || t.Raw.ABSRaw) && t.L2Mode == 2
	if rawActive {
		if state.cycleStart.IsZero() || now.After(state.holdUntil) {
			state.cycleStart = now
		}
		state.holdUntil = now.Add(92 * time.Millisecond)
		state.lastStrong = t.Raw.ABSSeverity >= 0.35 || t.Raw.ABSWheelCount >= 1 || t.L2Amplitude >= 4
		state.lastLock = t.Raw.LockHaptic || t.Raw.Lock || t.Raw.LockedWheelCount > 0
		state.lastControlHz = t.Raw.ABSControlHz
	}
	active := rawActive || (!state.holdUntil.IsZero() && now.Before(state.holdUntil))
	if !active {
		state.cycleStart = time.Time{}
		state.holdUntil = time.Time{}
		state.lastLock = false
		state.lastControlHz = 0
		return t, false, "off", 0, 0, 0, 0
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

	base := clampInt(feelProfile().Triggers.ABS.BaseStrength8, 1, 8)
	kick := clampInt(feelProfile().Triggers.ABS.KickStrength8, 1, 8)
	elapsed := now.Sub(state.cycleStart)
	phaseElapsed := elapsed % period
	releaseDuration := 5 * time.Millisecond
	kickDuration := 13 * time.Millisecond
	if elapsed < period {
		releaseDuration = 6 * time.Millisecond
		kickDuration = 16 * time.Millisecond
	}
	if phaseElapsed < releaseDuration {
		if state.lastLock {
			// A wheel-lock signal can overlap a genuine ABS intervention. In that
			// case the user explicitly wants no 0/8 hole: keep the 1/8 hydraulic
			// preload until the next kick.
			t.L2Mode = 1
			t.L2StartZone = 0
			t.L2StartStrength, t.L2EndStrength = base, base
			t.L2Amplitude, t.L2Hz = 0, 0
			return t, true, "lock_base_1_over_8", base, kick, kick, cadenceHz
		}
		t.L2Mode = 0
		t.L2StartZone = 0
		t.L2StartStrength, t.L2EndStrength = 0, 0
		t.L2Amplitude, t.L2Hz = 0, 0
		return t, true, "release_off", base, kick, kick, cadenceHz
	}
	if phaseElapsed < releaseDuration+kickDuration {
		t.L2Mode = 1
		t.L2StartZone = 0
		t.L2StartStrength, t.L2EndStrength = kick, kick
		t.L2Amplitude, t.L2Hz = 0, 0
		return t, true, "kick", base, kick, kick, cadenceHz
	}
	t.L2Mode = 1
	t.L2StartZone = 0
	t.L2StartStrength, t.L2EndStrength = base, base
	t.L2Amplitude, t.L2Hz = 0, 0
	return t, true, "base_1_over_8", base, kick, kick, cadenceHz
}

type adaptiveThrottleTriggerState struct {
	currentPosition float64
	currentStrength float64
	lastTime        time.Time
	lastLog         time.Time
}

// applyAdaptiveThrottleTrigger uses the legacy low-force byte only during genuine
// driven-wheel slip. The requested 1% corresponds to 3/255 at slip onset.
// Position and force are both smoothed; the effect never alternates Off/On.
func applyAdaptiveThrottleTrigger(t telemetry, now time.Time, state *adaptiveThrottleTriggerState, shiftTorqueCut bool) (telemetry, bool, float64, float64, float64) {
	if state == nil {
		return t, false, 0, 0, 0
	}
	if t.Raw == nil || !t.Active || !t.Raw.EngineRunning {
		state.currentPosition = 0
		state.currentStrength = 0
		state.lastTime = now
		t.R2Mode = 0
		t.R2StartZone, t.R2StartStrength, t.R2EndStrength = 0, 0, 0
		t.R2Amplitude, t.R2Hz = 0, 0
		return t, false, 0, 0, 0
	}

	// Airborne throttle: keep only the smallest fine-feedback command.
	// This overrides normal throttle resistance and all torque-related R2 effects
	// until at least one wheel is grounded again.
	if t.Raw.Airborne {
		state.currentPosition = 0
		state.currentStrength = 1
		state.lastTime = now
		t.R2Mode = 3
		t.R2StartZone = 0
		t.R2StartStrength = 1
		t.R2EndStrength = 1
		t.R2Amplitude, t.R2Hz = 0, 0
		return t, true, 0, 0, 0
	}

	// The validated shift cut is an exact 1/255 fine-feedback
	// command from position 0 for the whole cut window. Do not smooth this state:
	// smoothing was the reason earlier Bluetooth builds never reached the same
	// mechanical unload.
	if shiftTorqueCut {
		cfg := feelProfile().Triggers.R2
		state.currentPosition = float64(cfg.ShiftPosition255)
		state.currentStrength = float64(cfg.ShiftStrength255)
		state.lastTime = now
		t.R2Mode = 3
		t.R2StartZone = clampInt(cfg.ShiftPosition255, 0, 255)
		t.R2StartStrength = clampInt(cfg.ShiftStrength255, 1, 48)
		t.R2EndStrength = t.R2StartStrength
		t.R2Amplitude, t.R2Hz = 0, 0
		return t, true, 0, float64(cfg.ShiftPosition255), float64(cfg.ShiftPosition255)
	}

	// Genuine TCS and rev-limiter interventions keep their dedicated vibration.
	if t.Raw.TCS || t.Raw.TCSRaw || t.Raw.RevLimiter || t.R2Mode == 2 {
		state.lastTime = now
		return t, false, 0, state.currentPosition, state.currentPosition
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
			// 3/255 ≈ 1.18% when slip begins, falling to 1/255 at severe loss
			// of grip. The resistance onset also moves later for a clear unload.
			cfg := feelProfile().Triggers.R2
			targetStrength = cfg.WheelspinStartStrength255 + (cfg.WheelspinEndStrength255-cfg.WheelspinStartStrength255)*severity
			targetPosition = cfg.WheelspinStartPosition255 + (cfg.WheelspinEndPosition255-cfg.WheelspinStartPosition255)*severity
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
		t.R2Mode = 3
		t.R2StartZone = clampInt(int(math.Round(state.currentPosition)), 0, 180)
		t.R2StartStrength = clampInt(int(math.Round(state.currentStrength)), 1, 3)
		t.R2EndStrength = t.R2StartStrength
		t.R2Amplitude, t.R2Hz = 0, 0
		return t, true, severity, targetPosition, state.currentPosition
	}

	// No slip: keep a constant 1/8 accelerator resistance across the full pedal travel.
	state.currentPosition = 0
	state.currentStrength = 0
	t.R2Mode = 1
	t.R2StartZone = 0
	cfg := feelProfile().Triggers.R2
	t.R2StartStrength = clampInt(cfg.NormalStartStrength8, 1, 8)
	t.R2EndStrength = t.R2StartStrength
	t.R2Amplitude, t.R2Hz = 0, 0
	return t, false, 0, 0, 0
}

func audibleSurfaceStrength(profile string, base, low, high float64) float64 {
	if profile == "" || profile == "none" || base <= 0 {
		return 0
	}
	env := math.Max(low, high)
	if env <= 0.0015 {
		// Rough ground needs a low continuous material bed between the scheduler's
		// discrete asperities. Without it, a dense series of suspension events can
		// make terrain changes appear silent. Smooth asphalt intentionally keeps a
		// true zero bed so normal tarmac remains subtle.
		bed := 0.0
		switch profile {
		case "asphalt":
			bed = 0
		case "asphalt_wet":
			bed = 0.12
		case "slippery", "ice":
			bed = 0.08
		case "sand":
			bed = 0.32
		case "mud":
			bed = 0.38
		case "grass":
			bed = 0.18
		case "snow":
			bed = 0.14
		case "dirt", "dusty_dirt", "sandy_road":
			bed = 0.24
		case "gravel":
			bed = 0.30
		case "rock":
			bed = 0.26
		case "cobblestone":
			bed = 0.36
		case "rumble_strip":
			bed = 0.42
		}
		return math.Min(0.99, base*bed)
	}
	// Compress active asperities upward so they remain tactile without making
	// the material bed itself dominate the discrete bump layer.
	floor := 0.58
	switch profile {
	case "asphalt":
		floor = 0.62
	case "asphalt_wet", "slippery", "ice":
		floor = 0.60
	case "sand":
		floor = 0.82
	case "mud":
		floor = 0.76
	case "grass", "snow":
		floor = 0.66
	case "dirt", "dusty_dirt", "sandy_road":
		floor = 0.62
	case "gravel", "rock":
		floor = 0.58
	case "cobblestone":
		floor = 0.68
	case "rumble_strip":
		floor = 0.76
	}
	shaped := floor + (1-floor)*math.Sqrt(clamp01(env))
	return math.Min(0.99, base*shaped)
}

func surfaceCueCooldown(profile string) time.Duration {
	return surfaceCueCooldownAt(profile, 6.0)
}

func syntheticAsperityCue(profile string, state *surfacePatternState) (float64, int, bool) {
	if state == nil || profile == "" || profile == "none" {
		return 0, 0, false
	}
	peak := math.Max(state.peakLow, state.peakHigh)
	switch state.segmentKind {
	case surfaceSegmentImpact:
		switch profile {
		case "asphalt":
			return 0.25 + 0.15*peak, 42, true
		case "grass", "snow":
			return 0.30 + 0.18*peak, 58, true
		case "rock":
			return 0.60 + 0.22*peak, 82, true
		case "cobblestone":
			return 0.52 + 0.22*peak, 68, true
		case "rumble_strip":
			return 0.70 + 0.18*peak, 54, true
		default:
			return 0.38 + 0.22*peak, 60, true
		}
	case surfaceSegmentCluster:
		switch profile {
		case "gravel":
			return 0.45 + 0.24*peak, 64, true
		case "dirt", "dusty_dirt", "sandy_road":
			return 0.38 + 0.22*peak, 72, true
		default:
			return 0.36 + 0.20*peak, 64, true
		}
	default:
		return 0, 0, false
	}
}

func tactileBodyPeak(profile string, raw float64) float64 {
	if raw <= 0 {
		return 0
	}
	raw = clamp01(raw)
	switch profile {
	case "collision":
		return math.Min(0.99, 0.94+0.05*math.Sqrt(raw))
	case "landing":
		return math.Min(0.96, 0.72+0.24*math.Sqrt(raw))
	case "suspension_bump":
		// Vehicle Lua computes a physically meaningful severity before transport.
		// Preserve that dynamic range instead of remapping almost every bump to
		// 0.90..0.99. A 3 cm seam, a 7 cm bump and a 20 cm obstacle must feel
		// materially different. Only a very mild perceptual lift is applied.
		return math.Min(0.99, 0.04+0.96*math.Pow(raw, 0.90))
	case "suspension_secondary":
		return math.Min(0.88, 0.03+0.85*math.Pow(raw, 0.92))
	case "suspension_rebound":
		return math.Min(0.12, raw)
	default:
		return raw
	}
}

func tactileBeamNGBase(r *rawTelemetry) (low, high float64) {
	if r == nil || r.Airborne || r.GroundedWheels <= 0 || r.Speed < 0.35 {
		return 0, 0
	}
	raw := clamp01(r.NativeRumbleBaseForce)
	if raw < 0.0015 {
		return 0, 0
	}
	// BeamNG's generic jerk/slip signal was previously absent from the USB
	// path. A soft-knee expansion makes it tactile without saturating surfaces.
	level := math.Min(0.24, 0.035+0.46*math.Sqrt(raw))
	// Keep the generic BeamNG jerk/slip bed subject to the same low-speed
	// amplitude law as the per-wheel road surface. Without this, Bluetooth can
	// sound slower at walking pace while retaining an almost unchanged centered
	// vibration floor, masking the intended reduction in physical intensity.
	level *= lowSpeedSurfaceScale(r.Speed)
	return level * 0.68, level
}

func wheelspinRumbleAt(t telemetry, now time.Time) (low, high, severity float64) {
	if !t.Active || t.Raw == nil || !t.Raw.Wheelspin || t.Raw.TCS || t.Raw.TCSRaw || t.Raw.Airborne {
		return 0, 0, 0
	}
	severity = clamp01((t.Raw.DrivenSlip - 7.5) / (21.0 - 7.5))
	if severity <= 0 {
		return 0, 0, 0
	}
	// Tyre spin is a fast, irregular shudder rather than a continuous terrain
	// tone. Cadence and amplitude both rise with slip; every cycle receives a
	// deterministic jitter so it does not merge into the surface signature.
	cadence := 8.0 + 9.0*severity
	seconds := float64(now.UnixNano()) / float64(time.Second)
	cycle := math.Floor(seconds * cadence)
	phase := math.Mod(seconds*cadence, 1.0)
	seed := math.Mod(cycle*0.7548776662466927+0.318309886, 1.0)
	width := 0.38 + 0.22*seed
	if phase > width {
		return 0, 0, severity
	}
	x := phase / math.Max(width, 0.01)
	envelope := math.Sin(math.Pi * x)
	flutter := 0.62 + 0.38*math.Sin((2.0+math.Floor(seed*3.0))*math.Pi*x)
	low = (0.025 + 0.085*severity) * envelope * flutter
	high = (0.035 + 0.145*severity) * envelope * (0.72 + 0.28*seed)
	return low, high, severity
}

func hsvToRGB(h, s, v float64) (byte, byte, byte) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	r1, g1, b1 := 0.0, 0.0, 0.0
	switch {
	case h < 60:
		r1, g1 = c, x
	case h < 120:
		r1, g1 = x, c
	case h < 180:
		g1, b1 = c, x
	case h < 240:
		g1, b1 = x, c
	case h < 300:
		r1, b1 = x, c
	default:
		r1, b1 = c, x
	}
	roundByte := func(x float64) byte { return byte(clampInt(int(math.Floor(x*255+0.5)), 0, 255)) }
	return roundByte(r1 + m), roundByte(g1 + m), roundByte(b1 + m)
}

func rpmRGB(t telemetry, now time.Time) ([3]byte, bool) {
	var black [3]byte
	if !t.Active || t.Raw == nil || !t.Raw.EngineRunning || t.Raw.MaxRPM <= 1 {
		return black, false
	}
	cfg := feelProfile().LED
	ratio := clamp01(t.Raw.RPM / math.Max(t.Raw.MaxRPM, 1))
	if ratio < cfg.FirstRatio {
		return black, false
	}
	blink := t.Raw.RevLimiter && ratio >= cfg.BlinkMinRatio
	if !cfg.BlinkOnlyOnRevLimiter && ratio >= 0.985 {
		blink = true
	}
	if blink {
		hz := math.Max(1, cfg.BlinkHz)
		on := (int(math.Floor(float64(now.UnixNano())/1e9*hz*2.0)) % 2) == 0
		if !on {
			return black, true
		}
		return [3]byte{byte(clampInt(cfg.MaxBrightness, 0, 255)), 0, 0}, true
	}
	if ratio >= cfg.RedRatio {
		return [3]byte{byte(clampInt(cfg.MaxBrightness, 0, 255)), 0, 0}, false
	}
	x := clamp01((ratio - cfg.FirstRatio) / math.Max(0.001, cfg.RedRatio-cfg.FirstRatio))
	hue := 120 * (1 - x)
	brightness := float64(cfg.MinBrightness) + float64(cfg.MaxBrightness-cfg.MinBrightness)*clamp01(x*1.8)
	r, g, b := hsvToRGB(hue, 1.0, brightness/255.0)
	return [3]byte{r, g, b}, false
}

func surfaceDrive(profile string) float64 {
	switch profile {
	case "asphalt", "asphalt_wet", "slippery", "ice":
		return 1.20
	case "sand", "mud":
		return 1.35
	case "gravel", "rock", "cobblestone", "rumble_strip":
		return 1.55
	default:
		return 1.42
	}
}

func shiftCue(t telemetry) (level float64, durationMS int) {
	cfg := feelProfile().ShiftHaptic
	level = clamp(t.ShiftStrength, cfg.MinStrength, cfg.MaxStrength)
	if t.ShiftStrength <= 0 {
		level = cfg.MinStrength
	}
	durationMS = clampInt(t.ShiftDurationMS, cfg.MinDurationMS, cfg.MaxDurationMS)
	return
}

func bodyStereo(t telemetry) (profile string, left, right float64, durationMS int) {
	kind := t.BodyKind
	if kind == "" {
		kind = "wheel"
	}
	profile = t.BodyProfile
	if profile == "" || profile == "none" {
		switch kind {
		case "collision":
			profile = "collision"
		case "landing":
			profile = "landing"
		case "surface":
			profile = "dirt"
		default:
			profile = "suspension_bump"
		}
	}

	strengthMax, strengthMin := 0.48, 0.10
	durationMin, durationMax := 68, 128
	switch profile {
	case "suspension_bump":
		// Dynamic primary bump authored by Vehicle Lua. Do not flatten it.
		strengthMax, strengthMin = 0.99, 0.08
		durationMin, durationMax = 48, 116
	case "suspension_secondary":
		// Other axle crossing the same obstacle. It is real, but less dominant.
		strengthMax, strengthMin = 0.88, 0.06
		durationMin, durationMax = 40, 96
	case "suspension_rebound":
		// Consequence of an already-rendered impact: deliberately subtle.
		strengthMax, strengthMin = 0.12, 0.02
		durationMin, durationMax = 24, 42
	}
	switch kind {
	case "collision":
		strengthMax, strengthMin = 0.99, 0.72
		durationMin, durationMax = 150, 280
	case "landing":
		strengthMax, strengthMin = 0.82, 0.38
		durationMin, durationMax = 110, 200
	case "surface":
		strengthMax, strengthMin = 0.58, 0.18
		durationMin, durationMax = 58, 120
	}

	left = math.Max(0, math.Min(strengthMax, t.BodyLeftStrength))
	right = math.Max(0, math.Min(strengthMax, t.BodyRightStrength))
	peak := math.Max(left, right)
	if peak <= 0 {
		peak = math.Max(strengthMin, math.Min(strengthMax, t.BodyStrength))
		if t.BodySide < 0 {
			left = peak
		} else if t.BodySide > 0 {
			right = peak
		} else {
			left, right = peak, peak
		}
	} else if peak < strengthMin {
		scale := strengthMin / peak
		left = math.Min(strengthMax, left*scale)
		right = math.Min(strengthMax, right*scale)
	}

	// BodySide is authoritative after Vehicle Lua's multi-sensor correlation.
	if t.BodySide < 0 {
		dominant := math.Max(left, right)
		left, right = dominant, 0
	} else if t.BodySide > 0 {
		dominant := math.Max(left, right)
		left, right = 0, dominant
	} else if kind == "collision" {
		center := math.Max(left, right)
		left, right = center, center
	}

	peak = math.Max(left, right)
	if peak > 0 {
		tp := tactileBodyPeak(profile, peak)
		if isSuspensionBumpProfile(profile) && t.BodySide == 0 && left > 0 && right > 0 {
			// Equal total energy for a true same-axle centered impact/rebound.
			const equalEnergy = 0.7071067811865476
			left, right = tp*equalEnergy, tp*equalEnergy
		} else {
			scale := tp / peak
			left = math.Min(0.99, left*scale)
			right = math.Min(0.99, right*scale)
		}
	}
	durationMS = clampInt(t.BodyDurationMS, durationMin, durationMax)
	return
}

func mixBand(a, b float64) float64 {
	a, b = clamp01(a), clamp01(b)
	return 1 - (1-a)*(1-b)
}
