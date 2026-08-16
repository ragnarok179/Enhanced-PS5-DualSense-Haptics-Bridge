package main

import (
	"math"
	"time"
)

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
