package main

import (
	"math"
	"time"
)

func surfaceStrengthComponents(profile string, roughness, speed, excitation, slip float64) (rolling, sliding float64) {
	speedNorm := clamp01((speed - 0.35) / 32.0)
	excitation = clamp01(excitation)
	slip = clamp01(slip)
	if profile == "none" || speed < feelProfile().Surface.LowSpeed.MinSpeedMS {
		return 0, 0
	}
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
		return 0, 0
	}
	roughGain := 0.76 + clamp01(roughness)*0.24
	rolling = (base + speedGain*speedNorm + excitationGain*excitation) * roughGain
	sliding = (slipGain * slip) * roughGain
	return rolling, sliding
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
